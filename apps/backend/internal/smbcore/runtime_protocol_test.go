package smbcore

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	pathpkg "path"
	"strings"
	"testing"
	"time"

	ntlmssp "github.com/Azure/go-ntlmssp"
	"github.com/vadimi/go-ntlm/ntlm"
)

type fakeAuth struct {
	credentials map[string]string
}

func (a *fakeAuth) Authenticate(_ context.Context, username, password string) (string, error) {
	expected, ok := a.credentials[username]
	if !ok || expected != password {
		return "", os.ErrPermission
	}
	return username, nil
}

func (a *fakeAuth) ResolveSMBPassword(_ context.Context, username string) (string, error) {
	expected, ok := a.credentials[username]
	if !ok {
		return "", os.ErrNotExist
	}
	return expected, nil
}

type fakeAuthorizer struct {
	readAllowed   map[string]map[string]bool
	writeAllowed  map[string]map[string]bool
	manageAllowed map[string]map[string]bool
}

func (a *fakeAuthorizer) CanAccessSpace(_ context.Context, principal, spaceName string, required Permission) (bool, error) {
	var matrix map[string]map[string]bool
	switch required {
	case PermissionRead:
		matrix = a.readAllowed
	case PermissionWrite:
		matrix = a.writeAllowed
	case PermissionManage:
		matrix = a.manageAllowed
	default:
		return false, os.ErrPermission
	}
	spaces, ok := matrix[principal]
	if !ok {
		return false, nil
	}
	return spaces[spaceName], nil
}

type fakeFS struct {
	entries  map[string]DirEntry
	lists    map[string][]DirEntry
	contents map[string][]byte
}

type fakeTelemetry struct {
	events []Event
}

func (t *fakeTelemetry) OnEvent(event Event) {
	t.events = append(t.events, event)
}

func (f *fakeFS) Stat(_ context.Context, _ string, virtualPath string) (DirEntry, error) {
	if strings.Contains(virtualPath, "..") {
		return DirEntry{}, os.ErrPermission
	}
	entry, ok := f.entries[virtualPath]
	if !ok {
		return DirEntry{}, os.ErrNotExist
	}
	return entry, nil
}

func (f *fakeFS) List(_ context.Context, _ string, virtualPath string) ([]DirEntry, error) {
	entries, ok := f.lists[virtualPath]
	if !ok {
		return nil, os.ErrNotExist
	}
	return entries, nil
}

func (f *fakeFS) Read(_ context.Context, _ string, virtualPath string, offset, limit int64) ([]byte, error) {
	content, ok := f.contents[virtualPath]
	if !ok {
		return nil, os.ErrNotExist
	}
	if offset > int64(len(content)) {
		return []byte{}, nil
	}
	chunk := content[offset:]
	if limit > 0 && int64(len(chunk)) > limit {
		chunk = chunk[:limit]
	}
	return append([]byte(nil), chunk...), nil
}

func (f *fakeFS) CreateOrTruncate(_ context.Context, _ string, virtualPath string) (DirEntry, error) {
	if strings.Contains(virtualPath, "..") {
		return DirEntry{}, os.ErrPermission
	}
	if virtualPath == "" || virtualPath == "/" {
		return DirEntry{}, os.ErrPermission
	}

	parent := pathpkg.Dir(virtualPath)
	if parent == "." {
		parent = "/"
	}
	if parentEntry, ok := f.entries[parent]; !ok || !parentEntry.IsDir {
		return DirEntry{}, os.ErrNotExist
	}

	name := pathpkg.Base(virtualPath)
	entry := DirEntry{Name: name, IsDir: false, Size: 0}
	f.entries[virtualPath] = entry
	f.contents[virtualPath] = []byte{}
	f.upsertListEntry(parent, entry)
	return entry, nil
}

func (f *fakeFS) Write(_ context.Context, _ string, virtualPath string, offset int64, data []byte) (int64, error) {
	if strings.Contains(virtualPath, "..") {
		return 0, os.ErrPermission
	}
	if offset < 0 {
		return 0, os.ErrInvalid
	}
	entry, ok := f.entries[virtualPath]
	if !ok {
		return 0, os.ErrNotExist
	}
	if entry.IsDir {
		return 0, os.ErrPermission
	}

	content := append([]byte(nil), f.contents[virtualPath]...)
	end := int(offset) + len(data)
	if end > len(content) {
		expanded := make([]byte, end)
		copy(expanded, content)
		content = expanded
	}
	copy(content[int(offset):], data)
	f.contents[virtualPath] = content
	entry.Size = int64(len(content))
	f.entries[virtualPath] = entry
	parent := pathpkg.Dir(virtualPath)
	if parent == "." {
		parent = "/"
	}
	f.upsertListEntry(parent, entry)
	return int64(len(data)), nil
}

func (f *fakeFS) Mkdir(_ context.Context, _ string, virtualPath string) (DirEntry, error) {
	if strings.Contains(virtualPath, "..") {
		return DirEntry{}, os.ErrPermission
	}
	if virtualPath == "" || virtualPath == "/" {
		return DirEntry{}, os.ErrPermission
	}

	parent := pathpkg.Dir(virtualPath)
	if parent == "." {
		parent = "/"
	}
	if parentEntry, ok := f.entries[parent]; !ok || !parentEntry.IsDir {
		return DirEntry{}, os.ErrNotExist
	}

	name := pathpkg.Base(virtualPath)
	entry := DirEntry{Name: name, IsDir: true, Size: 0}
	f.entries[virtualPath] = entry
	if _, ok := f.lists[virtualPath]; !ok {
		f.lists[virtualPath] = []DirEntry{}
	}
	f.upsertListEntry(parent, entry)
	return entry, nil
}

func (f *fakeFS) Rename(_ context.Context, _ string, fromPath, toPath string) error {
	fromSpace, _, err := splitSpaceRelativePathForTest(fromPath)
	if err != nil {
		return os.ErrPermission
	}
	toSpace, _, err := splitSpaceRelativePathForTest(toPath)
	if err != nil {
		return os.ErrPermission
	}
	if fromSpace != toSpace {
		return errors.Join(ErrPathBoundary, os.ErrPermission)
	}

	entry, ok := f.entries[fromPath]
	if !ok {
		return os.ErrNotExist
	}
	fromParent := pathpkg.Dir(fromPath)
	if fromParent == "." {
		fromParent = "/"
	}
	toParent := pathpkg.Dir(toPath)
	if toParent == "." {
		toParent = "/"
	}
	if parentEntry, ok := f.entries[toParent]; !ok || !parentEntry.IsDir {
		return os.ErrNotExist
	}

	delete(f.entries, fromPath)
	if data, ok := f.contents[fromPath]; ok {
		delete(f.contents, fromPath)
		f.contents[toPath] = data
	}
	f.removeListEntry(fromParent, pathpkg.Base(fromPath))
	entry.Name = pathpkg.Base(toPath)
	f.entries[toPath] = entry
	f.upsertListEntry(toParent, entry)
	return nil
}

func (f *fakeFS) Delete(_ context.Context, _ string, virtualPath string, isDir bool) error {
	entry, ok := f.entries[virtualPath]
	if !ok {
		return os.ErrNotExist
	}
	if isDir && !entry.IsDir {
		return os.ErrPermission
	}
	parent := pathpkg.Dir(virtualPath)
	if parent == "." {
		parent = "/"
	}
	delete(f.entries, virtualPath)
	delete(f.contents, virtualPath)
	delete(f.lists, virtualPath)
	f.removeListEntry(parent, pathpkg.Base(virtualPath))
	return nil
}

func (f *fakeFS) upsertListEntry(parent string, entry DirEntry) {
	list := f.lists[parent]
	for i := range list {
		if list[i].Name == entry.Name {
			list[i] = entry
			f.lists[parent] = list
			return
		}
	}
	f.lists[parent] = append(list, entry)
}

func (f *fakeFS) removeListEntry(parent, name string) {
	list := f.lists[parent]
	if len(list) == 0 {
		return
	}
	next := list[:0]
	for _, item := range list {
		if item.Name == name {
			continue
		}
		next = append(next, item)
	}
	f.lists[parent] = append([]DirEntry(nil), next...)
}

func splitSpaceRelativePathForTest(pathValue string) (space, rel string, err error) {
	clean := pathpkg.Clean("/" + strings.TrimPrefix(pathValue, "/"))
	if clean == "/" {
		return "", "", os.ErrPermission
	}
	parts := strings.Split(strings.TrimPrefix(clean, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return "", "", os.ErrInvalid
	}
	space = parts[0]
	if len(parts) > 1 {
		rel = pathpkg.Join(parts[1:]...)
	}
	return space, rel, nil
}

func TestHandleConn_EnforcesDialectBoundsAndRejectsSMB1(t *testing.T) {
	engine := newProtocolTestEngine(t)

	{
		client, done := runEngineWithPipe(t, engine)
		defer closePipe(t, client, done)

		sendSMBPacket(t, client, buildNegotiateRequest(1, []uint16{0x0202}))
		resp := readSMBPacket(t, client)
		if got := responseStatus(resp); got != statusNotSupported {
			t.Fatalf("expected not-supported status, got 0x%08x", got)
		}
	}

	{
		client, done := runEngineWithPipe(t, engine)
		defer closePipe(t, client, done)

		sendSMBPacket(t, client, buildSMB1Probe())
		client.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
		_, err := readSMBPacketE(client)
		if err == nil {
			t.Fatal("expected smb1 probe connection closure")
		}
	}
}

func TestHandleConn_AuthAuthorizationListReadAndWriteDeny(t *testing.T) {
	engine := newProtocolTestEngine(t)
	client, done := runEngineWithPipe(t, engine)
	defer closePipe(t, client, done)

	sendSMBPacket(t, client, buildNegotiateRequest(1, []uint16{0x0210, 0x0311}))
	resp := readSMBPacket(t, client)
	if got := responseStatus(resp); got != statusSuccess {
		t.Fatalf("negotiate status: 0x%08x", got)
	}

	sessionID, finalStatus := runNTLMSessionSetupFlow(t, client, 2, 0, "user", "wrong")
	if finalStatus != statusLogonFailure {
		t.Fatalf("expected logon failure, got 0x%08x", finalStatus)
	}

	sessionID, finalStatus = runNTLMSessionSetupFlow(t, client, 10, sessionID, "user", "pass")
	if finalStatus != statusSuccess {
		t.Fatalf("session setup status: 0x%08x", finalStatus)
	}
	if sessionID == 0 {
		t.Fatal("expected non-zero session id")
	}

	sendSMBPacket(t, client, buildTreeConnectRequest(20, sessionID, `\\cohesion\beta`))
	resp = readSMBPacket(t, client)
	if got := responseStatus(resp); got != statusAccessDenied {
		t.Fatalf("expected access denied on unauthorized share, got 0x%08x", got)
	}

	sendSMBPacket(t, client, buildTreeConnectRequest(21, sessionID, `\\cohesion\alpha`))
	resp = readSMBPacket(t, client)
	if got := responseStatus(resp); got != statusSuccess {
		t.Fatalf("tree connect status: 0x%08x", got)
	}
	treeID := binary.LittleEndian.Uint32(resp[36:40])
	if treeID == 0 {
		t.Fatal("expected non-zero tree id")
	}

	sendSMBPacket(t, client, buildCreateRequest(22, sessionID, treeID, "", 1))
	resp = readSMBPacket(t, client)
	if got := responseStatus(resp); got != statusSuccess {
		t.Fatalf("create dir handle status: 0x%08x", got)
	}
	dirFileID := binary.LittleEndian.Uint64(resp[smb2HeaderSize+64 : smb2HeaderSize+72])

	sendSMBPacket(t, client, buildQueryDirectoryRequest(23, sessionID, treeID, dirFileID, 4096))
	resp = readSMBPacket(t, client)
	if got := responseStatus(resp); got != statusSuccess {
		t.Fatalf("query directory status: 0x%08x", got)
	}
	if !bytes.Contains(queryDirectoryData(resp), encodeUTF16LEForTest("docs")) {
		t.Fatalf("expected docs entry in query directory payload")
	}

	sendSMBPacket(t, client, buildCreateRequest(24, sessionID, treeID, `docs\report.txt`, 1))
	resp = readSMBPacket(t, client)
	if got := responseStatus(resp); got != statusSuccess {
		t.Fatalf("create file handle status: 0x%08x", got)
	}
	fileID := binary.LittleEndian.Uint64(resp[smb2HeaderSize+64 : smb2HeaderSize+72])

	sendSMBPacket(t, client, buildReadRequest(25, sessionID, treeID, fileID, 0, 5))
	resp = readSMBPacket(t, client)
	if got := responseStatus(resp); got != statusSuccess {
		t.Fatalf("read status: 0x%08x", got)
	}
	if got := string(readData(resp)); got != "hello" {
		t.Fatalf("unexpected read payload: %q", got)
	}

	sendSMBPacket(t, client, buildCloseRequest(26, sessionID, treeID, fileID))
	resp = readSMBPacket(t, client)
	if got := responseStatus(resp); got != statusSuccess {
		t.Fatalf("expected close success, got 0x%08x", got)
	}

	sendSMBPacket(t, client, buildReadRequest(27, sessionID, treeID, fileID, 0, 5))
	resp = readSMBPacket(t, client)
	if got := responseStatus(resp); got != statusFileClosed {
		t.Fatalf("expected read on closed handle to fail, got 0x%08x", got)
	}

	sendSMBPacket(t, client, buildWriteRequest(28, sessionID, treeID, fileID, []byte("!")))
	resp = readSMBPacket(t, client)
	if got := responseStatus(resp); got != statusAccessDenied {
		t.Fatalf("expected write denial, got 0x%08x", got)
	}

	sendSMBPacket(t, client, buildCreateRequest(29, sessionID, treeID, `docs\new.txt`, 2))
	resp = readSMBPacket(t, client)
	if got := responseStatus(resp); got != statusAccessDenied {
		t.Fatalf("expected create(truncate/write) denial, got 0x%08x", got)
	}

	sendSMBPacket(t, client, buildSetInfoRequest(30, sessionID, treeID, fileID))
	resp = readSMBPacket(t, client)
	if got := responseStatus(resp); got != statusAccessDenied {
		t.Fatalf("expected set-info denial, got 0x%08x", got)
	}
}

func TestHandleConn_WriteSafe_AllowsCreateAndWrite(t *testing.T) {
	engine := newProtocolTestEngineWithPhase(t, RolloutPhaseWriteSafe)
	client, done := runEngineWithPipe(t, engine)
	defer closePipe(t, client, done)

	sendSMBPacket(t, client, buildNegotiateRequest(1, []uint16{0x0210, 0x0311}))
	resp := readSMBPacket(t, client)
	if got := responseStatus(resp); got != statusSuccess {
		t.Fatalf("negotiate status: 0x%08x", got)
	}

	sessionID, finalStatus := runNTLMSessionSetupFlow(t, client, 2, 0, "user", "pass")
	if finalStatus != statusSuccess {
		t.Fatalf("session setup status: 0x%08x", finalStatus)
	}

	sendSMBPacket(t, client, buildTreeConnectRequest(10, sessionID, `\\cohesion\alpha`))
	resp = readSMBPacket(t, client)
	if got := responseStatus(resp); got != statusSuccess {
		t.Fatalf("tree connect status: 0x%08x", got)
	}
	treeID := binary.LittleEndian.Uint32(resp[36:40])

	sendSMBPacket(t, client, buildCreateRequest(11, sessionID, treeID, `docs\new.txt`, 2))
	resp = readSMBPacket(t, client)
	if got := responseStatus(resp); got != statusSuccess {
		t.Fatalf("create (write-safe) status: 0x%08x", got)
	}
	fileID := binary.LittleEndian.Uint64(resp[smb2HeaderSize+64 : smb2HeaderSize+72])

	sendSMBPacket(t, client, buildWriteRequest(12, sessionID, treeID, fileID, []byte("hello-safe")))
	resp = readSMBPacket(t, client)
	if got := responseStatus(resp); got != statusSuccess {
		t.Fatalf("write (write-safe) status: 0x%08x", got)
	}

	sendSMBPacket(t, client, buildReadRequest(13, sessionID, treeID, fileID, 0, 10))
	resp = readSMBPacket(t, client)
	if got := responseStatus(resp); got != statusSuccess {
		t.Fatalf("read after write status: 0x%08x", got)
	}
	if got := string(readData(resp)); got != "hello-safe" {
		t.Fatalf("unexpected read payload: %q", got)
	}
}

func TestHandleConn_ReadonlyPhaseDenied_EmitsTelemetryReason(t *testing.T) {
	telemetry := &fakeTelemetry{}
	engine := newProtocolTestEngineWithPhaseAndTelemetry(t, RolloutPhaseReadOnly, telemetry)
	client, done := runEngineWithPipe(t, engine)
	defer closePipe(t, client, done)

	sendSMBPacket(t, client, buildNegotiateRequest(1, []uint16{0x0210, 0x0311}))
	resp := readSMBPacket(t, client)
	if got := responseStatus(resp); got != statusSuccess {
		t.Fatalf("negotiate status: 0x%08x", got)
	}

	sessionID, finalStatus := runNTLMSessionSetupFlow(t, client, 2, 0, "user", "pass")
	if finalStatus != statusSuccess {
		t.Fatalf("session setup status: 0x%08x", finalStatus)
	}

	sendSMBPacket(t, client, buildTreeConnectRequest(10, sessionID, `\\cohesion\alpha`))
	resp = readSMBPacket(t, client)
	if got := responseStatus(resp); got != statusSuccess {
		t.Fatalf("tree connect status: 0x%08x", got)
	}
	treeID := binary.LittleEndian.Uint32(resp[36:40])

	sendSMBPacket(t, client, buildCreateRequest(11, sessionID, treeID, `docs\deny.txt`, 2))
	resp = readSMBPacket(t, client)
	if got := responseStatus(resp); got != statusAccessDenied {
		t.Fatalf("expected readonly create denial, got 0x%08x", got)
	}

	found := false
	for _, event := range telemetry.events {
		if event.Stage == "policy" && event.Reason == "readonly_phase_denied" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected telemetry event readonly_phase_denied")
	}
}

func TestHandleConn_RollbackToReadonly_DeniesNewWriteRequests(t *testing.T) {
	{
		engine := newProtocolTestEngineWithPhase(t, RolloutPhaseWriteSafe)
		client, done := runEngineWithPipe(t, engine)
		defer closePipe(t, client, done)

		sendSMBPacket(t, client, buildNegotiateRequest(1, []uint16{0x0210, 0x0311}))
		resp := readSMBPacket(t, client)
		if got := responseStatus(resp); got != statusSuccess {
			t.Fatalf("negotiate(write-safe) status: 0x%08x", got)
		}

		sessionID, finalStatus := runNTLMSessionSetupFlow(t, client, 2, 0, "user", "pass")
		if finalStatus != statusSuccess {
			t.Fatalf("session setup(write-safe) status: 0x%08x", finalStatus)
		}

		sendSMBPacket(t, client, buildTreeConnectRequest(10, sessionID, `\\cohesion\alpha`))
		resp = readSMBPacket(t, client)
		if got := responseStatus(resp); got != statusSuccess {
			t.Fatalf("tree connect(write-safe) status: 0x%08x", got)
		}
		treeID := binary.LittleEndian.Uint32(resp[36:40])

		sendSMBPacket(t, client, buildCreateRequest(11, sessionID, treeID, `docs\rollback.txt`, 2))
		resp = readSMBPacket(t, client)
		if got := responseStatus(resp); got != statusSuccess {
			t.Fatalf("expected write-safe create success before rollback, got 0x%08x", got)
		}
	}

	{
		engine := newProtocolTestEngineWithPhase(t, RolloutPhaseReadOnly)
		client, done := runEngineWithPipe(t, engine)
		defer closePipe(t, client, done)

		sendSMBPacket(t, client, buildNegotiateRequest(101, []uint16{0x0210, 0x0311}))
		resp := readSMBPacket(t, client)
		if got := responseStatus(resp); got != statusSuccess {
			t.Fatalf("negotiate(readonly) status: 0x%08x", got)
		}

		sessionID, finalStatus := runNTLMSessionSetupFlow(t, client, 102, 0, "user", "pass")
		if finalStatus != statusSuccess {
			t.Fatalf("session setup(readonly) status: 0x%08x", finalStatus)
		}

		sendSMBPacket(t, client, buildTreeConnectRequest(110, sessionID, `\\cohesion\alpha`))
		resp = readSMBPacket(t, client)
		if got := responseStatus(resp); got != statusSuccess {
			t.Fatalf("tree connect(readonly) status: 0x%08x", got)
		}
		treeID := binary.LittleEndian.Uint32(resp[36:40])

		sendSMBPacket(t, client, buildCreateRequest(111, sessionID, treeID, `docs\rollback.txt`, 2))
		resp = readSMBPacket(t, client)
		if got := responseStatus(resp); got != statusAccessDenied {
			t.Fatalf("expected readonly create denial after rollback, got 0x%08x", got)
		}
	}
}

func TestHandleConn_WriteFull_ManageOpsAndBoundaryReason(t *testing.T) {
	telemetry := &fakeTelemetry{}
	engine := newProtocolTestEngineWithPhaseAndTelemetry(t, RolloutPhaseWriteFull, telemetry)
	client, done := runEngineWithPipe(t, engine)
	defer closePipe(t, client, done)

	sendSMBPacket(t, client, buildNegotiateRequest(1, []uint16{0x0210, 0x0311}))
	resp := readSMBPacket(t, client)
	if got := responseStatus(resp); got != statusSuccess {
		t.Fatalf("negotiate status: 0x%08x", got)
	}

	sessionID, finalStatus := runNTLMSessionSetupFlow(t, client, 2, 0, "user", "pass")
	if finalStatus != statusSuccess {
		t.Fatalf("session setup status: 0x%08x", finalStatus)
	}

	sendSMBPacket(t, client, buildTreeConnectRequest(10, sessionID, `\\cohesion\alpha`))
	resp = readSMBPacket(t, client)
	if got := responseStatus(resp); got != statusSuccess {
		t.Fatalf("tree connect status: 0x%08x", got)
	}
	treeID := binary.LittleEndian.Uint32(resp[36:40])

	sendSMBPacket(t, client, buildCreateDirectoryRequest(11, sessionID, treeID, `docs\ops`))
	resp = readSMBPacket(t, client)
	if got := responseStatus(resp); got != statusSuccess {
		t.Fatalf("mkdir(create dir) status: 0x%08x", got)
	}

	sendSMBPacket(t, client, buildCreateRequest(12, sessionID, treeID, `docs\ops\target.txt`, 2))
	resp = readSMBPacket(t, client)
	if got := responseStatus(resp); got != statusSuccess {
		t.Fatalf("create file status: 0x%08x", got)
	}
	fileID := binary.LittleEndian.Uint64(resp[smb2HeaderSize+64 : smb2HeaderSize+72])

	sendSMBPacket(t, client, buildSetInfoRenameRequest(13, sessionID, treeID, fileID, `docs\ops\renamed.txt`))
	resp = readSMBPacket(t, client)
	if got := responseStatus(resp); got != statusSuccess {
		t.Fatalf("rename status: 0x%08x", got)
	}

	sendSMBPacket(t, client, buildSetInfoDispositionRequest(14, sessionID, treeID, fileID, true))
	resp = readSMBPacket(t, client)
	if got := responseStatus(resp); got != statusSuccess {
		t.Fatalf("delete status: 0x%08x", got)
	}

	sendSMBPacket(t, client, buildCreateRequest(15, sessionID, treeID, `docs\ops\cross.txt`, 2))
	resp = readSMBPacket(t, client)
	if got := responseStatus(resp); got != statusSuccess {
		t.Fatalf("create file for boundary test status: 0x%08x", got)
	}
	fileID = binary.LittleEndian.Uint64(resp[smb2HeaderSize+64 : smb2HeaderSize+72])

	sendSMBPacket(t, client, buildSetInfoRenameRequest(16, sessionID, treeID, fileID, `/beta/evil.txt`))
	resp = readSMBPacket(t, client)
	if got := responseStatus(resp); got != statusAccessDenied {
		t.Fatalf("expected cross-space rename denial, got 0x%08x", got)
	}

	foundBoundary := false
	for _, event := range telemetry.events {
		if event.Stage == "policy" && event.Reason == DenyReasonPathBoundary {
			foundBoundary = true
			break
		}
	}
	if !foundBoundary {
		t.Fatal("expected path_boundary_violation telemetry reason")
	}
}

func TestHandleConn_WriteFull_ManagePermissionDeniedReason(t *testing.T) {
	telemetry := &fakeTelemetry{}
	authz := &fakeAuthorizer{
		readAllowed: map[string]map[string]bool{
			"user": {"alpha": true},
		},
		writeAllowed: map[string]map[string]bool{
			"user": {"alpha": true},
		},
		manageAllowed: map[string]map[string]bool{
			"user": {"alpha": false},
		},
	}
	engine := newProtocolTestEngineWithAuthorizerAndTelemetry(t, RolloutPhaseWriteFull, authz, telemetry)
	client, done := runEngineWithPipe(t, engine)
	defer closePipe(t, client, done)

	sendSMBPacket(t, client, buildNegotiateRequest(1, []uint16{0x0210, 0x0311}))
	resp := readSMBPacket(t, client)
	if got := responseStatus(resp); got != statusSuccess {
		t.Fatalf("negotiate status: 0x%08x", got)
	}

	sessionID, finalStatus := runNTLMSessionSetupFlow(t, client, 2, 0, "user", "pass")
	if finalStatus != statusSuccess {
		t.Fatalf("session setup status: 0x%08x", finalStatus)
	}

	sendSMBPacket(t, client, buildTreeConnectRequest(10, sessionID, `\\cohesion\alpha`))
	resp = readSMBPacket(t, client)
	if got := responseStatus(resp); got != statusSuccess {
		t.Fatalf("tree connect status: 0x%08x", got)
	}
	treeID := binary.LittleEndian.Uint32(resp[36:40])

	sendSMBPacket(t, client, buildCreateDirectoryRequest(11, sessionID, treeID, `docs\manage-denied`))
	resp = readSMBPacket(t, client)
	if got := responseStatus(resp); got != statusAccessDenied {
		t.Fatalf("expected manage permission denied, got 0x%08x", got)
	}

	foundPermission := false
	for _, event := range telemetry.events {
		if event.Stage == "policy" && event.Reason == DenyReasonPermissionDenied {
			foundPermission = true
			break
		}
	}
	if !foundPermission {
		t.Fatal("expected permission_denied telemetry reason")
	}
}

func newProtocolTestEngine(t *testing.T) *Engine {
	t.Helper()
	return newProtocolTestEngineWithPhaseAndTelemetry(t, RolloutPhaseReadOnly, nil)
}

func newProtocolTestEngineWithPhase(t *testing.T, phase RolloutPhase) *Engine {
	t.Helper()
	return newProtocolTestEngineWithPhaseAndTelemetry(t, phase, nil)
}

func newProtocolTestEngineWithPhaseAndTelemetry(t *testing.T, phase RolloutPhase, telemetry Telemetry) *Engine {
	t.Helper()
	return newProtocolTestEngineWithAuthorizerAndTelemetry(t, phase, defaultFakeAuthorizer(), telemetry)
}

func newProtocolTestEngineWithAuthorizerAndTelemetry(t *testing.T, phase RolloutPhase, authz Authorizer, telemetry Telemetry) *Engine {
	t.Helper()
	if authz == nil {
		authz = defaultFakeAuthorizer()
	}
	engine, err := NewEngine(
		Config{
			MinDialect:   Dialect210,
			MaxDialect:   Dialect311,
			RolloutPhase: phase,
		},
		&fakeAuth{credentials: map[string]string{"user": "pass"}},
		authz,
		&fakeFS{
			entries: map[string]DirEntry{
				"/alpha":                 {Name: "alpha", IsDir: true},
				"/alpha/docs":            {Name: "docs", IsDir: true},
				"/alpha/docs/report.txt": {Name: "report.txt", IsDir: false, Size: 11},
			},
			lists: map[string][]DirEntry{
				"/alpha": {
					{Name: "docs", IsDir: true},
				},
				"/alpha/docs": {
					{Name: "report.txt", IsDir: false, Size: 11},
				},
			},
			contents: map[string][]byte{
				"/alpha/docs/report.txt": []byte("hello-world"),
			},
		},
		telemetry,
	)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	return engine
}

func defaultFakeAuthorizer() Authorizer {
	return &fakeAuthorizer{
		readAllowed: map[string]map[string]bool{
			"user": {"alpha": true},
		},
		writeAllowed: map[string]map[string]bool{
			"user": {"alpha": true},
		},
		manageAllowed: map[string]map[string]bool{
			"user": {"alpha": true},
		},
	}
}

func runEngineWithPipe(t *testing.T, engine *Engine) (net.Conn, chan error) {
	t.Helper()
	server, client := net.Pipe()
	done := make(chan error, 1)
	go func() {
		done <- engine.HandleConn(context.Background(), server)
	}()
	return client, done
}

func closePipe(t *testing.T, client net.Conn, done chan error) {
	t.Helper()
	_ = client.Close()
	select {
	case err := <-done:
		if err != nil && !errors.Is(err, io.EOF) {
			t.Fatalf("engine returned unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for engine shutdown")
	}
}

func sendSMBPacket(t *testing.T, conn net.Conn, payload []byte) {
	t.Helper()
	frame := make([]byte, 4+len(payload))
	frame[0] = 0x00
	frame[1] = byte(len(payload) >> 16)
	frame[2] = byte(len(payload) >> 8)
	frame[3] = byte(len(payload))
	copy(frame[4:], payload)
	conn.SetWriteDeadline(time.Now().Add(time.Second))
	if _, err := conn.Write(frame); err != nil {
		t.Fatalf("write smb packet: %v", err)
	}
}

func readSMBPacket(t *testing.T, conn net.Conn) []byte {
	t.Helper()
	payload, err := readSMBPacketE(conn)
	if err != nil {
		t.Fatalf("read smb packet: %v", err)
	}
	return payload
}

func readSMBPacketE(conn net.Conn) ([]byte, error) {
	conn.SetReadDeadline(time.Now().Add(time.Second))
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, err
	}
	size := int(header[1])<<16 | int(header[2])<<8 | int(header[3])
	payload := make([]byte, size)
	if _, err := io.ReadFull(conn, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func responseStatus(resp []byte) uint32 {
	if len(resp) < smb2HeaderSize {
		return statusInvalidParameter
	}
	return binary.LittleEndian.Uint32(resp[8:12])
}

func queryDirectoryData(resp []byte) []byte {
	if len(resp) < smb2HeaderSize+8 {
		return nil
	}
	body := resp[smb2HeaderSize:]
	length := int(binary.LittleEndian.Uint32(body[4:8]))
	if length <= 0 {
		return nil
	}
	if 8+length > len(body) {
		return nil
	}
	return body[8 : 8+length]
}

func readData(resp []byte) []byte {
	if len(resp) < smb2HeaderSize+16 {
		return nil
	}
	body := resp[smb2HeaderSize:]
	offset := int(body[2])
	length := int(binary.LittleEndian.Uint32(body[4:8]))
	if offset <= 0 || offset+length > len(resp) {
		return nil
	}
	return resp[offset : offset+length]
}

func buildSMB2Request(command uint16, messageID, sessionID uint64, treeID uint32, body []byte) []byte {
	packet := make([]byte, smb2HeaderSize+len(body))
	packet[0] = 0xFE
	packet[1] = 'S'
	packet[2] = 'M'
	packet[3] = 'B'
	binary.LittleEndian.PutUint16(packet[4:6], smb2HeaderSize)
	binary.LittleEndian.PutUint16(packet[12:14], command)
	binary.LittleEndian.PutUint64(packet[24:32], messageID)
	binary.LittleEndian.PutUint32(packet[36:40], treeID)
	binary.LittleEndian.PutUint64(packet[40:48], sessionID)
	copy(packet[smb2HeaderSize:], body)
	return packet
}

func buildNegotiateRequest(messageID uint64, dialects []uint16) []byte {
	body := make([]byte, 36+len(dialects)*2)
	binary.LittleEndian.PutUint16(body[0:2], 36)
	binary.LittleEndian.PutUint16(body[2:4], uint16(len(dialects)))
	start := 36
	for i, dialect := range dialects {
		binary.LittleEndian.PutUint16(body[start+i*2:start+i*2+2], dialect)
	}
	return buildSMB2Request(cmdNegotiate, messageID, 0, 0, body)
}

func buildSessionSetupRequest(messageID, sessionID uint64, token []byte) []byte {
	body := make([]byte, 24+len(token))
	binary.LittleEndian.PutUint16(body[0:2], 25)
	binary.LittleEndian.PutUint16(body[12:14], smb2HeaderSize+24)
	binary.LittleEndian.PutUint16(body[14:16], uint16(len(token)))
	copy(body[24:], token)
	return buildSMB2Request(cmdSessionSetup, messageID, sessionID, 0, body)
}

func runNTLMSessionSetupFlow(t *testing.T, conn net.Conn, startMessageID, sessionID uint64, username, password string) (uint64, uint32) {
	t.Helper()

	clientSession, err := ntlm.CreateClientSession(ntlm.Version2, ntlm.ConnectionlessMode)
	if err != nil {
		t.Fatalf("create client ntlm session: %v", err)
	}
	clientSession.SetUserInfo(username, password, "")

	negotiateToken, err := ntlmssp.NewNegotiateMessage("", "")
	if err != nil {
		t.Fatalf("generate negotiate token: %v", err)
	}
	sendSMBPacket(t, conn, buildSessionSetupRequest(startMessageID, sessionID, wrapSPNEGONegTokenInitForTest(negotiateToken)))

	challengeResp := readSMBPacket(t, conn)
	if got := responseStatus(challengeResp); got != statusMoreProcessing {
		return binary.LittleEndian.Uint64(challengeResp[40:48]), got
	}
	issuedSessionID := binary.LittleEndian.Uint64(challengeResp[40:48])
	if issuedSessionID == 0 {
		t.Fatal("expected session id in challenge response")
	}

	challengeToken := extractNTLMTokenForTest(sessionSetupSecurityTokenForTest(challengeResp))
	if len(challengeToken) == 0 {
		t.Fatal("expected NTLM challenge token")
	}
	challenge, err := ntlm.ParseChallengeMessage(challengeToken)
	if err != nil {
		t.Fatalf("parse challenge token: %v", err)
	}
	if err := clientSession.ProcessChallengeMessage(challenge); err != nil {
		t.Fatalf("process challenge token: %v", err)
	}

	authenticate, err := clientSession.GenerateAuthenticateMessage()
	if err != nil {
		t.Fatalf("generate authenticate token: %v", err)
	}
	sendSMBPacket(t, conn, buildSessionSetupRequest(startMessageID+1, issuedSessionID, wrapSPNEGONegTokenRespForTest(authenticate.Bytes())))

	finalResp := readSMBPacket(t, conn)
	return issuedSessionID, responseStatus(finalResp)
}

func sessionSetupSecurityTokenForTest(resp []byte) []byte {
	if len(resp) < smb2HeaderSize+8 {
		return nil
	}
	body := resp[smb2HeaderSize:]
	offset := int(binary.LittleEndian.Uint16(body[4:6]))
	length := int(binary.LittleEndian.Uint16(body[6:8]))
	if offset <= 0 || length <= 0 || offset+length > len(resp) {
		return nil
	}
	return resp[offset : offset+length]
}

func extractNTLMTokenForTest(blob []byte) []byte {
	marker := []byte("NTLMSSP\x00")
	idx := bytes.Index(blob, marker)
	if idx < 0 {
		return nil
	}
	return blob[idx:]
}

func wrapSPNEGONegTokenInitForTest(ntlmToken []byte) []byte {
	ntlmOID := []byte{0x2B, 0x06, 0x01, 0x04, 0x01, 0x82, 0x37, 0x02, 0x02, 0x0A}
	mechType := asn1WrapForTest(0x30, asn1WrapForTest(0x06, ntlmOID))
	inner := make([]byte, 0, len(ntlmToken)+64)
	inner = append(inner, asn1WrapForTest(0xA0, mechType)...)
	inner = append(inner, asn1WrapForTest(0xA2, asn1WrapForTest(0x04, ntlmToken))...)
	negInit := asn1WrapForTest(0xA0, asn1WrapForTest(0x30, inner))
	content := append(asn1WrapForTest(0x06, []byte{0x2B, 0x06, 0x01, 0x05, 0x05, 0x02}), negInit...)
	return asn1WrapForTest(0x60, content)
}

func wrapSPNEGONegTokenRespForTest(ntlmToken []byte) []byte {
	seq := asn1WrapForTest(0x30, asn1WrapForTest(0xA2, asn1WrapForTest(0x04, ntlmToken)))
	return asn1WrapForTest(0xA1, seq)
}

func asn1WrapForTest(tag byte, content []byte) []byte {
	result := make([]byte, 0, 2+len(content))
	result = append(result, tag)
	result = append(result, encodeASN1LengthForTest(len(content))...)
	result = append(result, content...)
	return result
}

func encodeASN1LengthForTest(n int) []byte {
	if n < 0x80 {
		return []byte{byte(n)}
	}
	if n <= 0xFF {
		return []byte{0x81, byte(n)}
	}
	return []byte{0x82, byte(n >> 8), byte(n)}
}

func buildTreeConnectRequest(messageID, sessionID uint64, uncPath string) []byte {
	pathBytes := encodeUTF16LEForTest(uncPath)
	body := make([]byte, 8+len(pathBytes))
	binary.LittleEndian.PutUint16(body[0:2], 9)
	binary.LittleEndian.PutUint16(body[4:6], smb2HeaderSize+8)
	binary.LittleEndian.PutUint16(body[6:8], uint16(len(pathBytes)))
	copy(body[8:], pathBytes)
	return buildSMB2Request(cmdTreeConnect, messageID, sessionID, 0, body)
}

func buildCreateRequest(messageID, sessionID uint64, treeID uint32, name string, disposition uint32) []byte {
	nameBytes := encodeUTF16LEForTest(name)
	body := make([]byte, 56+len(nameBytes))
	binary.LittleEndian.PutUint16(body[0:2], 57)
	binary.LittleEndian.PutUint32(body[36:40], disposition)
	binary.LittleEndian.PutUint16(body[44:46], smb2HeaderSize+56)
	binary.LittleEndian.PutUint16(body[46:48], uint16(len(nameBytes)))
	copy(body[56:], nameBytes)
	return buildSMB2Request(cmdCreate, messageID, sessionID, treeID, body)
}

func buildCreateDirectoryRequest(messageID, sessionID uint64, treeID uint32, name string) []byte {
	nameBytes := encodeUTF16LEForTest(name)
	body := make([]byte, 56+len(nameBytes))
	binary.LittleEndian.PutUint16(body[0:2], 57)
	binary.LittleEndian.PutUint32(body[36:40], 2)
	binary.LittleEndian.PutUint16(body[44:46], smb2HeaderSize+56)
	binary.LittleEndian.PutUint16(body[46:48], uint16(len(nameBytes)))
	binary.LittleEndian.PutUint32(body[48:52], createOptionDirectory)
	copy(body[56:], nameBytes)
	return buildSMB2Request(cmdCreate, messageID, sessionID, treeID, body)
}

func buildQueryDirectoryRequest(messageID, sessionID uint64, treeID uint32, fileID uint64, outputLength uint32) []byte {
	body := make([]byte, 32)
	binary.LittleEndian.PutUint16(body[0:2], 33)
	body[2] = fileIdBothDirInfoClass
	binary.LittleEndian.PutUint64(body[8:16], fileID)
	binary.LittleEndian.PutUint64(body[16:24], fileID)
	binary.LittleEndian.PutUint32(body[28:32], outputLength)
	return buildSMB2Request(cmdQueryDirectory, messageID, sessionID, treeID, body)
}

func buildCloseRequest(messageID, sessionID uint64, treeID uint32, fileID uint64) []byte {
	body := make([]byte, 24)
	binary.LittleEndian.PutUint16(body[0:2], 24)
	binary.LittleEndian.PutUint64(body[8:16], fileID)
	binary.LittleEndian.PutUint64(body[16:24], fileID)
	return buildSMB2Request(cmdClose, messageID, sessionID, treeID, body)
}

func buildReadRequest(messageID, sessionID uint64, treeID uint32, fileID uint64, offset uint64, length uint32) []byte {
	body := make([]byte, 48)
	binary.LittleEndian.PutUint16(body[0:2], 49)
	binary.LittleEndian.PutUint32(body[4:8], length)
	binary.LittleEndian.PutUint64(body[8:16], offset)
	binary.LittleEndian.PutUint64(body[16:24], fileID)
	binary.LittleEndian.PutUint64(body[24:32], fileID)
	return buildSMB2Request(cmdRead, messageID, sessionID, treeID, body)
}

func buildWriteRequest(messageID, sessionID uint64, treeID uint32, fileID uint64, data []byte) []byte {
	body := make([]byte, 48+len(data))
	binary.LittleEndian.PutUint16(body[0:2], 49)
	binary.LittleEndian.PutUint16(body[2:4], smb2HeaderSize+48)
	binary.LittleEndian.PutUint32(body[4:8], uint32(len(data)))
	binary.LittleEndian.PutUint64(body[16:24], fileID)
	binary.LittleEndian.PutUint64(body[24:32], fileID)
	copy(body[48:], data)
	return buildSMB2Request(cmdWrite, messageID, sessionID, treeID, body)
}

func buildSetInfoRequest(messageID, sessionID uint64, treeID uint32, fileID uint64) []byte {
	body := make([]byte, 32)
	binary.LittleEndian.PutUint16(body[0:2], 33)
	binary.LittleEndian.PutUint64(body[16:24], fileID)
	binary.LittleEndian.PutUint64(body[24:32], fileID)
	return buildSMB2Request(cmdSetInfo, messageID, sessionID, treeID, body)
}

func buildSetInfoRenameRequest(messageID, sessionID uint64, treeID uint32, fileID uint64, target string) []byte {
	targetBytes := encodeUTF16LEForTest(target)
	info := make([]byte, 20+len(targetBytes))
	binary.LittleEndian.PutUint32(info[16:20], uint32(len(targetBytes)))
	copy(info[20:], targetBytes)

	body := make([]byte, 32+len(info))
	binary.LittleEndian.PutUint16(body[0:2], 33)
	body[2] = setInfoTypeFile
	body[3] = fileInfoClassRename
	binary.LittleEndian.PutUint32(body[4:8], uint32(len(info)))
	binary.LittleEndian.PutUint16(body[8:10], smb2HeaderSize+32)
	binary.LittleEndian.PutUint64(body[16:24], fileID)
	binary.LittleEndian.PutUint64(body[24:32], fileID)
	copy(body[32:], info)
	return buildSMB2Request(cmdSetInfo, messageID, sessionID, treeID, body)
}

func buildSetInfoDispositionRequest(messageID, sessionID uint64, treeID uint32, fileID uint64, deletePending bool) []byte {
	info := []byte{0}
	if deletePending {
		info[0] = 1
	}
	body := make([]byte, 33)
	binary.LittleEndian.PutUint16(body[0:2], 33)
	body[2] = setInfoTypeFile
	body[3] = fileInfoClassDisposition
	binary.LittleEndian.PutUint32(body[4:8], uint32(len(info)))
	binary.LittleEndian.PutUint16(body[8:10], smb2HeaderSize+32)
	binary.LittleEndian.PutUint64(body[16:24], fileID)
	binary.LittleEndian.PutUint64(body[24:32], fileID)
	copy(body[32:], info)
	return buildSMB2Request(cmdSetInfo, messageID, sessionID, treeID, body)
}

func buildSMB1Probe() []byte {
	return []byte{0xFF, 'S', 'M', 'B', 0x72, 0x00, 0x00, 0x00}
}

func encodeUTF16LEForTest(value string) []byte {
	runes := []rune(value)
	encoded := make([]byte, len(runes)*2)
	for i, r := range runes {
		binary.LittleEndian.PutUint16(encoded[i*2:i*2+2], uint16(r))
	}
	return encoded
}
