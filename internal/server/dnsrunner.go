package server

import (
	"context"
	"log"
	"sync"

	"github.com/GalitskyKK/nekkus-gate/internal/blocklist"
	"github.com/GalitskyKK/nekkus-gate/internal/dns"
	"github.com/GalitskyKK/nekkus-gate/internal/stats"
)

// DNSRunner перезапускает DNS-сервер на другом порту (53 при включении фильтра, 5354 при выключении).
type DNSRunner struct {
	mu      sync.Mutex
	srv     *dns.Server
	bl      *blocklist.Blocklist
	st      *stats.Stats
	upstream string
	port    int
}

func NewDNSRunner(bl *blocklist.Blocklist, st *stats.Stats, upstream string) *DNSRunner {
	if upstream == "" {
		upstream = "8.8.8.8:53"
	}
	return &DNSRunner{bl: bl, st: st, upstream: upstream}
}

// Start запускает DNS-сервер на порту port в отдельной горутине.
func (r *DNSRunner) Start(port int) {
	addr, err := dns.ResolveAddr("127.0.0.1", port)
	if err != nil {
		log.Printf("Gate DNS resolve addr: %v", err)
		return
	}
	srv := dns.NewServer(addr, r.upstream, r.bl, r.st)
	r.mu.Lock()
	r.srv = srv
	r.port = port
	r.mu.Unlock()
	go func() {
		if err := srv.Start(context.Background()); err != nil {
			log.Printf("Gate DNS server: %v", err)
		}
	}()
}

// Restart останавливает текущий сервер и запускает на новом порту.
func (r *DNSRunner) Restart(port int) {
	r.mu.Lock()
	old := r.srv
	r.srv = nil
	r.mu.Unlock()
	if old != nil {
		if err := old.Shutdown(); err != nil {
			log.Printf("Gate DNS shutdown: %v", err)
		}
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

// Port возвращает текущий порт (53 или 5354).
func (r *DNSRunner) Port() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.port
}
