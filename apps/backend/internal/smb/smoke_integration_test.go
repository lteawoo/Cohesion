//go:build integration

package smb

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	ntlmssp "github.com/Azure/go-ntlmssp"
	"github.com/vadimi/go-ntlm/ntlm"
	"taeu.kr/cohesion/internal/account"
	"taeu.kr/cohesion/internal/space"
)

func TestSMBReadOnlySmoke_ConnectListRead(t *testing.T) {
	accountSvc, spaceSvc, db := setupGuardServices(t)
	defer db.Close()

	ctx := context.Background()
	spaceRoot := t.TempDir()
	docsDir := filepath.Join(spaceRoot, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	filePath := filepath.Join(docsDir, "smoke.txt")
	if err := os.WriteFile(filePath, []byte("smoke-data"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	created, err := spaceSvc.CreateSpace(ctx, &space.CreateSpaceRequest{
		SpaceName: "alpha",
		SpacePath: spaceRoot,
	})
	if err != nil {
		t.Fatalf("create space: %v", err)
	}

	user, err := accountSvc.CreateUser(ctx, &account.CreateUserRequest{
		Username: "smoke-user",
		Password: "smoke-user-password",
		Nickname: "Smoke User",
		Role:     account.RoleUser,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := accountSvc.ReplaceUserPermissions(ctx, user.ID, []*account.UserSpacePermission{{
		UserID:     user.ID,
		SpaceID:    created.ID,
		Permission: account.PermissionRead,
	}}); err != nil {
		t.Fatalf("replace permissions: %v", err)
	}

	port := reservePort(t)
	svc := NewService(
		spaceSvc,
		accountSvc,
		true,
		port,
		"readonly",
	)
	if err := svc.Start(); err != nil {
		t.Fatalf("start smb service: %v", err)
	}
	defer svc.Stop()

	readiness := svc.Readiness()
	if readiness.State != StateHealthy {
		t.Fatalf("expected healthy readiness for smoke test, got state=%q reason=%q message=%q", readiness.State, readiness.Reason, readiness.Message)
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", toPortString(port)), time.Second)
	if err != nil {
		t.Fatalf("dial smb service: %v", err)
	}
	defer conn.Close()

	sendSMBPacketForSmoke(t, conn, buildNegotiateRequestForSmoke(1, []uint16{0x0210, 0x0311}))
	resp := readSMBPacketForSmoke(t, conn)
	if got := responseStatusForSmoke(resp); got != 0x00000000 {
		t.Fatalf("negotiate status: 0x%08x", got)
	}

	sessionID, finalStatus := runNTLMSessionSetupFlowForSmoke(t, conn, 2, 0, "smoke-user", "smoke-user-password")
	if finalStatus != 0x00000000 {
		t.Fatalf("session setup status: 0x%08x", finalStatus)
	}
	if sessionID == 0 {
		t.Fatal("expected non-zero session id")
	}

	sendSMBPacketForSmoke(t, conn, buildTreeConnectRequestForSmoke(10, sessionID, `\\cohesion\alpha`))
	resp = readSMBPacketForSmoke(t, conn)
	if got := responseStatusForSmoke(resp); got != 0x00000000 {
		t.Fatalf("tree connect status: 0x%08x", got)
	}
	treeID := binary.LittleEndian.Uint32(resp[36:40])

	sendSMBPacketForSmoke(t, conn, buildCreateRequestForSmoke(11, sessionID, treeID, "", 1))
	resp = readSMBPacketForSmoke(t, conn)
	if got := responseStatusForSmoke(resp); got != 0x00000000 {
		t.Fatalf("open root directory status: 0x%08x", got)
	}
	dirFileID := binary.LittleEndian.Uint64(resp[64+64 : 64+72])

	sendSMBPacketForSmoke(t, conn, buildQueryDirectoryRequestForSmoke(12, sessionID, treeID, dirFileID, 4096))
	resp = readSMBPacketForSmoke(t, conn)
	if got := responseStatusForSmoke(resp); got != 0x00000000 {
		t.Fatalf("query directory status: 0x%08x", got)
	}
	if !bytes.Contains(queryDirectoryDataForSmoke(resp), encodeUTF16LEForSmoke("docs")) {
		t.Fatalf("expected docs entry in directory payload")
	}

	sendSMBPacketForSmoke(t, conn, buildCreateRequestForSmoke(13, sessionID, treeID, `docs\smoke.txt`, 1))
	resp = readSMBPacketForSmoke(t, conn)
	if got := responseStatusForSmoke(resp); got != 0x00000000 {
		t.Fatalf("open file status: 0x%08x", got)
	}
	fileID := binary.LittleEndian.Uint64(resp[64+64 : 64+72])

	sendSMBPacketForSmoke(t, conn, buildReadRequestForSmoke(14, sessionID, treeID, fileID, 0, 9))
	resp = readSMBPacketForSmoke(t, conn)
	if got := responseStatusForSmoke(resp); got != 0x00000000 {
		t.Fatalf("read status: 0x%08x", got)
	}
	if got := string(readDataForSmoke(resp)); got != "smoke-dat" {
		t.Fatalf("unexpected read payload: %q", got)
	}
}

func TestSMBWriteFullSmoke_CreateWriteTruncateRenameDelete(t *testing.T) {
	accountSvc, spaceSvc, db := setupGuardServices(t)
	defer db.Close()

	ctx := context.Background()
	spaceRoot := t.TempDir()
	docsDir := filepath.Join(spaceRoot, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}

	created, err := spaceSvc.CreateSpace(ctx, &space.CreateSpaceRequest{
		SpaceName: "alpha",
		SpacePath: spaceRoot,
	})
	if err != nil {
		t.Fatalf("create space: %v", err)
	}

	user, err := accountSvc.CreateUser(ctx, &account.CreateUserRequest{
		Username: "smoke-write-user",
		Password: "smoke-write-password",
		Nickname: "Smoke Write User",
		Role:     account.RoleUser,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := accountSvc.ReplaceUserPermissions(ctx, user.ID, []*account.UserSpacePermission{{
		UserID:     user.ID,
		SpaceID:    created.ID,
		Permission: account.PermissionManage,
	}}); err != nil {
		t.Fatalf("replace permissions: %v", err)
	}

	port := reservePort(t)
	svc := NewService(
		spaceSvc,
		accountSvc,
		true,
		port,
		"write-full",
	)
	if err := svc.Start(); err != nil {
		t.Fatalf("start smb service: %v", err)
	}
	defer svc.Stop()

	readiness := svc.Readiness()
	if readiness.State != StateHealthy {
		t.Fatalf("expected healthy readiness for write smoke, got state=%q reason=%q message=%q", readiness.State, readiness.Reason, readiness.Message)
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", toPortString(port)), time.Second)
	if err != nil {
		t.Fatalf("dial smb service: %v", err)
	}
	defer conn.Close()

	sendSMBPacketForSmoke(t, conn, buildNegotiateRequestForSmoke(1, []uint16{0x0210, 0x0311}))
	resp := readSMBPacketForSmoke(t, conn)
	if got := responseStatusForSmoke(resp); got != 0x00000000 {
		t.Fatalf("negotiate status: 0x%08x", got)
	}

	sessionID, finalStatus := runNTLMSessionSetupFlowForSmoke(t, conn, 2, 0, "smoke-write-user", "smoke-write-password")
	if finalStatus != 0x00000000 {
		t.Fatalf("session setup status: 0x%08x", finalStatus)
	}

	sendSMBPacketForSmoke(t, conn, buildTreeConnectRequestForSmoke(10, sessionID, `\\cohesion\alpha`))
	resp = readSMBPacketForSmoke(t, conn)
	if got := responseStatusForSmoke(resp); got != 0x00000000 {
		t.Fatalf("tree connect status: 0x%08x", got)
	}
	treeID := binary.LittleEndian.Uint32(resp[36:40])

	sendSMBPacketForSmoke(t, conn, buildCreateDirectoryRequestForSmoke(11, sessionID, treeID, `docs\smoke-mkdir`))
	resp = readSMBPacketForSmoke(t, conn)
	if got := responseStatusForSmoke(resp); got != 0x00000000 {
		t.Fatalf("mkdir status: 0x%08x", got)
	}

	sendSMBPacketForSmoke(t, conn, buildCreateRequestForSmoke(12, sessionID, treeID, `docs\smoke-write.txt`, 2))
	resp = readSMBPacketForSmoke(t, conn)
	if got := responseStatusForSmoke(resp); got != 0x00000000 {
		t.Fatalf("create status: 0x%08x", got)
	}
	fileID := binary.LittleEndian.Uint64(resp[64+64 : 64+72])

	sendSMBPacketForSmoke(t, conn, buildWriteRequestForSmoke(13, sessionID, treeID, fileID, []byte("hello-write")))
	resp = readSMBPacketForSmoke(t, conn)
	if got := responseStatusForSmoke(resp); got != 0x00000000 {
		t.Fatalf("write status: 0x%08x", got)
	}

	sendSMBPacketForSmoke(t, conn, buildReadRequestForSmoke(14, sessionID, treeID, fileID, 0, 11))
	resp = readSMBPacketForSmoke(t, conn)
	if got := responseStatusForSmoke(resp); got != 0x00000000 {
		t.Fatalf("read-after-write status: 0x%08x", got)
	}
	if got := string(readDataForSmoke(resp)); got != "hello-write" {
		t.Fatalf("unexpected read payload: %q", got)
	}

	sendSMBPacketForSmoke(t, conn, buildCreateRequestForSmoke(15, sessionID, treeID, `docs\smoke-write.txt`, 2))
	resp = readSMBPacketForSmoke(t, conn)
	if got := responseStatusForSmoke(resp); got != 0x00000000 {
		t.Fatalf("truncate(create again) status: 0x%08x", got)
	}
	truncatedFileID := binary.LittleEndian.Uint64(resp[64+64 : 64+72])

	sendSMBPacketForSmoke(t, conn, buildReadRequestForSmoke(16, sessionID, treeID, truncatedFileID, 0, 11))
	resp = readSMBPacketForSmoke(t, conn)
	if got := responseStatusForSmoke(resp); got != 0x00000000 {
		t.Fatalf("read-after-truncate status: 0x%08x", got)
	}
	if got := string(readDataForSmoke(resp)); got != "" {
		t.Fatalf("expected empty payload after truncate, got %q", got)
	}

	sendSMBPacketForSmoke(t, conn, buildWriteRequestForSmoke(17, sessionID, treeID, truncatedFileID, []byte("renamed")))
	resp = readSMBPacketForSmoke(t, conn)
	if got := responseStatusForSmoke(resp); got != 0x00000000 {
		t.Fatalf("write-before-rename status: 0x%08x", got)
	}

	sendSMBPacketForSmoke(t, conn, buildSetInfoRenameRequestForSmoke(18, sessionID, treeID, truncatedFileID, `docs\smoke-renamed.txt`))
	resp = readSMBPacketForSmoke(t, conn)
	if got := responseStatusForSmoke(resp); got != 0x00000000 {
		t.Fatalf("rename status: 0x%08x", got)
	}

	sendSMBPacketForSmoke(t, conn, buildSetInfoDispositionRequestForSmoke(19, sessionID, treeID, truncatedFileID, true))
	resp = readSMBPacketForSmoke(t, conn)
	if got := responseStatusForSmoke(resp); got != 0x00000000 {
		t.Fatalf("delete status: 0x%08x", got)
	}
}

func sendSMBPacketForSmoke(t *testing.T, conn net.Conn, payload []byte) {
	t.Helper()
	frame := make([]byte, 4+len(payload))
	frame[0] = 0x00
	frame[1] = byte(len(payload) >> 16)
	frame[2] = byte(len(payload) >> 8)
	frame[3] = byte(len(payload))
	copy(frame[4:], payload)
	conn.SetWriteDeadline(time.Now().Add(time.Second))
	if _, err := conn.Write(frame); err != nil {
		t.Fatalf("write packet: %v", err)
	}
}

func readSMBPacketForSmoke(t *testing.T, conn net.Conn) []byte {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(time.Second))
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		t.Fatalf("read header: %v", err)
	}
	size := int(header[1])<<16 | int(header[2])<<8 | int(header[3])
	payload := make([]byte, size)
	if _, err := io.ReadFull(conn, payload); err != nil {
		t.Fatalf("read payload: %v", err)
	}
	return payload
}

func responseStatusForSmoke(resp []byte) uint32 {
	return binary.LittleEndian.Uint32(resp[8:12])
}

func queryDirectoryDataForSmoke(resp []byte) []byte {
	body := resp[64:]
	length := int(binary.LittleEndian.Uint32(body[4:8]))
	if length <= 0 || len(body) < 8+length {
		return nil
	}
	return body[8 : 8+length]
}

func readDataForSmoke(resp []byte) []byte {
	body := resp[64:]
	offset := int(body[2])
	length := int(binary.LittleEndian.Uint32(body[4:8]))
	if offset <= 0 || len(resp) < offset+length {
		return nil
	}
	return resp[offset : offset+length]
}

func buildSMB2RequestForSmoke(command uint16, messageID, sessionID uint64, treeID uint32, body []byte) []byte {
	packet := make([]byte, 64+len(body))
	packet[0] = 0xFE
	packet[1] = 'S'
	packet[2] = 'M'
	packet[3] = 'B'
	binary.LittleEndian.PutUint16(packet[4:6], 64)
	binary.LittleEndian.PutUint16(packet[12:14], command)
	binary.LittleEndian.PutUint64(packet[24:32], messageID)
	binary.LittleEndian.PutUint32(packet[36:40], treeID)
	binary.LittleEndian.PutUint64(packet[40:48], sessionID)
	copy(packet[64:], body)
	return packet
}

func buildNegotiateRequestForSmoke(messageID uint64, dialects []uint16) []byte {
	body := make([]byte, 36+len(dialects)*2)
	binary.LittleEndian.PutUint16(body[0:2], 36)
	binary.LittleEndian.PutUint16(body[2:4], uint16(len(dialects)))
	for i, d := range dialects {
		binary.LittleEndian.PutUint16(body[36+i*2:36+i*2+2], d)
	}
	return buildSMB2RequestForSmoke(0x0000, messageID, 0, 0, body)
}

func buildSessionSetupRequestForSmoke(messageID, sessionID uint64, token []byte) []byte {
	body := make([]byte, 24+len(token))
	binary.LittleEndian.PutUint16(body[0:2], 25)
	binary.LittleEndian.PutUint16(body[12:14], 64+24)
	binary.LittleEndian.PutUint16(body[14:16], uint16(len(token)))
	copy(body[24:], token)
	return buildSMB2RequestForSmoke(0x0001, messageID, sessionID, 0, body)
}

func runNTLMSessionSetupFlowForSmoke(t *testing.T, conn net.Conn, startMessageID, sessionID uint64, username, password string) (uint64, uint32) {
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
	sendSMBPacketForSmoke(t, conn, buildSessionSetupRequestForSmoke(startMessageID, sessionID, wrapSPNEGONegTokenInitForSmoke(negotiateToken)))

	challengeResp := readSMBPacketForSmoke(t, conn)
	if got := responseStatusForSmoke(challengeResp); got != 0xC0000016 {
		return binary.LittleEndian.Uint64(challengeResp[40:48]), got
	}
	issuedSessionID := binary.LittleEndian.Uint64(challengeResp[40:48])
	if issuedSessionID == 0 {
		t.Fatal("expected session id in challenge response")
	}

	challengeToken := extractNTLMTokenForSmoke(sessionSetupSecurityTokenForSmoke(challengeResp))
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
	sendSMBPacketForSmoke(t, conn, buildSessionSetupRequestForSmoke(startMessageID+1, issuedSessionID, wrapSPNEGONegTokenRespForSmoke(authenticate.Bytes())))

	finalResp := readSMBPacketForSmoke(t, conn)
	return issuedSessionID, responseStatusForSmoke(finalResp)
}

func sessionSetupSecurityTokenForSmoke(resp []byte) []byte {
	if len(resp) < 64+8 {
		return nil
	}
	body := resp[64:]
	offset := int(binary.LittleEndian.Uint16(body[4:6]))
	length := int(binary.LittleEndian.Uint16(body[6:8]))
	if offset <= 0 || length <= 0 || offset+length > len(resp) {
		return nil
	}
	return resp[offset : offset+length]
}

func extractNTLMTokenForSmoke(blob []byte) []byte {
	marker := []byte("NTLMSSP\x00")
	idx := bytes.Index(blob, marker)
	if idx < 0 {
		return nil
	}
	return blob[idx:]
}

func wrapSPNEGONegTokenInitForSmoke(ntlmToken []byte) []byte {
	ntlmOID := []byte{0x2B, 0x06, 0x01, 0x04, 0x01, 0x82, 0x37, 0x02, 0x02, 0x0A}
	mechType := asn1WrapForSmoke(0x30, asn1WrapForSmoke(0x06, ntlmOID))
	inner := make([]byte, 0, len(ntlmToken)+64)
	inner = append(inner, asn1WrapForSmoke(0xA0, mechType)...)
	inner = append(inner, asn1WrapForSmoke(0xA2, asn1WrapForSmoke(0x04, ntlmToken))...)
	negInit := asn1WrapForSmoke(0xA0, asn1WrapForSmoke(0x30, inner))
	content := append(asn1WrapForSmoke(0x06, []byte{0x2B, 0x06, 0x01, 0x05, 0x05, 0x02}), negInit...)
	return asn1WrapForSmoke(0x60, content)
}

func wrapSPNEGONegTokenRespForSmoke(ntlmToken []byte) []byte {
	seq := asn1WrapForSmoke(0x30, asn1WrapForSmoke(0xA2, asn1WrapForSmoke(0x04, ntlmToken)))
	return asn1WrapForSmoke(0xA1, seq)
}

func asn1WrapForSmoke(tag byte, content []byte) []byte {
	result := make([]byte, 0, 2+len(content))
	result = append(result, tag)
	result = append(result, encodeASN1LengthForSmoke(len(content))...)
	result = append(result, content...)
	return result
}

func encodeASN1LengthForSmoke(n int) []byte {
	if n < 0x80 {
		return []byte{byte(n)}
	}
	if n <= 0xFF {
		return []byte{0x81, byte(n)}
	}
	return []byte{0x82, byte(n >> 8), byte(n)}
}

func buildTreeConnectRequestForSmoke(messageID, sessionID uint64, path string) []byte {
	encoded := encodeUTF16LEForSmoke(path)
	body := make([]byte, 8+len(encoded))
	binary.LittleEndian.PutUint16(body[0:2], 9)
	binary.LittleEndian.PutUint16(body[4:6], 64+8)
	binary.LittleEndian.PutUint16(body[6:8], uint16(len(encoded)))
	copy(body[8:], encoded)
	return buildSMB2RequestForSmoke(0x0003, messageID, sessionID, 0, body)
}

func buildCreateRequestForSmoke(messageID, sessionID uint64, treeID uint32, name string, disposition uint32) []byte {
	encoded := encodeUTF16LEForSmoke(name)
	body := make([]byte, 56+len(encoded))
	binary.LittleEndian.PutUint16(body[0:2], 57)
	binary.LittleEndian.PutUint32(body[36:40], disposition)
	binary.LittleEndian.PutUint16(body[44:46], 64+56)
	binary.LittleEndian.PutUint16(body[46:48], uint16(len(encoded)))
	copy(body[56:], encoded)
	return buildSMB2RequestForSmoke(0x0005, messageID, sessionID, treeID, body)
}

func buildCreateDirectoryRequestForSmoke(messageID, sessionID uint64, treeID uint32, name string) []byte {
	encoded := encodeUTF16LEForSmoke(name)
	body := make([]byte, 56+len(encoded))
	binary.LittleEndian.PutUint16(body[0:2], 57)
	binary.LittleEndian.PutUint32(body[36:40], 2)
	binary.LittleEndian.PutUint16(body[44:46], 64+56)
	binary.LittleEndian.PutUint16(body[46:48], uint16(len(encoded)))
	binary.LittleEndian.PutUint32(body[48:52], 0x00000001)
	copy(body[56:], encoded)
	return buildSMB2RequestForSmoke(0x0005, messageID, sessionID, treeID, body)
}

func buildQueryDirectoryRequestForSmoke(messageID, sessionID uint64, treeID uint32, fileID uint64, outputLength uint32) []byte {
	body := make([]byte, 32)
	binary.LittleEndian.PutUint16(body[0:2], 33)
	body[2] = 0x25 // FileIdBothDirectoryInformation
	binary.LittleEndian.PutUint64(body[8:16], fileID)
	binary.LittleEndian.PutUint64(body[16:24], fileID)
	binary.LittleEndian.PutUint32(body[28:32], outputLength)
	return buildSMB2RequestForSmoke(0x000E, messageID, sessionID, treeID, body)
}

func buildReadRequestForSmoke(messageID, sessionID uint64, treeID uint32, fileID uint64, offset uint64, length uint32) []byte {
	body := make([]byte, 48)
	binary.LittleEndian.PutUint16(body[0:2], 49)
	binary.LittleEndian.PutUint32(body[4:8], length)
	binary.LittleEndian.PutUint64(body[8:16], offset)
	binary.LittleEndian.PutUint64(body[16:24], fileID)
	binary.LittleEndian.PutUint64(body[24:32], fileID)
	return buildSMB2RequestForSmoke(0x0008, messageID, sessionID, treeID, body)
}

func buildWriteRequestForSmoke(messageID, sessionID uint64, treeID uint32, fileID uint64, data []byte) []byte {
	body := make([]byte, 48+len(data))
	binary.LittleEndian.PutUint16(body[0:2], 49)
	binary.LittleEndian.PutUint16(body[2:4], 64+48)
	binary.LittleEndian.PutUint32(body[4:8], uint32(len(data)))
	binary.LittleEndian.PutUint64(body[16:24], fileID)
	binary.LittleEndian.PutUint64(body[24:32], fileID)
	copy(body[48:], data)
	return buildSMB2RequestForSmoke(0x0009, messageID, sessionID, treeID, body)
}

func buildSetInfoRenameRequestForSmoke(messageID, sessionID uint64, treeID uint32, fileID uint64, target string) []byte {
	targetBytes := encodeUTF16LEForSmoke(target)
	info := make([]byte, 20+len(targetBytes))
	binary.LittleEndian.PutUint32(info[16:20], uint32(len(targetBytes)))
	copy(info[20:], targetBytes)

	body := make([]byte, 32+len(info))
	binary.LittleEndian.PutUint16(body[0:2], 33)
	body[2] = 0x01
	body[3] = 0x0A
	binary.LittleEndian.PutUint32(body[4:8], uint32(len(info)))
	binary.LittleEndian.PutUint16(body[8:10], 64+32)
	binary.LittleEndian.PutUint64(body[16:24], fileID)
	binary.LittleEndian.PutUint64(body[24:32], fileID)
	copy(body[32:], info)
	return buildSMB2RequestForSmoke(0x0011, messageID, sessionID, treeID, body)
}

func buildSetInfoDispositionRequestForSmoke(messageID, sessionID uint64, treeID uint32, fileID uint64, deletePending bool) []byte {
	info := []byte{0}
	if deletePending {
		info[0] = 1
	}
	body := make([]byte, 32+len(info))
	binary.LittleEndian.PutUint16(body[0:2], 33)
	body[2] = 0x01
	body[3] = 0x0D
	binary.LittleEndian.PutUint32(body[4:8], uint32(len(info)))
	binary.LittleEndian.PutUint16(body[8:10], 64+32)
	binary.LittleEndian.PutUint64(body[16:24], fileID)
	binary.LittleEndian.PutUint64(body[24:32], fileID)
	copy(body[32:], info)
	return buildSMB2RequestForSmoke(0x0011, messageID, sessionID, treeID, body)
}

func encodeUTF16LEForSmoke(value string) []byte {
	runes := []rune(value)
	encoded := make([]byte, len(runes)*2)
	for i, r := range runes {
		binary.LittleEndian.PutUint16(encoded[i*2:i*2+2], uint16(r))
	}
	return encoded
}
