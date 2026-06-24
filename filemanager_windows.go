//go:build windows

package main

import (
	"fmt"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	shell32                        = windows.NewLazySystemDLL("shell32.dll")
	procILFindLastID               = shell32.NewProc("ILFindLastID")
	procSHParseDisplayName         = shell32.NewProc("SHParseDisplayName")
	procSHOpenFolderAndSelectItems = shell32.NewProc("SHOpenFolderAndSelectItems")
)

const rpcEChangedMode = uintptr(0x80010106)

func openInFileManager(path string, isDir bool) error {
	if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED|windows.COINIT_DISABLE_OLE1DDE); err == nil {
		defer windows.CoUninitialize()
	} else if syscall.Errno(rpcEChangedMode) != err {
		return fmt.Errorf("初始化 COM 失败: %w", err)
	}

	if isDir {
		return openFolderInFileManager(path)
	}
	return selectItemInFileManager(path)
}

func openFolderInFileManager(path string) error {
	pidl, err := parseShellPath(path)
	if err != nil {
		return err
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(pidl))

	if hr, _, _ := procSHOpenFolderAndSelectItems.Call(pidl, 0, 0, 0); failedHRESULT(hr) {
		return fmt.Errorf("打开文件管理器失败: %s", syscall.Errno(hr).Error())
	}
	return nil
}

func selectItemInFileManager(path string) error {
	itemPidl, err := parseShellPath(path)
	if err != nil {
		return err
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(itemPidl))

	parentPidl, err := parseShellPath(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(parentPidl))

	childPidl, _, _ := procILFindLastID.Call(itemPidl)
	if childPidl == 0 {
		return fmt.Errorf("解析 Shell 子项失败: PIDL 为空")
	}

	if hr, _, _ := procSHOpenFolderAndSelectItems.Call(
		parentPidl,
		1,
		uintptr(unsafe.Pointer(&childPidl)),
		0,
	); failedHRESULT(hr) {
		return fmt.Errorf("打开文件管理器失败: %s", syscall.Errno(hr).Error())
	}
	return nil
}

func parseShellPath(path string) (uintptr, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, fmt.Errorf("解析路径失败: %w", err)
	}

	var pidl uintptr
	if hr, _, _ := procSHParseDisplayName.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		0,
		uintptr(unsafe.Pointer(&pidl)),
		0,
		0,
	); failedHRESULT(hr) {
		return 0, fmt.Errorf("解析 Shell 路径失败: %s", syscall.Errno(hr).Error())
	}
	if pidl == 0 {
		return 0, fmt.Errorf("解析 Shell 路径失败: PIDL 为空")
	}
	return pidl, nil
}

func failedHRESULT(hr uintptr) bool {
	return int32(hr) < 0
}
