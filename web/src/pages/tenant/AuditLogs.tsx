import { useState } from 'react';
import { useQuery } from '@apollo/client';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Badge } from '@/components/ui/badge';
import { ScrollArea } from '@/components/ui/scroll-area';
import {
  ClipboardList,
  Search,
  RefreshCw,
  ChevronLeft,
  ChevronRight,
  Eye,
  Clock,
  User,
  Globe,
  FileText,
  AlertTriangle,
  CheckCircle,
  XCircle
} from 'lucide-react';
import { GET_AUDIT_LOGS } from '@/graphql/operations';

interface AuditLog {
  id: string;
  timestamp: string;
  action: string;
  resourceType: string;
  resourceId: string;
  resourceName: string | null;
  actorId: string;
  actorEmail: string | null;
  actorType: string;
  ipAddress: string | null;
  userAgent: string | null;
  details: any;
  oldValue: any;
  newValue: any;
  status: string;
  errorMessage: string | null;
}

const actionColors: Record<string, string> = {
  CREATE: 'bg-green-500/20 text-green-400 border-green-500/30',
  UPDATE: 'bg-blue-500/20 text-blue-400 border-blue-500/30',
  DELETE: 'bg-red-500/20 text-red-400 border-red-500/30',
  REVOKE: 'bg-orange-500/20 text-orange-400 border-orange-500/30',
  LOGIN: 'bg-purple-500/20 text-purple-400 border-purple-500/30',
  LOGOUT: 'bg-gray-500/20 text-gray-400 border-gray-500/30',
};

const resourceTypeLabels: Record<string, string> = {
  ROLE: 'Role',
  POLICY: 'Policy',
  GROUP: 'Group',
  API_KEY: 'API Key',
  USER: 'User',
  PROVIDER: 'Provider',
  TENANT: 'Tenant',
  SESSION: 'Session',
};

export default function AuditLogs() {
  const [page, setPage] = useState(0);
  const [pageSize] = useState(20);
  const [selectedLog, setSelectedLog] = useState<AuditLog | null>(null);
  const [filters, setFilters] = useState({
    action: 'all',
    resourceType: 'all',
    search: '',
  });

  const { data, loading, error, refetch } = useQuery(GET_AUDIT_LOGS, {
    variables: {
      filter: {
        action: filters.action !== 'all' ? filters.action : undefined,
        resourceType: filters.resourceType !== 'all' ? filters.resourceType : undefined,
      },
      limit: pageSize,
      offset: page * pageSize,
    },
    pollInterval: 30000, // Auto-refresh every 30 seconds
    fetchPolicy: 'network-only',
  });

  // Debug log
  console.log('AuditLogs data:', { loading, error, data, logs: data?.auditLogs?.items });

  const logs = data?.auditLogs?.items || [];
  const totalCount = data?.auditLogs?.totalCount || 0;
  const hasMore = data?.auditLogs?.hasMore || false;

  const formatTimestamp = (ts: string) => {
    const date = new Date(ts);
    return date.toLocaleString();
  };

  const getActionIcon = (action: string, status: string) => {
    if (status === 'failure') {
      return <XCircle className="h-4 w-4 text-red-400" />;
    }
    switch (action) {
      case 'CREATE':
        return <CheckCircle className="h-4 w-4 text-green-400" />;
      case 'DELETE':
        return <XCircle className="h-4 w-4 text-red-400" />;
      case 'UPDATE':
        return <FileText className="h-4 w-4 text-blue-400" />;
      default:
        return <ClipboardList className="h-4 w-4 text-gray-400" />;
    }
  };

  const filteredLogs = logs.filter((log: AuditLog) => {
    if (!filters.search) return true;
    const searchLower = filters.search.toLowerCase();
    return (
      log.resourceName?.toLowerCase().includes(searchLower) ||
      log.actorEmail?.toLowerCase().includes(searchLower) ||
      log.resourceId.toLowerCase().includes(searchLower)
    );
  });

  if (error) {
    return (
      <div className="p-6">
        <div className="bg-red-50 border border-red-200 rounded-lg p-4">
          <h2 className="text-red-800 font-bold">Error loading audit logs</h2>
          <p className="text-red-600">{error.message}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <ClipboardList className="h-8 w-8 text-primary" />
          <div>
            <h1 className="text-2xl font-bold">Audit Logs</h1>
            <p className="text-muted-foreground">
              Track all security-relevant actions and changes
            </p>
          </div>
        </div>
        <Button variant="outline" onClick={() => refetch()}>
          <RefreshCw className="h-4 w-4 mr-2" />
          Refresh
        </Button>
      </div>

      {/* Filters */}
      <Card className="p-4">
        <div className="flex flex-wrap gap-4">
          <div className="flex items-center gap-2">
            <Search className="h-4 w-4 text-muted-foreground" />
            <Input
              placeholder="Search by name, email, or ID..."
              value={filters.search}
              onChange={(e) => setFilters({ ...filters, search: e.target.value })}
              className="w-64"
            />
          </div>
          <Select
            value={filters.action}
            onValueChange={(value) => setFilters({ ...filters, action: value })}
          >
            <SelectTrigger className="w-40">
              <SelectValue placeholder="All Actions" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All Actions</SelectItem>
              <SelectItem value="CREATE">Create</SelectItem>
              <SelectItem value="UPDATE">Update</SelectItem>
              <SelectItem value="DELETE">Delete</SelectItem>
              <SelectItem value="REVOKE">Revoke</SelectItem>
              <SelectItem value="LOGIN">Login</SelectItem>
              <SelectItem value="LOGOUT">Logout</SelectItem>
            </SelectContent>
          </Select>
          <Select
            value={filters.resourceType}
            onValueChange={(value) => setFilters({ ...filters, resourceType: value })}
          >
            <SelectTrigger className="w-40">
              <SelectValue placeholder="All Resources" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All Resources</SelectItem>
              <SelectItem value="ROLE">Role</SelectItem>
              <SelectItem value="POLICY">Policy</SelectItem>
              <SelectItem value="GROUP">Group</SelectItem>
              <SelectItem value="API_KEY">API Key</SelectItem>
              <SelectItem value="USER">User</SelectItem>
              <SelectItem value="PROVIDER">Provider</SelectItem>
            </SelectContent>
          </Select>
          {(filters.action !== 'all' || filters.resourceType !== 'all' || filters.search) && (
            <Button
              variant="ghost"
              onClick={() => setFilters({ action: 'all', resourceType: 'all', search: '' })}
            >
              Clear Filters
            </Button>
          )}
        </div>
      </Card>

      {/* Stats */}
      <div className="grid grid-cols-4 gap-4">
        <Card className="p-4">
          <div className="text-sm text-muted-foreground">Total Events</div>
          <div className="text-2xl font-bold">{totalCount}</div>
        </Card>
        <Card className="p-4">
          <div className="text-sm text-muted-foreground">Policy Changes</div>
          <div className="text-2xl font-bold text-blue-400">
            {logs.filter((l: AuditLog) => l.resourceType === 'POLICY').length}
          </div>
        </Card>
        <Card className="p-4">
          <div className="text-sm text-muted-foreground">API Key Actions</div>
          <div className="text-2xl font-bold text-purple-400">
            {logs.filter((l: AuditLog) => l.resourceType === 'API_KEY').length}
          </div>
        </Card>
        <Card className="p-4">
          <div className="text-sm text-muted-foreground">Failed Actions</div>
          <div className="text-2xl font-bold text-red-400">
            {logs.filter((l: AuditLog) => l.status === 'failure').length}
          </div>
        </Card>
      </div>

      {/* Logs Table */}
      <Card>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-12"></TableHead>
              <TableHead>Timestamp</TableHead>
              <TableHead>Action</TableHead>
              <TableHead>Resource</TableHead>
              <TableHead>Actor</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="w-16"></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading ? (
              <TableRow>
                <TableCell colSpan={7} className="text-center py-8">
                  Loading audit logs...
                </TableCell>
              </TableRow>
            ) : filteredLogs.length === 0 ? (
              <TableRow>
                <TableCell colSpan={7} className="text-center py-8 text-muted-foreground">
                  No audit logs found
                </TableCell>
              </TableRow>
            ) : (
              filteredLogs.map((log: AuditLog) => (
                <TableRow key={log.id} className="hover:bg-muted/50">
                  <TableCell>
                    {getActionIcon(log.action, log.status)}
                  </TableCell>
                  <TableCell className="font-mono text-sm">
                    <div className="flex items-center gap-2">
                      <Clock className="h-3 w-3 text-muted-foreground" />
                      {formatTimestamp(log.timestamp)}
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge className={actionColors[log.action] || 'bg-gray-500/20'}>
                      {log.action}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <div>
                      <span className="text-muted-foreground">
                        {resourceTypeLabels[log.resourceType] || log.resourceType}:
                      </span>{' '}
                      <span className="font-medium">
                        {log.resourceName || log.resourceId.slice(0, 8)}
                      </span>
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      <User className="h-3 w-3 text-muted-foreground" />
                      <span>{log.actorEmail || log.actorId}</span>
                      {log.actorType === 'admin' && (
                        <Badge variant="outline" className="text-xs">Admin</Badge>
                      )}
                    </div>
                  </TableCell>
                  <TableCell>
                    {log.status === 'success' ? (
                      <Badge className="bg-green-500/20 text-green-400">Success</Badge>
                    ) : (
                      <Badge className="bg-red-500/20 text-red-400">Failed</Badge>
                    )}
                  </TableCell>
                  <TableCell>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => setSelectedLog(log)}
                    >
                      <Eye className="h-4 w-4" />
                    </Button>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>

        {/* Pagination */}
        <div className="flex items-center justify-between p-4 border-t">
          <div className="text-sm text-muted-foreground">
            Showing {page * pageSize + 1} - {Math.min((page + 1) * pageSize, totalCount)} of {totalCount}
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setPage(p => p - 1)}
              disabled={page === 0}
            >
              <ChevronLeft className="h-4 w-4" />
              Previous
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setPage(p => p + 1)}
              disabled={!hasMore}
            >
              Next
              <ChevronRight className="h-4 w-4" />
            </Button>
          </div>
        </div>
      </Card>

      {/* Log Detail Dialog */}
      <Dialog open={!!selectedLog} onOpenChange={() => setSelectedLog(null)}>
        <DialogContent className="max-w-2xl max-h-[80vh]">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <ClipboardList className="h-5 w-5" />
              Audit Log Details
            </DialogTitle>
          </DialogHeader>
          {selectedLog && (
            <ScrollArea className="max-h-[60vh]">
              <div className="space-y-4 pr-4">
                {/* Basic Info */}
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className="text-sm text-muted-foreground">Timestamp</label>
                    <div className="font-mono">{formatTimestamp(selectedLog.timestamp)}</div>
                  </div>
                  <div>
                    <label className="text-sm text-muted-foreground">Status</label>
                    <div>
                      {selectedLog.status === 'success' ? (
                        <Badge className="bg-green-500/20 text-green-400">Success</Badge>
                      ) : (
                        <Badge className="bg-red-500/20 text-red-400">Failed</Badge>
                      )}
                    </div>
                  </div>
                  <div>
                    <label className="text-sm text-muted-foreground">Action</label>
                    <div>
                      <Badge className={actionColors[selectedLog.action]}>{selectedLog.action}</Badge>
                    </div>
                  </div>
                  <div>
                    <label className="text-sm text-muted-foreground">Resource Type</label>
                    <div>{resourceTypeLabels[selectedLog.resourceType] || selectedLog.resourceType}</div>
                  </div>
                  <div>
                    <label className="text-sm text-muted-foreground">Resource ID</label>
                    <div className="font-mono text-sm">{selectedLog.resourceId}</div>
                  </div>
                  <div>
                    <label className="text-sm text-muted-foreground">Resource Name</label>
                    <div>{selectedLog.resourceName || '-'}</div>
                  </div>
                </div>

                {/* Actor Info */}
                <div className="border-t pt-4">
                  <h4 className="font-medium mb-2 flex items-center gap-2">
                    <User className="h-4 w-4" />
                    Actor Information
                  </h4>
                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <label className="text-sm text-muted-foreground">Email</label>
                      <div>{selectedLog.actorEmail || '-'}</div>
                    </div>
                    <div>
                      <label className="text-sm text-muted-foreground">Type</label>
                      <div className="capitalize">{selectedLog.actorType}</div>
                    </div>
                    <div>
                      <label className="text-sm text-muted-foreground">IP Address</label>
                      <div className="flex items-center gap-2">
                        <Globe className="h-3 w-3 text-muted-foreground" />
                        {selectedLog.ipAddress || '-'}
                      </div>
                    </div>
                    <div>
                      <label className="text-sm text-muted-foreground">User Agent</label>
                      <div className="text-sm truncate">{selectedLog.userAgent || '-'}</div>
                    </div>
                  </div>
                </div>

                {/* Error Message */}
                {selectedLog.errorMessage && (
                  <div className="border-t pt-4">
                    <h4 className="font-medium mb-2 flex items-center gap-2 text-red-400">
                      <AlertTriangle className="h-4 w-4" />
                      Error Message
                    </h4>
                    <div className="bg-red-500/10 border border-red-500/20 rounded p-3 text-sm">
                      {selectedLog.errorMessage}
                    </div>
                  </div>
                )}

                {/* Changes */}
                {(selectedLog.oldValue || selectedLog.newValue) && (
                  <div className="border-t pt-4">
                    <h4 className="font-medium mb-2">Changes</h4>
                    <div className="grid grid-cols-2 gap-4">
                      {selectedLog.oldValue && (
                        <div>
                          <label className="text-sm text-muted-foreground">Before</label>
                          <pre className="bg-muted rounded p-2 text-xs overflow-auto max-h-40">
                            {JSON.stringify(selectedLog.oldValue, null, 2)}
                          </pre>
                        </div>
                      )}
                      {selectedLog.newValue && (
                        <div>
                          <label className="text-sm text-muted-foreground">After</label>
                          <pre className="bg-muted rounded p-2 text-xs overflow-auto max-h-40">
                            {JSON.stringify(selectedLog.newValue, null, 2)}
                          </pre>
                        </div>
                      )}
                    </div>
                  </div>
                )}

                {/* Additional Details */}
                {selectedLog.details && Object.keys(selectedLog.details).length > 0 && (
                  <div className="border-t pt-4">
                    <h4 className="font-medium mb-2">Additional Details</h4>
                    <pre className="bg-muted rounded p-2 text-xs overflow-auto">
                      {JSON.stringify(selectedLog.details, null, 2)}
                    </pre>
                  </div>
                )}
              </div>
            </ScrollArea>
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
}

