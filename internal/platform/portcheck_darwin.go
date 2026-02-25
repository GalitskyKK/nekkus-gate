//go:build darwin

package platform

import (
	"os/exec"
	"strconv"
	"strings"
)

func whoBlocks53() PortStatus {
	return whoBlocks53Darwin()
}

func whoBlocks53Darwin() PortStatus {
	out, err := exec.Command("lsof", "-i", "UDP:53", "-n", "-P").Output()
	if err != nil {
		return PortStatus{Available: false, Suggestion: "Port 53 in use. Use hosts mode or free the port."}
	}
	lines := strings.Split(string(out), "\n")
	if len(lines) < 2 {
		return PortStatus{Available: false, Suggestion: "Port 53 in use."}
	}
	// First data line: COMMAND PID USER ...
	fields := strings.Fields(lines[1])
	if len(fields) >= 2 {
		if pid, err := strconv.Atoi(fields[1]); err == nil {
			name := fields[0]
			return PortStatus{
				Available:  false,
				BlockedBy:  name,
				BlockerPID: pid,
				Suggestion: "Port 53 used by " + name + ". Use hosts mode or stop the process.",
			}
		}
	}
	return PortStatus{Available: false, Suggestion: "Port 53 in use."}
}
