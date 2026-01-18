import { useState, useMemo } from 'react';
import { useQuery } from '@apollo/client';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { GET_REQUEST_LOGS, GET_API_KEYS, GET_REQUEST_LOG_DETAIL } from '@/graphql/operations';
import { Loader2 } from 'lucide-react';

interface RequestLog {
  id: string;
  model: string;
  provider: string;
  inputTokens: number;
  outputTokens: number;
  costUSD: number;
  latencyMs: number;
  status: 'success' | 'error';
  apiKeyName?: string | null;
  errorMessage?: string;
  errorCode?: string;
  prompt?: string;
  createdAt: string;
}

export default function RequestLogs() {
  const [searchQuery, setSearchQuery] = useState('');
  const [periodFilter, setPeriodFilter] = useState('24h');
  const [statusFilter, setStatusFilter] = useState('all');
  const [modelFilter, setModelFilter] = useState('all');
  const [apiKeyFilter, setApiKeyFilter] = useState('all');
  const [selectedLog, setSelectedLog] = useState<RequestLog | null>(null);
  const [selectedLogId, setSelectedLogId] = useState<string | null>(null);
  const [isRefreshing, setIsRefreshing] = useState(false);

  // Fetch detailed log when selected
  const { data: detailData, loading: detailLoading } = useQuery(GET_REQUEST_LOG_DETAIL, {
    variables: { id: selectedLogId },
    skip: !selectedLogId,
    fetchPolicy: 'network-only',
  });

  // Calculate date range based on period filter
  const dateFilter = useMemo(() => {
    const now = new Date();
    let startDate = new Date();

    switch (periodFilter) {
      case '1h':
        startDate = new Date(now.getTime() - 60 * 60 * 1000);
        break;
      case '24h':
        startDate = new Date(now.getTime() - 24 * 60 * 60 * 1000);
        break;
      case '7d':
        startDate = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000);
        break;
      case '30d':
        startDate = new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000);
        break;
      default:
        startDate = new Date(now.getTime() - 24 * 60 * 60 * 1000);
    }

    return {
      startDate: startDate.toISOString(),
      endDate: now.toISOString(),
      status: statusFilter !== 'all' ? statusFilter : undefined,
      model: modelFilter !== 'all' ? modelFilter : undefined,
      apiKeyId: apiKeyFilter !== 'all' ? apiKeyFilter : undefined,
    };
  }, [periodFilter, statusFilter, modelFilter, apiKeyFilter]);

  // Fetch API keys for filter dropdown
  const { data: apiKeysData } = useQuery(GET_API_KEYS);

  // Fetch request logs from GraphQL API
  const { data, loading, error, refetch } = useQuery(GET_REQUEST_LOGS, {
    variables: {
      filter: dateFilter,
      first: 100,
    },
    fetchPolicy: 'network-only', // Always fetch fresh data
    pollInterval: 10000, // Refresh every 10 seconds
  });

  const logs = data?.requestLogs?.edges || [];
  const apiKeys = apiKeysData?.apiKeys || [];

  // Handle manual refresh
  const handleRefresh = async () => {
    setIsRefreshing(true);
    try {
      await refetch();
    } finally {
      setIsRefreshing(false);
    }
  };

  const filteredLogs = logs.filter((log: RequestLog) => {
    if (searchQuery && !log.model.toLowerCase().includes(searchQuery.toLowerCase()) &&
        !log.id.toLowerCase().includes(searchQuery.toLowerCase())) {
      return false;
    }
    return true;
  });

  const uniqueModels = [...new Set(logs.map((log: RequestLog) => log.model))] as string[];

  const formatTimestamp = (timestamp: string) => {
    const date = new Date(timestamp);
    return date.toLocaleString();
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Request Logs</h1>
        <p className="text-muted-foreground">View and search through API request history</p>
      </div>

      {/* Filters */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex flex-wrap gap-4">
            <div className="flex-1 min-w-[200px]">
              <Input
                placeholder="Search by request ID..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
              />
            </div>
            <Select value={periodFilter} onValueChange={setPeriodFilter}>
              <SelectTrigger className="w-[140px]">
                <SelectValue placeholder="Period" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="1h">Last hour</SelectItem>
                <SelectItem value="24h">Last 24 hours</SelectItem>
                <SelectItem value="7d">Last 7 days</SelectItem>
                <SelectItem value="30d">Last 30 days</SelectItem>
              </SelectContent>
            </Select>
            <Select value={statusFilter} onValueChange={setStatusFilter}>
              <SelectTrigger className="w-[140px]">
                <SelectValue placeholder="Status" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All statuses</SelectItem>
                <SelectItem value="success">Success</SelectItem>
                <SelectItem value="error">Error</SelectItem>
              </SelectContent>
            </Select>
            <Select value={modelFilter} onValueChange={setModelFilter}>
              <SelectTrigger className="w-[180px]">
                <SelectValue placeholder="Model" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All models</SelectItem>
                {uniqueModels.map((model: string) => (
                  <SelectItem key={model} value={model}>{model}</SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Select value={apiKeyFilter} onValueChange={setApiKeyFilter}>
              <SelectTrigger className="w-[180px]">
                <SelectValue placeholder="API Key" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All API keys</SelectItem>
                {apiKeys.map((key: any) => (
                  <SelectItem key={key.id} value={key.id}>{key.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button variant="outline" onClick={handleRefresh} disabled={loading || isRefreshing}>
              {isRefreshing ? (
                <Loader2 className="w-4 h-4 mr-2 animate-spin" />
              ) : (
                <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                </svg>
              )}
              Refresh
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Logs Table */}
      <Card>
        <CardHeader>
          <CardTitle>Logs</CardTitle>
          <CardDescription>
            {loading ? 'Loading...' : `Showing ${filteredLogs.length} request${filteredLogs.length !== 1 ? 's' : ''}`}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {loading && (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
            </div>
          )}

          {error && (
            <div className="text-center py-8">
              <p className="text-red-500">Error loading logs: {error.message}</p>
              <Button variant="outline" onClick={() => refetch()} className="mt-2">
                Retry
              </Button>
            </div>
          )}

          {!loading && !error && (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Time</TableHead>
                  <TableHead>ID</TableHead>
                  <TableHead>Model</TableHead>
                  <TableHead>API Key</TableHead>
                  <TableHead>Tokens</TableHead>
                  <TableHead>Cost</TableHead>
                  <TableHead>Latency</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredLogs.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={9} className="text-center text-muted-foreground py-8">
                      No request logs found for the selected filters
                    </TableCell>
                  </TableRow>
                ) : (
                  filteredLogs.map((log: RequestLog) => (
                    <TableRow key={log.id}>
                      <TableCell className="text-sm text-muted-foreground">
                        {formatTimestamp(log.createdAt)}
                      </TableCell>
                      <TableCell className="font-mono text-xs">{log.id.substring(0, 8)}...</TableCell>
                      <TableCell>
                        <div className="flex flex-col">
                          <span className="font-medium text-sm">{log.model}</span>
                          <span className="text-xs text-muted-foreground uppercase">{log.provider}</span>
                        </div>
                      </TableCell>
                      <TableCell>
                        <span className="text-sm font-medium">
                          {log.apiKeyName || <span className="text-muted-foreground">Unknown</span>}
                        </span>
                      </TableCell>
                      <TableCell>
                        <div className="flex flex-col text-sm">
                          <span>{(log.inputTokens + log.outputTokens).toLocaleString()}</span>
                          <span className="text-xs text-muted-foreground">
                            {log.inputTokens} in / {log.outputTokens} out
                          </span>
                        </div>
                      </TableCell>
                      <TableCell className="text-sm">${log.costUSD.toFixed(4)}</TableCell>
                      <TableCell className="text-sm">{log.latencyMs}ms</TableCell>
                      <TableCell>
                        <Badge variant={log.status === 'success' ? 'default' : 'destructive'}>
                          {log.status}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-right">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => {
                            setSelectedLog(log);
                            setSelectedLogId(log.id);
                          }}
                        >
                          View
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      {/* Log Detail Dialog */}
      <Dialog open={!!selectedLog} onOpenChange={() => {
        setSelectedLog(null);
        setSelectedLogId(null);
      }}>
        <DialogContent className="max-w-3xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>Request Details</DialogTitle>
            <DialogDescription className="font-mono text-xs break-all">
              {selectedLog?.id}
            </DialogDescription>
          </DialogHeader>
          {detailLoading && (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
            </div>
          )}
          {selectedLog && !detailLoading && (
            <div className="space-y-4">
              {/* Basic Information */}
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-1">
                  <label className="text-sm font-medium text-muted-foreground">Model</label>
                  <p className="font-medium">{selectedLog.model}</p>
                </div>
                <div className="space-y-1">
                  <label className="text-sm font-medium text-muted-foreground">Provider</label>
                  <p className="font-medium uppercase">{selectedLog.provider}</p>
                </div>
                <div className="space-y-1">
                  <label className="text-sm font-medium text-muted-foreground">Status</label>
                  <Badge variant={selectedLog.status === 'success' ? 'default' : 'destructive'} className="w-fit">
                    {selectedLog.status}
                  </Badge>
                </div>
                <div className="space-y-1">
                  <label className="text-sm font-medium text-muted-foreground">Timestamp</label>
                  <p className="text-sm">{formatTimestamp(selectedLog.createdAt)}</p>
                </div>
                <div className="space-y-1 col-span-2">
                  <label className="text-sm font-medium text-muted-foreground">API Key</label>
                  <p className="font-medium">{selectedLog.apiKeyName || <span className="text-muted-foreground">Unknown</span>}</p>
                </div>
              </div>

              {/* Token Usage */}
              <div className="border-t pt-4">
                <h4 className="font-medium mb-3">Token Usage</h4>
                <div className="grid grid-cols-3 gap-3">
                  <div className="bg-muted rounded-lg p-3">
                    <p className="text-xs text-muted-foreground mb-1">Input Tokens</p>
                    <p className="text-xl font-bold">{selectedLog.inputTokens.toLocaleString()}</p>
                  </div>
                  <div className="bg-muted rounded-lg p-3">
                    <p className="text-xs text-muted-foreground mb-1">Output Tokens</p>
                    <p className="text-xl font-bold">{selectedLog.outputTokens.toLocaleString()}</p>
                  </div>
                  <div className="bg-muted rounded-lg p-3">
                    <p className="text-xs text-muted-foreground mb-1">Total Tokens</p>
                    <p className="text-xl font-bold">{(selectedLog.inputTokens + selectedLog.outputTokens).toLocaleString()}</p>
                  </div>
                </div>
              </div>

              {/* Performance Metrics */}
              <div className="border-t pt-4">
                <h4 className="font-medium mb-3">Performance Metrics</h4>
                <div className="grid grid-cols-3 gap-3">
                  <div className="bg-muted rounded-lg p-3">
                    <p className="text-xs text-muted-foreground mb-1">Latency</p>
                    <p className="text-xl font-bold">{selectedLog.latencyMs.toLocaleString()}ms</p>
                    {selectedLog.latencyMs > 0 && (
                      <p className="text-xs text-muted-foreground mt-1">
                        {(selectedLog.latencyMs / 1000).toFixed(2)}s
                      </p>
                    )}
                  </div>
                  <div className="bg-muted rounded-lg p-3">
                    <p className="text-xs text-muted-foreground mb-1">Cost</p>
                    <p className="text-xl font-bold">${selectedLog.costUSD.toFixed(4)}</p>
                    {selectedLog.costUSD > 0 && (
                      <p className="text-xs text-muted-foreground mt-1">
                        ${(selectedLog.costUSD * 1000000).toFixed(2)}/1M tokens
                      </p>
                    )}
                  </div>
                  <div className="bg-muted rounded-lg p-3">
                    <p className="text-xs text-muted-foreground mb-1">Throughput</p>
                    <p className="text-xl font-bold">
                      {selectedLog.latencyMs > 0
                        ? ((selectedLog.outputTokens / selectedLog.latencyMs) * 1000).toFixed(1)
                        : '0'}
                    </p>
                    <p className="text-xs text-muted-foreground mt-1">tokens/sec</p>
                  </div>
                </div>
              </div>

              {/* Cost Breakdown */}
              {selectedLog.costUSD > 0 && (
                <div className="border-t pt-4">
                  <h4 className="font-medium mb-3">Cost Breakdown</h4>
                  <div className="bg-muted rounded-lg p-3 space-y-2">
                    <div className="flex justify-between text-sm">
                      <span className="text-muted-foreground">Input cost ({selectedLog.inputTokens.toLocaleString()} tokens)</span>
                      <span className="font-mono">${((selectedLog.inputTokens / (selectedLog.inputTokens + selectedLog.outputTokens)) * selectedLog.costUSD).toFixed(6)}</span>
                    </div>
                    <div className="flex justify-between text-sm">
                      <span className="text-muted-foreground">Output cost ({selectedLog.outputTokens.toLocaleString()} tokens)</span>
                      <span className="font-mono">${((selectedLog.outputTokens / (selectedLog.inputTokens + selectedLog.outputTokens)) * selectedLog.costUSD).toFixed(6)}</span>
                    </div>
                    <div className="border-t pt-2 mt-2 flex justify-between font-medium">
                      <span>Total cost</span>
                      <span className="font-mono">${selectedLog.costUSD.toFixed(6)}</span>
                    </div>
                  </div>
                </div>
              )}

              {/* Prompt */}
              {detailData?.requestLog?.prompt && (
                <div className="border-t pt-4">
                  <h4 className="font-medium mb-3 flex items-center gap-2">
                    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 10h.01M12 10h.01M16 10h.01M9 16H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-5l-5 5v-5z" />
                    </svg>
                    User Prompt
                  </h4>
                  <div className="bg-muted rounded-lg p-4">
                    <p className="text-sm whitespace-pre-wrap break-words">{detailData.requestLog.prompt}</p>
                  </div>
                </div>
              )}

              {/* Error Details */}
              {selectedLog.status === 'error' && (
                <div className="border-t pt-4">
                  <h4 className="font-medium mb-3 text-red-600 dark:text-red-400 flex items-center gap-2">
                    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                    Error Details
                  </h4>
                  <div className="bg-red-50 dark:bg-red-950/50 border border-red-200 dark:border-red-900 rounded-lg p-4 space-y-3">
                    {(detailData?.requestLog?.errorMessage || selectedLog.errorMessage) ? (
                      <>
                        <div>
                          <p className="text-xs font-medium text-red-600 dark:text-red-400 mb-1">Error Message</p>
                          <p className="font-mono text-sm text-red-900 dark:text-red-100">
                            {detailData?.requestLog?.errorMessage || selectedLog.errorMessage}
                          </p>
                        </div>
                        {(detailData?.requestLog?.errorCode || selectedLog.errorCode) && (
                          <div>
                            <p className="text-xs font-medium text-red-600 dark:text-red-400 mb-1">Error Code</p>
                            <p className="font-mono text-xs text-red-700 dark:text-red-300">
                              {detailData?.requestLog?.errorCode || selectedLog.errorCode}
                            </p>
                          </div>
                        )}
                        <div className="bg-red-100 dark:bg-red-900/30 rounded p-2">
                          <p className="text-xs text-red-700 dark:text-red-300">
                            <strong>Note:</strong> This request was blocked by policy enforcement and did not reach the LLM provider.
                            Zero tokens were consumed and no cost was incurred.
                          </p>
                        </div>
                      </>
                    ) : (
                      <div>
                        <p className="text-sm text-red-900 dark:text-red-100 mb-2">
                          Request failed but no detailed error message is available.
                        </p>
                        {(detailData?.requestLog?.errorCode || selectedLog.errorCode) && (
                          <div>
                            <p className="text-xs font-medium text-red-600 dark:text-red-400 mb-1">Error Code</p>
                            <p className="font-mono text-sm text-red-700 dark:text-red-300">
                              {detailData?.requestLog?.errorCode || selectedLog.errorCode}
                            </p>
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                </div>
              )}

              {/* Success Summary */}
              {selectedLog.status === 'success' && (
                <div className="border-t pt-4">
                  <div className="bg-green-50 dark:bg-green-950/30 border border-green-200 dark:border-green-900 rounded-lg p-3">
                    <div className="flex items-center gap-2 text-green-700 dark:text-green-300">
                      <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                      </svg>
                      <span className="text-sm font-medium">Request completed successfully</span>
                    </div>
                  </div>
                </div>
              )}
            </div>
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
}

