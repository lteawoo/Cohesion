package ftp

import "github.com/rs/zerolog/log"

type ftpLogger struct{}

func (l *ftpLogger) Print(sessionID string, message interface{}) {
	log.Debug().Str("session_id", sessionID).Interface("message", message).Msg("[FTP]")
}

func (l *ftpLogger) Printf(sessionID string, format string, v ...interface{}) {
	log.Debug().Str("session_id", sessionID).Msgf("[FTP] "+format, v...)
}

func (l *ftpLogger) PrintCommand(sessionID string, command string, params string) {
	if command == "PASS" {
		log.Debug().Str("session_id", sessionID).Str("command", command).Msg("[FTP] command")
		return
	}
	log.Debug().
		Str("session_id", sessionID).
		Str("command", command).
		Str("params", params).
		Msg("[FTP] command")
}

func (l *ftpLogger) PrintResponse(sessionID string, code int, message string) {
	log.Debug().
		Str("session_id", sessionID).
		Int("code", code).
		Str("message", message).
		Msg("[FTP] response")
}
