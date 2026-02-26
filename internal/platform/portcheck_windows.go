//go:build windows

package platform

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

func whoBlocks53() PortStatus {
	return whoBlocks53Windows()
}

func whoBlocks53Windows() PortStatus {
	cmd := exec.Command("netstat", "-ano", "-p", "UDP")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return PortStatus{Available: false, Suggestion: "Port 53 in use. Run as admin or use hosts mode."}
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, ":53 ") || strings.Contains(line, ":53\t") {
			fields := strings.Fields(line)
			if len(fields) < 4 {
				continue
			}
			pidStr := fields[len(fields)-1]
			pid, err := strconv.Atoi(pidStr)
			if err != nil || pid <= 0 {
				continue
			}
			name := getProcessNameWindows(pid)
			return PortStatus{
				Available:  false,
				BlockedBy:  name,
				BlockerPID: pid,
				Suggestion: fmt.Sprintf("Port 53 used by %s (PID %d). Often WSL2 or Hyper-V. Use hosts mode or stop the process.", name, pid),
			}
		}
	}
	return PortStatus{Available: false, Suggestion: "Port 53 in use. Use hosts mode or run as admin."}
}

func getProcessNameWindows(pid int) string {
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	s := strings.TrimSpace(string(out))
	if len(s) >= 2 && s[0] == '"' {
		end := strings.Index(s[1:], `"`)
		if end >= 0 {
			return s[1 : end+1]
		}
	}
	return "unknown"
}
