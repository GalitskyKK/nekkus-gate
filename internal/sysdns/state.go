package sysdns

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// FilterMode — способ работы фильтра: через системный DNS (порт 53) или через файл hosts.
const (
	ModeDNS   = "dns"
	ModeHosts = "hosts"
)

// State — сохранённое состояние перед включением фильтра (для восстановления при Выключить).
type State struct {
	// Mode — "dns" (переключили системный DNS на 127.0.0.1) или "hosts" (добавили блок-лист в hosts).
	Mode string `json:"mode,omitempty"`
	// WasDHCP — true если DNS получался по DHCP (восстанавливаем автоматически). Только для mode=dns.
	WasDHCP bool `json:"was_dhcp"`
	// Servers — сохранённые адреса DNS (если не DHCP). Только для mode=dns.
	Servers []string `json:"servers,omitempty"`
	// Adapter — идентификатор адаптера (один, для совместимости). Только для mode=dns.
	Adapter string `json:"adapter,omitempty"`
	// Adapters — список адаптеров, на которых меняли DNS (восстанавливаем все). Только для mode=dns.
	Adapters []string `json:"adapters,omitempty"`
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
