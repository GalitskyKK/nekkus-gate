package apptrack

import (
	"net"
	"strconv"
	"sync"
	"time"
)

// Resolver по source port (из client address) возвращает имя процесса. Кэш на короткое время.
type Resolver interface {
	Lookup(clientAddr string) string
}

// CachingResolver кэширует результат Lookup по порту на cacheTTL.
type CachingResolver struct {
	impl     Resolver
	cache    map[int]cacheEntry
	cacheTTL time.Duration
	mu       sync.RWMutex
}

type cacheEntry struct {
	app string
	at  time.Time
}

// NewCachingResolver оборачивает Resolver кэшем.
func NewCachingResolver(impl Resolver, cacheTTL time.Duration) *CachingResolver {
	if cacheTTL <= 0 {
		cacheTTL = 3 * time.Second
	}
	return &CachingResolver{impl: impl, cache: make(map[int]cacheEntry), cacheTTL: cacheTTL}
}

func (c *CachingResolver) Lookup(clientAddr string) string {
	_, portStr, err := net.SplitHostPort(clientAddr)
	if err != nil {
		return ""
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		return ""
	}
	c.mu.RLock()
	if e, ok := c.cache[port]; ok && time.Since(e.at) < c.cacheTTL {
		c.mu.RUnlock()
		return e.app
	}
	c.mu.RUnlock()
	app := c.impl.Lookup(clientAddr)
	c.mu.Lock()
	c.cache[port] = cacheEntry{app: app, at: time.Now()}
	// Периодически чистить старые
	if len(c.cache) > 1000 {
		cut := time.Now().Add(-c.cacheTTL)
		for p, e := range c.cache {
			if e.at.Before(cut) {
				delete(c.cache, p)
			}
		}
	}
	c.mu.Unlock()
	return app
}

// NoopResolver всегда возвращает пустую строку (если определение процесса недоступно).
type NoopResolver struct{}

func (NoopResolver) Lookup(clientAddr string) string { return "" }
