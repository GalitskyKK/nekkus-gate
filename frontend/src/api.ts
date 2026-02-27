const BASE = ''

export interface GateStats {
  total_queries: number
  blocked_today: number
  blocked_total: number
  blocked_percent: number
  blocklist_count: number
  timestamp: number
}

export interface TopBlockedEntry {
  domain: string
  count: number
}

export async function fetchStats(): Promise<GateStats> {
  const res = await fetch(`${BASE}/api/stats`)
  if (!res.ok) throw new Error(res.statusText)
  return res.json()
}

export async function fetchTopBlocked(limit?: number): Promise<TopBlockedEntry[]> {
  const url = limit != null ? `${BASE}/api/top_blocked?limit=${limit}` : `${BASE}/api/top_blocked`
  const res = await fetch(url)
  if (!res.ok) throw new Error(res.statusText)
  return res.json()
}

export type FilterMode = 'dns' | 'hosts'

export interface FilterStatus {
  active: boolean
  mode: FilterMode
  port: number
  blocklist_count?: number
  helper_running?: boolean
}

export async function fetchFilterStatus(): Promise<FilterStatus> {
  const res = await fetch(`${BASE}/api/filter/status`)
  if (!res.ok) throw new Error(res.statusText)
  return res.json()
}

export async function enableFilter(): Promise<{ ok: boolean; active: boolean; mode?: FilterMode }> {
  const res = await fetch(`${BASE}/api/filter/enable`, { method: 'POST' })
  const data = await res.json()
  if (!res.ok) throw new Error((data as { error?: string }).error || res.statusText)
  return data
}

export async function disableFilter(): Promise<{ ok: boolean; active: boolean }> {
  const res = await fetch(`${BASE}/api/filter/disable`, { method: 'POST' })
  const data = await res.json()
  if (!res.ok) throw new Error((data as { error?: string }).error || res.statusText)
  return data
}

export async function installHelper(): Promise<{ ok: boolean }> {
  const res = await fetch(`${BASE}/api/helper/install`, { method: 'POST' })
  const data = await res.json()
  if (!res.ok) throw new Error((data as { error?: string }).error || res.statusText)
  return data
}

export interface PortCheck {
  available: boolean
  blocked_by?: string
  blocker_pid?: number
  suggestion?: string
}

export async function fetchPortCheck(): Promise<PortCheck> {
  const res = await fetch(`${BASE}/api/dns/port-check`)
  if (!res.ok) throw new Error(res.statusText)
  return res.json()
}

export interface QueryLogEntry {
  timestamp: string
  domain: string
  type: string
  blocked: boolean
  cached: boolean
  rule?: string
  latency_ms: number
  is_tracker?: boolean
  app_name?: string
}

export async function fetchQueries(limit = 100): Promise<QueryLogEntry[]> {
  const res = await fetch(`${BASE}/api/queries?limit=${limit}`)
  if (!res.ok) throw new Error(res.statusText)
  const data = await res.json()
  return Array.isArray(data) ? data : []
}

export async function blockDomain(domain: string): Promise<{ ok: boolean; blocklist_count: number }> {
  const res = await fetch(`${BASE}/api/block`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ domain: domain.replace(/\.$/,'') }),
  })
  const data = await res.json()
  if (!res.ok) throw new Error((data as { error?: string }).error || res.statusText)
  return data
}

export interface PrivacyData {
  score: number
  total_queries: number
  tracker_queries: number
  tracker_blocked: number
  blocked_percent: number
  top_blocked: TopBlockedEntry[]
}

export async function fetchPrivacy(): Promise<PrivacyData> {
  const res = await fetch(`${BASE}/api/privacy`)
  if (!res.ok) throw new Error(res.statusText)
  return res.json()
}

export interface AppPrivacyStats {
  app_name: string
  score: number
  total_queries: number
  tracker_queries: number
  tracker_blocked: number
}

export async function fetchPrivacyApps(): Promise<AppPrivacyStats[]> {
  const res = await fetch(`${BASE}/api/privacy/apps`)
  if (!res.ok) throw new Error(res.statusText)
  const data = await res.json()
  return Array.isArray(data) ? data : []
}
