//go:build !windows

package platform

// IsAdmin на неподдерживаемых ОС не проверяется — не блокируем включение.
func IsAdmin() bool {
	return true
}
