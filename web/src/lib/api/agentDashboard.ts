// API service for Agent Dashboard

export interface TimeRange {
  start_time: string
  end_time: string
}

export interface ProviderModelUsage {
  provider: string
  model: string
  request_count: number
  token_count: number
  cost_usd: number
}

export interface TokenBreakdown {
  input_tokens: number
  output_tokens: number
  thinking_tokens: number
  cost_usd: number
}

export interface TokenMetrics {
  total_input: number
  total_output: number
  total_thinking: number
  total_cost_usd: number
  by_model: Record<string, TokenBreakdown>
}

export interface CacheStatistics {
  total_hits: number
  total_misses: number
  hit_rate: number
  tokens_saved: number
  cost_saved_usd: number
}

export interface ToolCallStatistic {
  tool_name: string
  success_count: number
  failure_count: number
  total_count: number
  success_rate: number
}

export interface PolicyViolationStat {
  violation_type: string
  count: number
  avg_severity: number
}

export interface RiskAssessment {
  score: number
  level: string
  total_violations: number
  details: Record<string, number>
}

export interface AgentDashboardStats {
  api_key_id: string
  api_key_name: string
  time_range: TimeRange
  provider_usage: ProviderModelUsage[]
  token_metrics: TokenMetrics
  cache_stats: CacheStatistics
  tool_call_stats: ToolCallStatistic[]
  violations: PolicyViolationStat[]
  risk_score: RiskAssessment
}

export interface AgentInfo {
  id: string
  name: string
  key_prefix: string
  role_id?: string
  role_name?: string
  group_id?: string
  group_name?: string
  created_at: string
  last_used_at?: string
  revoked: boolean
}

export interface AgentListResponse {
  agents: AgentInfo[]
  total: number
}

// Get auth token from localStorage
function getAuthToken(): string | null {
  return localStorage.getItem('authToken')
}

// Get tenant slug from localStorage
function getTenantSlug(): string | null {
  return localStorage.getItem('tenantSlug')
}

// Base API URL (adjust as needed)
const API_BASE_URL = '/v1'

// Fetch agent dashboard stats
export async function fetchAgentDashboardStats(
  apiKeyId: string,
  startTime: string,
  endTime: string
): Promise<AgentDashboardStats> {
  const token = getAuthToken()
  const tenantSlug = getTenantSlug()
  const params = new URLSearchParams({
    api_key_id: apiKeyId,
    start_time: startTime,
    end_time: endTime,
  })

  const response = await fetch(`${API_BASE_URL}/agents/dashboard/stats?${params}`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
      ...(token && { Authorization: `Bearer ${token}` }),
      ...(tenantSlug && { 'X-Tenant': tenantSlug }),
    },
  })

  if (!response.ok) {
    const error = await response.text()
    throw new Error(`Failed to fetch dashboard stats: ${error}`)
  }

  return response.json()
}

// Fetch risk assessment only
export async function fetchRiskAssessment(
  apiKeyId: string,
  startTime: string,
  endTime: string
): Promise<RiskAssessment> {
  const token = getAuthToken()
  const tenantSlug = getTenantSlug()
  const params = new URLSearchParams({
    api_key_id: apiKeyId,
    start_time: startTime,
    end_time: endTime,
  })

  const response = await fetch(`${API_BASE_URL}/agents/dashboard/risk?${params}`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
      ...(token && { Authorization: `Bearer ${token}` }),
      ...(tenantSlug && { 'X-Tenant': tenantSlug }),
    },
  })

  if (!response.ok) {
    const error = await response.text()
    throw new Error(`Failed to fetch risk assessment: ${error}`)
  }

  return response.json()
}

// List all agents (API keys)
export async function fetchAgentList(): Promise<AgentListResponse> {
  const token = getAuthToken()
  const tenantSlug = getTenantSlug()

  console.log('[Agent Dashboard] Fetching agent list', { token: token ? 'present' : 'missing', tenantSlug })

  const response = await fetch(`${API_BASE_URL}/agents/list`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
      ...(token && { Authorization: `Bearer ${token}` }),
      ...(tenantSlug && { 'X-Tenant': tenantSlug }),
    },
  })

  console.log('[Agent Dashboard] Agent list response', { status: response.status, ok: response.ok })

  if (!response.ok) {
    const error = await response.text()
    console.error('[Agent Dashboard] Agent list error', error)
    throw new Error(`Failed to fetch agent list: ${error}`)
  }

  const data = await response.json()
  console.log('[Agent Dashboard] Agent list data', data)
  return data
}

// Helper function to format date to ISO8601
export function formatDateToISO(date: Date): string {
  return date.toISOString()
}

// Helper to get default time range (last 24 hours)
export function getDefaultTimeRange(): { start_time: string; end_time: string } {
  const endTime = new Date()
  const startTime = new Date(endTime.getTime() - 24 * 60 * 60 * 1000)

  return {
    start_time: formatDateToISO(startTime),
    end_time: formatDateToISO(endTime),
  }
}

// Helper to get time range for a specific period
export function getTimeRange(period: '24h' | '7d' | '30d' | 'custom', customStart?: Date, customEnd?: Date): { start_time: string; end_time: string } {
  const endTime = new Date()
  let startTime: Date

  switch (period) {
    case '24h':
      startTime = new Date(endTime.getTime() - 24 * 60 * 60 * 1000)
      break
    case '7d':
      startTime = new Date(endTime.getTime() - 7 * 24 * 60 * 60 * 1000)
      break
    case '30d':
      startTime = new Date(endTime.getTime() - 30 * 24 * 60 * 60 * 1000)
      break
    case 'custom':
      if (!customStart || !customEnd) {
        throw new Error('Custom period requires start and end dates')
      }
      return {
        start_time: formatDateToISO(customStart),
        end_time: formatDateToISO(customEnd),
      }
    default:
      startTime = new Date(endTime.getTime() - 24 * 60 * 60 * 1000)
  }

  return {
    start_time: formatDateToISO(startTime),
    end_time: formatDateToISO(endTime),
  }
}
