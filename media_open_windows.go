//go:build windows

package main

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var shellExecuteW = windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteW")

// openMediaWithSystem 使用文件关联的默认程序打开媒体文件。
// 只传经校验的文件路径，不经 shell 拼接命令；ShellExecuteW 返回值 <= 32 表示失败。
func openMediaWithSystem(path string) error {
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}

	const swShowNormal = 1
	result, _, callErr := shellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		0,
		0,
		swShowNormal,
	)
	if result > 32 {
		return nil
	}
	if callErr == nil || callErr == windows.ERROR_SUCCESS {
		callErr = windows.GetLastError()
	}
	return fmt.Errorf("ShellExecuteW 失败: %w", callErr)
}
