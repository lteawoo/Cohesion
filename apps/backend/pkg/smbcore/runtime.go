package smbcore

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	pathpkg "path"
	"strings"
	"sync/atomic"
	"unicode/utf16"

	"github.com/vadimi/go-ntlm/ntlm"
)

var (
	ErrRuntimeNotImplemented      = errors.New("smbcore runtime not implemented")
	ErrRuntimeDependenciesMissing = errors.New("smbcore runtime dependencies missing")
	ErrInvalidDialectBounds       = errors.New("invalid smb dialect bounds")
)

const (
	smb2HeaderSize = 64

	statusSuccess            uint32 = 0x00000000
	statusInvalidParameter   uint32 = 0xC000000D
	statusAccessDenied       uint32 = 0xC0000022
	statusObjectNameNotFound uint32 = 0xC0000034
	statusLogonFailure       uint32 = 0xC000006D
	statusMoreProcessing     uint32 = 0xC0000016
	statusNotSupported       uint32 = 0xC00000BB
	statusBadNetworkName     uint32 = 0xC00000CC
	statusNotDirectory       uint32 = 0xC0000103
	statusFileClosed         uint32 = 0xC0000128

	cmdNegotiate      uint16 = 0x0000
	cmdSessionSetup   uint16 = 0x0001
	cmdLogoff         uint16 = 0x0002
	cmdTreeConnect    uint16 = 0x0003
	cmdTreeDisconnect uint16 = 0x0004
	cmdCreate         uint16 = 0x0005
	cmdClose          uint16 = 0x0006
	cmdRead           uint16 = 0x0008
	cmdWrite          uint16 = 0x0009
	cmdQueryDirectory uint16 = 0x000E
	cmdSetInfo        uint16 = 0x0011

	smb2FlagServerToClient uint32 = 0x00000001
	createOptionDirectory   uint32 = 0x00000001

	setInfoTypeFile uint8 = 0x01

	fileInfoClassRename      uint8 = 0x0A
	fileInfoClassDisposition uint8 = 0x0D

	fileDirectoryInformationClass uint8 = 0x01
	fileIdBothDirInfoClass        uint8 = 0x25

	ntlmMessageTypeNegotiate    uint32 = 1
	ntlmMessageTypeAuthenticate uint32 = 3
)

type Config struct {
	MinDialect   Dialect
	MaxDialect   Dialect
	RolloutPhase RolloutPhase
}

type Engine struct {
	cfg       Config
	auth      Authenticator
	authz     Authorizer
	fs        FileSystem
	telemetry Telemetry
	seq       uint64
}

type requestHeader struct {
	Command   uint16
	MessageID uint64
	TreeID    uint32
	SessionID uint64
}

type treeState struct {
	SpaceName string
}

type fileHandle struct {
	VirtualPath string
	IsDir       bool
}

type connState struct {
	negotiated bool
	dialect    Dialect

	authenticated bool
	principal     string
	sessionID     uint64
	ntlmSession   ntlm.ServerSession
	ntlmSpnego    bool

	trees      map[uint32]treeState
	handles    map[uint64]fileHandle
	nextTreeID uint32
	nextFileID uint64
}

func NewEngine(cfg Config, auth Authenticator, authz Authorizer, fs FileSystem, telemetry Telemetry) (*Engine, error) {
	if cfg.RolloutPhase == "" {
		cfg.RolloutPhase = RolloutPhaseReadOnly
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &Engine{
		cfg:       cfg,
		auth:      auth,
		authz:     authz,
		fs:        fs,
		telemetry: telemetry,
	}, nil
}

func (e *Engine) IsReadOnly() bool {
	return e.cfg.RolloutPhase == RolloutPhaseReadOnly
}

func (e *Engine) Phase() RolloutPhase {
	return e.cfg.RolloutPhase
}

func (e *Engine) Supports(dialect Dialect) bool {
	ord := map[Dialect]int{
		Dialect210: 1,
		Dialect300: 2,
		Dialect302: 3,
		Dialect311: 4,
	}
	val, ok := ord[dialect]
	if !ok {
		return false
	}
	return val >= ord[e.cfg.MinDialect] && val <= ord[e.cfg.MaxDialect]
}

func (e *Engine) CheckUsability(ctx context.Context) error {
	_ = ctx

	if e.auth == nil || e.authz == nil || e.fs == nil {
		return ErrRuntimeDependenciesMissing
	}
	if !e.Supports(e.cfg.MinDialect) || !e.Supports(e.cfg.MaxDialect) {
		return errors.New("configured dialect bounds are not usable")
	}

	probe := []DirEntry{{Name: "probe.txt", IsDir: false, Size: 1}}
	for _, infoClass := range []uint8{fileDirectoryInformationClass, fileIdBothDirInfoClass} {
		encoded := encodeDirectoryEntries(probe, 4096, infoClass)
		if len(encoded) == 0 {
			return fmt.Errorf("directory payload self-check failed for info class 0x%02x", infoClass)
		}
	}

	return nil
}

func (e *Engine) HandleConn(ctx context.Context, conn net.Conn) error {
	if conn == nil {
		return nil
	}
	defer conn.Close()

	if e.auth == nil || e.authz == nil || e.fs == nil {
		if e.telemetry != nil {
			e.telemetry.OnEvent(Event{
				Stage:  "session",
				Reason: "runtime_not_ready",
				Err:    ErrRuntimeDependenciesMissing,
			})
		}
		return ErrRuntimeDependenciesMissing
	}

	state := &connState{
		trees:      make(map[uint32]treeState),
		handles:    make(map[uint64]fileHandle),
		nextTreeID: 1,
		nextFileID: 1,
	}

	for {
		packet, err := readNetBIOSPacket(conn)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		if len(packet) == 0 {
			continue
		}

		if isSMB1Packet(packet) {
			if e.telemetry != nil {
				e.telemetry.OnEvent(Event{
					Stage:  "negotiate",
					Reason: "smb1_rejected",
					Err:    nil,
				})
			}
			return nil
		}

		header, body, err := parseRequestPacket(packet)
		if err != nil {
			if writeErr := writeNetBIOSPacket(conn, buildErrorResponse(header, statusInvalidParameter)); writeErr != nil {
				return writeErr
			}
			continue
		}

		resp, closeConn := e.dispatch(ctx, state, header, body)
		if writeErr := writeNetBIOSPacket(conn, resp); writeErr != nil {
			return writeErr
		}
		if closeConn {
			return nil
		}
	}
}

func (e *Engine) dispatch(ctx context.Context, state *connState, header requestHeader, body []byte) ([]byte, bool) {
	switch header.Command {
	case cmdNegotiate:
		return e.handleNegotiate(state, header, body), false
	case cmdSessionSetup:
		return e.handleSessionSetup(ctx, state, header, body), false
	case cmdTreeConnect:
		return e.handleTreeConnect(ctx, state, header, body), false
	case cmdTreeDisconnect:
		return e.handleTreeDisconnect(state, header), false
	case cmdCreate:
		return e.handleCreate(ctx, state, header, body), false
	case cmdClose:
		return e.handleClose(state, header, body), false
	case cmdQueryDirectory:
		return e.handleQueryDirectory(ctx, state, header, body), false
	case cmdRead:
		return e.handleRead(ctx, state, header, body), false
	case cmdWrite:
		return e.handleWrite(ctx, state, header, body), false
	case cmdSetInfo:
		return e.handleSetInfo(ctx, state, header, body), false
	case cmdLogoff:
		return e.handleLogoff(state, header), true
	default:
		return buildErrorResponse(header, statusNotSupported), false
	}
}

func (e *Engine) handleNegotiate(state *connState, header requestHeader, body []byte) []byte {
	dialects, err := parseNegotiateDialects(body)
	if err != nil {
		return buildErrorResponse(header, statusInvalidParameter)
	}

	selected, ok := e.selectDialect(dialects)
	if !ok {
		return buildErrorResponse(header, statusNotSupported)
	}

	state.negotiated = true
	state.dialect = selected
	return buildResponse(header, 0, statusSuccess, buildNegotiateBody(selected))
}

func (e *Engine) handleSessionSetup(ctx context.Context, state *connState, header requestHeader, body []byte) []byte {
	if !state.negotiated {
		return buildErrorResponse(header, statusInvalidParameter)
	}

	packet := append(buildHeaderStub(header), body...)
	token, err := parseSessionSetupToken(packet)
	if err != nil {
		return buildErrorResponse(header, statusLogonFailure)
	}

	if state.sessionID == 0 {
		state.sessionID = atomic.AddUint64(&e.seq, 1)
	}

	switch token.MessageType {
	case ntlmMessageTypeNegotiate:
		serverSession, err := ntlm.CreateServerSession(ntlm.Version2, ntlm.ConnectionlessMode)
		if err != nil {
			return buildErrorResponse(header, statusLogonFailure)
		}
		if err := serverSession.ProcessNegotiateMessage(&ntlm.NegotiateMessage{Bytes: token.NTLMToken}); err != nil {
			return buildErrorResponse(header, statusLogonFailure)
		}

		challenge, err := serverSession.GenerateChallengeMessage()
		if err != nil {
			return buildErrorResponse(header, statusLogonFailure)
		}

		challengeToken := challenge.Bytes()
		if token.SpnegoWrapped {
			challengeToken = buildSPNEGONegTokenResp(challengeToken, false)
		}

		state.ntlmSession = serverSession
		state.ntlmSpnego = token.SpnegoWrapped
		return buildResponse(header, state.sessionID, statusMoreProcessing, buildSessionSetupBody(challengeToken))
	case ntlmMessageTypeAuthenticate:
		if state.ntlmSession == nil {
			return buildErrorResponse(header, statusLogonFailure)
		}

		authMsg, err := ntlm.ParseAuthenticateMessage(token.NTLMToken, int(ntlm.Version2))
		if err != nil {
			state.ntlmSession = nil
			state.ntlmSpnego = false
			return buildErrorResponse(header, statusLogonFailure)
		}

		username := strings.TrimSpace(authMsg.UserName.String())
		domain := strings.TrimSpace(authMsg.DomainName.String())
		if username == "" {
			state.ntlmSession = nil
			state.ntlmSpnego = false
			return buildErrorResponse(header, statusLogonFailure)
		}

		password, err := e.auth.ResolveSMBPassword(ctx, username)
		if err != nil {
			state.ntlmSession = nil
			state.ntlmSpnego = false
			return buildErrorResponse(header, statusLogonFailure)
		}

		state.ntlmSession.SetUserInfo(username, password, domain)
		if err := state.ntlmSession.ProcessAuthenticateMessage(authMsg); err != nil {
			state.ntlmSession = nil
			state.ntlmSpnego = false
			return buildErrorResponse(header, statusLogonFailure)
		}

		principal, err := e.auth.Authenticate(ctx, username, password)
		if err != nil || strings.TrimSpace(principal) == "" {
			state.ntlmSession = nil
			state.ntlmSpnego = false
			return buildErrorResponse(header, statusLogonFailure)
		}

		state.authenticated = true
		state.principal = principal
		state.ntlmSession = nil

		var finalToken []byte
		if state.ntlmSpnego {
			finalToken = buildSPNEGONegTokenResp(nil, true)
		}
		state.ntlmSpnego = false
		return buildResponse(header, state.sessionID, statusSuccess, buildSessionSetupBody(finalToken))
	default:
		return buildErrorResponse(header, statusLogonFailure)
	}
}

func (e *Engine) handleTreeConnect(ctx context.Context, state *connState, header requestHeader, body []byte) []byte {
	if !state.authenticated || state.sessionID == 0 {
		return buildResponse(header, 0, statusAccessDenied, nil)
	}

	packet := append(buildHeaderStub(header), body...)
	sharePath, err := parseTreeConnectPath(packet)
	if err != nil {
		return buildResponse(header, state.sessionID, statusInvalidParameter, nil)
	}
	spaceName, err := extractSpaceNameFromUNC(sharePath)
	if err != nil {
		return buildResponse(header, state.sessionID, statusBadNetworkName, nil)
	}

	allowed, err := e.authz.CanAccessSpace(ctx, state.principal, spaceName, PermissionRead)
	if err != nil || !allowed {
		return e.buildDeniedResponse(header, state.sessionID, header.TreeID, DenyReasonPermissionDenied)
	}

	treeID := state.nextTreeID
	state.nextTreeID++
	state.trees[treeID] = treeState{SpaceName: spaceName}

	return buildResponseWithTree(header, state.sessionID, treeID, statusSuccess, buildTreeConnectBody())
}

func (e *Engine) handleTreeDisconnect(state *connState, header requestHeader) []byte {
	if state.sessionID == 0 || header.TreeID == 0 {
		return buildResponse(header, state.sessionID, statusInvalidParameter, nil)
	}
	delete(state.trees, header.TreeID)
	return buildResponse(header, state.sessionID, statusSuccess, buildTreeDisconnectBody())
}

func (e *Engine) handleCreate(ctx context.Context, state *connState, header requestHeader, body []byte) []byte {
	tree, ok := state.trees[header.TreeID]
	if !ok {
		return buildResponse(header, state.sessionID, statusBadNetworkName, nil)
	}

	packet := append(buildHeaderStub(header), body...)
	name, disposition, createOptions, err := parseCreateRequest(packet)
	if err != nil {
		return buildResponse(header, state.sessionID, statusInvalidParameter, nil)
	}
	if disposition != 1 && disposition != 2 && disposition != 3 {
		return buildResponse(header, state.sessionID, statusInvalidParameter, nil)
	}

	virtualPath := joinTreePath(tree.SpaceName, name)
	var entry DirEntry
	isDirectoryCreate := (createOptions & createOptionDirectory) != 0

	switch disposition {
	case 1:
		entry, err = e.fs.Stat(ctx, state.principal, virtualPath)
	case 2:
		if isDirectoryCreate {
			if !e.phaseAllowsManage() {
				return e.buildPhaseDeniedResponse(header, state.sessionID, header.TreeID)
			}
			allowed, authErr := e.ensureSpacePermission(ctx, state, tree.SpaceName, PermissionManage)
			if authErr != nil || !allowed {
				return e.buildDeniedResponse(header, state.sessionID, header.TreeID, DenyReasonPermissionDenied)
			}
			entry, err = e.fs.Mkdir(ctx, state.principal, virtualPath)
		} else {
			if !e.phaseAllowsWrite() {
				return e.buildPhaseDeniedResponse(header, state.sessionID, header.TreeID)
			}
			allowed, authErr := e.ensureSpacePermission(ctx, state, tree.SpaceName, PermissionWrite)
			if authErr != nil || !allowed {
				return e.buildDeniedResponse(header, state.sessionID, header.TreeID, DenyReasonPermissionDenied)
			}
			entry, err = e.fs.CreateOrTruncate(ctx, state.principal, virtualPath)
		}
	case 3:
		if isDirectoryCreate {
			if !e.phaseAllowsManage() {
				return e.buildPhaseDeniedResponse(header, state.sessionID, header.TreeID)
			}
			allowed, authErr := e.ensureSpacePermission(ctx, state, tree.SpaceName, PermissionManage)
			if authErr != nil || !allowed {
				return e.buildDeniedResponse(header, state.sessionID, header.TreeID, DenyReasonPermissionDenied)
			}
			entry, err = e.fs.Stat(ctx, state.principal, virtualPath)
			if errors.Is(err, os.ErrNotExist) {
				entry, err = e.fs.Mkdir(ctx, state.principal, virtualPath)
			}
		} else {
			if !e.phaseAllowsWrite() {
				return e.buildPhaseDeniedResponse(header, state.sessionID, header.TreeID)
			}
			allowed, authErr := e.ensureSpacePermission(ctx, state, tree.SpaceName, PermissionWrite)
			if authErr != nil || !allowed {
				return e.buildDeniedResponse(header, state.sessionID, header.TreeID, DenyReasonPermissionDenied)
			}
			entry, err = e.fs.Stat(ctx, state.principal, virtualPath)
			if errors.Is(err, os.ErrNotExist) {
				entry, err = e.fs.CreateOrTruncate(ctx, state.principal, virtualPath)
			}
		}
	}

	if err != nil {
		if errors.Is(err, os.ErrPermission) || errors.Is(err, ErrPermissionDenied) || errors.Is(err, ErrPathBoundary) {
			return e.buildDeniedResponse(header, state.sessionID, header.TreeID, denyReasonFromError(err))
		}
		if errors.Is(err, os.ErrNotExist) {
			return buildResponse(header, state.sessionID, statusObjectNameNotFound, nil)
		}
		return buildResponse(header, state.sessionID, statusInvalidParameter, nil)
	}

	fileID := state.nextFileID
	state.nextFileID++
	state.handles[fileID] = fileHandle{
		VirtualPath: virtualPath,
		IsDir:       entry.IsDir,
	}

	return buildResponseWithTree(header, state.sessionID, header.TreeID, statusSuccess, buildCreateBody(fileID, entry))
}

func (e *Engine) handleClose(state *connState, header requestHeader, body []byte) []byte {
	packet := append(buildHeaderStub(header), body...)
	fileID, err := parseCloseRequest(packet)
	if err != nil {
		return buildResponse(header, state.sessionID, statusInvalidParameter, nil)
	}
	handle, ok := state.handles[fileID]
	if !ok {
		return buildResponse(header, state.sessionID, statusFileClosed, nil)
	}
	delete(state.handles, fileID)
	return buildResponseWithTree(header, state.sessionID, header.TreeID, statusSuccess, buildCloseBody(fileID, handle.IsDir))
}

func (e *Engine) handleQueryDirectory(ctx context.Context, state *connState, header requestHeader, body []byte) []byte {
	packet := append(buildHeaderStub(header), body...)
	fileID, limit, infoClass, err := parseQueryDirectoryRequest(packet)
	if err != nil {
		return buildResponse(header, state.sessionID, statusInvalidParameter, nil)
	}
	if !isSupportedDirectoryInfoClass(infoClass) {
		return buildResponse(header, state.sessionID, statusNotSupported, nil)
	}

	handle, ok := state.handles[fileID]
	if !ok {
		return buildResponse(header, state.sessionID, statusFileClosed, nil)
	}
	if !handle.IsDir {
		return buildResponse(header, state.sessionID, statusNotDirectory, nil)
	}

	entries, err := e.fs.List(ctx, state.principal, handle.VirtualPath)
	if err != nil {
		if errors.Is(err, os.ErrPermission) || errors.Is(err, ErrPermissionDenied) || errors.Is(err, ErrPathBoundary) {
			return e.buildDeniedResponse(header, state.sessionID, header.TreeID, denyReasonFromError(err))
		}
		return buildResponse(header, state.sessionID, statusInvalidParameter, nil)
	}

	data := encodeDirectoryEntries(entries, limit, infoClass)
	return buildResponseWithTree(header, state.sessionID, header.TreeID, statusSuccess, buildQueryDirectoryBody(data))
}

func (e *Engine) handleRead(ctx context.Context, state *connState, header requestHeader, body []byte) []byte {
	packet := append(buildHeaderStub(header), body...)
	fileID, offset, length, err := parseReadRequest(packet)
	if err != nil {
		return buildResponse(header, state.sessionID, statusInvalidParameter, nil)
	}

	handle, ok := state.handles[fileID]
	if !ok {
		return buildResponse(header, state.sessionID, statusFileClosed, nil)
	}
	if handle.IsDir {
		return e.buildDeniedResponse(header, state.sessionID, header.TreeID, DenyReasonPermissionDenied)
	}
	if offset > uint64(^uint64(0)>>1) {
		return buildResponse(header, state.sessionID, statusInvalidParameter, nil)
	}

	data, err := e.fs.Read(ctx, state.principal, handle.VirtualPath, int64(offset), int64(length))
	if err != nil {
		if errors.Is(err, os.ErrPermission) || errors.Is(err, ErrPermissionDenied) || errors.Is(err, ErrPathBoundary) {
			return e.buildDeniedResponse(header, state.sessionID, header.TreeID, denyReasonFromError(err))
		}
		if errors.Is(err, os.ErrNotExist) {
			return buildResponse(header, state.sessionID, statusObjectNameNotFound, nil)
		}
		return buildResponse(header, state.sessionID, statusInvalidParameter, nil)
	}

	return buildResponseWithTree(header, state.sessionID, header.TreeID, statusSuccess, buildReadBody(data))
}

func (e *Engine) handleWrite(ctx context.Context, state *connState, header requestHeader, body []byte) []byte {
	if !e.phaseAllowsWrite() {
		return e.buildPhaseDeniedResponse(header, state.sessionID, header.TreeID)
	}

	packet := append(buildHeaderStub(header), body...)
	fileID, offset, data, err := parseWriteRequest(packet)
	if err != nil {
		return buildResponse(header, state.sessionID, statusInvalidParameter, nil)
	}

	handle, ok := state.handles[fileID]
	if !ok {
		return buildResponse(header, state.sessionID, statusFileClosed, nil)
	}
	if handle.IsDir {
		return e.buildDeniedResponse(header, state.sessionID, header.TreeID, DenyReasonPermissionDenied)
	}

	written, err := e.fs.Write(ctx, state.principal, handle.VirtualPath, int64(offset), data)
	if err != nil {
		if errors.Is(err, os.ErrPermission) || errors.Is(err, ErrPermissionDenied) || errors.Is(err, ErrPathBoundary) {
			return e.buildDeniedResponse(header, state.sessionID, header.TreeID, denyReasonFromError(err))
		}
		if errors.Is(err, os.ErrNotExist) {
			return buildResponse(header, state.sessionID, statusObjectNameNotFound, nil)
		}
		return buildResponse(header, state.sessionID, statusInvalidParameter, nil)
	}
	if written < 0 {
		return buildResponse(header, state.sessionID, statusInvalidParameter, nil)
	}

	writtenCount := uint32(written)
	if written > int64(^uint32(0)) {
		writtenCount = ^uint32(0)
	}
	return buildResponseWithTree(header, state.sessionID, header.TreeID, statusSuccess, buildWriteBody(writtenCount))
}

func (e *Engine) handleSetInfo(ctx context.Context, state *connState, header requestHeader, body []byte) []byte {
	if !e.phaseAllowsManage() {
		return e.buildPhaseDeniedResponse(header, state.sessionID, header.TreeID)
	}

	packet := append(buildHeaderStub(header), body...)
	fileID, infoType, infoClass, payload, err := parseSetInfoRequest(packet)
	if err != nil {
		return buildResponse(header, state.sessionID, statusInvalidParameter, nil)
	}
	if infoType != setInfoTypeFile {
		return buildErrorResponse(header, statusNotSupported)
	}

	handle, ok := state.handles[fileID]
	if !ok {
		return buildResponse(header, state.sessionID, statusFileClosed, nil)
	}

	spaceName := spaceNameFromVirtualPath(handle.VirtualPath)
	if spaceName == "" {
		return buildResponse(header, state.sessionID, statusInvalidParameter, nil)
	}
	allowed, authErr := e.ensureSpacePermission(ctx, state, spaceName, PermissionManage)
	if authErr != nil || !allowed {
		return e.buildDeniedResponse(header, state.sessionID, header.TreeID, DenyReasonPermissionDenied)
	}

	switch infoClass {
	case fileInfoClassRename:
		renameTo, parseErr := parseRenameInfo(payload)
		if parseErr != nil {
			return buildResponse(header, state.sessionID, statusInvalidParameter, nil)
		}
		targetPath := joinTreePath(spaceName, renameTo)
		if strings.HasPrefix(renameTo, "/") {
			targetPath = pathpkg.Clean("/" + strings.TrimPrefix(renameTo, "/"))
		}
		if err := e.fs.Rename(ctx, state.principal, handle.VirtualPath, targetPath); err != nil {
			if errors.Is(err, os.ErrPermission) || errors.Is(err, ErrPermissionDenied) || errors.Is(err, ErrPathBoundary) {
				return e.buildDeniedResponse(header, state.sessionID, header.TreeID, denyReasonFromError(err))
			}
			if errors.Is(err, os.ErrNotExist) {
				return buildResponse(header, state.sessionID, statusObjectNameNotFound, nil)
			}
			return buildResponse(header, state.sessionID, statusInvalidParameter, nil)
		}
		handle.VirtualPath = targetPath
		state.handles[fileID] = handle
		return buildResponseWithTree(header, state.sessionID, header.TreeID, statusSuccess, buildSetInfoBody())
	case fileInfoClassDisposition:
		deletePending, parseErr := parseDispositionInfo(payload)
		if parseErr != nil {
			return buildResponse(header, state.sessionID, statusInvalidParameter, nil)
		}
		if !deletePending {
			return buildResponseWithTree(header, state.sessionID, header.TreeID, statusSuccess, buildSetInfoBody())
		}
		if err := e.fs.Delete(ctx, state.principal, handle.VirtualPath, handle.IsDir); err != nil {
			if errors.Is(err, os.ErrPermission) || errors.Is(err, ErrPermissionDenied) || errors.Is(err, ErrPathBoundary) {
				return e.buildDeniedResponse(header, state.sessionID, header.TreeID, denyReasonFromError(err))
			}
			if errors.Is(err, os.ErrNotExist) {
				return buildResponse(header, state.sessionID, statusObjectNameNotFound, nil)
			}
			return buildResponse(header, state.sessionID, statusInvalidParameter, nil)
		}
		delete(state.handles, fileID)
		return buildResponseWithTree(header, state.sessionID, header.TreeID, statusSuccess, buildSetInfoBody())
	default:
		return buildErrorResponse(header, statusNotSupported)
	}
}

func (e *Engine) handleLogoff(state *connState, header requestHeader) []byte {
	state.authenticated = false
	state.principal = ""
	state.sessionID = 0
	state.trees = make(map[uint32]treeState)
	state.handles = make(map[uint64]fileHandle)
	return buildResponse(header, 0, statusSuccess, buildLogoffBody())
}

func (e *Engine) phaseAllowsWrite() bool {
	switch e.cfg.RolloutPhase {
	case RolloutPhaseWriteSafe, RolloutPhaseWriteFull:
		return true
	default:
		return false
	}
}

func (e *Engine) phaseAllowsManage() bool {
	return e.cfg.RolloutPhase == RolloutPhaseWriteFull
}

func (e *Engine) ensureSpacePermission(ctx context.Context, state *connState, spaceName string, required Permission) (bool, error) {
	if e.authz == nil || strings.TrimSpace(state.principal) == "" {
		return false, os.ErrPermission
	}
	return e.authz.CanAccessSpace(ctx, state.principal, spaceName, required)
}

func (e *Engine) buildPhaseDeniedResponse(header requestHeader, sessionID uint64, treeID uint32) []byte {
	return e.buildDeniedResponse(header, sessionID, treeID, DenyReasonReadonlyPhaseDenied)
}

func (e *Engine) buildDeniedResponse(header requestHeader, sessionID uint64, treeID uint32, reason string) []byte {
	if e.telemetry != nil {
		e.telemetry.OnEvent(Event{
			Stage:  "policy",
			Reason: reason,
			Err:    nil,
		})
	}
	return buildResponseWithTree(header, sessionID, treeID, statusAccessDenied, nil)
}

func denyReasonFromError(err error) string {
	switch {
	case errors.Is(err, ErrPathBoundary):
		return DenyReasonPathBoundary
	default:
		return DenyReasonPermissionDenied
	}
}

func spaceNameFromVirtualPath(virtualPath string) string {
	clean := pathpkg.Clean("/" + strings.TrimPrefix(strings.ReplaceAll(virtualPath, "\\", "/"), "/"))
	if clean == "/" {
		return ""
	}
	parts := strings.Split(strings.TrimPrefix(clean, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

func (e *Engine) selectDialect(dialects []Dialect) (Dialect, bool) {
	best := Dialect("")
	for _, dialect := range dialects {
		if !e.Supports(dialect) {
			continue
		}
		if best == "" || dialectOrder(dialect) > dialectOrder(best) {
			best = dialect
		}
	}
	if best == "" {
		return "", false
	}
	return best, true
}

func dialectOrder(d Dialect) int {
	switch d {
	case Dialect210:
		return 1
	case Dialect300:
		return 2
	case Dialect302:
		return 3
	case Dialect311:
		return 4
	default:
		return 0
	}
}

func (c Config) Validate() error {
	ord := map[Dialect]int{
		Dialect210: 1,
		Dialect300: 2,
		Dialect302: 3,
		Dialect311: 4,
	}

	if _, ok := ord[c.MinDialect]; !ok {
		return fmt.Errorf("%w: unsupported min dialect %q", ErrInvalidDialectBounds, c.MinDialect)
	}
	if _, ok := ord[c.MaxDialect]; !ok {
		return fmt.Errorf("%w: unsupported max dialect %q", ErrInvalidDialectBounds, c.MaxDialect)
	}
	if ord[c.MinDialect] > ord[c.MaxDialect] {
		return fmt.Errorf("%w: min dialect %q higher than max dialect %q", ErrInvalidDialectBounds, c.MinDialect, c.MaxDialect)
	}
	switch c.RolloutPhase {
	case RolloutPhaseReadOnly, RolloutPhaseWriteSafe, RolloutPhaseWriteFull:
	default:
		return fmt.Errorf("unsupported smb rollout phase %q", c.RolloutPhase)
	}
	return nil
}

func readNetBIOSPacket(r io.Reader) ([]byte, error) {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}

	length := int(header[1])<<16 | int(header[2])<<8 | int(header[3])
	if length < 0 {
		return nil, io.ErrUnexpectedEOF
	}
	if length == 0 {
		return nil, nil
	}

	packet := make([]byte, length)
	if _, err := io.ReadFull(r, packet); err != nil {
		return nil, err
	}
	return packet, nil
}

func writeNetBIOSPacket(w io.Writer, payload []byte) error {
	if payload == nil {
		payload = []byte{}
	}
	size := len(payload)
	header := []byte{
		0x00,
		byte(size >> 16),
		byte(size >> 8),
		byte(size),
	}
	if _, err := w.Write(header); err != nil {
		return err
	}
	if size == 0 {
		return nil
	}
	_, err := w.Write(payload)
	return err
}

func parseRequestPacket(packet []byte) (requestHeader, []byte, error) {
	var zero requestHeader
	if len(packet) < smb2HeaderSize {
		return zero, nil, io.ErrUnexpectedEOF
	}
	if !isSMB2Packet(packet) {
		return zero, nil, errors.New("not smb2")
	}

	header := requestHeader{
		Command:   binary.LittleEndian.Uint16(packet[12:14]),
		MessageID: binary.LittleEndian.Uint64(packet[24:32]),
		TreeID:    binary.LittleEndian.Uint32(packet[36:40]),
		SessionID: binary.LittleEndian.Uint64(packet[40:48]),
	}
	return header, packet[smb2HeaderSize:], nil
}

func isSMB2Packet(packet []byte) bool {
	return len(packet) >= 4 &&
		packet[0] == 0xFE &&
		packet[1] == 'S' &&
		packet[2] == 'M' &&
		packet[3] == 'B'
}

func isSMB1Packet(packet []byte) bool {
	return len(packet) >= 4 &&
		packet[0] == 0xFF &&
		packet[1] == 'S' &&
		packet[2] == 'M' &&
		packet[3] == 'B'
}

func buildErrorResponse(req requestHeader, status uint32) []byte {
	return buildResponse(req, req.SessionID, status, nil)
}

func buildResponse(req requestHeader, sessionID uint64, status uint32, body []byte) []byte {
	return buildResponseWithTree(req, sessionID, req.TreeID, status, body)
}

func buildResponseWithTree(req requestHeader, sessionID uint64, treeID uint32, status uint32, body []byte) []byte {
	resp := make([]byte, smb2HeaderSize+len(body))
	resp[0] = 0xFE
	resp[1] = 'S'
	resp[2] = 'M'
	resp[3] = 'B'
	binary.LittleEndian.PutUint16(resp[4:6], smb2HeaderSize)
	binary.LittleEndian.PutUint16(resp[6:8], 1)
	binary.LittleEndian.PutUint32(resp[8:12], status)
	binary.LittleEndian.PutUint16(resp[12:14], req.Command)
	binary.LittleEndian.PutUint16(resp[14:16], 1)
	binary.LittleEndian.PutUint32(resp[16:20], smb2FlagServerToClient)
	binary.LittleEndian.PutUint64(resp[24:32], req.MessageID)
	binary.LittleEndian.PutUint32(resp[36:40], treeID)
	binary.LittleEndian.PutUint64(resp[40:48], sessionID)
	copy(resp[smb2HeaderSize:], body)
	return resp
}

func buildHeaderStub(req requestHeader) []byte {
	buf := make([]byte, smb2HeaderSize)
	buf[0] = 0xFE
	buf[1] = 'S'
	buf[2] = 'M'
	buf[3] = 'B'
	binary.LittleEndian.PutUint16(buf[4:6], smb2HeaderSize)
	binary.LittleEndian.PutUint16(buf[12:14], req.Command)
	binary.LittleEndian.PutUint64(buf[24:32], req.MessageID)
	binary.LittleEndian.PutUint32(buf[36:40], req.TreeID)
	binary.LittleEndian.PutUint64(buf[40:48], req.SessionID)
	return buf
}

func parseNegotiateDialects(body []byte) ([]Dialect, error) {
	if len(body) < 36 {
		return nil, io.ErrUnexpectedEOF
	}
	count := int(binary.LittleEndian.Uint16(body[2:4]))
	if count <= 0 {
		return nil, errors.New("empty dialect list")
	}
	start := 36
	end := start + count*2
	if len(body) < end {
		return nil, io.ErrUnexpectedEOF
	}

	result := make([]Dialect, 0, count)
	for i := 0; i < count; i++ {
		code := binary.LittleEndian.Uint16(body[start+i*2 : start+i*2+2])
		dialect, ok := dialectFromCode(code)
		if !ok {
			continue
		}
		result = append(result, dialect)
	}
	return result, nil
}

func dialectFromCode(code uint16) (Dialect, bool) {
	switch code {
	case 0x0210:
		return Dialect210, true
	case 0x0300:
		return Dialect300, true
	case 0x0302:
		return Dialect302, true
	case 0x0311:
		return Dialect311, true
	default:
		return "", false
	}
}

func codeFromDialect(d Dialect) uint16 {
	switch d {
	case Dialect210:
		return 0x0210
	case Dialect300:
		return 0x0300
	case Dialect302:
		return 0x0302
	case Dialect311:
		return 0x0311
	default:
		return 0
	}
}

type sessionSetupToken struct {
	NTLMToken     []byte
	MessageType   uint32
	SpnegoWrapped bool
}

func parseSessionSetupToken(packet []byte) (*sessionSetupToken, error) {
	if len(packet) < smb2HeaderSize+24 {
		return nil, io.ErrUnexpectedEOF
	}

	body := packet[smb2HeaderSize:]
	offset := int(binary.LittleEndian.Uint16(body[12:14]))
	length := int(binary.LittleEndian.Uint16(body[14:16]))
	if offset <= 0 || length <= 0 || offset+length > len(packet) {
		return nil, errors.New("invalid security buffer")
	}
	blob := packet[offset : offset+length]
	if len(blob) == 0 {
		return nil, errors.New("empty security buffer")
	}

	ntlmMarker := []byte("NTLMSSP\x00")
	idx := bytes.Index(blob, ntlmMarker)
	if idx < 0 {
		return nil, errors.New("ntlm token not found")
	}
	ntlmToken := append([]byte(nil), blob[idx:]...)
	if len(ntlmToken) < 12 {
		return nil, errors.New("invalid ntlm token")
	}

	return &sessionSetupToken{
		NTLMToken:     ntlmToken,
		MessageType:   binary.LittleEndian.Uint32(ntlmToken[8:12]),
		SpnegoWrapped: idx > 0 || isSpnegoEnvelope(blob),
	}, nil
}

func isSpnegoEnvelope(blob []byte) bool {
	if len(blob) == 0 {
		return false
	}
	switch blob[0] {
	case 0x60, 0xA0, 0xA1:
		return true
	default:
		return false
	}
}

func buildSPNEGONegTokenResp(responseToken []byte, complete bool) []byte {
	// NTLMSSP OID: 1.3.6.1.4.1.311.2.2.10
	ntlmOID := []byte{0x2B, 0x06, 0x01, 0x04, 0x01, 0x82, 0x37, 0x02, 0x02, 0x0A}

	negState := byte(0x01) // accept-incomplete
	if complete {
		negState = 0x00 // accept-completed
	}

	content := make([]byte, 0, 128)
	content = append(content, asn1Wrap(0xA0, asn1Wrap(0x0A, []byte{negState}))...)
	if !complete {
		content = append(content, asn1Wrap(0xA1, asn1Wrap(0x06, ntlmOID))...)
	}
	if len(responseToken) > 0 {
		content = append(content, asn1Wrap(0xA2, asn1Wrap(0x04, responseToken))...)
	}

	seq := asn1Wrap(0x30, content)
	return asn1Wrap(0xA1, seq)
}

func asn1Wrap(tag byte, content []byte) []byte {
	result := make([]byte, 0, 2+len(content))
	result = append(result, tag)
	result = append(result, encodeASN1Length(len(content))...)
	result = append(result, content...)
	return result
}

func encodeASN1Length(n int) []byte {
	if n < 0 {
		return []byte{0}
	}
	if n < 0x80 {
		return []byte{byte(n)}
	}
	if n <= 0xFF {
		return []byte{0x81, byte(n)}
	}
	if n <= 0xFFFF {
		return []byte{0x82, byte(n >> 8), byte(n)}
	}
	return []byte{0x83, byte(n >> 16), byte(n >> 8), byte(n)}
}

func parseTreeConnectPath(packet []byte) (string, error) {
	if len(packet) < smb2HeaderSize+8 {
		return "", io.ErrUnexpectedEOF
	}

	body := packet[smb2HeaderSize:]
	offset := int(binary.LittleEndian.Uint16(body[4:6]))
	length := int(binary.LittleEndian.Uint16(body[6:8]))
	if offset <= 0 || length <= 0 || offset+length > len(packet) {
		return "", errors.New("invalid tree path")
	}
	raw := packet[offset : offset+length]
	return decodeUTF16LE(raw), nil
}

func parseCreateRequest(packet []byte) (name string, disposition uint32, createOptions uint32, err error) {
	if len(packet) < smb2HeaderSize+56 {
		return "", 0, 0, io.ErrUnexpectedEOF
	}
	body := packet[smb2HeaderSize:]

	disposition = binary.LittleEndian.Uint32(body[36:40])
	createOptions = binary.LittleEndian.Uint32(body[48:52])
	nameOffset := int(binary.LittleEndian.Uint16(body[44:46]))
	nameLength := int(binary.LittleEndian.Uint16(body[46:48]))

	if nameLength == 0 {
		return "", disposition, createOptions, nil
	}
	if nameOffset <= 0 || nameOffset+nameLength > len(packet) {
		return "", 0, 0, errors.New("invalid create name")
	}
	name = decodeUTF16LE(packet[nameOffset : nameOffset+nameLength])
	return name, disposition, createOptions, nil
}

func parseCloseRequest(packet []byte) (fileID uint64, err error) {
	if len(packet) < smb2HeaderSize+24 {
		return 0, io.ErrUnexpectedEOF
	}
	body := packet[smb2HeaderSize:]
	fileID = binary.LittleEndian.Uint64(body[8:16])
	if fileID == 0 {
		return 0, errors.New("invalid file id")
	}
	return fileID, nil
}

func parseQueryDirectoryRequest(packet []byte) (fileID uint64, outputLength uint32, infoClass uint8, err error) {
	if len(packet) < smb2HeaderSize+32 {
		return 0, 0, 0, io.ErrUnexpectedEOF
	}
	body := packet[smb2HeaderSize:]
	infoClass = body[2]
	fileID = binary.LittleEndian.Uint64(body[8:16])
	outputLength = binary.LittleEndian.Uint32(body[28:32])
	if fileID == 0 {
		return 0, 0, 0, errors.New("invalid file id")
	}
	return fileID, outputLength, infoClass, nil
}

func parseReadRequest(packet []byte) (fileID uint64, offset uint64, length uint32, err error) {
	if len(packet) < smb2HeaderSize+48 {
		return 0, 0, 0, io.ErrUnexpectedEOF
	}
	body := packet[smb2HeaderSize:]
	length = binary.LittleEndian.Uint32(body[4:8])
	offset = binary.LittleEndian.Uint64(body[8:16])
	fileID = binary.LittleEndian.Uint64(body[16:24])
	if fileID == 0 {
		return 0, 0, 0, errors.New("invalid file id")
	}
	return fileID, offset, length, nil
}

func parseWriteRequest(packet []byte) (fileID uint64, offset uint64, data []byte, err error) {
	if len(packet) < smb2HeaderSize+48 {
		return 0, 0, nil, io.ErrUnexpectedEOF
	}
	body := packet[smb2HeaderSize:]
	dataOffset := int(binary.LittleEndian.Uint16(body[2:4]))
	dataLength := int(binary.LittleEndian.Uint32(body[4:8]))
	offset = binary.LittleEndian.Uint64(body[8:16])
	fileID = binary.LittleEndian.Uint64(body[16:24])

	if fileID == 0 {
		return 0, 0, nil, errors.New("invalid file id")
	}
	if dataOffset <= 0 || dataLength < 0 || dataOffset+dataLength > len(packet) {
		return 0, 0, nil, errors.New("invalid write buffer")
	}
	data = append([]byte(nil), packet[dataOffset:dataOffset+dataLength]...)
	return fileID, offset, data, nil
}

func parseSetInfoRequest(packet []byte) (fileID uint64, infoType uint8, infoClass uint8, payload []byte, err error) {
	if len(packet) < smb2HeaderSize+32 {
		return 0, 0, 0, nil, io.ErrUnexpectedEOF
	}
	body := packet[smb2HeaderSize:]
	infoType = body[2]
	infoClass = body[3]
	bufferLength := int(binary.LittleEndian.Uint32(body[4:8]))
	bufferOffset := int(binary.LittleEndian.Uint16(body[8:10]))
	fileID = binary.LittleEndian.Uint64(body[16:24])
	if fileID == 0 {
		return 0, 0, 0, nil, errors.New("invalid file id")
	}
	if bufferLength < 0 {
		return 0, 0, 0, nil, errors.New("invalid set-info buffer length")
	}
	if bufferLength == 0 {
		return fileID, infoType, infoClass, []byte{}, nil
	}
	if bufferOffset <= 0 || bufferOffset+bufferLength > len(packet) {
		return 0, 0, 0, nil, errors.New("invalid set-info buffer")
	}
	return fileID, infoType, infoClass, append([]byte(nil), packet[bufferOffset:bufferOffset+bufferLength]...), nil
}

func parseRenameInfo(payload []byte) (string, error) {
	if len(payload) < 20 {
		return "", io.ErrUnexpectedEOF
	}
	nameLength := int(binary.LittleEndian.Uint32(payload[16:20]))
	if nameLength <= 0 || 20+nameLength > len(payload) {
		return "", errors.New("invalid rename target")
	}
	name := decodeUTF16LE(payload[20 : 20+nameLength])
	name = strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	if name == "" {
		return "", errors.New("empty rename target")
	}
	return name, nil
}

func parseDispositionInfo(payload []byte) (bool, error) {
	if len(payload) < 1 {
		return false, io.ErrUnexpectedEOF
	}
	return payload[0] != 0, nil
}

func extractSpaceNameFromUNC(pathValue string) (string, error) {
	trimmed := strings.TrimSpace(pathValue)
	trimmed = strings.TrimPrefix(trimmed, "\\\\")
	parts := strings.Split(trimmed, "\\")
	if len(parts) < 2 {
		return "", errors.New("invalid unc path")
	}
	spaceName := strings.TrimSpace(parts[1])
	if spaceName == "" {
		return "", errors.New("empty space name")
	}
	return spaceName, nil
}

func joinTreePath(spaceName, pathName string) string {
	name := strings.ReplaceAll(pathName, "\\", "/")
	name = strings.TrimSpace(name)
	clean := pathpkg.Clean("/" + strings.TrimPrefix(name, "/"))
	if clean == "." || clean == "/" {
		return "/" + strings.TrimSpace(spaceName)
	}
	return "/" + strings.TrimSpace(spaceName) + clean
}

func decodeUTF16LE(data []byte) string {
	if len(data)%2 != 0 {
		data = data[:len(data)-1]
	}
	words := make([]uint16, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		words = append(words, binary.LittleEndian.Uint16(data[i:i+2]))
	}
	return string(utf16.Decode(words))
}

func buildNegotiateBody(dialect Dialect) []byte {
	body := make([]byte, 65)
	binary.LittleEndian.PutUint16(body[0:2], 65)
	binary.LittleEndian.PutUint16(body[2:4], 1)
	binary.LittleEndian.PutUint16(body[4:6], codeFromDialect(dialect))
	binary.LittleEndian.PutUint32(body[24:28], 64*1024)
	binary.LittleEndian.PutUint32(body[28:32], 64*1024)
	binary.LittleEndian.PutUint32(body[32:36], 64*1024)
	return body
}

func buildSessionSetupBody(securityToken []byte) []byte {
	body := make([]byte, 8+len(securityToken))
	binary.LittleEndian.PutUint16(body[0:2], 9)
	if len(securityToken) > 0 {
		binary.LittleEndian.PutUint16(body[4:6], smb2HeaderSize+8)
		binary.LittleEndian.PutUint16(body[6:8], uint16(len(securityToken)))
		copy(body[8:], securityToken)
	}
	return body
}

func buildTreeConnectBody() []byte {
	body := make([]byte, 16)
	binary.LittleEndian.PutUint16(body[0:2], 16)
	body[2] = 0x01 // disk share
	binary.LittleEndian.PutUint32(body[12:16], 0x00120089)
	return body
}

func buildTreeDisconnectBody() []byte {
	body := make([]byte, 4)
	binary.LittleEndian.PutUint16(body[0:2], 4)
	return body
}

func buildCreateBody(fileID uint64, entry DirEntry) []byte {
	body := make([]byte, 88)
	binary.LittleEndian.PutUint16(body[0:2], 89)
	if entry.IsDir {
		binary.LittleEndian.PutUint32(body[56:60], 0x00000010)
	}
	binary.LittleEndian.PutUint64(body[64:72], fileID)
	binary.LittleEndian.PutUint64(body[72:80], fileID)
	return body
}

func buildCloseBody(fileID uint64, isDir bool) []byte {
	body := make([]byte, 60)
	binary.LittleEndian.PutUint16(body[0:2], 60)
	if isDir {
		binary.LittleEndian.PutUint32(body[56:60], 0x00000010)
	} else {
		binary.LittleEndian.PutUint32(body[56:60], 0x00000080)
	}
	_ = fileID
	return body
}

func buildQueryDirectoryBody(data []byte) []byte {
	body := make([]byte, 8+len(data))
	binary.LittleEndian.PutUint16(body[0:2], 9)
	binary.LittleEndian.PutUint16(body[2:4], smb2HeaderSize+8)
	binary.LittleEndian.PutUint32(body[4:8], uint32(len(data)))
	copy(body[8:], data)
	return body
}

func buildReadBody(data []byte) []byte {
	body := make([]byte, 16+len(data))
	binary.LittleEndian.PutUint16(body[0:2], 17)
	body[2] = byte(smb2HeaderSize + 16)
	binary.LittleEndian.PutUint32(body[4:8], uint32(len(data)))
	copy(body[16:], data)
	return body
}

func buildWriteBody(written uint32) []byte {
	body := make([]byte, 16)
	binary.LittleEndian.PutUint16(body[0:2], 17)
	binary.LittleEndian.PutUint32(body[4:8], written)
	return body
}

func buildSetInfoBody() []byte {
	body := make([]byte, 2)
	binary.LittleEndian.PutUint16(body[0:2], 2)
	return body
}

func buildLogoffBody() []byte {
	body := make([]byte, 4)
	binary.LittleEndian.PutUint16(body[0:2], 4)
	return body
}

func encodeDirectoryEntries(entries []DirEntry, maxBytes uint32, infoClass uint8) []byte {
	if len(entries) == 0 {
		return []byte{}
	}

	limit := int(maxBytes)
	if limit <= 0 {
		limit = 64 * 1024
	}

	encoded := make([][]byte, 0, len(entries))
	total := 0
	for _, entry := range entries {
		var item []byte
		switch infoClass {
		case fileIdBothDirInfoClass:
			item = encodeFileIDBothDirectoryEntry(entry)
		default:
			item = encodeFileDirectoryEntry(entry)
		}
		if len(item) > limit {
			break
		}
		if total+len(item) > limit {
			break
		}
		encoded = append(encoded, item)
		total += len(item)
	}

	if len(encoded) == 0 {
		return []byte{}
	}

	result := make([]byte, 0, total)
	for idx, item := range encoded {
		if idx == len(encoded)-1 {
			binary.LittleEndian.PutUint32(item[0:4], 0)
		} else {
			binary.LittleEndian.PutUint32(item[0:4], uint32(len(item)))
		}
		result = append(result, item...)
	}
	return result
}

func isSupportedDirectoryInfoClass(infoClass uint8) bool {
	return infoClass == fileDirectoryInformationClass || infoClass == fileIdBothDirInfoClass
}

func encodeFileDirectoryEntry(entry DirEntry) []byte {
	name := encodeUTF16LE(entry.Name)
	base := 64
	size := align8(base + len(name))
	buf := make([]byte, size)

	binary.LittleEndian.PutUint64(buf[40:48], uint64(entry.Size))
	binary.LittleEndian.PutUint64(buf[48:56], uint64(entry.Size))
	binary.LittleEndian.PutUint32(buf[56:60], fileAttributes(entry.IsDir))
	binary.LittleEndian.PutUint32(buf[60:64], uint32(len(name)))
	copy(buf[64:], name)
	return buf
}

func encodeFileIDBothDirectoryEntry(entry DirEntry) []byte {
	name := encodeUTF16LE(entry.Name)
	base := 104
	size := align8(base + len(name))
	buf := make([]byte, size)

	binary.LittleEndian.PutUint64(buf[40:48], uint64(entry.Size))
	binary.LittleEndian.PutUint64(buf[48:56], uint64(entry.Size))
	binary.LittleEndian.PutUint32(buf[56:60], fileAttributes(entry.IsDir))
	binary.LittleEndian.PutUint32(buf[60:64], uint32(len(name)))
	// EaSize left as 0.
	// ShortNameLength/ShortName left as 0.
	// FileId left as 0 for read-only phase.
	copy(buf[104:], name)
	return buf
}

func fileAttributes(isDir bool) uint32 {
	if isDir {
		return 0x00000010
	}
	return 0x00000080
}

func align8(v int) int {
	rest := v % 8
	if rest == 0 {
		return v
	}
	return v + (8 - rest)
}

func encodeUTF16LE(value string) []byte {
	runes := utf16.Encode([]rune(value))
	out := make([]byte, len(runes)*2)
	for i, r := range runes {
		binary.LittleEndian.PutUint16(out[i*2:i*2+2], r)
	}
	return out
}
