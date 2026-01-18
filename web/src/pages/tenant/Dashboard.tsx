import { useQuery } from '@apollo/client'
import { useParams } from 'react-router-dom'
import {
  LineChart,
  Line,
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
import { Activity, DollarSign, Zap, Clock, TrendingUp, TrendingDown } from 'lucide-react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { GET_DASHBOARD } from '@/graphql/operations'
import { formatNumber, formatCurrency, providerColors } from '@/lib/utils'

const COLORS = ['#8b5cf6', '#06b6d4', '#10b981', '#f59e0b', '#ef4444', '#ec4899']

export function DashboardPage() {
  const { tenant } = useParams()
  const { data, loading, error } = useQuery(GET_DASHBOARD, {
    pollInterval: 30000, // Refresh every 30 seconds
  })

  if (loading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <div className="animate-pulse text-muted-foreground">Loading dashboard...</div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="rounded-lg bg-destructive/10 p-4 text-destructive">
        Failed to load dashboard: {error.message}
      </div>
    )
  }

  const stats = data?.dashboard

  const statCards = [
    {
      title: 'Total Requests',
      value: formatNumber(stats?.totalRequests || 0),
      icon: Activity,
      trend: '+12.5%',
      trendUp: true,
    },
    {
      title: 'Total Tokens',
      value: formatNumber(stats?.totalTokens || 0),
      icon: Zap,
      trend: '+8.2%',
      trendUp: true,
    },
    {
      title: 'Total Cost',
      value: formatCurrency(stats?.totalCostUSD || 0),
      icon: DollarSign,
      trend: '+5.4%',
      trendUp: true,
    },
    {
      title: 'Avg Latency',
      value: `${(stats?.avgLatencyMs || 0).toFixed(0)}ms`,
      icon: Clock,
      trend: '-3.2%',
      trendUp: false,
    },
  ]

  return (
    <div className="space-y-8">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold">Dashboard</h1>
        <p className="text-muted-foreground">
          Overview of your LLM usage and performance
        </p>
      </div>

      {/* Stat Cards */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        {statCards.map((stat) => (
          <Card key={stat.title}>
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <CardTitle className="text-sm font-medium text-muted-foreground">
                {stat.title}
              </CardTitle>
              <stat.icon className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{stat.value}</div>
              <div className="flex items-center gap-1 text-xs">
                {stat.trendUp ? (
                  <TrendingUp className="h-3 w-3 text-green-500" />
                ) : (
                  <TrendingDown className="h-3 w-3 text-red-500" />
                )}
                <span className={stat.trendUp ? 'text-green-500' : 'text-red-500'}>
                  {stat.trend}
                </span>
                <span className="text-muted-foreground">from last period</span>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>

      {/* Charts Row */}
      <div className="grid gap-4 lg:grid-cols-2">
        {/* Usage Over Time */}
        <Card>
          <CardHeader>
            <CardTitle>Usage Over Time</CardTitle>
            <CardDescription>Requests and tokens in the last 24 hours</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="h-80">
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={stats?.requestsByHour || []}>
                  <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" />
                  <XAxis 
                    dataKey="hour" 
                    stroke="hsl(var(--muted-foreground))"
                    fontSize={12}
                  />
                  <YAxis 
                    stroke="hsl(var(--muted-foreground))"
                    fontSize={12}
                  />
                  <Tooltip
                    contentStyle={{
                      backgroundColor: 'hsl(var(--card))',
                      border: '1px solid hsl(var(--border))',
                      borderRadius: '8px',
                    }}
                  />
                  <Legend />
                  <Line
                    type="monotone"
                    dataKey="requests"
                    stroke="#8b5cf6"
                    strokeWidth={2}
                    dot={false}
                  />
                  <Line
                    type="monotone"
                    dataKey="tokens"
                    stroke="#06b6d4"
                    strokeWidth={2}
                    dot={false}
                  />
                </LineChart>
              </ResponsiveContainer>
            </div>
          </CardContent>
        </Card>

        {/* Cost Trend */}
        <Card>
          <CardHeader>
            <CardTitle>Cost Trend</CardTitle>
            <CardDescription>Daily spending over the last 7 days</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="h-80">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={stats?.costTrend || []}>
                  <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" />
                  <XAxis 
                    dataKey="date" 
                    stroke="hsl(var(--muted-foreground))"
                    fontSize={12}
                  />
                  <YAxis 
                    stroke="hsl(var(--muted-foreground))"
                    fontSize={12}
                    tickFormatter={(value) => `$${value}`}
                  />
                  <Tooltip
                    contentStyle={{
                      backgroundColor: 'hsl(var(--card))',
                      border: '1px solid hsl(var(--border))',
                      borderRadius: '8px',
                    }}
                    formatter={(value) => formatCurrency(value as number)}
                  />
                  <Bar dataKey="cost" fill="#8b5cf6" radius={[4, 4, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Bottom Row */}
      <div className="grid gap-4 lg:grid-cols-3">
        {/* Provider Breakdown */}
        <Card>
          <CardHeader>
            <CardTitle>Provider Breakdown</CardTitle>
            <CardDescription>Request distribution by provider</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="h-64">
              <ResponsiveContainer width="100%" height="100%">
                <PieChart>
                  <Pie
                    data={stats?.providerBreakdown || []}
                    cx="50%"
                    cy="50%"
                    innerRadius={60}
                    outerRadius={80}
                    paddingAngle={5}
                    dataKey="percentage"
                    nameKey="provider"
                  >
                    {(stats?.providerBreakdown || []).map((entry: any, index: number) => (
                      <Cell
                        key={entry.provider}
                        fill={providerColors[entry.provider] || COLORS[index % COLORS.length]}
                      />
                    ))}
                  </Pie>
                  <Tooltip
                    contentStyle={{
                      backgroundColor: 'hsl(var(--card))',
                      border: '1px solid hsl(var(--border))',
                      borderRadius: '8px',
                    }}
                    formatter={(value) => `${(value as number).toFixed(1)}%`}
                  />
                  <Legend />
                </PieChart>
              </ResponsiveContainer>
            </div>
          </CardContent>
        </Card>

        {/* Top Models */}
        <Card className="lg:col-span-2">
          <CardHeader>
            <CardTitle>Top Models</CardTitle>
            <CardDescription>Most used models by request count</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              {(stats?.topModels || []).slice(0, 5).map((model: any, index: number) => (
                <div key={model.model} className="flex items-center gap-4">
                  <div className="flex h-8 w-8 items-center justify-center rounded-full bg-primary/10 text-sm font-medium text-primary">
                    {index + 1}
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="truncate font-medium">{model.model}</span>
                      <Badge variant="secondary" className="text-xs">
                        {formatNumber(model.requests)} requests
                      </Badge>
                    </div>
                    <div className="text-sm text-muted-foreground">
                      {formatNumber(model.tokens)} tokens · {formatCurrency(model.cost)}
                    </div>
                  </div>
                  <div className="h-2 w-24 overflow-hidden rounded-full bg-muted">
                    <div
                      className="h-full bg-primary"
                      style={{
                        width: `${(model.requests / (stats?.topModels?.[0]?.requests || 1)) * 100}%`,
                      }}
                    />
                  </div>
                </div>
              ))}
              {(!stats?.topModels || stats.topModels.length === 0) && (
                <div className="py-8 text-center text-muted-foreground">
                  No model usage data yet
                </div>
              )}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* API Key Breakdown */}
      <Card>
        <CardHeader>
          <CardTitle>API Key Breakdown</CardTitle>
          <CardDescription>Usage statistics by API key</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            {(stats?.apiKeyBreakdown || []).slice(0, 10).map((apiKey: any, index: number) => (
              <div key={apiKey.apiKeyId} className="flex items-center gap-4">
                <div className="flex h-8 w-8 items-center justify-center rounded-full bg-primary/10 text-sm font-medium text-primary">
                  {index + 1}
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="truncate font-medium">{apiKey.apiKeyName}</span>
                    <Badge variant="secondary" className="text-xs">
                      {formatNumber(apiKey.requests)} requests
                    </Badge>
                  </div>
                  <div className="text-sm text-muted-foreground">
                    {formatNumber(apiKey.tokens)} tokens · {formatCurrency(apiKey.cost)} · {apiKey.percentage.toFixed(1)}% of total
                  </div>
                </div>
                <div className="h-2 w-24 overflow-hidden rounded-full bg-muted">
                  <div
                    className="h-full bg-primary"
                    style={{
                      width: `${apiKey.percentage}%`,
                    }}
                  />
                </div>
              </div>
            ))}
            {(!stats?.apiKeyBreakdown || stats.apiKeyBreakdown.length === 0) && (
              <div className="py-8 text-center text-muted-foreground">
                No API key usage data yet
              </div>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

