package blocklist

import (
	"bufio"
	"os"
	"strings"
	"sync"
)

// Blocklist — множество доменов для блокировки (например, трекеры). Потокобезопасно.
type Blocklist struct {
	mu   sync.RWMutex
	set  map[string]struct{}
	path string
}

// New создаёт пустой Blocklist.
func New() *Blocklist {
	return &Blocklist{set: make(map[string]struct{})}
}

// Load загружает домены из файла (один домен на строку, # — комментарий). Пустые строки игнорируются.
func (b *Blocklist) Load(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	set := make(map[string]struct{})
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.ToLower(line)
		set[line] = struct{}{}
	}
	b.mu.Lock()
	b.set = set
	b.path = path
	b.mu.Unlock()
	return sc.Err()
}

// Blocked возвращает true, если домен в блок-листе. qname — имя из DNS-запроса (может заканчиваться точкой).
func (b *Blocklist) Blocked(qname string) bool {
	qname = strings.TrimSuffix(strings.ToLower(qname), ".")
	b.mu.RLock()
	defer b.mu.RUnlock()
	if _, ok := b.set[qname]; ok {
		return true
	}
	// Проверяем поддомены: a.b.tracker.com → ищем b.tracker.com, tracker.com
	parts := strings.Split(qname, ".")
	for i := 0; i < len(parts)-1; i++ {
		parent := strings.Join(parts[i:], ".")
		if _, ok := b.set[parent]; ok {
			return true
		}
	}
	return false
}

// Count возвращает количество доменов в списке.
func (b *Blocklist) Count() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.set)
}

// All возвращает копию списка доменов (для экспорта в hosts и т.п.).
func (b *Blocklist) All() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]string, 0, len(b.set))
	for d := range b.set {
		out = append(out, d)
	}
	return out
}

// Path возвращает путь к загруженному файлу (или пусто).
func (b *Blocklist) Path() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.path
}

// AddDomain добавляет домен в список и сохраняет в файл (для блокировки из UI).
func (b *Blocklist) AddDomain(domain string) error {
	domain = strings.TrimSpace(strings.ToLower(domain))
	if domain == "" {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.path == "" {
		return nil
	}
	b.set[domain] = struct{}{}
	// Перезаписать файл: все домены из set, по одному на строку.
	f, err := os.Create(b.path)
	if err != nil {
		return err
	}
	defer f.Close()
	for d := range b.set {
		if _, err := f.WriteString(d + "\n"); err != nil {
			return err
		}
	}
	return nil
}
