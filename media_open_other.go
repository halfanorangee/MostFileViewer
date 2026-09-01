//go:build !windows

package main

import (
	"fmt"
	"os/exec"
	"runtime"
)

// openMediaWithSystem 使用桌面环境的默认关联程序打开媒体文件。
// exec.Command 逐参数传递路径，不经 shell 解释。
func openMediaWithSystem(path string) error {
	var command string
	switch runtime.GOOS {
	case "darwin":
		command = "open"
	default:
		command = "xdg-open"
	}
	if err := exec.Command(command, path).Start(); err != nil {
		return fmt.Errorf("启动 %s 失败: %w", command, err)
	}
	return nil
}
