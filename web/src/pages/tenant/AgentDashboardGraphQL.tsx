import { useState, useEffect, useMemo } from 'react'
import { useQuery } from '@apollo/client'
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
  CheckCircle,
  XCircle,
  TrendingUp,
} from 'lucide-react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Badge } from '@/components/ui/badge'
import { formatCurrency, formatNumber } from '@/lib/utils'
import { GET_AGENT_DASHBOARD, GET_API_KEYS } from '@/graphql/operations'

const COLORS = ['#8b5cf6', '#06b6d4', '#10b981', '#f59e0b', '#ef4444', '#ec4899']

type TimeRange = '24h' | '7d' | '30d' | '90d'

export function AgentDashboardPage() {
  const [selectedAgent, setSelectedAgent] = useState<string>('')
  const [timeRange, setTimeRange] = useState<TimeRange>('7d')

  // Fetch agents list
  const { data: apiKeysData, loading: loadingAgents } = useQuery(GET_API_KEYS)

  // Memoize agents list to prevent recreating array on every render
  const agents = useMemo(() => {
    return apiKeysData?.apiKeys?.filter((a: any) => !a.revoked) || []
  }, [apiKeysData])

  // Set default agent when data loads (using useEffect to avoid infinite loop)
  useEffect(() => {
    if (agents.length > 0 && !selectedAgent) {
      setSelectedAgent(agents[0].id)
    }
  }, [agents, selectedAgent])

  // Memoize time range calculation to prevent recreating on every render
  const timeRangeValues = useMemo(() => {
    const end = new Date()
    const start = new Date()
    switch (timeRange) {
      case '24h':
        start.setHours(start.getHours() - 24)
        break
      case '7d':
        start.setDate(start.getDate() - 7)
        break
      case '30d':
        start.setDate(start.getDate() - 30)
        break
      case '90d':
        start.setDate(start.getDate() - 90)
        break
    }
    return {
      start_time: start.toISOString(),
      end_time: end.toISOString(),
    }
  }, [timeRange])

  // Fetch dashboard stats for selected agent
  const { data, loading, error } = useQuery(GET_AGENT_DASHBOARD, {
    variables: {
      apiKeyId: selectedAgent,
      startTime: timeRangeValues.start_time,
      endTime: timeRangeValues.end_time,
    },
    skip: !selectedAgent,
    pollInterval: 30000, // Refresh every 30 seconds
  })

  const stats = data?.agentDashboard

  if (loadingAgents) {
    return (
      <div className="flex h-64 items-center justify-center">
        <div className="animate-pulse text-muted-foreground">Loading agents...</div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="rounded-lg bg-destructive/10 p-4 text-destructive">
        Failed to load agent dashboard: {error.message}
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-3xl font-bold tracking-tight">Agent Dashboard</h2>
        <p className="text-muted-foreground">
          Monitor your AI agents' performance, usage, and security metrics
        </p>
      </div>

      {/* Agent and Time Range Selection */}
      <div className="flex gap-4">
        <div className="w-64">
          <label className="text-sm font-medium mb-2 block">Select Agent</label>
          <Select value={selectedAgent} onValueChange={setSelectedAgent}>
            <SelectTrigger>
              <SelectValue placeholder="Select an agent" />
            </SelectTrigger>
            <SelectContent>
              {agents.map((agent: any) => (
                <SelectItem key={agent.id} value={agent.id}>
                  {agent.name} ({agent.keyPrefix})
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="w-48">
          <label className="text-sm font-medium mb-2 block">Time Range</label>
          <Select value={timeRange} onValueChange={(v) => setTimeRange(v as TimeRange)}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="24h">Last 24 hours</SelectItem>
              <SelectItem value="7d">Last 7 days</SelectItem>
              <SelectItem value="30d">Last 30 days</SelectItem>
              <SelectItem value="90d">Last 90 days</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      {loading && (
        <div className="flex h-64 items-center justify-center">
          <div className="animate-pulse text-muted-foreground">Loading dashboard...</div>
        </div>
      )}

      {stats && (
        <>
          {/* Overview Cards */}
          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-5">
            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">Total Tokens</CardTitle>
                <Activity className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">
                  {formatNumber(
                    stats.tokenMetrics.totalInput +
                      stats.tokenMetrics.totalOutput +
                      stats.tokenMetrics.totalThinking
                  )}
                </div>
                <p className="text-xs text-muted-foreground">
                  {formatNumber(stats.tokenMetrics.totalInput)} input •{' '}
                  {formatNumber(stats.tokenMetrics.totalOutput)} output
                </p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">Total Cost</CardTitle>
                <DollarSign className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">
                  {formatCurrency(stats.tokenMetrics.totalCost)}
                </div>
                <p className="text-xs text-muted-foreground">
                  {stats.cacheMetrics.costSaved > 0 &&
                    `${formatCurrency(stats.cacheMetrics.costSaved)} saved via cache`}
                </p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">Cache Hit Rate</CardTitle>
                <Zap className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">
                  {stats.cacheMetrics.hitRate.toFixed(1)}%
                </div>
                <p className="text-xs text-muted-foreground">
                  {formatNumber(stats.cacheMetrics.tokensSaved)} tokens saved
                </p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">Risk Score</CardTitle>
                <Shield className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">
                  {stats.riskAssessment.overallRiskScore.toFixed(1)}
                </div>
                <Badge
                  variant={
                    stats.riskAssessment.riskLevel === 'low'
                      ? 'default'
                      : stats.riskAssessment.riskLevel === 'medium'
                      ? 'secondary'
                      : 'destructive'
                  }
                >
                  {stats.riskAssessment.riskLevel}
                </Badge>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">Policy Violations</CardTitle>
                <AlertTriangle className={`h-4 w-4 ${stats.riskAssessment.policyViolations.length > 0 ? 'text-orange-500' : 'text-muted-foreground'}`} />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">
                  {stats.riskAssessment.policyViolations.reduce((sum: number, v: any) => sum + v.count, 0)}
                </div>
                <p className="text-xs text-muted-foreground">
                  {stats.riskAssessment.policyViolations.length} violation types
                </p>
              </CardContent>
            </Card>
          </div>

          {/* Charts Grid */}
          <div className="grid gap-4 md:grid-cols-2">
            {/* Provider/Model Usage */}
            <Card>
              <CardHeader>
                <CardTitle>Provider & Model Usage</CardTitle>
                <CardDescription>Request distribution by provider and model</CardDescription>
              </CardHeader>
              <CardContent>
                <ResponsiveContainer width="100%" height={300}>
                  <BarChart data={stats.providerModelUsage}>
                    <CartesianGrid strokeDasharray="3 3" />
                    <XAxis dataKey="model" angle={-45} textAnchor="end" height={100} />
                    <YAxis />
                    <Tooltip />
                    <Bar dataKey="requestCount" fill="#8b5cf6" />
                  </BarChart>
                </ResponsiveContainer>
              </CardContent>
            </Card>

            {/* Tool Call Success Rate */}
            <Card>
              <CardHeader>
                <CardTitle>Tool Calls</CardTitle>
                <CardDescription>
                  Success rate: {stats.toolCallMetrics.successRate.toFixed(1)}%
                </CardDescription>
              </CardHeader>
              <CardContent>
                <div className="space-y-4">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <CheckCircle className="h-4 w-4 text-green-500" />
                      <span>Successful</span>
                    </div>
                    <span className="font-bold">{formatNumber(stats.toolCallMetrics.successCount)}</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <XCircle className="h-4 w-4 text-red-500" />
                      <span>Failed</span>
                    </div>
                    <span className="font-bold">{formatNumber(stats.toolCallMetrics.failureCount)}</span>
                  </div>
                  {stats.toolCallMetrics.byTool.length > 0 && (
                    <div className="mt-4">
                      <h4 className="text-sm font-medium mb-2">By Tool</h4>
                      <ResponsiveContainer width="100%" height={200}>
                        <BarChart data={stats.toolCallMetrics.byTool}>
                          <CartesianGrid strokeDasharray="3 3" />
                          <XAxis dataKey="toolName" angle={-45} textAnchor="end" height={100} />
                          <YAxis />
                          <Tooltip />
                          <Bar dataKey="successCount" fill="#10b981" name="Success" />
                          <Bar dataKey="failureCount" fill="#ef4444" name="Failure" />
                        </BarChart>
                      </ResponsiveContainer>
                    </div>
                  )}
                </div>
              </CardContent>
            </Card>
          </div>

          {/* Policy Violations - Always Visible */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                Policy Violations & Security
                <Badge variant={stats.riskAssessment.policyViolations.length > 0 ? 'destructive' : 'default'}>
                  {stats.riskAssessment.policyViolations.length > 0
                    ? `${stats.riskAssessment.policyViolations.reduce((sum: number, v: any) => sum + v.count, 0)} violations`
                    : 'No violations'}
                </Badge>
              </CardTitle>
              <CardDescription>Security and policy compliance monitoring</CardDescription>
            </CardHeader>
            <CardContent>
              {stats.riskAssessment.policyViolations.length > 0 ? (
                <>
                  <div className="space-y-4">
                    {stats.riskAssessment.policyViolations.map((violation: any) => (
                      <div key={violation.violationType} className="flex items-center justify-between p-3 rounded-lg border bg-card">
                        <div className="flex items-center gap-3">
                          <AlertTriangle className="h-5 w-5 text-orange-500" />
                          <div>
                            <div className="font-medium">{violation.violationType}</div>
                            <div className="text-sm text-muted-foreground">
                              Average severity: {violation.avgSeverity.toFixed(1)}/5
                            </div>
                          </div>
                        </div>
                        <Badge variant="outline" className="text-base font-semibold">
                          {violation.count} occurrences
                        </Badge>
                      </div>
                    ))}
                  </div>

                  {stats.riskAssessment.recommendations.length > 0 && (
                    <div className="mt-6 p-4 rounded-lg bg-muted/50">
                      <h4 className="text-sm font-semibold mb-3 flex items-center gap-2">
                        <TrendingUp className="h-4 w-4" />
                        Security Recommendations
                      </h4>
                      <ul className="space-y-2">
                        {stats.riskAssessment.recommendations.map((rec: string, i: number) => (
                          <li key={i} className="text-sm flex items-start gap-2">
                            <CheckCircle className="h-4 w-4 text-green-500 mt-0.5 flex-shrink-0" />
                            <span>{rec}</span>
                          </li>
                        ))}
                      </ul>
                    </div>
                  )}
                </>
              ) : (
                <div className="flex flex-col items-center justify-center py-8 text-center">
                  <CheckCircle className="h-12 w-12 text-green-500 mb-3" />
                  <h3 className="text-lg font-semibold mb-1">All Clear!</h3>
                  <p className="text-sm text-muted-foreground max-w-md">
                    No policy violations detected in the selected time range. Your agent is operating within all defined security policies.
                  </p>
                </div>
              )}
            </CardContent>
          </Card>
        </>
      )}
    </div>
  )
}
