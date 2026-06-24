//go:build !windows

package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
)

func openInFileManager(path string, isDir bool) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", "-R", path).Start()
	case "linux":
		target := path
		if !isDir {
			target = filepath.Dir(path)
		}
		return exec.Command("xdg-open", target).Start()
	default:
		return fmt.Errorf("不支持的操作系统: %s", runtime.GOOS)
	}
}
