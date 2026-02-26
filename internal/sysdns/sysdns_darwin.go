//go:build darwin

package sysdns

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func getCurrent() (*State, error) {
	services, err := listNetworkServices()
	if err != nil {
		return nil, err
	}
	for _, svc := range services {
		servers, err := getDNSServers(svc)
		if err != nil {
			continue
		}
		// Первый активный сервис с DNS (пустой список = DHCP)
		return &State{
			Adapter: svc,
			WasDHCP: len(servers) == 0,
			Servers: servers,
			Platform: "darwin",
		}, nil
	}
	return nil, fmt.Errorf("no network service with DNS found")
}

func listNetworkServices() ([]string, error) {
	cmd := exec.Command("networksetup", "-listallnetworkservices")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("networksetup list: %w", err)
	}
	var list []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "An asterisk") {
			continue
		}
		if strings.HasPrefix(line, "*") {
			continue // отключённый
		}
		list = append(list, line)
	}
	return list, nil
}

func getDNSServers(service string) ([]string, error) {
	cmd := exec.Command("networksetup", "-getdnsservers", service)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(string(out))
	if text == "" || strings.Contains(text, "aren't any") {
		return nil, nil
	}
	var servers []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			servers = append(servers, line)
		}
	}
	return servers, nil
}

func prepareForEnableImpl(dataDir string) error { return nil }
func cleanupAfterDisableImpl(dataDir string)    {}
func flushDNSCacheImpl()                        {}

func setSystemDNS(adapter string, useDHCP bool, servers []string, dataDir string) error {
	if adapter == "" {
		return fmt.Errorf("adapter (network service) is required")
	}
	args := []string{"-setdnsservers", adapter}
	if useDHCP {
		args = append(args, "")
	} else {
		if len(servers) == 0 {
			return fmt.Errorf("need at least one DNS server when not using DHCP")
		}
		args = append(args, servers...)
	}
	cmd := exec.Command("networksetup", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("networksetup setdnsservers: %w (%s)", err, bytes.TrimSpace(out))
	}
	return nil
}
