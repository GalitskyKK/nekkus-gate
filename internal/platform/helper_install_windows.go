//go:build windows

package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

const (
	swHide = 0
)

var (
	shell32           = syscall.NewLazyDLL("shell32.dll")
	procShellExecuteW = shell32.NewProc("ShellExecuteW")
)

// InstallHelper запускает nekkus-gate-helper.exe --install с запросом UAC (runas).
// Ожидается, что nekkus-gate-helper.exe лежит рядом с исполняемым файлом Gate
// или в текущей директории. Возвращает ошибку при неудачном запуске (не ждёт завершения установки).
func InstallHelper() error {
	helperPath, err := findHelperExe()
	if err != nil {
		return err
	}
	dir := filepath.Dir(helperPath)
	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString(helperPath)
	params, _ := syscall.UTF16PtrFromString("--install")
	cwd, _ := syscall.UTF16PtrFromString(dir)
	ret, _, _ := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		uintptr(unsafe.Pointer(params)),
		uintptr(unsafe.Pointer(cwd)),
		swHide,
	)
	// ShellExecute returns value > 32 on success
	if ret <= 32 {
		return fmt.Errorf("ShellExecute failed: code %d", ret)
	}
	return nil
}

// HelperExePath возвращает путь к nekkus-gate-helper.exe для отображения в UI.
func HelperExePath() string {
	path, _ := findHelperExe()
	return path
}

func findHelperExe() (string, error) {
	// Рядом с текущим exe (Gate или тест)
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(exe)
	for _, name := range []string{"nekkus-gate-helper.exe", "helper.exe"} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("nekkus-gate-helper.exe not found next to %s", exe)
}
