package main

import (
	"bytes"
	"strings"
	"testing"

	"taeu.kr/cohesion/internal/platform/logging"
)

func TestNewUpdaterLogger_MirroredWriter(t *testing.T) {
	var appBuffer bytes.Buffer
	var terminalBuffer bytes.Buffer

	logger := newUpdaterLogger(logging.NewMirroredWriter(&appBuffer, &terminalBuffer))
	logging.Event(logger.Info(), logging.ComponentUpdater, logging.EventBootStart).
		Int("pid", 1234).
		Msg("updater flow started")

	appLine := appBuffer.String()
	if !strings.Contains(appLine, "event=boot.start") {
		t.Fatalf("expected key=value event in updater file sink, got %q", appLine)
	}
	if !strings.Contains(appLine, "component=updater") {
		t.Fatalf("expected component field in updater file sink, got %q", appLine)
	}

	terminalLine := terminalBuffer.String()
	if !strings.Contains(terminalLine, "INFO [updater] boot.start - updater flow started") {
		t.Fatalf("expected terminal pattern output, got %q", terminalLine)
	}
	if strings.Contains(terminalLine, "event=boot.start") {
		t.Fatalf("expected pattern-style terminal output, got key=value output %q", terminalLine)
	}
}
