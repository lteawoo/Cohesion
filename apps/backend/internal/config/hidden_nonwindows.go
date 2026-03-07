//go:build !windows

package config

func applyHiddenAttribute(string) error {
	return nil
}
