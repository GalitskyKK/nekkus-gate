package trackers

import (
	"bufio"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// KnownTrackers — множество известных доменов трекеров/рекламы. Потокобезопасно.
// Используется для расчёта Privacy Score (доля заблокированных запросов к трекерам).
type KnownTrackers struct {
	mu   sync.RWMutex
	set  map[string]struct{}
	path string // путь к кэшу (если загружали по URL)
}

// New создаёт пустой KnownTrackers. Вызови LoadBuiltin() и/или LoadFromURL().
func New() *KnownTrackers {
	return &KnownTrackers{set: make(map[string]struct{})}
}

// Contains возвращает true, если домен совпадает или является поддоменом известного трекера.
// qname — имя из DNS-запроса (может заканчиваться точкой).
func (k *KnownTrackers) Contains(qname string) bool {
	qname = strings.TrimSuffix(strings.ToLower(qname), ".")
	k.mu.RLock()
	defer k.mu.RUnlock()
	if _, ok := k.set[qname]; ok {
		return true
	}
	parts := strings.Split(qname, ".")
	for i := 0; i < len(parts)-1; i++ {
		parent := strings.Join(parts[i:], ".")
		if _, ok := k.set[parent]; ok {
			return true
		}
	}
	return false
}

// Count возвращает количество базовых доменов в списке.
func (k *KnownTrackers) Count() int {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return len(k.set)
}

// addDomains добавляет домены в set (вызывать под lock).
func (k *KnownTrackers) addDomains(domains []string) {
	for _, d := range domains {
		d = strings.TrimSpace(strings.ToLower(d))
		if d == "" || strings.HasPrefix(d, "#") {
			continue
		}
		// Убрать возможный префикс 0.0.0.0 / 127.0.0.1
		if idx := strings.Index(d, " "); idx > 0 {
			d = strings.TrimSpace(d[idx+1:])
		}
		if len(d) > 0 {
			k.set[d] = struct{}{}
		}
	}
}

// LoadBuiltin загружает встроенный список известных трекеров (реклама, аналитика, соцсети).
func (k *KnownTrackers) LoadBuiltin() {
	builtin := []string{
		"google-analytics.com",
		"googletagmanager.com",
		"doubleclick.net",
		"googlesyndication.com",
		"googleadservices.com",
		"2mdn.net",
		"facebook.com",
		"facebook.net",
		"fbcdn.net",
		"connect.facebook.net",
		"pixel.facebook.com",
		"analytics.facebook.com",
		"mc.yandex.ru",
		"yandex.ru",
		"yandexadexchange.net",
		"yandex.com",
		"an.yandex.ru",
		"adnxs.com",
		"adsrvr.org",
		"criteo.com",
		"criteo.net",
		"outbrain.com",
		"taboola.com",
		"amazon-adsystem.com",
		"amazonads.com",
		"adservice.google.com",
		"ad.doubleclick.net",
		"stats.g.doubleclick.net",
		"pagead2.googlesyndication.com",
		"tpc.googlesyndication.com",
		"securepubads.g.doubleclick.net",
		"partner.googleadservices.com",
		"www.google-analytics.com",
		"ssl.google-analytics.com",
		"www.googletagmanager.com",
		"hotjar.com",
		"static.hotjar.com",
		"script.hotjar.com",
		"mouseflow.com",
		"mixpanel.com",
		"segment.io",
		"segment.com",
		"intercom.io",
		"intercom.com",
		"fullstory.com",
		"clarity.ms",
		"linkedin.com",
		"snap.licdn.com",
		"px.ads.linkedin.com",
		"twitter.com",
		"analytics.twitter.com",
		"t.co",
		"tiktok.com",
		"analytics.tiktok.com",
		"vimeo.com",
		"vimeo.com/api",
		"newrelic.com",
		"nr-data.net",
		"datadoghq.com",
		"sentry.io",
		"sentry-cdn.com",
		"branch.io",
		"app.link",
		"adjust.com",
		"appsflyer.com",
		"moatads.com",
		"scorecardresearch.com",
		"comscore.com",
		"chartbeat.com",
		"quantserve.com",
		"exelator.com",
		"bluekai.com",
		"krxd.net",
		"rlcdn.com",
		"casalemedia.com",
		"adform.net",
		"mathtag.com",
		"demdex.net",
		"omtrdc.net",
		"everesttech.net",
		"agkn.com",
		"rubiconproject.com",
		"pubmatic.com",
		"openx.net",
		"spotxchange.com",
		"lijit.com",
		"bidswitch.net",
		"contextweb.com",
		"yieldmanager.com",
		"zedo.com",
		"advertising.com",
		"liveintent.com",
		"mediagrid.com",
		"teads.tv",
		"tribalfusion.com",
		"1rx.io",
		"sharethrough.com",
		"sonobi.com",
		"smartyads.com",
		"synacor.com",
		"triplelift.com",
		"e-planning.net",
		"improvedigital.com",
		"w55c.net",
		"adgrx.com",
		"tapad.com",
		"adsymptotic.com",
		"adroll.com",
		"marinsm.com",
		"turn.com",
		"invitemedia.com",
		"undertone.com",
		"lijit.com",
		"exponential.com",
		"mediamath.com",
		"tremorhub.com",
		"spotx.tv",
		"smartadserver.com",
		"criteo.com",
		"c.amazon-adsystem.com",
		"fls.doubleclick.net",
		"stats.bannersnack.com",
		"tracking.etracker.com",
		"etracker.com",
		"etracker.de",
		"innovid.com",
		"vidible.tv",
		"teads.tv",
		"ads.adaptv.advertising.com",
		"ad.360yield.com",
		"ads.yahoo.com",
		"gemini.yahoo.com",
		"adsymptotic.com",
	}
	k.mu.Lock()
	if k.set == nil {
		k.set = make(map[string]struct{})
	}
	k.addDomains(builtin)
	k.mu.Unlock()
}

// parseFilterLine извлекает домен из строки фильтра AdGuard/hosts.
// Поддерживает: "||domain.com^", "0.0.0.0 domain.com", "domain.com".
func parseFilterLine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "!") || strings.HasPrefix(line, "#") {
		return ""
	}
	// AdGuard: ||domain.com^
	if strings.HasPrefix(line, "||") {
		end := strings.IndexAny(line[2:], "^/\r\n")
		if end == -1 {
			return strings.ToLower(line[2:])
		}
		return strings.ToLower(line[2 : 2+end])
	}
	// hosts: 0.0.0.0 domain.com или 127.0.0.1 domain.com
	parts := strings.Fields(line)
	if len(parts) >= 2 {
		return strings.ToLower(parts[1])
	}
	// просто домен
	return strings.ToLower(line)
}

// LoadFromFile загружает домены из файла (один на строку, AdGuard/hosts форматы).
func (k *KnownTrackers) LoadFromFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var domains []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if d := parseFilterLine(sc.Text()); d != "" {
			domains = append(domains, d)
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	k.mu.Lock()
	if k.set == nil {
		k.set = make(map[string]struct{})
	}
	k.addDomains(domains)
	k.path = path
	k.mu.Unlock()
	return nil
}

// LoadFromURL загружает список по URL (AdGuard/hosts/domains-only), сохраняет кэш в dataDir.
// cacheFileName — имя файла кэша (например "trackers_cache.txt").
func (k *KnownTrackers) LoadFromURL(url string, dataDir string, cacheFileName string) error {
	if url == "" {
		return nil
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var domains []string
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		if d := parseFilterLine(sc.Text()); d != "" {
			domains = append(domains, d)
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	k.mu.Lock()
	if k.set == nil {
		k.set = make(map[string]struct{})
	}
	k.addDomains(domains)
	if dataDir != "" && cacheFileName != "" {
		cachePath := filepath.Join(dataDir, cacheFileName)
		k.path = cachePath
		_ = os.MkdirAll(dataDir, 0755)
		f, _ := os.Create(cachePath)
		if f != nil {
			for _, d := range domains {
				_, _ = f.WriteString(d + "\n")
			}
			_ = f.Close()
		}
	}
	k.mu.Unlock()
	return nil
}

// Path возвращает путь к кэшу (если загружали из файла/URL).
func (k *KnownTrackers) Path() string {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.path
}
