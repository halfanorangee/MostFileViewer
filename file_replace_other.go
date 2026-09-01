//go:build !windows

package main

import "os"

// replaceFileAtomically relies on rename(2), which atomically replaces an
// existing directory entry on the supported Unix-like platforms.
func replaceFileAtomically(path string, tmpName string) error {
	return os.Rename(tmpName, path)
}
