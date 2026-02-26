//go:build windows

package apptrack

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// WindowsResolver по UDP netstat находит PID по локальному порту клиента, затем имя процесса через tasklist.
type WindowsResolver struct{}

func NewWindowsResolver() *WindowsResolver {
	return &WindowsResolver{}
}

func (w *WindowsResolver) Lookup(clientAddr string) string {
	_, portStr, err := net.SplitHostPort(clientAddr)
	if err != nil {
		return ""
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		return ""
	}
	pid := findUDPClientPID(port)
	if pid <= 0 {
		return ""
	}
	return getProcessNameWindows(pid)
}

// findUDPClientPID ищет в netstat -ano -p UDP строку с локальным портом port (клиент, обращающийся к DNS).
func findUDPClientPID(port int) int {
	cmd := exec.Command("netstat", "-ano", "-p", "UDP")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	suffix := ":" + strconv.Itoa(port)
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if !strings.Contains(line, "UDP") {
			continue
		}
		// Локальный адрес заканчивается на :port (127.0.0.1:54321 или [::1]:54321)
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		localAddr := fields[1]
		if !strings.HasSuffix(localAddr, suffix) {
			continue
		}
		pidStr := fields[len(fields)-1]
		pid, err := strconv.Atoi(pidStr)
		if err != nil || pid <= 0 {
			continue
		}
		return pid
	}
	return 0
}

// NewResolver на Windows возвращает кэширующий резолвер по netstat + tasklist.
func NewResolver() Resolver {
	return NewCachingResolver(NewWindowsResolver(), 3*time.Second)
}

func getProcessNameWindows(pid int) string {
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	s := strings.TrimSpace(string(out))
	if len(s) >= 2 && s[0] == '"' {
		end := strings.Index(s[1:], `"`)
		if end >= 0 {
			return s[1 : end+1]
		}
	}
	return ""
}
