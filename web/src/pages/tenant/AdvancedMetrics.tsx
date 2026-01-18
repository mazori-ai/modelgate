import { useState, useEffect } from 'react';
import { useQuery } from '@apollo/client';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { GET_ADVANCED_METRICS } from '@/graphql/operations';
import {
  TrendingUp,
  TrendingDown,
  Zap,
  Shield,
  Activity,
  AlertCircle,
  CheckCircle2,
  XCircle,
  Clock,
  DollarSign,
  Gauge,
  BarChart3,
  Route,
  RefreshCw
} from 'lucide-react';

interface CacheMetrics {
  hits: number;
  misses: number;
  hitRate: number;
  tokensSaved: number;
  costSaved: number;
  avgLatencyMs: number;
  entries: number;
}

interface RoutingMetrics {
  decisions: number;
  strategyDistribution: { strategy: string; count: number }[];
  modelSwitches: { fromModel: string; toModel: string; count: number }[];
  failures: number;
}

interface ResilienceMetrics {
  circuitBreakers: { provider: string; state: string; failures: number }[];
  retryAttempts: number;
  fallbackInvocations: number;
  fallbackSuccessRate: number;
}

interface ProviderHealthMetrics {
  providers: {
    provider: string;
    model: string;
    healthScore: number;
    successRate: number;
    p95LatencyMs: number;
    requests: number;
  }[];
}

interface AdvancedMetricsData {
  advancedMetrics: {
    cache: CacheMetrics;
    routing: RoutingMetrics;
    resilience: ResilienceMetrics;
    providerHealth: ProviderHealthMetrics;
  };
}

export default function AdvancedMetrics() {
  // Fetch real data from GraphQL
  const { data, loading, error, refetch } = useQuery<AdvancedMetricsData>(GET_ADVANCED_METRICS, {
    pollInterval: 10000, // Auto-refresh every 10 seconds
    fetchPolicy: 'network-only',
  });

  // Extract metrics from query data with defaults
  const cacheMetrics: CacheMetrics = data?.advancedMetrics?.cache || {
    hits: 0,
    misses: 0,
    hitRate: 0,
    tokensSaved: 0,
    costSaved: 0,
    avgLatencyMs: 0,
    entries: 0,
  };

  const routingMetrics: RoutingMetrics = data?.advancedMetrics?.routing || {
    decisions: 0,
    strategyDistribution: [],
    modelSwitches: [],
    failures: 0,
  };

  const resilienceMetrics: ResilienceMetrics = data?.advancedMetrics?.resilience || {
    circuitBreakers: [],
    retryAttempts: 0,
    fallbackInvocations: 0,
    fallbackSuccessRate: 0,
  };

  const providerHealth: ProviderHealthMetrics = data?.advancedMetrics?.providerHealth || {
    providers: [],
  };

  const formatNumber = (num: number) => num.toLocaleString();
  const formatPercent = (value: number) => `${(value * 100).toFixed(1)}%`;
  const formatCurrency = (value: number) => `$${value.toFixed(2)}`;

  if (error) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Advanced Metrics</h1>
          <p className="text-muted-foreground">
            Real-time insights into semantic caching, intelligent routing, and resilience
          </p>
        </div>
        <Card>
          <CardContent className="pt-6">
            <div className="flex items-center gap-2 text-red-600">
              <AlertCircle className="h-5 w-5" />
              <span>Failed to load metrics: {error.message}</span>
            </div>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Advanced Metrics</h1>
          <p className="text-muted-foreground">
            Real-time insights into semantic caching, intelligent routing, and resilience
          </p>
        </div>
        <button 
          onClick={() => refetch()} 
          className="flex items-center gap-2 px-3 py-2 text-sm border rounded-md hover:bg-gray-50"
          disabled={loading}
        >
          <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
          Refresh
        </button>
      </div>

      <Tabs defaultValue="cache" className="space-y-4">
        <TabsList className="grid w-full grid-cols-4">
          <TabsTrigger value="cache">
            <Zap className="w-4 h-4 mr-2" />
            Cache
          </TabsTrigger>
          <TabsTrigger value="routing">
            <Route className="w-4 h-4 mr-2" />
            Routing
          </TabsTrigger>
          <TabsTrigger value="resilience">
            <Shield className="w-4 h-4 mr-2" />
            Resilience
          </TabsTrigger>
          <TabsTrigger value="health">
            <Activity className="w-4 h-4 mr-2" />
            Provider Health
          </TabsTrigger>
        </TabsList>

        {/* CACHE METRICS TAB */}
        <TabsContent value="cache" className="space-y-4">
          {/* Summary Cards */}
          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">Cache Hit Rate</CardTitle>
                <TrendingUp className="h-4 w-4 text-green-500" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{formatPercent(cacheMetrics.hitRate)}</div>
                <p className="text-xs text-muted-foreground">
                  {formatNumber(cacheMetrics.hits)} hits, {formatNumber(cacheMetrics.misses)} misses
                </p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">Cost Saved</CardTitle>
                <DollarSign className="h-4 w-4 text-yellow-500" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{formatCurrency(cacheMetrics.costSaved)}</div>
                <p className="text-xs text-muted-foreground">
                  {formatNumber(cacheMetrics.tokensSaved)} tokens saved
                </p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">Avg Latency</CardTitle>
                <Clock className="h-4 w-4 text-blue-500" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{cacheMetrics.avgLatencyMs}ms</div>
                <p className="text-xs text-muted-foreground">
                  Cache lookup time
                </p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">Cache Entries</CardTitle>
                <BarChart3 className="h-4 w-4 text-purple-500" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{formatNumber(cacheMetrics.entries)}</div>
                <p className="text-xs text-muted-foreground">
                  Stored responses
                </p>
              </CardContent>
            </Card>
          </div>

          {/* Cache Performance Chart */}
          <Card>
            <CardHeader>
              <CardTitle>Cache Performance Over Time</CardTitle>
              <CardDescription>Hit rate, cost savings, and latency trends</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="h-[300px] flex items-center justify-center text-muted-foreground">
                <p>Chart placeholder - integrate with Recharts or similar library</p>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        {/* ROUTING METRICS TAB */}
        <TabsContent value="routing" className="space-y-4">
          {/* Summary Cards */}
          <div className="grid gap-4 md:grid-cols-3">
            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">Total Decisions</CardTitle>
                <Route className="h-4 w-4 text-indigo-500" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{formatNumber(routingMetrics.decisions)}</div>
                <p className="text-xs text-muted-foreground">Routing decisions made</p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">Model Switches</CardTitle>
                <TrendingUp className="h-4 w-4 text-green-500" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">
                  {formatNumber(routingMetrics.modelSwitches.reduce((sum, s) => sum + s.count, 0))}
                </div>
                <p className="text-xs text-muted-foreground">Optimized model selections</p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">Failures</CardTitle>
                <AlertCircle className="h-4 w-4 text-red-500" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{routingMetrics.failures}</div>
                <p className="text-xs text-muted-foreground">Routing errors</p>
              </CardContent>
            </Card>
          </div>

          {/* Strategy Distribution */}
          <Card>
            <CardHeader>
              <CardTitle>Routing Strategy Distribution</CardTitle>
              <CardDescription>Distribution of routing strategies used</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              {routingMetrics.strategyDistribution.map((item) => {
                const percentage = (item.count / routingMetrics.decisions) * 100;
                return (
                  <div key={item.strategy}>
                    <div className="flex items-center justify-between mb-1">
                      <span className="text-sm font-medium capitalize">{item.strategy}</span>
                      <span className="text-sm text-muted-foreground">
                        {formatNumber(item.count)} ({percentage.toFixed(1)}%)
                      </span>
                    </div>
                    <div className="w-full bg-gray-200 rounded-full h-2">
                      <div
                        className="bg-indigo-600 h-2 rounded-full transition-all"
                        style={{ width: `${percentage}%` }}
                      />
                    </div>
                  </div>
                );
              })}
            </CardContent>
          </Card>

          {/* Top Model Switches */}
          <Card>
            <CardHeader>
              <CardTitle>Top Model Switches</CardTitle>
              <CardDescription>Most frequent model optimizations</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="space-y-2">
                {routingMetrics.modelSwitches.length === 0 ? (
                  <p className="text-muted-foreground text-sm">No model switches recorded yet</p>
                ) : (
                  routingMetrics.modelSwitches.map((item, idx) => (
                    <div key={idx} className="flex items-center justify-between p-3 border rounded-lg">
                      <div className="flex items-center gap-3">
                        <Badge variant="outline">{item.fromModel}</Badge>
                        <span className="text-muted-foreground">→</span>
                        <Badge variant="outline">{item.toModel}</Badge>
                      </div>
                      <span className="font-semibold">{formatNumber(item.count)}</span>
                    </div>
                  ))
                )}
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        {/* RESILIENCE METRICS TAB */}
        <TabsContent value="resilience" className="space-y-4">
          {/* Summary Cards */}
          <div className="grid gap-4 md:grid-cols-3">
            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">Retry Attempts</CardTitle>
                <Activity className="h-4 w-4 text-orange-500" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{formatNumber(resilienceMetrics.retryAttempts)}</div>
                <p className="text-xs text-muted-foreground">Total retries</p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">Fallback Invocations</CardTitle>
                <Shield className="h-4 w-4 text-blue-500" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{formatNumber(resilienceMetrics.fallbackInvocations)}</div>
                <p className="text-xs text-muted-foreground">
                  {formatPercent(resilienceMetrics.fallbackSuccessRate)} success rate
                </p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">Circuit Breakers</CardTitle>
                <Gauge className="h-4 w-4 text-purple-500" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{resilienceMetrics.circuitBreakers.length}</div>
                <p className="text-xs text-muted-foreground">Tracked providers</p>
              </CardContent>
            </Card>
          </div>

          {/* Circuit Breaker States */}
          <Card>
            <CardHeader>
              <CardTitle>Circuit Breaker States</CardTitle>
              <CardDescription>Current state of circuit breakers per provider</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="space-y-3">
                {resilienceMetrics.circuitBreakers.map((cb) => {
                  const StateIcon = cb.state === 'closed' ? CheckCircle2 : cb.state === 'open' ? XCircle : AlertCircle;
                  const stateColor = cb.state === 'closed' ? 'text-green-500' : cb.state === 'open' ? 'text-red-500' : 'text-yellow-500';
                  const stateBadge = cb.state === 'closed' ? 'bg-green-100 text-green-800' : cb.state === 'open' ? 'bg-red-100 text-red-800' : 'bg-yellow-100 text-yellow-800';

                  return (
                    <div key={cb.provider} className="flex items-center justify-between p-4 border rounded-lg">
                      <div className="flex items-center gap-3">
                        <StateIcon className={`h-5 w-5 ${stateColor}`} />
                        <div>
                          <p className="font-medium capitalize">{cb.provider}</p>
                          <p className="text-sm text-muted-foreground">{cb.failures} recent failures</p>
                        </div>
                      </div>
                      <Badge className={stateBadge}>{cb.state.replace('_', ' ')}</Badge>
                    </div>
                  );
                })}
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        {/* PROVIDER HEALTH TAB */}
        <TabsContent value="health" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Provider Health Scores</CardTitle>
              <CardDescription>Health, success rate, and latency metrics per provider/model</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="space-y-4">
                {providerHealth.providers.map((p, idx) => {
                  const healthColor = p.healthScore >= 0.95 ? 'text-green-500' : p.healthScore >= 0.8 ? 'text-yellow-500' : 'text-red-500';
                  const healthBg = p.healthScore >= 0.95 ? 'bg-green-100' : p.healthScore >= 0.8 ? 'bg-yellow-100' : 'bg-red-100';

                  return (
                    <div key={idx} className="p-4 border rounded-lg space-y-3">
                      <div className="flex items-center justify-between">
                        <div>
                          <p className="font-semibold capitalize">{p.provider}</p>
                          <p className="text-sm text-muted-foreground">{p.model}</p>
                        </div>
                        <div className="flex items-center gap-2">
                          <Gauge className={`h-5 w-5 ${healthColor}`} />
                          <span className={`text-lg font-bold ${healthColor}`}>
                            {formatPercent(p.healthScore)}
                          </span>
                        </div>
                      </div>

                      <div className="grid grid-cols-3 gap-4 pt-2 border-t">
                        <div>
                          <p className="text-xs text-muted-foreground">Success Rate</p>
                          <p className="text-sm font-medium">{formatPercent(p.successRate)}</p>
                        </div>
                        <div>
                          <p className="text-xs text-muted-foreground">P95 Latency</p>
                          <p className="text-sm font-medium">{p.p95LatencyMs}ms</p>
                        </div>
                        <div>
                          <p className="text-xs text-muted-foreground">Requests</p>
                          <p className="text-sm font-medium">{formatNumber(p.requests)}</p>
                        </div>
                      </div>

                      {/* Health Score Bar */}
                      <div className="w-full bg-gray-200 rounded-full h-2">
                        <div
                          className={`h-2 rounded-full transition-all ${
                            p.healthScore >= 0.95 ? 'bg-green-500' : p.healthScore >= 0.8 ? 'bg-yellow-500' : 'bg-red-500'
                          }`}
                          style={{ width: `${p.healthScore * 100}%` }}
                        />
                      </div>
                    </div>
                  );
                })}
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
