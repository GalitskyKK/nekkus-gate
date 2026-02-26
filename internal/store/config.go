package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Config — настройки Gate (upstream DNS, кастомные правила, опция списка трекеров).
type Config struct {
	Upstreams      []string    `json:"upstreams"`       // ["8.8.8.8", "https://1.1.1.1/dns-query"]
	Rules          []RuleEntry `json:"rules"`          // block/allow по домену
	TrackerListURL string      `json:"tracker_list_url"` // опционально: URL списка трекеров (AdGuard/hosts)
	mu             sync.RWMutex
	path           string
}

// RuleEntry — одно правило: блокировать или разрешить домен (поддержка wildcard *.).
type RuleEntry struct {
	Domain string `json:"domain"` // example.com или *.ads.example.com
	Action string `json:"action"` // "block" или "allow"
}

const configFileName = "config.json"

// DefaultConfig возвращает конфиг по умолчанию с указанным dataDir (path для сохранения).
func DefaultConfig(dataDir string) *Config {
	return &Config{
		Upstreams: []string{"8.8.8.8", "1.1.1.1"},
		Rules:     nil,
		path:      filepath.Join(dataDir, configFileName),
	}
}

// LoadConfig загружает config.json из dataDir. Если файла нет — возвращает конфиг по умолчанию.
func LoadConfig(dataDir string) (*Config, error) {
	path := filepath.Join(dataDir, configFileName)
	c := &Config{
		Upstreams: []string{"8.8.8.8", "1.1.1.1"},
		Rules:     nil,
		path:      path,
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, c); err != nil {
		return c, nil
	}
	if len(c.Upstreams) == 0 {
		c.Upstreams = []string{"8.8.8.8", "1.1.1.1"}
	}
	return c, nil
}

func (c *Config) save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, data, 0600)
}

// SetUpstreams сохраняет список upstream и записывает config.
func (c *Config) SetUpstreams(upstreams []string) error {
	c.mu.Lock()
	c.Upstreams = upstreams
	c.mu.Unlock()
	return c.save()
}

// GetUpstreams возвращает копию списка upstream.
func (c *Config) GetUpstreams() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]string, len(c.Upstreams))
	copy(out, c.Upstreams)
	return out
}

// AddRule добавляет правило и сохраняет config.
func (c *Config) AddRule(domain, action string) error {
	c.mu.Lock()
	c.Rules = append(c.Rules, RuleEntry{Domain: domain, Action: action})
	c.mu.Unlock()
	return c.save()
}

// RemoveRule удаляет правило по домену и сохраняет config.
func (c *Config) RemoveRule(domain string) error {
	c.mu.Lock()
	var newRules []RuleEntry
	for _, r := range c.Rules {
		if r.Domain != domain {
			newRules = append(newRules, r)
		}
	}
	c.Rules = newRules
	c.mu.Unlock()
	return c.save()
}

// GetRules возвращает копию правил.
func (c *Config) GetRules() []RuleEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]RuleEntry, len(c.Rules))
	copy(out, c.Rules)
	return out
}
