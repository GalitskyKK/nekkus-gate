package dns

import (
	"sync"
	"time"

	"github.com/miekg/dns"
)

type cacheEntry struct {
	msg       *dns.Msg
	expiresAt time.Time
}

// Cache — in-memory DNS-кэш с TTL. Потокобезопасно.
type Cache struct {
	entries  map[string]*cacheEntry
	mu       sync.RWMutex
	maxSize  int
	ttlMin   time.Duration
	ttlMax   time.Duration
}

func cacheKey(name string, qtype uint16) string {
	return name + ":" + dns.TypeToString[qtype]
}

// NewCache создаёт кэш с ограничением по размеру и TTL.
func NewCache(maxSize int, ttlMinSec, ttlMaxSec int) *Cache {
	c := &Cache{
		entries: make(map[string]*cacheEntry),
		maxSize: maxSize,
		ttlMin:  time.Duration(ttlMinSec) * time.Second,
		ttlMax:  time.Duration(ttlMaxSec) * time.Second,
	}
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		for range ticker.C {
			c.cleanup()
		}
	}()
	return c
}

func (c *Cache) Get(name string, qtype uint16) (*dns.Msg, bool) {
	c.mu.RLock()
	entry, ok := c.entries[cacheKey(name, qtype)]
	c.mu.RUnlock()
	if !ok || entry == nil || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.msg.Copy(), true
}

func (c *Cache) Set(name string, qtype uint16, msg *dns.Msg) {
	if msg == nil || len(msg.Answer) == 0 {
		return
	}
	ttl := time.Duration(msg.Answer[0].Header().Ttl) * time.Second
	if ttl < c.ttlMin {
		ttl = c.ttlMin
	}
	if ttl > c.ttlMax {
		ttl = c.ttlMax
	}
	c.mu.Lock()
	if len(c.entries) >= c.maxSize {
		c.evictOne()
	}
	c.entries[cacheKey(name, qtype)] = &cacheEntry{
		msg:       msg.Copy(),
		expiresAt: time.Now().Add(ttl),
	}
	c.mu.Unlock()
}

func (c *Cache) cleanup() {
	c.mu.Lock()
	now := time.Now()
	for k, e := range c.entries {
		if now.After(e.expiresAt) {
			delete(c.entries, k)
		}
	}
	c.mu.Unlock()
}

func (c *Cache) evictOne() {
	var oldest string
	var oldestT time.Time
	for k, e := range c.entries {
		if oldest == "" || e.expiresAt.Before(oldestT) {
			oldest = k
			oldestT = e.expiresAt
		}
	}
	if oldest != "" {
		delete(c.entries, oldest)
	}
}

func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}
