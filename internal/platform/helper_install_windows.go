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
	swHide       = 0
	swShowNormal = 1
)

var (
	shell32           = syscall.NewLazyDLL("shell32.dll")
	procShellExecuteW = shell32.NewProc("ShellExecuteW")
)

// InstallHelper запускает nekkus-gate-helper.exe --install с запросом UAC (runas).
// Ожидается, что nekkus-gate-helper.exe лежит рядом с исполняемым файлом Gate.
func InstallHelper() error {
	helperPath, err := findHelperExe()
	if err != nil {
		return err
	}
	absPath, err := filepath.Abs(helperPath)
	if err != nil {
		absPath = helperPath
	}
	dir := filepath.Dir(absPath)
	lpFile := absPath
	if containsSpace(absPath) {
		lpFile = `"` + absPath + `"`
	}
	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString(lpFile)
	params, _ := syscall.UTF16PtrFromString("--install")
	cwd, _ := syscall.UTF16PtrFromString(dir)
	// SW_SHOWNORMAL (1), не SW_HIDE — иначе на части систем UAC не показывается.
	ret, _, _ := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		uintptr(unsafe.Pointer(params)),
		uintptr(unsafe.Pointer(cwd)),
		swShowNormal,
	)
	if ret <= 32 {
		hint := shellExecuteHint(ret)
		if int32(ret) == 5 {
			hint = fmt.Sprintf("%s Установите вручную: откройте cmd от имени администратора, выполните: cd /d %q и затем nekkus-gate-helper.exe --install", hint, dir)
		}
		return fmt.Errorf("%s (код %d). %s", shellExecuteErrMessage(ret), int32(ret), hint)
	}
	return nil
}

func containsSpace(s string) bool {
	for _, r := range s {
		if r == ' ' || r == '\t' {
			return true
		}
	}
	return false
}

func shellExecuteErrMessage(ret uintptr) string {
	switch int32(ret) {
	case 0:
		return "Нехватка ресурсов"
	case 2:
		return "Файл не найден — убедитесь, что nekkus-gate-helper.exe лежит рядом с nekkus-gate.exe"
	case 5:
		return "Отказано в доступе"
	case 31:
		return "Нет ассоциации для запуска"
	default:
		return "Не удалось запустить установку Helper"
	}
}

func shellExecuteHint(ret uintptr) string {
	if int32(ret) == 5 {
		return "UAC не сработал. Если Helper уже установлен, но Gate его не видит — переустановите из cmd (от админа): cd /d \"папка_с_Gate\", затем nekkus-gate-helper.exe --install"
	}
	return "Проверьте, что nekkus-gate-helper.exe в той же папке, что и nekkus-gate.exe."
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
