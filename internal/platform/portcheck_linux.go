//go:build linux

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func whoBlocks53() PortStatus {
	return whoBlocks53Linux()
}

func whoBlocks53Linux() PortStatus {
	// ss -ulnp или netstat -ulnp
	out, err := exec.Command("ss", "-ulnp").Output()
	if err != nil {
		out, err = exec.Command("netstat", "-ulnp").Output()
	}
	if err != nil {
		return PortStatus{Available: false, Suggestion: "Port 53 in use. Use hosts mode or free the port."}
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, ":53 ") {
			pid := parsePIDFromSs(line)
			if pid > 0 {
				name := getProcessNameLinux(pid)
				return PortStatus{
					Available:  false,
					BlockedBy:  name,
					BlockerPID: pid,
					Suggestion:  "Port 53 used by " + name + ". Use hosts mode or stop the process.",
				}
			}
		}
	}
	return PortStatus{Available: false, Suggestion: "Port 53 in use. Use hosts mode."}
}

func parsePIDFromSs(line string) int {
	// ss: ... pid=1234 ...
	idx := strings.Index(line, "pid=")
	if idx < 0 {
		return 0
	}
	rest := line[idx+4:]
	end := 0
	for _, c := range rest {
		if c >= '0' && c <= '9' {
			end++
		} else {
			break
		}
	}
	if end == 0 {
		return 0
	}
	pid, _ := strconv.Atoi(rest[:end])
	return pid
}

func getProcessNameLinux(pid int) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(data))
}
