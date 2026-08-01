//go:build windows

package remote

import (
	"syscall"
	"unsafe"
)

var (
	modKernel32                     = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess                 = modKernel32.NewProc("OpenProcess")
	procGetExitCodeProcess          = modKernel32.NewProc("GetExitCodeProcess")
	procCloseHandle                 = modKernel32.NewProc("CloseHandle")
	processQueryLimitedInfo uintptr = 0x1000
	stillActive             uint32  = 259
)

func windowsProcessAlive(pid uint32) bool {
	if pid == 0 {
		return false
	}
	handle, _, _ := procOpenProcess.Call(processQueryLimitedInfo, 0, uintptr(pid))
	if handle == 0 {
		return false
	}
	defer procCloseHandle.Call(handle)
	var code uint32
	r, _, _ := procGetExitCodeProcess.Call(handle, uintptr(unsafe.Pointer(&code)))
	if r == 0 {
		return false
	}
	return code == stillActive
}
