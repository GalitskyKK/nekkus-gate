package stats

import (
	"sync/atomic"
	"time"
)

// Stats хранит счётчики запросов для виджета и API. Потокобезопасно.
type Stats struct {
	totalQueries   atomic.Uint64
	blockedToday   atomic.Uint64
	blockedTotal   atomic.Uint64
	dayStartUnix   int64 // начало «дня» для сброса blockedToday (UTC)
}

// New создаёт Stats с обнулёнными счётчиками.
func New() *Stats {
	s := &Stats{dayStartUnix: time.Now().UTC().Truncate(24 * time.Hour).Unix()}
	return s
}

// IncTotal увеличивает общее число запросов на 1.
func (s *Stats) IncTotal() {
	s.totalQueries.Add(1)
}

// IncBlocked увеличивает счётчики заблокированных (сегодня и всего).
func (s *Stats) IncBlocked() {
	s.blockedToday.Add(1)
	s.blockedTotal.Add(1)
}

// MaybeResetDay сбрасывает blockedToday если наступил новый день (UTC).
func (s *Stats) MaybeResetDay() {
	now := time.Now().UTC().Truncate(24 * time.Hour).Unix()
	if now != s.dayStartUnix {
		s.blockedToday.Store(0)
		s.dayStartUnix = now
	}
}

// Snapshot возвращает копию текущих значений для API.
func (s *Stats) Snapshot() (totalQueries, blockedToday, blockedTotal uint64) {
	s.MaybeResetDay()
	return s.totalQueries.Load(), s.blockedToday.Load(), s.blockedTotal.Load()
}
