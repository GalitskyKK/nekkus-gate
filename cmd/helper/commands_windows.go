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

func setDNS(ip string) Response {
	if net.ParseIP(ip) == nil {
		return Response{Success: false, Error: "invalid IP"}
	}
	adapters := getActiveAdapters()
	if len(adapters) == 0 {
		return Response{Success: false, Error: "no connected adapters found"}
	}
	for _, adapter := range adapters {
		cmd := exec.Command("netsh", "interface", "ipv4", "set", "dns",
			"name="+`"`+adapter+`"`, "static", ip, "primary")
		hideWindow(cmd)
		if out, err := cmd.CombinedOutput(); err != nil {
			return Response{Success: false, Error: fmt.Sprintf("%s: %s", err, bytes.TrimSpace(out))}
		}
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
	names := getActiveAdaptersFromShowInterface()
	if len(names) == 0 {
		names = getActiveAdaptersFromShowConfig()
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
		}
	}
	return names
}

// getActiveAdaptersFromShowConfig — запасной вариант: имена из "netsh interface ipv4 show config" (Configuration for interface "...").
func getActiveAdaptersFromShowConfig() []string {
	cmd := exec.Command("netsh", "interface", "ipv4", "show", "config")
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	out = bytes.ReplaceAll(out, []byte("\r\n"), []byte("\n"))
	// Configuration for interface "Ethernet" / Конфигурация интерфейса "Ethernet"
	re := regexp.MustCompile(`(?m)^(?:Configuration for interface|Конфигурация (?:(?:для )?интерфейса|интерфейса))\s*"([^"]+)"`)
	matches := re.FindAllSubmatch(out, -1)
	var names []string
	seen := make(map[string]bool)
	for _, m := range matches {
		if len(m) >= 2 {
			name := string(m[2])
			name = strings.TrimSpace(name)
			lower := strings.ToLower(name)
			if name != "" && !strings.Contains(lower, "loopback") && !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	return names
}
