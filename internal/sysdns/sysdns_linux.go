//go:build linux

package sysdns

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const resolvConf = "/etc/resolv.conf"
const resolvBackupName = "resolv.conf.gate_backup"

func prepareForEnableImpl(dataDir string) error {
	backupPath := filepath.Join(dataDir, resolvBackupName)
	data, err := os.ReadFile(resolvConf)
	if err != nil {
		return fmt.Errorf("backup resolv.conf: %w", err)
	}
	return os.WriteFile(backupPath, data, 0644)
}

func cleanupAfterDisableImpl(dataDir string) {
	_ = os.Remove(filepath.Join(dataDir, resolvBackupName))
}

func getCurrent() (*State, error) {
	// Сохраняем путь к resolv.conf (часто симлинк на /run/systemd/resolve/stub-resolv.conf).
	path, err := filepath.EvalSymlinks(resolvConf)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("resolv.conf not found")
		}
		return nil, err
	}
	servers, err := readResolvNameservers(resolvConf)
	if err != nil {
		return nil, err
	}
	// На Linux не различаем DHCP/статик по файлу — сохраняем как есть; при восстановлении просто пишем обратно.
	return &State{
		Adapter:  path,
		WasDHCP:  false,
		Servers:  servers,
		Platform: "linux",
	}, nil
}

func readResolvNameservers(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var servers []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && (fields[0] == "nameserver" || fields[0] == "nameserver\t") {
			servers = append(servers, fields[1])
		}
	}
	return servers, sc.Err()
}

func setSystemDNS(adapter string, useDHCP bool, servers []string, dataDir string) error {
	target := resolvConf
	if useDHCP {
		// Восстановить из бэкапа (созданного в prepareForEnableImpl)
		backupPath := filepath.Join(dataDir, resolvBackupName)
		data, err := os.ReadFile(backupPath)
		if err != nil {
			return fmt.Errorf("restore resolv.conf from backup: %w", err)
		}
		return os.WriteFile(target, data, 0644)
	}
	// Записать статический список (при Enable — 127.0.0.1; при Restore — сохранённые)
	var content strings.Builder
	for _, s := range servers {
		content.WriteString("nameserver ")
		content.WriteString(s)
		content.WriteString("\n")
	}
	return os.WriteFile(target, []byte(content.String()), 0644)
}
