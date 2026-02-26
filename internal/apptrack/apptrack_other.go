//go:build !windows

package apptrack

// NewResolver на неподдерживаемых ОС возвращает noop (имя приложения не определяется).
func NewResolver() Resolver {
	return NoopResolver{}
}
