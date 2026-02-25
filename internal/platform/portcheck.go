package platform

import (
	"net"
)

// PortStatus — результат проверки порта 53.
type PortStatus struct {
	Available  bool   `json:"available"`
	BlockedBy  string `json:"blocked_by,omitempty"`
	BlockerPID int    `json:"blocker_pid,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
}

// whoBlocks53 реализован в portcheck_windows.go, portcheck_linux.go, portcheck_darwin.go, portcheck_stub.go.

// CheckPort53 проверяет, свободен ли 127.0.0.1:53. Если занят — возвращает кто занял (PID, имя процесса).
func CheckPort53() PortStatus {
	pc, err := net.ListenPacket("udp", "127.0.0.1:53")
	if err == nil {
		_ = pc.Close()
		return PortStatus{Available: true}
	}
	return whoBlocks53()
}
