package sysdns

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// State — сохранённое состояние DNS перед переключением на Gate (для восстановления).
type State struct {
	// WasDHCP — true если DNS получался по DHCP (восстанавливаем автоматически).
	WasDHCP bool `json:"was_dhcp"`
	// Servers — сохранённые адреса DNS (если не DHCP). Восстанавливаем как статический список.
	Servers []string `json:"servers,omitempty"`
	// Adapter — идентификатор адаптера/интерфейса (платформо-специфичный).
	Adapter string `json:"adapter,omitempty"`
	// Platform — windows, darwin, linux (для отладки).
	Platform string `json:"platform,omitempty"`
}

const stateFileName = "dns_filter_backup.json"

// SaveToFile сохраняет состояние в dataDir для последующего Restore.
func SaveToFile(dataDir string, s *State) error {
	path := filepath.Join(dataDir, stateFileName)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// LoadFromFile загружает сохранённое состояние. Если файла нет — nil, nil.
func LoadFromFile(dataDir string) (*State, error) {
	path := filepath.Join(dataDir, stateFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// RemoveBackup удаляет файл бэкапа (после успешного восстановления).
func RemoveBackup(dataDir string) error {
	path := filepath.Join(dataDir, stateFileName)
	return os.Remove(path)
}

// HasBackup сообщает, есть ли сохранённое состояние (фильтр был включён и не восстановлен).
func HasBackup(dataDir string) bool {
	path := filepath.Join(dataDir, stateFileName)
	_, err := os.Stat(path)
	return err == nil
}
