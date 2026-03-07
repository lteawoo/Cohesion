package system

import "strings"

const (
	InstallChannelEnv      = "COHESION_INSTALL_CHANNEL"
	InstallChannelDirect   = "direct"
	InstallChannelHomebrew = "homebrew"
	InstallChannelSystemd  = "systemd"
)

func ParseInstallChannel(raw string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case InstallChannelDirect:
		return InstallChannelDirect, true
	case InstallChannelHomebrew:
		return InstallChannelHomebrew, true
	case InstallChannelSystemd:
		return InstallChannelSystemd, true
	default:
		return "", false
	}
}

func NormalizeInstallChannel(raw string) string {
	if channel, ok := ParseInstallChannel(raw); ok {
		return channel
	}
	return InstallChannelDirect
}
