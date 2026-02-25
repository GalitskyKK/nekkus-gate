package server

import (
	"log"
	"sync"

	"github.com/GalitskyKK/nekkus-gate/internal/dns"
	"github.com/GalitskyKK/nekkus-gate/internal/filter"
	"github.com/GalitskyKK/nekkus-gate/internal/querylog"
	"github.com/GalitskyKK/nekkus-gate/internal/stats"
	"github.com/GalitskyKK/nekkus-gate/internal/store"
)

// DNSRunner перезапускает DNS-сервер на другом порту (53 при включении фильтра, defaultPort при выключении).
type DNSRunner struct {
	mu     sync.Mutex
	srv    *dns.Server
	engine *filter.Engine
	st     *stats.Stats
	config *store.Config
	qlog   *querylog.Log
	port   int
}

const cacheMaxSize = 2000
const cacheTTLMinSec = 60
const cacheTTLMaxSec = 3600

// NewDNSRunner создаёт раннер DNS с движком фильтрации, статистикой, конфигом и логом запросов.
func NewDNSRunner(engine *filter.Engine, st *stats.Stats, config *store.Config, qlog *querylog.Log) *DNSRunner {
	return &DNSRunner{
		engine: engine,
		st:     st,
		config: config,
		qlog:   qlog,
	}
}

// Start запускает DNS-сервер на порту port (UDP+TCP, кэш, upstream из config).
func (r *DNSRunner) Start(port int) {
	addr, err := dns.ResolveAddr("127.0.0.1", port)
	if err != nil {
		log.Printf("Gate DNS resolve addr: %v", err)
		return
	}
	cache := dns.NewCache(cacheMaxSize, cacheTTLMinSec, cacheTTLMaxSec)
	upstream := dns.NewUpstreamResolver(r.config.GetUpstreams())
	srv := dns.NewServer(addr, cache, upstream, r.engine, r.qlog, r.st)
	r.mu.Lock()
	r.srv = srv
	r.port = port
	r.mu.Unlock()
	if err := srv.Start(); err != nil {
		log.Printf("Gate DNS server start: %v", err)
	}
}

// Restart останавливает текущий сервер и запускает на новом порту.
func (r *DNSRunner) Restart(port int) {
	r.mu.Lock()
	old := r.srv
	r.srv = nil
	r.mu.Unlock()
	if old != nil {
		_ = old.Shutdown()
	}
	r.Start(port)
}

// Shutdown останавливает DNS-сервер (при выходе из приложения).
func (r *DNSRunner) Shutdown() {
	r.mu.Lock()
	old := r.srv
	r.srv = nil
	r.mu.Unlock()
	if old != nil {
		_ = old.Shutdown()
	}
}

// Port возвращает текущий порт.
func (r *DNSRunner) Port() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.port
}

// SetUpstreams обновляет конфиг и перезапускает DNS на текущем порту (чтобы подхватить новые upstream).
func (r *DNSRunner) SetUpstreams(upstreams []string) error {
	if err := r.config.SetUpstreams(upstreams); err != nil {
		return err
	}
	p := r.Port()
	if p > 0 {
		r.Restart(p)
	}
	return nil
}

// TopBlockedEntry — одна запись для API топ заблокированных.
type TopBlockedEntry struct {
	Domain string `json:"domain"`
	Count  int    `json:"count"`
}

// GetTopBlocked возвращает топ n заблокированных доменов из query log.
func (r *DNSRunner) GetTopBlocked(n int) []TopBlockedEntry {
	if r.qlog == nil || n <= 0 {
		return nil
	}
	raw := r.qlog.TopBlocked(n)
	out := make([]TopBlockedEntry, len(raw))
	for i := range raw {
		out[i] = TopBlockedEntry{Domain: raw[i].Domain, Count: raw[i].Count}
	}
	return out
}
