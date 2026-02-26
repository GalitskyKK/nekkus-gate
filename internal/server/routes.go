package server

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	coreserver "github.com/GalitskyKK/nekkus-core/pkg/server"
	"github.com/GalitskyKK/nekkus-gate/internal/blocklist"
	"github.com/GalitskyKK/nekkus-gate/internal/hostsfilter"
	"github.com/GalitskyKK/nekkus-gate/internal/platform"
	"github.com/GalitskyKK/nekkus-gate/internal/querylog"
	"github.com/GalitskyKK/nekkus-gate/internal/recovery"
	"github.com/GalitskyKK/nekkus-gate/internal/stats"
	"github.com/GalitskyKK/nekkus-gate/internal/sysdns"
)

// tryListenUDP53 проверяет, свободен ли порт 53 на 127.0.0.1. Если занят — возвращает ошибку.
// Так мы не переключаем системный DNS, пока не убедимся, что Gate сможет слушать 53.
func tryListenUDP53() error {
	pc, err := net.ListenPacket("udp", "127.0.0.1:53")
	if err != nil {
		return err
	}
	_ = pc.Close()
	return nil
}

func setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func writeJSON(w http.ResponseWriter, v interface{}, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// RegisterRoutes регистрирует API маршруты для nekkus-gate.
// dataDir — каталог данных (для бэкапа DNS и состояния фильтра).
// runner — перезапуск DNS на порту 53 (фильтр вкл.) / defaultDNSPort (фильтр выкл.).
func RegisterRoutes(srv *coreserver.Server, st *stats.Stats, bl *blocklist.Blocklist, dataDir string, runner *DNSRunner, defaultDNSPort int) {
	srv.Mux.HandleFunc("GET /api/stats", func(w http.ResponseWriter, _ *http.Request) {
		setCORS(w)
		w.Header().Set("Content-Type", "application/json")
		total, blockedToday, blockedTotal := st.Snapshot()
		var pct float64
		if total > 0 {
			pct = float64(blockedToday) / float64(total) * 100
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"total_queries":    total,
			"blocked_today":   blockedToday,
			"blocked_total":   blockedTotal,
			"blocked_percent": pct,
			"blocklist_count": bl.Count(),
			"timestamp":       time.Now().Unix(),
		})
	})

	srv.Mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		setCORS(w)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	srv.Mux.HandleFunc("GET /api/top_blocked", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		w.Header().Set("Content-Type", "application/json")
		limit := 10
		if s := r.URL.Query().Get("limit"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 100 {
				limit = n
			}
		}
		entries := runner.GetTopBlocked(limit)
		if entries == nil {
			entries = []TopBlockedEntry{}
		}
		_ = json.NewEncoder(w).Encode(entries)
	})

	srv.Mux.HandleFunc("GET /api/dns/port-check", func(w http.ResponseWriter, _ *http.Request) {
		setCORS(w)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(platform.CheckPort53())
	})

	srv.Mux.HandleFunc("GET /api/queries", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		w.Header().Set("Content-Type", "application/json")
		limit := 100
		if s := r.URL.Query().Get("limit"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 500 {
				limit = n
			}
		}
		entries := runner.GetQueryLog(limit)
		if entries == nil {
			entries = []querylog.Entry{}
		}
		_ = json.NewEncoder(w).Encode(entries)
	})

	srv.Mux.HandleFunc("POST /api/block", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		w.Header().Set("Content-Type", "application/json")
		var body struct {
			Domain string `json:"domain"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, map[string]string{"error": "invalid body"}, http.StatusBadRequest)
			return
		}
		domain := strings.TrimSpace(strings.ToLower(body.Domain))
		if domain == "" {
			writeJSON(w, map[string]string{"error": "domain required"}, http.StatusBadRequest)
			return
		}
		if err := bl.AddDomain(domain); err != nil {
			writeJSON(w, map[string]string{"error": err.Error()}, http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{"ok": true, "blocklist_count": bl.Count()}, http.StatusOK)
	})

	// Состояние фильтра: active, mode (dns | hosts), port.
	srv.Mux.HandleFunc("GET /api/filter/status", func(w http.ResponseWriter, _ *http.Request) {
		setCORS(w)
		active := sysdns.HasBackup(dataDir)
		mode := ""
		if state, _ := sysdns.LoadFromFile(dataDir); state != nil {
			mode = state.Mode
			if mode == "" {
				mode = sysdns.ModeDNS
			}
		}
		port := defaultDNSPort
		if runner != nil {
			port = runner.Port()
		}
		writeJSON(w, map[string]interface{}{
			"active":          active,
			"mode":            mode,
			"port":            port,
			"blocklist_count": bl.Count(),
		}, http.StatusOK)
	})

	srv.Mux.HandleFunc("POST /api/filter/enable", func(w http.ResponseWriter, _ *http.Request) {
		setCORS(w)
		// Уже включено — не запускать netsh/PowerShell повторно (нет моргания окон).
		if sysdns.HasBackup(dataDir) {
			state, _ := sysdns.LoadFromFile(dataDir)
			mode := sysdns.ModeDNS
			if state != nil && state.Mode != "" {
				mode = state.Mode
			}
			writeJSON(w, map[string]interface{}{"ok": true, "active": true, "mode": mode}, http.StatusOK)
			return
		}
		// Пробуем порт 53. Если свободен — режим DNS (системный DNS → 127.0.0.1, Gate на 53).
		// Если занят — режим hosts: дописываем блок-лист в hosts, ничего не останавливаем.
		if err := tryListenUDP53(); err == nil {
			if err := sysdns.Enable(dataDir); err != nil {
				msg := err.Error()
				if strings.Contains(strings.ToLower(msg), "access is denied") ||
					strings.Contains(strings.ToLower(msg), "отказано в доступе") ||
					strings.Contains(strings.ToLower(msg), "denied") {
					msg = "Запустите Gate от имени администратора: ПКМ по программе → «Запуск от имени администратора», затем снова нажмите «Включить»."
				}
				writeJSON(w, map[string]string{"error": msg}, http.StatusInternalServerError)
				return
			}
			if runner != nil {
				runner.Restart(53)
			}
			_ = recovery.Lock(dataDir)
			writeJSON(w, map[string]interface{}{"ok": true, "active": true, "mode": sysdns.ModeDNS}, http.StatusOK)
			return
		}
		// Порт 53 занят — включаем через файл hosts (без переключения DNS, ничего не останавливаем).
		domains := bl.All()
		if err := hostsfilter.Enable(dataDir, domains); err != nil {
			msg := err.Error()
			if strings.Contains(strings.ToLower(msg), "access is denied") ||
				strings.Contains(strings.ToLower(msg), "отказано") ||
				strings.Contains(strings.ToLower(msg), "denied") ||
				strings.Contains(strings.ToLower(msg), "permission") {
				msg = "Запустите Gate от имени администратора, затем снова нажмите «Включить»."
			}
			writeJSON(w, map[string]string{"error": msg}, http.StatusInternalServerError)
			return
		}
		if err := sysdns.SaveStateHostsMode(dataDir); err != nil {
			writeJSON(w, map[string]string{"error": err.Error()}, http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{"ok": true, "active": true, "mode": sysdns.ModeHosts}, http.StatusOK)
	})

	srv.Mux.HandleFunc("POST /api/filter/disable", func(w http.ResponseWriter, _ *http.Request) {
		setCORS(w)
		state, _ := sysdns.LoadFromFile(dataDir)
		if state != nil && state.Mode == sysdns.ModeHosts {
			if err := hostsfilter.Disable(dataDir); err != nil {
				writeJSON(w, map[string]string{"error": err.Error()}, http.StatusInternalServerError)
				return
			}
		}
		if err := sysdns.Disable(dataDir); err != nil {
			writeJSON(w, map[string]string{"error": err.Error()}, http.StatusInternalServerError)
			return
		}
		recovery.Unlock(dataDir)
		if runner != nil && (state == nil || state.Mode != sysdns.ModeHosts) {
			runner.Restart(defaultDNSPort)
		}
		writeJSON(w, map[string]interface{}{"ok": true, "active": false}, http.StatusOK)
	})
}
