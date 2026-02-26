package querylog

import (
	"sync"
	"time"
)

// Entry — одна запись лога DNS-запроса.
type Entry struct {
	Timestamp time.Time `json:"timestamp"`
	Domain    string    `json:"domain"`
	Type      string    `json:"type"`   // A, AAAA, etc.
	Blocked   bool      `json:"blocked"`
	Cached    bool      `json:"cached"`
	Rule      string    `json:"rule,omitempty"`
	LatencyMs int64     `json:"latency_ms"`
	IsTracker bool      `json:"is_tracker"` // домен из базы известных трекеров
	AppName   string    `json:"app_name,omitempty"` // процесс (по source port), если определён
}

// Log — кольцевой буфер последних запросов. Потокобезопасно.
type Log struct {
	entries []Entry
	mu      sync.RWMutex
	head    int
	size   int
}

const defaultMaxEntries = 500

// New создаёт лог с максимум maxEntries записей.
func New(maxEntries int) *Log {
	if maxEntries <= 0 {
		maxEntries = defaultMaxEntries
	}
	return &Log{
		entries: make([]Entry, maxEntries),
		size:    maxEntries,
	}
}

// Append добавляет запись (перезаписывает самую старую при переполнении).
func (l *Log) Append(e Entry) {
	l.mu.Lock()
	l.entries[l.head] = e
	l.head = (l.head + 1) % l.size
	l.mu.Unlock()
}

// Last возвращает последние n записей (новые первые).
func (l *Log) Last(n int) []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if n <= 0 || l.size == 0 {
		return nil
	}
	if n > l.size {
		n = l.size
	}
	out := make([]Entry, 0, n)
	for i := 0; i < n; i++ {
		idx := (l.head - 1 - i + l.size*2) % l.size
		e := l.entries[idx]
		if e.Domain == "" && e.Timestamp.IsZero() {
			continue
		}
		out = append(out, e)
	}
	return out
}

// PrivacyStats — агрегат по трекерам для глобального Privacy Score.
type PrivacyStats struct {
	TrackerQueries  int     // запросов к известным трекерам
	TrackerBlocked  int     // из них заблокировано
	Score           int     // 0–100: (TrackerBlocked/TrackerQueries)*100, или 100 если трекеров не было
	TotalQueries    int     // всего запросов (для контекста)
}

// AppPrivacyStats — статистика по одному приложению.
type AppPrivacyStats struct {
	AppName         string `json:"app_name"`
	Score           int    `json:"score"`             // 0–100
	TotalQueries    int    `json:"total_queries"`
	TrackerQueries  int    `json:"tracker_queries"`
	TrackerBlocked  int   `json:"tracker_blocked"`
}

// PrivacyFromLast вычисляет глобальную и per-app статистику по последним maxEntries записям.
func (l *Log) PrivacyFromLast(maxEntries int) (global PrivacyStats, apps []AppPrivacyStats) {
	entries := l.Last(maxEntries)
	global.TotalQueries = len(entries)
	appMap := make(map[string]*struct {
		total, tracker, blocked int
	})
	for _, e := range entries {
		if e.IsTracker {
			global.TrackerQueries++
			if e.Blocked {
				global.TrackerBlocked++
			}
		}
		appName := e.AppName
		if appName == "" {
			appName = "(unknown)"
		}
		if _, ok := appMap[appName]; !ok {
			appMap[appName] = &struct{ total, tracker, blocked int }{}
		}
		appMap[appName].total++
		if e.IsTracker {
			appMap[appName].tracker++
			if e.Blocked {
				appMap[appName].blocked++
			}
		}
	}
	if global.TrackerQueries > 0 {
		global.Score = int(float64(global.TrackerBlocked)/float64(global.TrackerQueries)*100 + 0.5)
	} else {
		global.Score = 100
	}
	if global.Score > 100 {
		global.Score = 100
	}
	for name, st := range appMap {
		score := 100
		if st.tracker > 0 {
			score = int(float64(st.blocked)/float64(st.tracker)*100 + 0.5)
			if score > 100 {
				score = 100
			}
		}
		apps = append(apps, AppPrivacyStats{
			AppName:        name,
			Score:          score,
			TotalQueries:   st.total,
			TrackerQueries: st.tracker,
			TrackerBlocked: st.blocked,
		})
	}
	return global, apps
}

// TopBlocked возвращает топ n заблокированных доменов по количеству запросов (из последних записей).
func (l *Log) TopBlocked(n int) []struct {
	Domain string
	Count int
} {
	entries := l.Last(2000)
	counts := make(map[string]int)
	for _, e := range entries {
		if e.Blocked && e.Domain != "" {
			counts[e.Domain]++
		}
	}
	type pair struct {
		domain string
		count  int
	}
	var pairs []pair
	for d, c := range counts {
		pairs = append(pairs, pair{d, c})
	}
	for i := 0; i < len(pairs)-1; i++ {
		for j := i + 1; j < len(pairs); j++ {
			if pairs[j].count > pairs[i].count {
				pairs[i], pairs[j] = pairs[j], pairs[i]
			}
		}
	}
	if n > len(pairs) {
		n = len(pairs)
	}
	out := make([]struct {
		Domain string
		Count  int
	}, n)
	for i := 0; i < n; i++ {
		out[i].Domain = pairs[i].domain
		out[i].Count = pairs[i].count
	}
	return out
}
