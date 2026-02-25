package server

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	coreserver "github.com/GalitskyKK/nekkus-core/pkg/server"
	"github.com/GalitskyKK/nekkus-gate/internal/blocklist"
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

	srv.Mux.HandleFunc("GET /api/top_blocked", func(w http.ResponseWriter, _ *http.Request) {
		setCORS(w)
		w.Header().Set("Content-Type", "application/json")
		type entry struct{ Domain string `json:"domain"`; Count int `json:"count"` }
		_ = json.NewEncoder(w).Encode([]entry{})
	})

	// Состояние DNS-фильтра: включён ли (системный DNS = 127.0.0.1, сервер на 53).
	srv.Mux.HandleFunc("GET /api/filter/status", func(w http.ResponseWriter, _ *http.Request) {
		setCORS(w)
		active := sysdns.HasBackup(dataDir)
		port := defaultDNSPort
		if runner != nil {
			port = runner.Port()
		}
		writeJSON(w, map[string]interface{}{
			"active": active,
			"port":   port,
		}, http.StatusOK)
	})

	srv.Mux.HandleFunc("POST /api/filter/enable", func(w http.ResponseWriter, _ *http.Request) {
		setCORS(w)
		// Сначала проверяем, что порт 53 свободен. Иначе не трогаем системный DNS — пользователь не останется без интернета.
		if err := tryListenUDP53(); err != nil {
			msg := "Порт 53 занят другим приложением (например, служба «Кэширование DNS»). Остановите его в «Службы» или диспетчере задач, затем нажмите «Включить» снова."
			writeJSON(w, map[string]string{"error": msg}, http.StatusInternalServerError)
			return
		}
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
		writeJSON(w, map[string]interface{}{"ok": true, "active": true}, http.StatusOK)
	})

	srv.Mux.HandleFunc("POST /api/filter/disable", func(w http.ResponseWriter, _ *http.Request) {
		setCORS(w)
		if err := sysdns.Disable(dataDir); err != nil {
			writeJSON(w, map[string]string{"error": err.Error()}, http.StatusInternalServerError)
			return
		}
		if runner != nil {
			runner.Restart(defaultDNSPort)
		}
		writeJSON(w, map[string]interface{}{"ok": true, "active": false}, http.StatusOK)
	})
}
