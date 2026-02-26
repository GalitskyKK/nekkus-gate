package dns

import (
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/GalitskyKK/nekkus-gate/internal/apptrack"
	"github.com/GalitskyKK/nekkus-gate/internal/filter"
	"github.com/GalitskyKK/nekkus-gate/internal/querylog"
	"github.com/GalitskyKK/nekkus-gate/internal/stats"
	"github.com/GalitskyKK/nekkus-gate/internal/trackers"
	"github.com/miekg/dns"
)

// Server — локальный DNS-сервер (UDP + TCP): фильтр, кэш, upstream, лог.
type Server struct {
	addr      string
	handler   *Handler
	udpServer *dns.Server
	tcpServer *dns.Server
	mu        sync.Mutex
}

// NewServer создаёт DNS-сервер. addr — listen (например "127.0.0.1:53").
// knownTrack и appResolver — для лога (is_tracker, app_name); могут быть nil.
func NewServer(addr string, cache *Cache, upstream *UpstreamResolver, engine *filter.Engine, qlog *querylog.Log, st *stats.Stats, knownTrack *trackers.KnownTrackers, appResolver apptrack.Resolver) *Server {
	handler := NewHandler(cache, upstream, engine, qlog, st, knownTrack, appResolver)
	return &Server{
		addr:    addr,
		handler: handler,
	}
}

// Start запускает UDP и TCP серверы в фоне и возвращается.
func (s *Server) Start() error {
	s.mu.Lock()
	s.udpServer = &dns.Server{
		Addr:    s.addr,
		Net:     "udp",
		Handler: s.handler,
	}
	s.tcpServer = &dns.Server{
		Addr:    s.addr,
		Net:     "tcp",
		Handler: s.handler,
	}
	s.mu.Unlock()

	log.Printf("Gate DNS → udp+tcp://%s", s.addr)

	go func() {
		if err := s.udpServer.ListenAndServe(); err != nil {
			log.Printf("Gate DNS UDP: %v", err)
		}
	}()
	go func() {
		if err := s.tcpServer.ListenAndServe(); err != nil {
			log.Printf("Gate DNS TCP: %v", err)
		}
	}()
	return nil
}

// Shutdown останавливает UDP и TCP серверы.
func (s *Server) Shutdown() error {
	s.mu.Lock()
	udp, tcp := s.udpServer, s.tcpServer
	s.udpServer, s.tcpServer = nil, nil
	s.mu.Unlock()

	var err error
	if udp != nil {
		err = udp.Shutdown()
	}
	if tcp != nil {
		if e := tcp.Shutdown(); e != nil {
			err = e
		}
	}
	return err
}

// Addr возвращает адрес сервера.
func (s *Server) Addr() string { return s.addr }

// ResolveAddr возвращает "host:port" для указанного хоста и порта.
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
