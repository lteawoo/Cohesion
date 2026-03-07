//go:build windows

package config

import "syscall"

func applyHiddenAttribute(path string) error {
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}

	attrs, err := syscall.GetFileAttributes(pathPtr)
	if err != nil {
		return err
	}

	return syscall.SetFileAttributes(pathPtr, attrs|syscall.FILE_ATTRIBUTE_HIDDEN)
}
