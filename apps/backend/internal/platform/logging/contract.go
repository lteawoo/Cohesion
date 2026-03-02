package logging

import (
	"strings"

	"github.com/rs/zerolog"
)

const (
	FieldTimestamp = "ts"
	FieldLevel     = "level"
	FieldEvent     = "event"
	FieldComponent = "component"
	FieldMessage   = "msg"
)

const (
	ComponentMain    = "main"
	ComponentServer  = "server"
	ComponentConfig  = "config"
	ComponentDB      = "database"
	ComponentRuntime = "runtime"
	ComponentAuth    = "auth"
	ComponentAccess  = "access"
	ComponentUpdater = "updater"
	ComponentStorage = "storage"
)

const (
	EventBootStart            = "boot.start"
	EventConfigLoaded         = "config.loaded"
	EventDatabaseReady        = "db.ready"
	EventServiceReady         = "service.ready"
	EventServerReady          = "server.ready"
	EventServerRestartRequest = "server.restart_requested"
	EventServerRestarted      = "server.restart_completed"
	EventServerShutdownSignal = "server.shutdown_signal"
	EventServerShutdownDone   = "server.shutdown_completed"
	EventHTTPAccess           = "http.access"
)

var terminalInfoEvents = map[string]struct{}{
	EventBootStart:            {},
	EventConfigLoaded:         {},
	EventDatabaseReady:        {},
	EventServiceReady:         {},
	EventServerReady:          {},
	EventServerRestartRequest: {},
	EventServerRestarted:      {},
	EventServerShutdownSignal: {},
	EventServerShutdownDone:   {},
}

func TerminalInfoEventSet() map[string]struct{} {
	copied := make(map[string]struct{}, len(terminalInfoEvents))
	for eventName := range terminalInfoEvents {
		copied[eventName] = struct{}{}
	}
	return copied
}

func Event(evt *zerolog.Event, component, eventName string) *zerolog.Event {
	return evt.
		Str(FieldComponent, normalizeComponent(component)).
		Str(FieldEvent, normalizeEvent(eventName))
}

func normalizeComponent(component string) string {
	component = strings.TrimSpace(component)
	if component == "" {
		return ComponentRuntime
	}
	return component
}

func normalizeEvent(eventName string) string {
	eventName = strings.TrimSpace(eventName)
	if eventName == "" {
		return "runtime.log"
	}
	return eventName
}
