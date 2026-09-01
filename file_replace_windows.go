//go:build windows

package main

import (
	"errors"
	"os"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var replaceFileW = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReplaceFileW")

// replaceFileAtomically uses ReplaceFileW when the destination exists. Unlike
// a pair of Rename calls, ReplaceFileW performs the replacement as one OS
// operation and preserves the destination's creation time and metadata.
func replaceFileAtomically(path string, tmpName string) error {
	if _, err := os.Stat(path); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return windows.Rename(tmpName, path)
	}

	replacedName, err := windows.UTF16PtrFromString(toWindowsLongPath(path))
	if err != nil {
		return err
	}
	replacementName, err := windows.UTF16PtrFromString(toWindowsLongPath(tmpName))
	if err != nil {
		return err
	}

	result, _, callErr := replaceFileW.Call(
		uintptr(unsafe.Pointer(replacedName)),
		uintptr(unsafe.Pointer(replacementName)),
		0,
		0,
		0,
		0,
	)
	if result != 0 {
		return nil
	}
	if callErr == nil || callErr == windows.ERROR_SUCCESS {
		callErr = windows.GetLastError()
	}
	return callErr
}

func toWindowsLongPath(path string) string {
	if strings.HasPrefix(path, `\\?\`) {
		return path
	}
	if !strings.HasPrefix(path, `\\`) && len(path) < windowsPathLimit {
		return path
	}
	if strings.HasPrefix(path, `\\`) {
		return `\\?\UNC\` + strings.TrimPrefix(path, `\\`)
	}
	return `\\?\` + path
}

const windowsPathLimit = 260
