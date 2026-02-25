package dns

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// UpstreamResolver поддерживает plain UDP, DoT и DoH.
type UpstreamResolver struct {
	upstreams []upstreamEntry
	lastUsed  string
	mu        sync.RWMutex
	client    *http.Client
}

type upstreamEntry struct {
	address  string
	protocol string // "doh", "dot", "plain"
}

// NewUpstreamResolver создаёт резолвер из списка адресов (8.8.8.8, https://1.1.1.1/dns-query, tls://8.8.8.8).
func NewUpstreamResolver(addrs []string) *UpstreamResolver {
	r := &UpstreamResolver{
		client: &http.Client{Timeout: 5 * time.Second},
	}
	for _, addr := range addrs {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		e := upstreamEntry{address: addr}
		if strings.HasPrefix(addr, "https://") {
			e.protocol = "doh"
		} else if strings.HasPrefix(addr, "tls://") {
			e.protocol = "dot"
			e.address = strings.TrimPrefix(addr, "tls://")
			if !strings.Contains(e.address, ":") {
				e.address += ":853"
			}
		} else {
			e.protocol = "plain"
			if !strings.Contains(addr, ":") {
				e.address = addr + ":53"
			} else {
				e.address = addr
			}
		}
		r.upstreams = append(r.upstreams, e)
	}
	if len(r.upstreams) == 0 {
		r.upstreams = []upstreamEntry{
			{address: "8.8.8.8:53", protocol: "plain"},
			{address: "1.1.1.1:53", protocol: "plain"},
		}
	}
	return r
}

// Resolve отправляет запрос на первый доступный upstream.
func (r *UpstreamResolver) Resolve(ctx context.Context, msg *dns.Msg) (*dns.Msg, error) {
	var lastErr error
	for _, u := range r.upstreams {
		var resp *dns.Msg
		var err error
		switch u.protocol {
		case "doh":
			resp, err = r.resolveDoH(ctx, u.address, msg)
		case "dot":
			resp, err = r.resolveDoT(ctx, u.address, msg)
		default:
			resp, err = r.resolvePlain(ctx, u.address, msg)
		}
		if err != nil {
			lastErr = err
			continue
		}
		if resp != nil {
			r.mu.Lock()
			r.lastUsed = u.address
			r.mu.Unlock()
			return resp, nil
		}
	}
	return nil, fmt.Errorf("all upstreams failed: %w", lastErr)
}

func (r *UpstreamResolver) LastUsed() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lastUsed
}

func (r *UpstreamResolver) resolveDoH(ctx context.Context, url string, msg *dns.Msg) (*dns.Msg, error) {
	packed, err := msg.Pack()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(packed))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	out := new(dns.Msg)
	if err := out.Unpack(body); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *UpstreamResolver) resolveDoT(ctx context.Context, addr string, msg *dns.Msg) (*dns.Msg, error) {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	rawConn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	conn := tls.Client(rawConn, &tls.Config{MinVersion: tls.VersionTLS12})
	if err := conn.HandshakeContext(ctx); err != nil {
		rawConn.Close()
		return nil, err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	c := &dns.Conn{Conn: conn}
	if err := c.WriteMsg(msg); err != nil {
		return nil, err
	}
	return c.ReadMsg()
}

func (r *UpstreamResolver) resolvePlain(ctx context.Context, addr string, msg *dns.Msg) (*dns.Msg, error) {
	client := &dns.Client{Net: "udp", Timeout: 3 * time.Second}
	resp, _, err := client.ExchangeContext(ctx, msg, addr)
	return resp, err
}
