package dns

import (
	"context"
	"net"
	"time"

	"github.com/GalitskyKK/nekkus-gate/internal/filter"
	"github.com/GalitskyKK/nekkus-gate/internal/querylog"
	"github.com/GalitskyKK/nekkus-gate/internal/stats"
	"github.com/miekg/dns"
)

// Handler обрабатывает DNS-запросы: фильтр → кэш → upstream, логирует в querylog и stats.
type Handler struct {
	cache    *Cache
	upstream *UpstreamResolver
	engine   *filter.Engine
	qlog     *querylog.Log
	stats    *stats.Stats
}

// NewHandler создаёт обработчик DNS.
func NewHandler(cache *Cache, upstream *UpstreamResolver, engine *filter.Engine, qlog *querylog.Log, st *stats.Stats) *Handler {
	return &Handler{
		cache:    cache,
		upstream: upstream,
		engine:   engine,
		qlog:     qlog,
		stats:    st,
	}
}

// ServeDNS реализует dns.Handler.
func (h *Handler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	if len(r.Question) == 0 {
		dns.HandleFailed(w, r)
		return
	}
	start := time.Now()
	q := r.Question[0]
	name := q.Name
	qtype := q.Qtype
	typeStr := dns.TypeToString[qtype]
	clientAddr := ""
	if addr := w.RemoteAddr(); addr != nil {
		clientAddr = addr.String()
	}

	domain := dns.Fqdn(name)
	blocked, rule := h.engine.Check(domain)
	if blocked {
		h.stats.IncTotal()
		h.stats.IncBlocked()
		reply := h.buildBlockedResponse(r, q)
		_ = w.WriteMsg(reply)
		h.qlog.Append(querylog.Entry{
			Timestamp: start,
			Domain:    domain,
			Type:      typeStr,
			Blocked:   true,
			Rule:      rule,
			LatencyMs: time.Since(start).Milliseconds(),
		})
		return
	}

	if cached, ok := h.cache.Get(domain, qtype); ok {
		cached.SetReply(r)
		_ = w.WriteMsg(cached)
		h.stats.IncTotal()
		h.qlog.Append(querylog.Entry{
			Timestamp: start,
			Domain:    domain,
			Type:      typeStr,
			Blocked:   false,
			Cached:    true,
			LatencyMs: time.Since(start).Milliseconds(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := h.upstream.Resolve(ctx, r)
	if err != nil {
		reply := new(dns.Msg)
		reply.SetRcode(r, dns.RcodeServerFailure)
		_ = w.WriteMsg(reply)
		_ = clientAddr
		return
	}

	if resp != nil && len(resp.Answer) > 0 {
		h.cache.Set(domain, qtype, resp)
	}
	resp.SetReply(r)
	_ = w.WriteMsg(resp)
	h.stats.IncTotal()
	h.qlog.Append(querylog.Entry{
		Timestamp: start,
		Domain:    domain,
		Type:      typeStr,
		Blocked:   false,
		Cached:    false,
		LatencyMs: time.Since(start).Milliseconds(),
	})
}

func (h *Handler) buildBlockedResponse(r *dns.Msg, q dns.Question) *dns.Msg {
	reply := new(dns.Msg)
	reply.SetReply(r)
	reply.Authoritative = true
	switch q.Qtype {
	case dns.TypeA:
		reply.Answer = append(reply.Answer, &dns.A{
			Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
			A:   net.ParseIP("0.0.0.0"),
		})
	case dns.TypeAAAA:
		reply.Answer = append(reply.Answer, &dns.AAAA{
			Hdr:  dns.RR_Header{Name: q.Name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 300},
			AAAA: net.ParseIP("::"),
		})
	default:
		reply.SetRcode(r, dns.RcodeNameError)
	}
	return reply
}
