//go:build windows

package platform

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	// TokenElevationType — тип запроса для GetTokenInformation.
	tokenElevationType = 18
	// TokenElevationTypeFull — процесс запущен с повышенными правами (UAC).
	tokenElevationTypeFull = 2
)

var (
	advapi32    = windows.NewLazySystemDLL("advapi32.dll")
	procOpenPT  = advapi32.NewProc("OpenProcessToken")
	procGetTI   = advapi32.NewProc("GetTokenInformation")
	kernel32    = windows.NewLazySystemDLL("kernel32.dll")
	procGetCurr = kernel32.NewProc("GetCurrentProcess")
)

const tokenQuery = 0x0008

// IsAdmin возвращает true, если текущий процесс выполняется с правами администратора (UAC elevated).
func IsAdmin() bool {
	proc, _, _ := procGetCurr.Call()
	if proc == 0 {
		return false
	}
	var token windows.Handle
	r, _, _ := procOpenPT.Call(proc, uintptr(tokenQuery), uintptr(unsafe.Pointer(&token)))
	if r == 0 || token == 0 {
		return false
	}
	defer windows.CloseHandle(token)
	var elevType uint32
	var retLen uint32
	r, _, _ = procGetTI.Call(
		uintptr(token),
		uintptr(tokenElevationType),
		uintptr(unsafe.Pointer(&elevType)),
		uintptr(unsafe.Sizeof(elevType)),
		uintptr(unsafe.Pointer(&retLen)),
	)
	if r == 0 {
		return false
	}
	return elevType == tokenElevationTypeFull
}
