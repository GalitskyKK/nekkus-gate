//go:build !windows

package platform

import "fmt"

func IsHelperRunning() bool {
	return false
}

func HelperSetDNS(_ string) error {
	return fmt.Errorf("helper only supported on Windows")
}

func HelperRestoreDNS(_ []string, _ bool, _ []string) error {
	return fmt.Errorf("helper only supported on Windows")
}

func HelperFlushDNS() error {
	return fmt.Errorf("helper only supported on Windows")
}

func HelperWriteHosts(_ string) error {
	return fmt.Errorf("helper only supported on Windows")
}

func HelperGetDNSStatus() (string, error) {
	return "", fmt.Errorf("helper only supported on Windows")
}
