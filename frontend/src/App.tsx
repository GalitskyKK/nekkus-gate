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
  enableFilter,
  disableFilter,
  type GateStats,
  type TopBlockedEntry,
  type PortCheck,
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
  const [filterActiveMode, setFilterActiveMode] = useState<'dns' | 'hosts'>('dns')
  const [filterBlocklistCount, setFilterBlocklistCount] = useState(0)
  const [filterBusy, setFilterBusy] = useState(false)
  const [filterError, setFilterError] = useState<string | null>(null)
  const [portCheck, setPortCheck] = useState<PortCheck | null>(null)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    try {
      setError(null)
      const [s, t, f, pc] = await Promise.all([
        fetchStats(),
        fetchTopBlocked(10).catch(() => []),
        fetchFilterStatus().catch(() => ({ active: false, mode: 'dns' as const, port: 5354, blocklist_count: 0 })),
        fetchPortCheck().catch(() => ({ available: true })),
      ])
      setStats(s)
      setTopBlocked(t)
      setFilterActive(f.active)
      setFilterActiveMode((f.mode === 'hosts' ? 'hosts' : 'dns'))
      setFilterBlocklistCount(f.blocklist_count ?? 0)
      setPortCheck(pc)
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
        const res = await enableFilter()
        setFilterActive(true)
        setFilterActiveMode(res.mode === 'hosts' ? 'hosts' : 'dns')
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
