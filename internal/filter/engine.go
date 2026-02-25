package filter

import (
	"strings"
	"sync"

	"github.com/GalitskyKK/nekkus-gate/internal/blocklist"
	"github.com/GalitskyKK/nekkus-gate/internal/store"
)

// Engine объединяет блок-лист, whitelist и кастомные правила (block/allow).
type Engine struct {
	bl     *blocklist.Blocklist
	config *store.Config
	mu     sync.RWMutex
}

// New создаёт движок фильтрации.
func New(bl *blocklist.Blocklist, config *store.Config) *Engine {
	return &Engine{bl: bl, config: config}
}

// Check возвращает true, если домен нужно заблокировать, и строку правила (какое сработало).
// Сначала проверяются allow-правила, затем блок-лист и block-правила.
func (e *Engine) Check(domain string) (blocked bool, rule string) {
	domain = strings.TrimSuffix(strings.ToLower(domain), ".")
	if domain == "" {
		return false, ""
	}

	rules := e.config.GetRules()
	for _, r := range rules {
		if !matchDomain(r.Domain, domain) {
			continue
		}
		if r.Action == "allow" {
			return false, ""
		}
		return true, "rule:" + r.Domain
	}

	if e.bl.Blocked(domain) {
		return true, "blocklist"
	}
	return false, ""
}

// matchDomain: *.*.example.com → sub.example.com; example.com → example.com и *.example.com
func matchDomain(pattern, domain string) bool {
	pattern = strings.ToLower(pattern)
	if pattern == domain {
		return true
	}
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:]
		return domain == suffix || strings.HasSuffix(domain, suffix)
	}
	return strings.HasSuffix(domain, "."+pattern)
}
