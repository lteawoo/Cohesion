package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type KeyValueWriter struct {
	appWriter          io.Writer
	terminalWriter     io.Writer
	terminalInfoEvents map[string]struct{}
	mirrorAllLevels    bool
	mu                 sync.Mutex
}

func NewOperationalWriter(appWriter, terminalWriter io.Writer) io.Writer {
	return &KeyValueWriter{
		appWriter:          appWriter,
		terminalWriter:     terminalWriter,
		terminalInfoEvents: TerminalInfoEventSet(),
	}
}

func NewMirroredWriter(appWriter, terminalWriter io.Writer) io.Writer {
	return &KeyValueWriter{
		appWriter:       appWriter,
		terminalWriter:  terminalWriter,
		mirrorAllLevels: true,
	}
}

func NewKeyValueWriter(out io.Writer) io.Writer {
	return &KeyValueWriter{
		appWriter: out,
	}
}

func (w *KeyValueWriter) Write(p []byte) (int, error) {
	trimmed := bytes.TrimSpace(p)
	if len(trimmed) == 0 {
		return len(p), nil
	}

	record := make(map[string]any)
	if err := json.Unmarshal(trimmed, &record); err != nil {
		return w.writeRaw(trimmed, len(p))
	}

	line, level, eventName := buildKeyValueLine(record)
	if len(line) == 0 {
		return w.writeRaw(trimmed, len(p))
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.appWriter.Write(line); err != nil {
		return 0, err
	}

	if w.shouldEmitTerminal(level, eventName) {
		patternLine := buildTerminalPatternLine(record)
		if len(patternLine) == 0 {
			patternLine = line
		}
		if _, err := w.terminalWriter.Write(patternLine); err != nil {
			return 0, err
		}
	}

	return len(p), nil
}

func (w *KeyValueWriter) writeRaw(trimmed []byte, originalLength int) (int, error) {
	line := append([]byte{}, trimmed...)
	if len(line) == 0 || line[len(line)-1] != '\n' {
		line = append(line, '\n')
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.appWriter.Write(line); err != nil {
		return 0, err
	}
	if w.terminalWriter != nil {
		if _, err := w.terminalWriter.Write(line); err != nil {
			return 0, err
		}
	}

	return originalLength, nil
}

func (w *KeyValueWriter) shouldEmitTerminal(level, eventName string) bool {
	if w.terminalWriter == nil {
		return false
	}

	if w.mirrorAllLevels {
		return true
	}

	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "WARN", "ERROR", "FATAL", "PANIC":
		return true
	case "INFO":
		_, ok := w.terminalInfoEvents[eventName]
		return ok
	default:
		return false
	}
}

func buildTerminalPatternLine(record map[string]any) []byte {
	ts := strings.TrimSpace(valueAsString(firstNonNil(record, FieldTimestamp, "time")))
	if ts == "" {
		ts = time.Now().UTC().Format(time.RFC3339)
	}

	level := strings.ToUpper(strings.TrimSpace(valueAsString(firstNonNil(record, FieldLevel))))
	if level == "" {
		level = "INFO"
	}

	eventName := normalizeEvent(valueAsString(firstNonNil(record, FieldEvent)))
	component := normalizeComponent(valueAsString(firstNonNil(record, FieldComponent)))
	message := strings.TrimSpace(valueAsString(firstNonNil(record, FieldMessage, "message")))
	if message == "" {
		message = "-"
	}

	excluded := map[string]struct{}{
		FieldTimestamp: {},
		"time":         {},
		FieldLevel:     {},
		FieldEvent:     {},
		FieldComponent: {},
		FieldMessage:   {},
		"message":      {},
	}

	extraKeys := make([]string, 0, len(record))
	for key := range record {
		if _, ok := excluded[key]; ok {
			continue
		}
		extraKeys = append(extraKeys, key)
	}
	sort.Strings(extraKeys)

	extras := make([]string, 0, len(extraKeys))
	for _, key := range extraKeys {
		extras = append(extras, formatPair(key, valueAsString(record[key])))
	}

	line := fmt.Sprintf("%s %s [%s] %s - %s", ts, level, component, eventName, message)
	if len(extras) > 0 {
		line += " " + strings.Join(extras, " ")
	}
	return []byte(line + "\n")
}

func buildKeyValueLine(record map[string]any) ([]byte, string, string) {
	ts := strings.TrimSpace(valueAsString(firstNonNil(record, FieldTimestamp, "time")))
	if ts == "" {
		ts = time.Now().UTC().Format(time.RFC3339)
	}

	level := strings.ToUpper(strings.TrimSpace(valueAsString(firstNonNil(record, FieldLevel))))
	if level == "" {
		level = "INFO"
	}

	eventName := normalizeEvent(valueAsString(firstNonNil(record, FieldEvent)))
	component := normalizeComponent(valueAsString(firstNonNil(record, FieldComponent)))
	message := strings.TrimSpace(valueAsString(firstNonNil(record, FieldMessage, "message")))

	pairs := make([]string, 0, len(record)+5)
	pairs = append(pairs, formatPair(FieldTimestamp, ts))
	pairs = append(pairs, formatPair(FieldLevel, level))
	pairs = append(pairs, formatPair(FieldEvent, eventName))
	pairs = append(pairs, formatPair(FieldComponent, component))
	pairs = append(pairs, formatPair(FieldMessage, message))

	excluded := map[string]struct{}{
		FieldTimestamp: {},
		"time":         {},
		FieldLevel:     {},
		FieldEvent:     {},
		FieldComponent: {},
		FieldMessage:   {},
		"message":      {},
	}

	extraKeys := make([]string, 0, len(record))
	for key := range record {
		if _, ok := excluded[key]; ok {
			continue
		}
		extraKeys = append(extraKeys, key)
	}
	sort.Strings(extraKeys)

	for _, key := range extraKeys {
		pairs = append(pairs, formatPair(key, valueAsString(record[key])))
	}

	if len(pairs) == 0 {
		return nil, level, eventName
	}

	return []byte(strings.Join(pairs, " ") + "\n"), level, eventName
}

func firstNonNil(record map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := record[key]; ok && value != nil {
			return value
		}
	}
	return nil
}

func valueAsString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case []byte:
		return string(typed)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case json.Number:
		return typed.String()
	default:
		switch number := typed.(type) {
		case float64:
			return strconv.FormatFloat(number, 'f', -1, 64)
		case float32:
			return strconv.FormatFloat(float64(number), 'f', -1, 32)
		case int, int8, int16, int32, int64:
			return fmt.Sprintf("%d", number)
		case uint, uint8, uint16, uint32, uint64:
			return fmt.Sprintf("%d", number)
		}
		encoded, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		return string(encoded)
	}
}

func formatPair(key, value string) string {
	if value == "" {
		return key + "=\"\""
	}
	if needsQuotes(value) {
		return key + "=" + strconv.Quote(value)
	}
	return key + "=" + value
}

func needsQuotes(value string) bool {
	for _, r := range value {
		if r == '"' || r == '=' || r == '\t' || r == '\n' || r == '\r' || r == ' ' {
			return true
		}
	}
	return false
}
