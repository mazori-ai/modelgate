import { useState, useEffect } from 'react'
import {
  BarChart,
  Bar,
  PieChart,
  Pie,
  Cell,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend,
} from 'recharts'
import {
  Activity,
  Shield,
  Zap,
  DollarSign,
  AlertTriangle,
  CheckCircle2,
  XCircle,
  Database,
  Clock,
} from 'lucide-react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  fetchAgentDashboardStats,
  fetchAgentList,
  getTimeRange,
  type AgentDashboardStats,
  type AgentInfo,
} from '@/lib/api/agentDashboard'
import { formatNumber, formatCurrency } from '@/lib/utils'

const COLORS = ['#8b5cf6', '#06b6d4', '#10b981', '#f59e0b', '#ef4444', '#ec4899']

export function AgentDashboardPage() {
  const [agents, setAgents] = useState<AgentInfo[]>([])
  const [selectedAgent, setSelectedAgent] = useState<string>('')
  const [timeRange, setTimeRange] = useState<'24h' | '7d' | '30d'>('24h')
  const [stats, setStats] = useState<AgentDashboardStats | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Fetch agents list on mount
  useEffect(() => {
    fetchAgentList()
      .then((response) => {
        setAgents(response.agents.filter((a) => !a.revoked))
        if (response.agents.length > 0) {
          setSelectedAgent(response.agents[0].id)
        }
      })
      .catch((err) => setError(err.message))
  }, [])

  // Fetch dashboard stats when agent or time range changes
  useEffect(() => {
    if (!selectedAgent) return

    setLoading(true)
    setError(null)

    const { start_time, end_time } = getTimeRange(timeRange)

    fetchAgentDashboardStats(selectedAgent, start_time, end_time)
      .then((data) => {
        setStats(data)
        setLoading(false)
      })
      .catch((err) => {
        setError(err.message)
        setLoading(false)
      })
  }, [selectedAgent, timeRange])

  if (loading && !stats) {
    return (
      <div className="flex h-64 items-center justify-center">
        <div className="animate-pulse text-muted-foreground">Loading agent dashboard...</div>
      </div>
    )
  }

  if (error && !stats) {
    return (
      <div className="rounded-lg bg-destructive/10 p-4 text-destructive">
        Failed to load agent dashboard: {error}
      </div>
    )
  }

  const selectedAgentInfo = agents.find((a) => a.id === selectedAgent)

  // Risk level colors and text
  const getRiskColor = (level: string) => {
    switch (level) {
      case 'critical':
        return 'bg-red-500'
      case 'high':
        return 'bg-orange-500'
      case 'medium':
        return 'bg-yellow-500'
      case 'low':
        return 'bg-green-500'
      default:
        return 'bg-gray-500'
    }
  }

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">Agent Dashboard</h1>
          <p className="text-muted-foreground">
            Monitor agent activity, usage, and security risks
          </p>
        </div>

        <div className="flex gap-4">
          {/* Agent Selector */}
          <Select value={selectedAgent} onValueChange={setSelectedAgent}>
            <SelectTrigger className="w-[280px]">
              <SelectValue placeholder="Select an agent" />
            </SelectTrigger>
            <SelectContent>
              {agents.map((agent) => (
                <SelectItem key={agent.id} value={agent.id}>
                  <div className="flex items-center gap-2">
                    <span className="font-medium">{agent.name}</span>
                    <span className="text-xs text-muted-foreground">
                      ({agent.key_prefix}...)
                    </span>
                  </div>
                </SelectItem>
              ))}
            </SelectContent>
          </Select>

          {/* Time Range Selector */}
          <Select value={timeRange} onValueChange={(v) => setTimeRange(v as any)}>
            <SelectTrigger className="w-[180px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="24h">Last 24 Hours</SelectItem>
              <SelectItem value="7d">Last 7 Days</SelectItem>
              <SelectItem value="30d">Last 30 Days</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      {stats && (
        <>
          {/* Agent Info Card */}
          {selectedAgentInfo && (
            <Card>
              <CardHeader>
                <CardTitle>Agent Information</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
                  <div>
                    <p className="text-sm text-muted-foreground">Name</p>
                    <p className="font-medium">{selectedAgentInfo.name}</p>
                  </div>
                  <div>
                    <p className="text-sm text-muted-foreground">Key Prefix</p>
                    <p className="font-mono text-sm">{selectedAgentInfo.key_prefix}...</p>
                  </div>
                  <div>
                    <p className="text-sm text-muted-foreground">Role</p>
                    <p className="font-medium">{selectedAgentInfo.role_name || 'N/A'}</p>
                  </div>
                  <div>
                    <p className="text-sm text-muted-foreground">Last Used</p>
                    <p className="text-sm">
                      {selectedAgentInfo.last_used_at
                        ? new Date(selectedAgentInfo.last_used_at).toLocaleString()
                        : 'Never'}
                    </p>
                  </div>
                </div>
              </CardContent>
            </Card>
          )}

          {/* Risk Assessment Card */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Shield className="h-5 w-5" />
                Risk Assessment
              </CardTitle>
              <CardDescription>
                Security risk level based on policy violations
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="flex items-center gap-6">
                <div className="relative h-32 w-32">
                  <div className="absolute inset-0 flex items-center justify-center">
                    <div className="text-center">
                      <div className="text-3xl font-bold">{stats.risk_score.score.toFixed(0)}</div>
                      <div className="text-sm text-muted-foreground">Score</div>
                    </div>
                  </div>
                  <svg className="h-32 w-32 -rotate-90 transform">
                    <circle
                      cx="64"
                      cy="64"
                      r="52"
                      stroke="currentColor"
                      strokeWidth="8"
                      fill="none"
                      className="text-gray-200"
                    />
                    <circle
                      cx="64"
                      cy="64"
                      r="52"
                      stroke="currentColor"
                      strokeWidth="8"
                      fill="none"
                      strokeDasharray={`${(stats.risk_score.score / 100) * 326.73} 326.73`}
                      className={getRiskColor(stats.risk_score.level).replace('bg-', 'text-')}
                    />
                  </svg>
                </div>

                <div className="flex-1 space-y-4">
                  <div className="flex items-center gap-2">
                    <Badge
                      className={`${getRiskColor(stats.risk_score.level)} text-white`}
                      variant="default"
                    >
                      {stats.risk_score.level.toUpperCase()}
                    </Badge>
                    <span className="text-sm text-muted-foreground">
                      {stats.risk_score.total_violations} total violations
                    </span>
                  </div>

                  <div className="space-y-2">
                    {Object.entries(stats.risk_score.details).map(([type, score]) => (
                      <div key={type} className="flex items-center justify-between text-sm">
                        <span className="capitalize">{type.replace('_', ' ')}</span>
                        <span className="font-medium">{score.toFixed(1)} pts</span>
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Stat Cards */}
          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
            <Card>
              <CardHeader className="flex flex-row items-center justify-between pb-2">
                <CardTitle className="text-sm font-medium text-muted-foreground">
                  Total Tokens
                </CardTitle>
                <Zap className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">
                  {formatNumber(
                    stats.token_metrics.total_input + stats.token_metrics.total_output
                  )}
                </div>
                <p className="text-xs text-muted-foreground">
                  {formatNumber(stats.token_metrics.total_input)} in /{' '}
                  {formatNumber(stats.token_metrics.total_output)} out
                </p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between pb-2">
                <CardTitle className="text-sm font-medium text-muted-foreground">
                  Total Cost
                </CardTitle>
                <DollarSign className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">
                  {formatCurrency(stats.token_metrics.total_cost_usd)}
                </div>
                <p className="text-xs text-muted-foreground">Across all providers</p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between pb-2">
                <CardTitle className="text-sm font-medium text-muted-foreground">
                  Cache Hit Rate
                </CardTitle>
                <Database className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">
                  {stats.cache_stats.hit_rate.toFixed(1)}%
                </div>
                <p className="text-xs text-muted-foreground">
                  Saved {formatCurrency(stats.cache_stats.cost_saved_usd)}
                </p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between pb-2">
                <CardTitle className="text-sm font-medium text-muted-foreground">
                  Policy Violations
                </CardTitle>
                <AlertTriangle className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">
                  {stats.violations.reduce((sum, v) => sum + v.count, 0)}
                </div>
                <p className="text-xs text-muted-foreground">
                  {stats.violations.length} types detected
                </p>
              </CardContent>
            </Card>
          </div>

          {/* Provider & Model Usage */}
          <Card>
            <CardHeader>
              <CardTitle>Provider & Model Usage</CardTitle>
              <CardDescription>Distribution of requests across providers and models</CardDescription>
            </CardHeader>
            <CardContent>
              <ResponsiveContainer width="100%" height={300}>
                <BarChart data={stats.provider_usage}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis dataKey="model" angle={-45} textAnchor="end" height={100} />
                  <YAxis />
                  <Tooltip />
                  <Legend />
                  <Bar dataKey="request_count" fill="#8b5cf6" name="Requests" />
                </BarChart>
              </ResponsiveContainer>
            </CardContent>
          </Card>

          {/* Token Usage by Model */}
          <div className="grid gap-4 md:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle>Token Distribution by Model</CardTitle>
                <CardDescription>Input vs Output tokens</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="space-y-4">
                  {Object.entries(stats.token_metrics.by_model).slice(0, 5).map(([model, breakdown]) => (
                    <div key={model} className="space-y-2">
                      <div className="flex items-center justify-between text-sm">
                        <span className="font-medium">{model}</span>
                        <span className="text-muted-foreground">
                          {formatNumber(breakdown.input_tokens + breakdown.output_tokens)} tokens
                        </span>
                      </div>
                      <div className="flex h-2 overflow-hidden rounded-full bg-gray-200">
                        <div
                          className="bg-blue-500"
                          style={{
                            width: `${
                              (breakdown.input_tokens /
                                (breakdown.input_tokens + breakdown.output_tokens)) *
                              100
                            }%`,
                          }}
                        />
                        <div
                          className="bg-green-500"
                          style={{
                            width: `${
                              (breakdown.output_tokens /
                                (breakdown.input_tokens + breakdown.output_tokens)) *
                              100
                            }%`,
                          }}
                        />
                      </div>
                      <div className="flex justify-between text-xs text-muted-foreground">
                        <span>Input: {formatNumber(breakdown.input_tokens)}</span>
                        <span>Output: {formatNumber(breakdown.output_tokens)}</span>
                      </div>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>

            {/* Tool Calls */}
            <Card>
              <CardHeader>
                <CardTitle>Tool Calls</CardTitle>
                <CardDescription>Success and failure rates</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="space-y-4">
                  {stats.tool_call_stats.slice(0, 5).map((tool) => (
                    <div key={tool.tool_name} className="flex items-center justify-between">
                      <div className="flex-1">
                        <p className="text-sm font-medium">{tool.tool_name}</p>
                        <div className="flex items-center gap-4 text-xs text-muted-foreground">
                          <span className="flex items-center gap-1">
                            <CheckCircle2 className="h-3 w-3 text-green-500" />
                            {tool.success_count} success
                          </span>
                          <span className="flex items-center gap-1">
                            <XCircle className="h-3 w-3 text-red-500" />
                            {tool.failure_count} failed
                          </span>
                        </div>
                      </div>
                      <div className="text-sm font-medium">{tool.success_rate.toFixed(1)}%</div>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>
          </div>

          {/* Policy Violations */}
          {stats.violations.length > 0 && (
            <Card>
              <CardHeader>
                <CardTitle>Policy Violations</CardTitle>
                <CardDescription>Breakdown by violation type and severity</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="grid gap-4 md:grid-cols-2">
                  <div>
                    <ResponsiveContainer width="100%" height={250}>
                      <PieChart>
                        <Pie
                          data={stats.violations as any}
                          dataKey="count"
                          nameKey="violation_type"
                          cx="50%"
                          cy="50%"
                          outerRadius={80}
                          label
                        >
                          {stats.violations.map((entry, index) => (
                            <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
                          ))}
                        </Pie>
                        <Tooltip />
                        <Legend />
                      </PieChart>
                    </ResponsiveContainer>
                  </div>
                  <div className="space-y-3">
                    {stats.violations.map((violation, index) => (
                      <div key={violation.violation_type} className="flex items-center gap-3">
                        <div
                          className="h-4 w-4 rounded"
                          style={{ backgroundColor: COLORS[index % COLORS.length] }}
                        />
                        <div className="flex-1">
                          <p className="text-sm font-medium capitalize">
                            {violation.violation_type.replace('_', ' ')}
                          </p>
                          <p className="text-xs text-muted-foreground">
                            {violation.count} violations • Avg severity: {violation.avg_severity.toFixed(1)}
                          </p>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              </CardContent>
            </Card>
          )}

          {/* Cache Statistics */}
          <Card>
            <CardHeader>
              <CardTitle>Cache Performance</CardTitle>
              <CardDescription>Semantic cache efficiency and savings</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="grid gap-4 md:grid-cols-3">
                <div className="space-y-2">
                  <p className="text-sm text-muted-foreground">Total Hits</p>
                  <p className="text-2xl font-bold">{formatNumber(stats.cache_stats.total_hits)}</p>
                </div>
                <div className="space-y-2">
                  <p className="text-sm text-muted-foreground">Total Misses</p>
                  <p className="text-2xl font-bold">{formatNumber(stats.cache_stats.total_misses)}</p>
                </div>
                <div className="space-y-2">
                  <p className="text-sm text-muted-foreground">Tokens Saved</p>
                  <p className="text-2xl font-bold">{formatNumber(stats.cache_stats.tokens_saved)}</p>
                </div>
              </div>
              <div className="mt-4 flex h-4 overflow-hidden rounded-full bg-gray-200">
                <div
                  className="bg-green-500"
                  style={{
                    width: `${
                      (stats.cache_stats.total_hits /
                        (stats.cache_stats.total_hits + stats.cache_stats.total_misses)) *
                      100
                    }%`,
                  }}
                />
                <div className="flex-1 bg-red-500" />
              </div>
              <div className="mt-2 flex justify-between text-xs text-muted-foreground">
                <span>Hit Rate: {stats.cache_stats.hit_rate.toFixed(1)}%</span>
                <span>Cost Saved: {formatCurrency(stats.cache_stats.cost_saved_usd)}</span>
              </div>
            </CardContent>
          </Card>
        </>
      )}
    </div>
  )
}

export default AgentDashboardPage
