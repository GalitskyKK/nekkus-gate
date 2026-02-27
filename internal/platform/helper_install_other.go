//go:build !windows

package platform

import "fmt"

func InstallHelper() error {
	return fmt.Errorf("Nekkus Gate Helper is only available on Windows")
}

func HelperExePath() string {
	return ""
}
