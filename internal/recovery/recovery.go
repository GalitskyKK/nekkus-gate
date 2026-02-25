package recovery

import (
	"os"
	"path/filepath"
)

const lockFileName = "gate.lock"

// CheckAndRecover вызывается при старте Gate. Если lock-файл есть — прошлый запуск
// не завершился чисто (краш/kill). Восстановление DNS делает вызывающий (sysdns.Disable).
// Возвращает true, если восстановление необходимо (lock был).
func CheckAndRecover(dataDir string) (hadLock bool, err error) {
	lockPath := filepath.Join(dataDir, lockFileName)
	if _, err := os.Stat(lockPath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	_ = os.Remove(lockPath)
	return true, nil
}

// Lock создаёт lock-файл при включении фильтра (режим DNS).
func Lock(dataDir string) error {
	lockPath := filepath.Join(dataDir, lockFileName)
	return os.WriteFile(lockPath, []byte("active"), 0644)
}

// Unlock удаляет lock при выключении фильтра.
func Unlock(dataDir string) {
	_ = os.Remove(filepath.Join(dataDir, lockFileName))
}

// HasLock сообщает, есть ли lock (для отладки/API).
func HasLock(dataDir string) bool {
	_, err := os.Stat(filepath.Join(dataDir, lockFileName))
	return err == nil
}
