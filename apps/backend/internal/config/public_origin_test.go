package config

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestPublicOriginValidation(t *testing.T) {
	for _, value := range []string{"http://files.example.com", "//files.example.com", "https://", "https://evil.example@files.example.com", "https://files.example.com/path", "https://files.example.com/?", "https://files.example.com/#", "https://localhost", "https://app.localhost.", "https://127.0.0.3", "https://127.1", "https://[::1]", "https://[::]", "https://files.example.com:0", "https://files.example.com:88888", "https://files.example.com%2fexample.com"} {
		if _, err := normalizePublicOrigin(value); err == nil {
			t.Errorf("accepted invalid origin %q", value)
		}
	}
	for input, want := range map[string]string{"": "", " HTTPS://FILES.Example.com:443/ ": "https://files.example.com", "https://files.example.com:8443": "https://files.example.com:8443", "https://[2001:db8::8]:443/": "https://[2001:db8::8]"} {
		got, err := normalizePublicOrigin(input)
		if got != want || err != nil {
			t.Errorf("%q: got %q, %v; want %q", input, got, err, want)
		}
	}
}

func TestPublicOriginLifecycle(t *testing.T) {
	setupConfigHandlerState(t)
	root := t.TempDir()
	t.Setenv(ProductionStateRootEnv, root)
	dir := filepath.Join(root, "config")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	configFilePath = filepath.Join(dir, "config.prod.yaml")
	handler := NewHandler()
	update := func(fields map[string]any, want int) {
		t.Helper()
		fields["server"] = Conf.Server
		body, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		code := http.StatusOK
		if e := handler.UpdateConfig(rec, req); e != nil {
			code = e.Code
		}
		if code != want {
			t.Fatalf("update got %d; want %d", code, want)
		}
	}
	persisted := func(want string) {
		t.Helper()
		reloaded, _, err := ReadExistingConfig("production")
		if err != nil || reloaded.PublicOrigin != want || PublicShareOrigin() != want {
			t.Fatalf("active %q, disk %q, err %v", PublicShareOrigin(), reloaded.PublicOrigin, err)
		}
	}
	update(map[string]any{"publicOrigin": "https://FILEs.example.com/"}, 200)
	persisted("https://files.example.com")
	update(map[string]any{}, 200)
	update(map[string]any{"publicOrigin": "https://localhost"}, 400)
	persisted("https://files.example.com")
	update(map[string]any{"publicOrigin": ""}, 200)
	persisted("")
}
