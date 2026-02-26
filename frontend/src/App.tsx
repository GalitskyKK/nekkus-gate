import { useCallback, useEffect, useState } from 'react'
import {
  AppShell,
  Button,
  Card,
  DataText,
  PageLayout,
  Section,
  StatusDot,
} from '@nekkus/ui-kit'
import {
  fetchStats,
  fetchFilterStatus,
  fetchTopBlocked,
  fetchPortCheck,
  fetchQueries,
  fetchPrivacy,
  fetchPrivacyApps,
  enableFilter,
  disableFilter,
  blockDomain,
  type GateStats,
  type TopBlockedEntry,
  type PortCheck,
  type QueryLogEntry,
  type PrivacyData,
  type AppPrivacyStats,
} from './api'

const REFRESH_MS = 3000
const QUERIES_REFRESH_MS = 2000

function formatNumber(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return String(n)
}

export default function App() {
  const [stats, setStats] = useState<GateStats | null>(null)
  const [topBlocked, setTopBlocked] = useState<TopBlockedEntry[]>([])
  const [filterActive, setFilterActive] = useState<boolean | null>(null)
  const [filterActiveMode, setFilterActiveMode] = useState<'dns' | 'hosts'>('dns')
  const [filterBlocklistCount, setFilterBlocklistCount] = useState(0)
  const [filterBusy, setFilterBusy] = useState(false)
  const [filterError, setFilterError] = useState<string | null>(null)
  const [portCheck, setPortCheck] = useState<PortCheck | null>(null)
  const [queries, setQueries] = useState<QueryLogEntry[]>([])
  const [privacy, setPrivacy] = useState<PrivacyData | null>(null)
  const [privacyApps, setPrivacyApps] = useState<AppPrivacyStats[]>([])
  const [blockingDomain, setBlockingDomain] = useState<string | null>(null)
  const [showOnlyTrackers, setShowOnlyTrackers] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    try {
      setError(null)
      const [s, t, f, pc, pr, prApps] = await Promise.all([
        fetchStats(),
        fetchTopBlocked(10).catch(() => []),
        fetchFilterStatus().catch(() => ({ active: false, mode: 'dns' as const, port: 5354, blocklist_count: 0 })),
        fetchPortCheck().catch(() => ({ available: true })),
        fetchPrivacy().catch(() => null),
        fetchPrivacyApps().catch(() => []),
      ])
      setStats(s)
      setTopBlocked(t)
      setFilterActive(f.active)
      setFilterActiveMode((f.mode === 'hosts' ? 'hosts' : 'dns'))
      setFilterBlocklistCount(f.blocklist_count ?? 0)
      setPortCheck(pc)
      setPrivacy(pr ?? null)
      setPrivacyApps(Array.isArray(prApps) ? prApps : [])
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Ошибка загрузки')
    }
  }, [])

  const loadQueries = useCallback(async () => {
    try {
      const list = await fetchQueries(80)
      setQueries(list)
    } catch {
      setQueries([])
    }
  }, [])

  const toggleFilter = useCallback(async () => {
    if (filterBusy || filterActive === null) return
    setFilterBusy(true)
    setFilterError(null)
    try {
      if (filterActive) {
        await disableFilter()
        setFilterActive(false)
      } else {
        const res = await enableFilter()
        setFilterActive(true)
        setFilterActiveMode(res.mode === 'hosts' ? 'hosts' : 'dns')
      }
      await load()
    } catch (e) {
      setFilterError(e instanceof Error ? e.message : 'Ошибка переключения фильтра')
    } finally {
      setFilterBusy(false)
    }
  }, [filterActive, filterBusy])

  const handleBlockDomain = useCallback(async (domain: string) => {
    const normalized = domain.replace(/\.$/,'').trim().toLowerCase()
    if (!normalized || blockingDomain) return
    setBlockingDomain(normalized)
    try {
      await blockDomain(normalized)
      await Promise.all([load(), loadQueries()])
    } finally {
      setBlockingDomain(null)
    }
  }, [load, loadQueries, blockingDomain])

  useEffect(() => {
    load()
    loadQueries()
    const t = setInterval(load, REFRESH_MS)
    const tq = setInterval(loadQueries, QUERIES_REFRESH_MS)
    return () => {
      clearInterval(t)
      clearInterval(tq)
    }
  }, [load, loadQueries])

  if (error) {
    return (
      <div className="nekkus-theme nekkus-glass-root" data-nekkus-root style={{ padding: 24 }}>
        <Card variant="default" className="nekkus-glass-card gate-card">
          <DataText>Ошибка: {error}</DataText>
          <button type="button" onClick={load}>Повторить</button>
        </Card>
      </div>
    )
  }

  if (!stats) {
    return (
      <div className="nekkus-theme nekkus-glass-root" data-nekkus-root style={{ padding: 24 }}>
        <p>Загрузка…</p>
      </div>
    )
  }

  return (
    <div className="nekkus-theme nekkus-glass-root" data-nekkus-root>
      <PageLayout>
        <AppShell
          logo="Nekkus"
          title="Gate"
          description="DNS-блокировка: реклама и трекеры."
        >
          <Section title="DNS-фильтр">
            <Card variant="elevated" moduleGlow="gate" className="nekkus-glass-card gate-card gate-filter-card">
              <div className="gate-filter-row">
                <StatusDot
                  status={filterActive ? 'online' : 'offline'}
                  label={
                    filterActive
                      ? filterActiveMode === 'hosts'
                        ? 'Фильтр включён (через файл hosts)'
                        : 'Фильтр включён (DNS 127.0.0.1)'
                      : 'Фильтр выключен'
                  }
                  pulse={!!filterActive}
                />
                <Button
                  variant={filterActive ? 'secondary' : 'primary'}
                  size="md"
                  onClick={toggleFilter}
                  disabled={filterBusy}
                >
                  {filterBusy ? '…' : filterActive ? 'Выключить' : 'Включить'}
                </Button>
              </div>
              <p className="gate-filter-hint">
                Запуск от администратора → «Включить» и всё работает (если порт 53 занят, блокировка пойдёт через файл hosts). «Выключить» — всё восстанавливается. При запуске через Hub запустите Hub от администратора, иначе смена DNS не сработает.
              </p>
              {portCheck && !portCheck.available && !filterActive && (
                <p className="gate-filter-hint gate-filter-port-hint" role="status">
                  {portCheck.suggestion ?? 'Порт 53 занят. При нажатии «Включить» будет использован режим через файл hosts.'}
                </p>
              )}
              {filterActive && filterActiveMode === 'hosts' && (
                <p className="gate-filter-hint gate-filter-hosts-note">
                  В файл hosts записано {filterBlocklistCount} доменов из блок-листа (плюс www-варианты). Если сайты не блокируются: в cmd выполните <code>ipconfig /flushdns</code>, затем перезапустите браузер.
                </p>
              )}
              {filterBlocklistCount === 0 && (
                <p className="gate-filter-warning" role="status">
                  В блок-листе 0 доменов — блокировать нечего. Добавьте домены в <code>blocklist.txt</code> (один на строку) в папке данных Gate (Windows: <code>%APPDATA%\nekkus\gate</code>), перезапустите Gate и снова нажмите «Включить».
                </p>
              )}
              {filterError && (
                <p className="gate-filter-error" role="alert">
                  {filterError}
                </p>
              )}
            </Card>
          </Section>

          <Section title="Сегодня">
            <div className="gate-overview">
              <Card variant="elevated" moduleGlow="gate" className="nekkus-glass-card gate-card gate-card--hero">
                <div className="gate-value">{formatNumber(stats.blocked_today)}</div>
                <div className="gate-label">Заблокировано</div>
                <div className="gate-extra">за сегодня</div>
              </Card>
              <Card variant="elevated" moduleGlow="gate" className="nekkus-glass-card gate-card gate-card--hero">
                <div className="gate-value">{formatNumber(stats.total_queries)}</div>
                <div className="gate-label">Запросов</div>
                <div className="gate-extra">всего</div>
              </Card>
              <Card variant="elevated" moduleGlow="gate" className="nekkus-glass-card gate-card gate-card--hero">
                <div className="gate-value">{stats.blocked_percent.toFixed(1)}%</div>
                <div className="gate-label">Блокировка</div>
                <div className="gate-extra">доля от запросов</div>
              </Card>
              <Card variant="elevated" moduleGlow="gate" className="nekkus-glass-card gate-card gate-card--hero">
                <div className="gate-value">{stats.blocklist_count}</div>
                <div className="gate-label">Доменов</div>
                <div className="gate-extra">в блок-листе</div>
              </Card>
            </div>
          </Section>

          <Section title="Privacy Score">
            <Card variant="elevated" moduleGlow="gate" className="nekkus-glass-card gate-card gate-privacy-card">
              <div className="gate-privacy-score-wrap">
                <div
                  className={`gate-privacy-score-value gate-privacy-score--${privacy ? (privacy.score >= 70 ? 'high' : privacy.score >= 40 ? 'mid' : 'low') : 'none'}`}
                  aria-label={`Privacy Score: ${privacy?.score ?? 0} из 100`}
                >
                  {privacy != null ? privacy.score : '—'}
                </div>
                <div className="gate-privacy-score-label">из 100</div>
                <p className="gate-privacy-score-desc">
                  Защита от известных трекеров: доля заблокированных запросов к трекерам. Чем выше — тем лучше.
                </p>
                {privacy != null && privacy.tracker_queries >= 0 && (
                  <p className="gate-privacy-tracker-stats" role="status">
                    Запросов к трекерам: {privacy.tracker_queries}, заблокировано: {privacy.tracker_blocked}
                  </p>
                )}
                <div className="gate-privacy-bar" role="progressbar" aria-valuenow={privacy?.score ?? 0} aria-valuemin={0} aria-valuemax={100}>
                  <div
                    className={`gate-privacy-bar-fill gate-privacy-bar-fill--${privacy ? (privacy.score >= 70 ? 'high' : privacy.score >= 40 ? 'mid' : 'low') : 'none'}`}
                    style={{ width: `${privacy?.score ?? 0}%` }}
                  />
                </div>
              </div>
              {privacy && privacy.top_blocked.length > 0 && (
                <div className="gate-privacy-top">
                  <div className="gate-privacy-top-title">Топ заблокированных (трекеры/реклама)</div>
                  <ul className="gate-top-list">
                    {privacy.top_blocked.map(({ domain, count }) => (
                      <li key={domain}>
                        <span className="gate-domain">{domain}</span>
                        <span className="gate-count">{formatNumber(count)}</span>
                      </li>
                    ))}
                  </ul>
                </div>
              )}
            </Card>
          </Section>

          {privacyApps.length > 0 && (
            <Section title="По приложениям">
              <Card variant="default" className="nekkus-glass-card gate-card gate-apps-card">
                <p className="gate-privacy-score-desc gate-apps-desc">
                  Privacy Score по приложению: доля заблокированных запросов к трекерам от этого процесса.
                </p>
                <div className="gate-queries-table-wrap">
                  <table className="gate-queries-table gate-apps-table" role="grid">
                    <thead>
                      <tr>
                        <th scope="col">Приложение</th>
                        <th scope="col">Score</th>
                        <th scope="col">Запросов</th>
                        <th scope="col">Трекеров</th>
                        <th scope="col">Заблокировано</th>
                      </tr>
                    </thead>
                    <tbody>
                      {privacyApps
                        .filter((a) => a.app_name && a.total_queries > 0)
                        .sort((a, b) => b.tracker_queries - a.tracker_queries)
                        .map((a) => (
                          <tr key={a.app_name}>
                            <td className="gate-queries-domain">{a.app_name}</td>
                            <td>
                              <span className={`gate-privacy-score-value gate-privacy-score--${a.score >= 70 ? 'high' : a.score >= 40 ? 'mid' : 'low'}`} style={{ fontSize: '1rem' }}>
                                {a.score}
                              </span>
                            </td>
                            <td>{formatNumber(a.total_queries)}</td>
                            <td>{formatNumber(a.tracker_queries)}</td>
                            <td>{formatNumber(a.tracker_blocked)}</td>
                          </tr>
                        ))}
                    </tbody>
                  </table>
                </div>
              </Card>
            </Section>
          )}

          <Section title="Последние запросы">
            <Card variant="default" className="nekkus-glass-card gate-card gate-queries-card">
              <div className="gate-queries-toolbar">
                <label className="gate-queries-filter">
                  <input
                    type="checkbox"
                    checked={showOnlyTrackers}
                    onChange={(e) => setShowOnlyTrackers(e.target.checked)}
                    aria-label="Показать только запросы к трекерам"
                  />
                  <span>Только трекеры</span>
                </label>
                {showOnlyTrackers && (
                  <span className="gate-queries-filter-hint">
                    {queries.filter((q) => q.is_tracker).length} из {queries.length}
                  </span>
                )}
              </div>
              <div className="gate-queries-table-wrap">
                <table className="gate-queries-table" role="grid">
                  <thead>
                    <tr>
                      <th scope="col">Время</th>
                      <th scope="col">Домен</th>
                      <th scope="col">Тип</th>
                      <th scope="col">Трекер</th>
                      <th scope="col">Приложение</th>
                      <th scope="col">Статус</th>
                      <th scope="col" aria-label="Действия" />
                    </tr>
                  </thead>
                  <tbody>
                    {(() => {
                      const filtered = showOnlyTrackers ? queries.filter((q) => q.is_tracker) : queries;
                      if (filtered.length === 0) {
                        return (
                          <tr>
                            <td colSpan={7} className="gate-queries-empty">
                              {showOnlyTrackers
                                ? 'Нет запросов к трекерам в последних записях. Откройте сайты — трекеры появятся здесь.'
                                : 'Нет записей. Включите фильтр и откройте сайты — запросы появятся здесь.'}
                            </td>
                          </tr>
                        );
                      }
                      return filtered.map((q, i) => (
                        <tr key={`${q.domain}-${q.timestamp}-${i}`}>
                          <td className="gate-queries-time">
                            {q.timestamp ? new Date(q.timestamp).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit', second: '2-digit' }) : '—'}
                          </td>
                          <td className="gate-queries-domain">{q.domain?.replace(/\.$/,'') || '—'}</td>
                          <td className="gate-queries-type">{q.type || '—'}</td>
                          <td>
                            {q.is_tracker ? (
                              <span className="gate-queries-tracker-badge" title="Известный трекер — можно заблокировать">
                                Трекер
                              </span>
                            ) : (
                              '—'
                            )}
                          </td>
                          <td className="gate-queries-app" title={q.app_name || undefined}>
                            {q.app_name || '—'}
                          </td>
                          <td>
                            <span className={`gate-queries-status gate-queries-status--${q.blocked ? 'blocked' : q.cached ? 'cached' : 'allowed'}`}>
                              {q.blocked ? 'Заблокирован' : q.cached ? 'Кэш' : 'Разрешён'}
                            </span>
                          </td>
                          <td>
                            <Button
                              variant="secondary"
                              size="sm"
                              onClick={() => handleBlockDomain(q.domain)}
                              disabled={!!blockingDomain || q.blocked}
                              aria-label={`Заблокировать ${q.domain}`}
                            >
                              {blockingDomain === (q.domain?.replace(/\.$/,'')?.toLowerCase()) ? '…' : 'Блок'}
                            </Button>
                          </td>
                        </tr>
                      ));
                    })()}
                  </tbody>
                </table>
              </div>
            </Card>
          </Section>

          {topBlocked.length > 0 && (
            <Section title="Топ заблокированных">
              <Card variant="default" className="nekkus-glass-card gate-card">
                <ul className="gate-top-list">
                  {topBlocked.map(({ domain, count }) => (
                    <li key={domain}>
                      <span className="gate-domain">{domain}</span>
                      <span className="gate-count">{formatNumber(count)}</span>
                    </li>
                  ))}
                </ul>
              </Card>
            </Section>
          )}

          <p className="gate-hint">
            Блок-лист: файл <code>blocklist.txt</code> в папке данных (один домен на строку). Windows: <code>%APPDATA%\nekkus\gate</code>, Linux: <code>~/.config/nekkus/gate</code>. После изменения перезапустите Gate.
          </p>
        </AppShell>
      </PageLayout>
    </div>
  )
}
