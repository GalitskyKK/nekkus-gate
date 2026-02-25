//go:build !windows && !darwin && !linux

package sysdns

import "fmt"

var errUnsupported = fmt.Errorf("system DNS control is not supported on this OS")

func getCurrent() (*State, error) {
	return nil, errUnsupported
}

func prepareForEnableImpl(dataDir string) error { return errUnsupported }
func cleanupAfterDisableImpl(dataDir string)   {}

func setSystemDNS(adapter string, useDHCP bool, servers []string, dataDir string) error {
	return errUnsupported
}
