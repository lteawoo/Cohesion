package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultProductionAppDirName       = ".cohesion"
	defaultProductionConfigDirName    = "config"
	defaultProductionDataDirName      = "data"
	defaultProductionSecretsDirName   = "secrets"
	defaultProductionDatabaseFileName = "cohesion.db"
	defaultProductionDatabaseURL      = "../data/cohesion.db"
	legacyProductionDatabaseURL       = "data/cohesion.db"
)

func ResolveProductionHomeDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	homeDir = strings.TrimSpace(homeDir)
	if homeDir == "" {
		return "", errors.New("user home directory is empty")
	}
	return filepath.Join(homeDir, defaultProductionAppDirName), nil
}

func ResolveProductionConfigDir() (string, error) {
	homeDir, err := ResolveProductionHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, defaultProductionConfigDirName), nil
}

func ResolveProductionDataDir() (string, error) {
	homeDir, err := ResolveProductionHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, defaultProductionDataDirName), nil
}

func ResolveProductionSecretsDir() (string, error) {
	homeDir, err := ResolveProductionHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, defaultProductionSecretsDirName), nil
}

func ResolveProductionDatabasePath() (string, error) {
	dataDir, err := ResolveProductionDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, defaultProductionDatabaseFileName), nil
}

func DefaultProductionDatabaseURL() string {
	return defaultProductionDatabaseURL
}

func IsDefaultProductionConfigDir(path string) bool {
	configDir, err := ResolveProductionConfigDir()
	if err != nil {
		return false
	}
	return filepath.Clean(path) == filepath.Clean(configDir)
}

func IsLegacyProductionDatabaseURL(path string) bool {
	return normalizePath(path) == normalizePath(legacyProductionDatabaseURL)
}

func ExpandHomePath(path string) (string, bool) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", false
	}

	homeDir, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		return trimmed, false
	}

	switch {
	case trimmed == "~":
		return filepath.Clean(homeDir), true
	case strings.HasPrefix(trimmed, "~/"):
		suffix := strings.TrimPrefix(trimmed, "~/")
		return filepath.Clean(filepath.Join(homeDir, filepath.FromSlash(suffix))), true
	case strings.HasPrefix(trimmed, "~\\"):
		suffix := strings.TrimPrefix(trimmed, "~\\")
		suffix = strings.ReplaceAll(suffix, "\\", "/")
		return filepath.Clean(filepath.Join(homeDir, filepath.FromSlash(suffix))), true
	default:
		return trimmed, false
	}
}

func normalizePath(path string) string {
	return filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
}
