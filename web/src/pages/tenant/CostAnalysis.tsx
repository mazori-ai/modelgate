import { useState, useMemo } from 'react';
import { useQuery } from '@apollo/client';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, BarChart, Bar, PieChart, Pie, Cell } from 'recharts';
import { GET_COST_ANALYSIS } from '@/graphql/operations';
import { Loader2 } from 'lucide-react';

const COLORS = ['#6366f1', '#22c55e', '#f59e0b', '#ef4444', '#8b5cf6', '#06b6d4'];

export default function CostAnalysis() {
  const [period, setPeriod] = useState('month');

  // Calculate date range based on period filter
  const dateRange = useMemo(() => {
    const now = new Date();
    let startDate = new Date();

    switch (period) {
      case 'week':
        startDate = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000);
        break;
      case 'month':
        startDate = new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000);
        break;
      case 'quarter':
        startDate = new Date(now.getTime() - 90 * 24 * 60 * 60 * 1000);
        break;
      default:
        startDate = new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000);
    }

    return {
      startDate: startDate.toISOString(),
      endDate: now.toISOString(),
    };
  }, [period]);

  // Fetch cost analysis data from GraphQL API
  const { data, loading, error, refetch } = useQuery(GET_COST_ANALYSIS, {
    variables: {
      startDate: dateRange.startDate,
      endDate: dateRange.endDate,
    },
    fetchPolicy: 'network-only',
    pollInterval: 30000, // Refresh every 30 seconds
  });

  const costAnalysis = data?.costAnalysis;

  // Calculate metrics from real data
  const dailyCosts = costAnalysis?.dailyCosts || [];
  const costByModel = costAnalysis?.costByModel || [];
  const costByProvider = costAnalysis?.costByProvider || [];
  const totalCost = costAnalysis?.totalCost || 0;
  const dailyAverage = dailyCosts.length > 0 ? totalCost / dailyCosts.length : 0;
  const projectedMonthly = costAnalysis?.projectedMonthlyCost || dailyAverage * 30;

  const suggestions = [
    "gpt-4o accounts for 38% of costs. Consider using gpt-4o-mini for simpler tasks.",
    "You're projected to use 85% of your monthly budget. Review usage patterns.",
    "Enable caching to reduce repeated requests and save up to 20% on costs.",
  ];

  if (loading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Cost Analysis</h1>
          <p className="text-muted-foreground">Monitor spending and optimize costs</p>
        </div>
        <div className="rounded-lg bg-destructive/10 p-4 text-center">
          <p className="text-red-500">Error loading cost analysis: {error.message}</p>
          <Button variant="outline" onClick={() => refetch()} className="mt-2">
            Retry
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Cost Analysis</h1>
          <p className="text-muted-foreground">Monitor spending and optimize costs</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => refetch()} disabled={loading}>
            <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
            Refresh
          </Button>
          <Select value={period} onValueChange={setPeriod}>
            <SelectTrigger className="w-[140px]">
              <SelectValue placeholder="Period" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="week">Last 7 days</SelectItem>
              <SelectItem value="month">Last 30 days</SelectItem>
              <SelectItem value="quarter">Last 90 days</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      {/* Summary Cards */}
      <div className="grid gap-4 md:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Total Cost</CardDescription>
            <CardTitle className="text-3xl">${totalCost.toFixed(2)}</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-xs text-muted-foreground">This period</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Daily Average</CardDescription>
            <CardTitle className="text-3xl">${dailyAverage.toFixed(2)}</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-xs text-muted-foreground">Per day</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Projected Monthly</CardDescription>
            <CardTitle className="text-3xl">${projectedMonthly.toFixed(2)}</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-xs text-green-500">Within budget</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Budget Utilization</CardDescription>
            <CardTitle className="text-3xl">{((costAnalysis?.budgetUtilization || 0) * 100).toFixed(0)}%</CardTitle>
          </CardHeader>
          <CardContent>
            <p className={`text-xs ${(costAnalysis?.budgetUtilization || 0) < 0.8 ? 'text-green-500' : 'text-amber-500'}`}>
              {(costAnalysis?.budgetUtilization || 0) < 0.8 ? 'Within budget' : 'Approaching limit'}
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Cost Trend Chart */}
      <Card>
        <CardHeader>
          <CardTitle>Cost Trend</CardTitle>
          <CardDescription>Daily spending over time</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="h-[300px]">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={dailyCosts}>
                <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" />
                <XAxis dataKey="date" stroke="hsl(var(--muted-foreground))" fontSize={12} />
                <YAxis stroke="hsl(var(--muted-foreground))" fontSize={12} tickFormatter={(value) => `$${value}`} />
                <Tooltip
                  contentStyle={{
                    backgroundColor: 'hsl(var(--popover))',
                    border: '1px solid hsl(var(--border))',
                    borderRadius: '8px',
                  }}
                  formatter={(value: number | undefined) => [`$${(value ?? 0).toFixed(2)}`, 'Cost']}
                />
                <Line
                  type="monotone"
                  dataKey="cost"
                  stroke="#6366f1"
                  strokeWidth={2}
                  dot={{ fill: '#6366f1', strokeWidth: 2 }}
                />
              </LineChart>
            </ResponsiveContainer>
          </div>
        </CardContent>
      </Card>

      {/* Cost Breakdown */}
      <div className="grid gap-6 md:grid-cols-2">
        {/* By Model */}
        <Card>
          <CardHeader>
            <CardTitle>Cost by Model</CardTitle>
            <CardDescription>Spending breakdown by model</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="h-[250px]">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={costByModel} layout="vertical">
                  <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" />
                  <XAxis type="number" stroke="hsl(var(--muted-foreground))" fontSize={12} tickFormatter={(value) => `$${value}`} />
                  <YAxis type="category" dataKey="model" stroke="hsl(var(--muted-foreground))" fontSize={12} width={120} />
                  <Tooltip
                    contentStyle={{
                      backgroundColor: 'hsl(var(--popover))',
                      border: '1px solid hsl(var(--border))',
                      borderRadius: '8px',
                    }}
                    formatter={(value: number | undefined) => [`$${(value ?? 0).toFixed(2)}`, 'Cost']}
                  />
                  <Bar dataKey="cost" fill="#6366f1" radius={[0, 4, 4, 0]} />
                </BarChart>
              </ResponsiveContainer>
            </div>
          </CardContent>
        </Card>

        {/* By Provider */}
        <Card>
          <CardHeader>
            <CardTitle>Cost by Provider</CardTitle>
            <CardDescription>Spending breakdown by provider</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="h-[250px] flex items-center">
              <ResponsiveContainer width="50%" height="100%">
                <PieChart>
                  <Pie
                    data={costByProvider}
                    cx="50%"
                    cy="50%"
                    innerRadius={50}
                    outerRadius={80}
                    dataKey="cost"
                    nameKey="provider"
                  >
                    {costByProvider.map((entry: any, index: number) => (
                      <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
                    ))}
                  </Pie>
                  <Tooltip
                    contentStyle={{
                      backgroundColor: 'hsl(var(--popover))',
                      border: '1px solid hsl(var(--border))',
                      borderRadius: '8px',
                    }}
                    formatter={(value: number | undefined) => [`$${(value ?? 0).toFixed(2)}`, 'Cost']}
                  />
                </PieChart>
              </ResponsiveContainer>
              <div className="flex-1 space-y-2">
                {costByProvider.map((item: any, index: number) => (
                  <div key={item.provider} className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <div
                        className="w-3 h-3 rounded-full"
                        style={{ backgroundColor: COLORS[index % COLORS.length] }}
                      />
                      <span className="text-sm">{item.provider}</span>
                    </div>
                    <span className="text-sm font-medium">${item.cost.toFixed(2)}</span>
                  </div>
                ))}
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Optimization Suggestions */}
      {costByModel.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Optimization Suggestions</CardTitle>
            <CardDescription>Recommendations to reduce costs</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-3">
              {costByModel.length > 0 && costByModel[0].cost > totalCost * 0.3 && (
                <div className="flex items-start gap-3 p-3 rounded-lg bg-amber-50 dark:bg-amber-950 border border-amber-200 dark:border-amber-900">
                  <svg className="w-5 h-5 text-amber-500 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                  </svg>
                  <p className="text-sm">
                    {costByModel[0].model} accounts for {((costByModel[0].cost / totalCost) * 100).toFixed(0)}% of costs. Consider using a more cost-effective model for simpler tasks.
                  </p>
                </div>
              )}
              {(costAnalysis?.budgetUtilization || 0) > 0.8 && (
                <div className="flex items-start gap-3 p-3 rounded-lg bg-amber-50 dark:bg-amber-950 border border-amber-200 dark:border-amber-900">
                  <svg className="w-5 h-5 text-amber-500 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                  </svg>
                  <p className="text-sm">
                    You're projected to use {((costAnalysis?.budgetUtilization || 0) * 100).toFixed(0)}% of your monthly budget. Review usage patterns.
                  </p>
                </div>
              )}
              {costByModel.length === 0 && (
                <div className="py-8 text-center text-muted-foreground">
                  No optimization suggestions available yet
                </div>
              )}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Budget Alerts */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <div>
            <CardTitle>Budget Alerts</CardTitle>
            <CardDescription>Get notified when spending exceeds thresholds</CardDescription>
          </div>
          <Button>
            <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
            </svg>
            Add Alert
          </Button>
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            <div className="flex items-center justify-between p-3 rounded-lg border">
              <div>
                <p className="font-medium">Daily Cost Alert</p>
                <p className="text-sm text-muted-foreground">Notify when daily cost exceeds $50</p>
              </div>
              <div className="flex items-center gap-2">
                <span className="text-sm text-green-500">Active</span>
                <Button variant="ghost" size="sm">Edit</Button>
              </div>
            </div>
            <div className="flex items-center justify-between p-3 rounded-lg border">
              <div>
                <p className="font-medium">Monthly Budget Warning</p>
                <p className="text-sm text-muted-foreground">Notify at 80% of $500 monthly budget</p>
              </div>
              <div className="flex items-center gap-2">
                <span className="text-sm text-green-500">Active</span>
                <Button variant="ghost" size="sm">Edit</Button>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

