package dns

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/GalitskyKK/nekkus-gate/internal/blocklist"
	"github.com/GalitskyKK/nekkus-gate/internal/stats"
	"github.com/miekg/dns"
)

// Server — локальный DNS-сервер: блокирует домены из blocklist, остальное форвардит на upstream.
type Server struct {
	addr      string
	upstream  string
	blocklist *blocklist.Blocklist
	stats     *stats.Stats
	pc        *dns.Server
	mu        sync.Mutex
}

// NewServer создаёт DNS-сервер. addr — listen (например "127.0.0.1:5353"), upstream — например "8.8.8.8:53".
func NewServer(addr, upstream string, bl *blocklist.Blocklist, st *stats.Stats) *Server {
	if upstream == "" {
		upstream = "8.8.8.8:53"
	}
	return &Server{
		addr:      addr,
		upstream:  upstream,
		blocklist: bl,
		stats:     st,
	}
}

// Start запускает UDP-сервер. Блокирует до ctx.Done() или ошибки.
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	s.pc = &dns.Server{
		Addr:    s.addr,
		Net:     "udp",
		Handler: dns.HandlerFunc(s.serve),
	}
	s.mu.Unlock()
	log.Printf("Gate DNS → udp://%s (upstream %s)", s.addr, s.upstream)
	return s.pc.ListenAndServe()
}

func (s *Server) serve(w dns.ResponseWriter, req *dns.Msg) {
	if len(req.Question) == 0 {
		dns.HandleFailed(w, req)
		return
	}
	q := req.Question[0]
	qname := q.Name
	s.stats.IncTotal()
	if s.blocklist.Blocked(qname) {
		s.stats.IncBlocked()
		m := new(dns.Msg)
		m.SetRcode(req, dns.RcodeNameError)
		m.RecursionAvailable = true
		_ = w.WriteMsg(m)
		return
	}
	// Форвард к upstream
	client := &dns.Client{Net: "udp", Timeout: 3 * time.Second}
	resp, _, err := client.Exchange(req, s.upstream)
	if err != nil {
		m := new(dns.Msg)
		m.SetRcode(req, dns.RcodeServerFailure)
		_ = w.WriteMsg(m)
		return
	}
	_ = w.WriteMsg(resp)
}

// Addr возвращает адрес сервера (host:port).
func (s *Server) Addr() string { return s.addr }

// Shutdown останавливает сервер (для перезапуска на другом порту или при выходе).
func (s *Server) Shutdown() error {
	s.mu.Lock()
	pc := s.pc
	s.mu.Unlock()
	if pc == nil {
		return nil
	}
	return pc.Shutdown()
}

// SetUpstream меняет upstream (например "1.1.1.1:53").
func (s *Server) SetUpstream(upstream string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.upstream = upstream
}

// ResolveAddr возвращает host:port для указанного хоста. Для "127.0.0.1" или "" возвращает s.addr.
func ResolveAddr(host string, port int) (string, error) {
	if host == "" || host == "127.0.0.1" || host == "localhost" {
		return fmt.Sprintf("127.0.0.1:%d", port), nil
	}
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return "", fmt.Errorf("resolve %s: %w", host, err)
	}
	return fmt.Sprintf("%s:%d", ips[0].String(), port), nil
}
