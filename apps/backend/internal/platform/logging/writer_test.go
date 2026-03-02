package logging

import (
	"bytes"
	"testing"
)

func TestBuildTerminalPatternLine_OrderAndExtras(t *testing.T) {
	record := map[string]any{
		FieldTimestamp: "2026-03-03T01:30:21.123Z",
		FieldLevel:     "info",
		FieldComponent: "server",
		FieldEvent:     "server.ready",
		FieldMessage:   "server ready",
		"port":         "3000",
		"service":      "http",
	}

	got := string(buildTerminalPatternLine(record))
	want := "2026-03-03T01:30:21.123Z INFO [server] server.ready - server ready port=3000 service=http\n"
	if got != want {
		t.Fatalf("unexpected terminal pattern line\nwant: %q\ngot:  %q", want, got)
	}
}

func TestBuildTerminalPatternLine_MissingFields(t *testing.T) {
	record := map[string]any{
		FieldTimestamp: "2026-03-03T01:30:21.123Z",
		FieldLevel:     "warn",
	}

	got := string(buildTerminalPatternLine(record))
	want := "2026-03-03T01:30:21.123Z WARN [runtime] runtime.log - -\n"
	if got != want {
		t.Fatalf("unexpected missing-field rendering\nwant: %q\ngot:  %q", want, got)
	}
}

func TestBuildTerminalPatternLine_DeterministicExtraFieldOrder(t *testing.T) {
	record := map[string]any{
		FieldTimestamp: "2026-03-03T01:30:21.123Z",
		FieldLevel:     "error",
		FieldComponent: "storage",
		FieldEvent:     "warn.storage.cleanup_failed",
		FieldMessage:   "cleanup failed",
		"z":            "last",
		"a":            "first",
		"m":            "middle",
	}

	first := string(buildTerminalPatternLine(record))
	for i := 0; i < 10; i++ {
		got := string(buildTerminalPatternLine(record))
		if got != first {
			t.Fatalf("expected deterministic output; run %d mismatch\nfirst: %q\ngot:   %q", i, first, got)
		}
	}

	wantSuffix := " a=first m=middle z=last\n"
	if len(first) < len(wantSuffix) || first[len(first)-len(wantSuffix):] != wantSuffix {
		t.Fatalf("unexpected extra-field order in %q", first)
	}
}

func TestNewOperationalWriter_FileAndTerminalSplit(t *testing.T) {
	var appBuf bytes.Buffer
	var terminalBuf bytes.Buffer

	writer := NewOperationalWriter(&appBuf, &terminalBuf)

	input := []byte(`{"ts":"2026-03-03T01:30:21.123Z","level":"info","event":"server.ready","component":"server","msg":"server ready","port":"3000"}`)
	if _, err := writer.Write(input); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	appLine := appBuf.String()
	terminalLine := terminalBuf.String()

	if appLine == "" || terminalLine == "" {
		t.Fatalf("expected both app and terminal lines, got app=%q terminal=%q", appLine, terminalLine)
	}
	if appLine == terminalLine {
		t.Fatalf("expected file and terminal lines to differ, both=%q", appLine)
	}
	if !bytes.Contains([]byte(appLine), []byte("event=server.ready")) {
		t.Fatalf("expected key=value file line, got %q", appLine)
	}
	if !bytes.Contains([]byte(terminalLine), []byte("[server] server.ready - server ready")) {
		t.Fatalf("expected pattern terminal line, got %q", terminalLine)
	}
}

func TestNewOperationalWriter_TerminalInfoWhitelist(t *testing.T) {
	var appBuf bytes.Buffer
	var terminalBuf bytes.Buffer

	writer := NewOperationalWriter(&appBuf, &terminalBuf)
	input := []byte(`{"ts":"2026-03-03T01:30:21.123Z","level":"info","event":"custom.info","component":"main","msg":"custom info"}`)
	if _, err := writer.Write(input); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	if appBuf.Len() == 0 {
		t.Fatal("expected app log line")
	}
	if terminalBuf.Len() != 0 {
		t.Fatalf("expected no terminal info line for non-whitelisted event, got %q", terminalBuf.String())
	}
}

func TestNewMirroredWriter_MirrorsInfoToTerminal(t *testing.T) {
	var appBuf bytes.Buffer
	var terminalBuf bytes.Buffer

	writer := NewMirroredWriter(&appBuf, &terminalBuf)
	input := []byte(`{"ts":"2026-03-03T01:30:21.123Z","level":"info","event":"updater.completed","component":"updater","msg":"done"}`)
	if _, err := writer.Write(input); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	if appBuf.Len() == 0 || terminalBuf.Len() == 0 {
		t.Fatalf("expected mirrored output, got app=%q terminal=%q", appBuf.String(), terminalBuf.String())
	}
}
