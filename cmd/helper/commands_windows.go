//go:build windows

package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

const createNoWindow = 0x08000000

func hideWindow(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags = createNoWindow
}

// Типичные имена адаптеров для запасного варианта (если netsh не вернул список).
var fallbackAdapterNames = []string{"Ethernet", "Wi-Fi", "WLAN", "Беспроводная сеть", "Ethernet 2", "Local Area Connection"}

func setDNS(ip string) Response {
	if net.ParseIP(ip) == nil {
		return Response{Success: false, Error: "invalid IP"}
	}
	adapters := getActiveAdapters()
	if len(adapters) == 0 {
		adapters = fallbackAdapterNames
	}
	var lastErr error
	successCount := 0
	for _, adapter := range adapters {
		cmd := exec.Command("netsh", "interface", "ipv4", "set", "dns",
			"name="+`"`+adapter+`"`, "static", ip, "primary")
		hideWindow(cmd)
		out, err := cmd.CombinedOutput()
		if err != nil {
			lastErr = fmt.Errorf("%s: %s", err, bytes.TrimSpace(out))
			continue
		}
		successCount++
	}
	if successCount == 0 {
		if lastErr != nil {
			return Response{Success: false, Error: "no connected adapters found: " + lastErr.Error()}
		}
		return Response{Success: false, Error: "no connected adapters found"}
	}
	flushDNSCmd()
	return Response{Success: true, Data: "DNS set to " + ip}
}

func restoreDNS(req Request) Response {
	adaptersStr := req.Params["adapters"]
	wasDHCP := req.Params["was_dhcp"] == "true" || req.Params["was_dhcp"] == "1"
	serversStr := req.Params["servers"]

	adapters := parseCommaList(adaptersStr)
	if len(adapters) == 0 {
		adapters = getActiveAdapters()
	}
	if len(adapters) == 0 {
		return Response{Success: false, Error: "no adapters"}
	}

	for _, adapter := range adapters {
		if wasDHCP {
			cmd := exec.Command("netsh", "interface", "ipv4", "set", "dns",
				"name="+`"`+adapter+`"`, "dhcp")
			hideWindow(cmd)
			_ = cmd.Run()
			continue
		}
		if serversStr != "" {
			servers := parseCommaList(serversStr)
			if len(servers) == 0 {
				continue
			}
			cmd := exec.Command("netsh", "interface", "ipv4", "set", "dns",
				"name="+`"`+adapter+`"`, "static", servers[0], "primary")
			hideWindow(cmd)
			if out, err := cmd.CombinedOutput(); err != nil {
				return Response{Success: false, Error: fmt.Sprintf("%s: %s", err, bytes.TrimSpace(out))}
			}
			for i := 1; i < len(servers); i++ {
				c := exec.Command("netsh", "interface", "ipv4", "add", "dns",
					"name="+`"`+adapter+`"`, servers[i], strconv.Itoa(i+1))
				hideWindow(c)
				_ = c.Run()
			}
			continue
		}
		cmd := exec.Command("netsh", "interface", "ipv4", "set", "dns",
			"name="+`"`+adapter+`"`, "dhcp")
		hideWindow(cmd)
		_ = cmd.Run()
	}
	flushDNSCmd()
	return Response{Success: true, Data: "DNS restored"}
}

func parseCommaList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func getDNSStatus() Response {
	cmd := exec.Command("netsh", "interface", "ipv4", "show", "config")
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return Response{Success: false, Error: err.Error()}
	}
	return Response{Success: true, Data: string(out)}
}

func flushDNSCmd() {
	cmd := exec.Command("ipconfig", "/flushdns")
	hideWindow(cmd)
	_ = cmd.Run()
}

func flushDNS() Response {
	flushDNSCmd()
	return Response{Success: true, Data: "DNS cache flushed"}
}

func writeHosts(content string) Response {
	const maxHostsSize = 2 * 1024 * 1024
	if len(content) > maxHostsSize {
		return Response{Success: false, Error: "hosts content too large"}
	}
	path := hostsPath()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return Response{Success: false, Error: err.Error()}
	}
	return Response{Success: true, Data: "hosts written"}
}

func hostsPath() string {
	sysRoot := os.Getenv("SystemRoot")
	if sysRoot == "" {
		sysRoot = "C:\\Windows"
	}
	return filepath.Join(sysRoot, "System32", "drivers", "etc", "hosts")
}

func getActiveAdapters() []string {
	// Сначала "ipv4 show interfaces" — предсказуемый формат (Инд Мет MTU Состояние Имя), в т.ч. русская локаль.
	names := getActiveAdaptersFromIPv4Interfaces()
	if len(names) == 0 {
		names = getActiveAdaptersFromShowInterface()
	}
	if len(names) == 0 {
		names = getActiveAdaptersFromShowConfig()
	}
	if len(names) == 0 {
		names = getActiveAdaptersFromPowerShell()
	}
	return names
}

// getActiveAdaptersFromShowInterface парсит "netsh interface show interface" (Connected/Подключено).
func getActiveAdaptersFromShowInterface() []string {
	cmd := exec.Command("netsh", "interface", "show", "interface")
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	text := strings.ReplaceAll(string(out), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	var names []string
	for _, line := range lines {
		lower := strings.ToLower(line)
		if !strings.Contains(lower, "connected") && !strings.Contains(lower, "подключено") && !strings.Contains(lower, "verbunden") {
			continue
		}
		parts := regexp.MustCompile(`\s{2,}`).Split(strings.TrimSpace(line), -1)
		if len(parts) >= 4 {
			name := strings.TrimSpace(strings.Join(parts[3:], " "))
			if name != "" && !strings.Contains(strings.ToLower(name), "loopback") {
				names = append(names, name)
			}
		} else if len(parts) >= 1 {
			// Запас: имя может быть в последнем столбце (любая строка с connected).
			name := strings.TrimSpace(parts[len(parts)-1])
			if name != "" && !strings.Contains(strings.ToLower(name), "loopback") && name != "connected" && name != "подключено" {
				names = append(names, name)
			}
		}
	}
	return names
}

// getActiveAdaptersFromShowConfig — имена из "netsh interface ipv4 show config" (Configuration for interface "...").
func getActiveAdaptersFromShowConfig() []string {
	cmd := exec.Command("netsh", "interface", "ipv4", "show", "config")
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	out = bytes.ReplaceAll(out, []byte("\r\n"), []byte("\n"))
	// Строгий шаблон и общий: любая строка с interface "Имя" или интерфейса "Имя"
	reStrict := regexp.MustCompile(`(?mi)(?:Configuration for interface|Конфигурация (?:(?:для )?интерфейса|интерфейса))\s*"([^"]+)"`)
	reLoose := regexp.MustCompile(`(?mi)(?:interface|интерфейса|interfejsu)\s*"([^"]+)"`)
	matches := reStrict.FindAllSubmatch(out, -1)
	if len(matches) == 0 {
		matches = reLoose.FindAllSubmatch(out, -1)
	}
	var names []string
	seen := make(map[string]bool)
	for _, m := range matches {
		if len(m) >= 2 {
			name := strings.TrimSpace(string(m[1]))
			lower := strings.ToLower(name)
			if name != "" && !strings.Contains(lower, "loopback") && !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	return names
}

// getActiveAdaptersFromIPv4Interfaces — "netsh interface ipv4 show interfaces".
// Формат: Инд  Мет  MTU  Состояние  Имя (EN: Idx  Met  MTU  State  Name). Имя = всё после connected/подключено.
func getActiveAdaptersFromIPv4Interfaces() []string {
	cmd := exec.Command("netsh", "interface", "ipv4", "show", "interfaces")
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	text := strings.ReplaceAll(string(out), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	var names []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		// Пропускаем заголовок и не connected.
		if strings.HasPrefix(lower, "idx") || strings.HasPrefix(lower, "инд") || strings.HasPrefix(lower, "---") {
			continue
		}
		idx := -1
		for _, state := range []string{"connected", "подключено", "verbunden"} {
			if i := strings.Index(lower, state); i >= 0 {
				idx = i + len(state)
				break
			}
		}
		if idx < 0 {
			continue
		}
		name := strings.TrimSpace(trimmed[idx:])
		if name == "" || strings.Contains(strings.ToLower(name), "loopback") {
			continue
		}
		names = append(names, name)
	}
	return names
}

// getActiveAdaptersFromPowerShell — Get-NetAdapter | Where-Object Status -eq Up.
func getActiveAdaptersFromPowerShell() []string {
	script := `Get-NetAdapter | Where-Object Status -eq 'Up' | ForEach-Object { $_.Name }`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	text := strings.ReplaceAll(string(out), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	var names []string
	for _, line := range lines {
		name := strings.TrimSpace(line)
		if name != "" && !strings.Contains(strings.ToLower(name), "loopback") {
			names = append(names, name)
		}
	}
	return names
}
