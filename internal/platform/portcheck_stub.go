//go:build !windows && !linux && !darwin

package platform

func whoBlocks53() PortStatus {
	return PortStatus{Available: false, Suggestion: "Port 53 check not supported on this OS."}
}
