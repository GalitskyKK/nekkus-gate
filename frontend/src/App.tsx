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
  enableFilter,
  disableFilter,
  type GateStats,
  type TopBlockedEntry,
} from './api'

const REFRESH_MS = 3000

function formatNumber(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return String(n)
}

export default function App() {
  const [stats, setStats] = useState<GateStats | null>(null)
  const [topBlocked, setTopBlocked] = useState<TopBlockedEntry[]>([])
  const [filterActive, setFilterActive] = useState<boolean | null>(null)
  const [filterBusy, setFilterBusy] = useState(false)
  const [filterError, setFilterError] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    try {
      setError(null)
      const [s, t, f] = await Promise.all([
        fetchStats(),
        fetchTopBlocked(10).catch(() => []),
        fetchFilterStatus().catch(() => ({ active: false, port: 5354 })),
      ])
      setStats(s)
      setTopBlocked(t)
      setFilterActive(f.active)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Ошибка загрузки')
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
        await enableFilter()
        setFilterActive(true)
      }
    } catch (e) {
      setFilterError(e instanceof Error ? e.message : 'Ошибка переключения фильтра')
    } finally {
      setFilterBusy(false)
    }
  }, [filterActive, filterBusy])

  useEffect(() => {
    load()
    const t = setInterval(load, REFRESH_MS)
    return () => clearInterval(t)
  }, [load])

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
                  label={filterActive ? 'Фильтр включён — система использует 127.0.0.1' : 'Фильтр выключен — DNS из настроек системы'}
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
                Запуск от администратора → один клик «Включить», DNS везде идёт через Gate. «Выключить» — настройки системы восстанавливаются.
              </p>
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
            Блок-лист: <code>blocklist.txt</code> в папке данных Gate.
          </p>
        </AppShell>
      </PageLayout>
    </div>
  )
}
