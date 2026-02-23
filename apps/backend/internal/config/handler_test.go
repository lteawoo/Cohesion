package config

import (
	"net/http"
	"testing"
)

func TestValidateServerConfig(t *testing.T) {
	testCases := []struct {
		name        string
		server      Server
		wantMessage string
	}{
		{
			name: "valid config without sftp",
			server: Server{
				Port:          "3000",
				WebdavEnabled: true,
				SftpEnabled:   false,
				SftpPort:      2222,
			},
		},
		{
			name: "valid config with trimmed port and sftp",
			server: Server{
				Port:          " 3000 ",
				WebdavEnabled: true,
				SftpEnabled:   true,
				SftpPort:      2222,
			},
		},
		{
			name: "missing server port",
			server: Server{
				Port:          "",
				WebdavEnabled: true,
				SftpEnabled:   false,
				SftpPort:      2222,
			},
			wantMessage: "server.port is required",
		},
		{
			name: "invalid server port format",
			server: Server{
				Port:          "abc",
				WebdavEnabled: true,
				SftpEnabled:   false,
				SftpPort:      2222,
			},
			wantMessage: "server.port must be an integer between 1 and 65535",
		},
		{
			name: "invalid server port range",
			server: Server{
				Port:          "65536",
				WebdavEnabled: true,
				SftpEnabled:   false,
				SftpPort:      2222,
			},
			wantMessage: "server.port must be an integer between 1 and 65535",
		},
		{
			name: "invalid sftp port when enabled",
			server: Server{
				Port:          "3000",
				WebdavEnabled: true,
				SftpEnabled:   true,
				SftpPort:      0,
			},
			wantMessage: "server.sftpPort must be an integer between 1 and 65535 when sftp is enabled",
		},
		{
			name: "sftp port must differ from web port",
			server: Server{
				Port:          "3000",
				WebdavEnabled: true,
				SftpEnabled:   true,
				SftpPort:      3000,
			},
			wantMessage: "server.sftpPort must be different from server.port",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateServerConfig(tc.server)
			if tc.wantMessage == "" {
				if err != nil {
					t.Fatalf("expected no error, got %+v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error %q, got nil", tc.wantMessage)
			}
			if err.Code != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d", http.StatusBadRequest, err.Code)
			}
			if err.Message != tc.wantMessage {
				t.Fatalf("expected message %q, got %q", tc.wantMessage, err.Message)
			}
		})
	}
}
