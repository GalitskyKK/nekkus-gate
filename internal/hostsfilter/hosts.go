package hostsfilter

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	hostsBackupName = "hosts_backup"
	gateMarker      = "# Nekkus Gate blocklist"
)

// Path возвращает путь к системному файлу hosts.
func Path() string {
	return hostsPath()
}

// HostsBackupName возвращает имя файла бэкапа hosts в dataDir.
func HostsBackupName() string {
	return hostsBackupName
}

// BuildHostsContent формирует содержимое hosts: currentData + блокировка domains (0.0.0.0 domain).
func BuildHostsContent(currentData []byte, domains []string) string {
	var sb strings.Builder
	sb.Write(currentData)
	if len(currentData) > 0 && currentData[len(currentData)-1] != '\n' {
		sb.WriteByte('\n')
	}
	sb.WriteString("\n")
	sb.WriteString(gateMarker)
	sb.WriteString("\n")
	written := make(map[string]struct{})
	for _, d := range domains {
		d = strings.TrimSpace(strings.ToLower(d))
		if d == "" {
			continue
		}
		sb.WriteString("0.0.0.0 ")
		sb.WriteString(d)
		sb.WriteString("\n")
		written[d] = struct{}{}
		if !strings.HasPrefix(d, "www.") {
			www := "www." + d
			if _, ok := written[www]; !ok {
				sb.WriteString("0.0.0.0 ")
				sb.WriteString(www)
				sb.WriteString("\n")
				written[www] = struct{}{}
			}
		}
	}
	return sb.String()
}

func hostsPath() string {
	if runtime.GOOS == "windows" {
		sysRoot := os.Getenv("SystemRoot")
		if sysRoot == "" {
			sysRoot = "C:\\Windows"
		}
		return filepath.Join(sysRoot, "System32", "drivers", "etc", "hosts")
	}
	return "/etc/hosts"
}

// Enable сохраняет текущий hosts в dataDir, дописывает в конец блокировку доменов (0.0.0.0 domain).
// Требуются права администратора.
func Enable(dataDir string, domains []string) error {
	path := hostsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read hosts: %w", err)
	}
	backupPath := filepath.Join(dataDir, hostsBackupName)
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return fmt.Errorf("backup hosts: %w", err)
	}
	var sb strings.Builder
	sb.Write(data)
	if len(data) > 0 && data[len(data)-1] != '\n' {
		sb.WriteByte('\n')
	}
	sb.WriteString("\n")
	sb.WriteString(gateMarker)
	sb.WriteString("\n")
	written := make(map[string]struct{})
	for _, d := range domains {
		d = strings.TrimSpace(strings.ToLower(d))
		if d == "" {
			continue
		}
		// Точное совпадение: doubleclick.net
		sb.WriteString("0.0.0.0 ")
		sb.WriteString(d)
		sb.WriteString("\n")
		written[d] = struct{}{}
		// www-вариант: иначе www.doubleclick.net не блокируется (hosts не поддерживает поддомены)
		if !strings.HasPrefix(d, "www.") {
			www := "www." + d
			if _, ok := written[www]; !ok {
				sb.WriteString("0.0.0.0 ")
				sb.WriteString(www)
				sb.WriteString("\n")
				written[www] = struct{}{}
			}
		}
	}
	return os.WriteFile(path, []byte(sb.String()), 0644)
}

// Disable восстанавливает hosts из бэкапа в dataDir и удаляет файл бэкапа.
func Disable(dataDir string) error {
	backupPath := filepath.Join(dataDir, hostsBackupName)
	data, err := os.ReadFile(backupPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read hosts backup: %w", err)
	}
	path := hostsPath()
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("restore hosts: %w", err)
	}
	return os.Remove(backupPath)
}

// HasBackup сообщает, есть ли сохранённая копия hosts (фильтр был включён в режиме hosts).
func HasBackup(dataDir string) bool {
	_, err := os.Stat(filepath.Join(dataDir, hostsBackupName))
	return err == nil
}

// RemoveStaleBackup удаляет бэкап без восстановления (например, при несовпадении режима).
func RemoveStaleBackup(dataDir string) error {
	return os.Remove(filepath.Join(dataDir, hostsBackupName))
}
