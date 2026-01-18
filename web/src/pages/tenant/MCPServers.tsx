import { useState } from 'react'
import { useQuery, useMutation } from '@apollo/client'
import { gql } from '@apollo/client'
import { 
  Plus, Server, RefreshCw, Plug, PlugZap, Trash2, Settings,
  CheckCircle, XCircle, Clock, AlertTriangle, Wrench
} from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useToast } from '@/components/ui/use-toast'

// GraphQL Queries and Mutations
const GET_MCP_SERVERS = gql`
  query GetMCPServers {
    mcpServers {
      id
      name
      description
      serverType
      endpoint
      authType
      version
      status
      lastHealthCheck
      lastSyncAt
      errorMessage
      toolCount
      tags
      autoSync
      syncIntervalMinutes
      createdAt
      updatedAt
    }
  }
`

const GET_MCP_TOOLS = gql`
  query GetMCPTools($serverId: ID) {
    mcpTools(serverId: $serverId) {
      id
      serverId
      serverName
      name
      description
      category
      inputSchema
      deferLoading
      isDeprecated
      executionCount
      avgExecutionTimeMs
    }
  }
`

const CREATE_MCP_SERVER = gql`
  mutation CreateMCPServer($input: CreateMCPServerInput!) {
    createMCPServer(input: $input) {
      id
      name
      status
    }
  }
`

const DELETE_MCP_SERVER = gql`
  mutation DeleteMCPServer($id: ID!) {
    deleteMCPServer(id: $id)
  }
`

const CONNECT_MCP_SERVER = gql`
  mutation ConnectMCPServer($id: ID!) {
    connectMCPServer(id: $id) {
      id
      status
      lastHealthCheck
    }
  }
`

const DISCONNECT_MCP_SERVER = gql`
  mutation DisconnectMCPServer($id: ID!) {
    disconnectMCPServer(id: $id) {
      id
      status
    }
  }
`

const SYNC_MCP_SERVER = gql`
  mutation SyncMCPServer($id: ID!) {
    syncMCPServer(id: $id) {
      id
      version
      toolCount
      changesSummary
      hasBreakingChanges
    }
  }
`

const UPDATE_MCP_SERVER = gql`
  mutation UpdateMCPServer($id: ID!, $input: UpdateMCPServerInput!) {
    updateMCPServer(id: $id, input: $input) {
      id
      name
      description
      endpoint
      authType
      status
    }
  }
`

const SEARCH_TOOLS = gql`
  query SearchTools($input: ToolSearchInput!) {
    searchTools(input: $input) {
      query
      totalAvailable
      totalAllowed
      tools {
        tool {
          id
          name
          description
          category
          serverName
        }
        score
        matchReason
      }
    }
  }
`

interface MCPServer {
  id: string
  name: string
  description: string | null
  serverType: string
  endpoint: string
  authType: string
  version: string | null
  status: string
  lastHealthCheck: string | null
  lastSyncAt: string | null
  errorMessage: string | null
  toolCount: number
  tags: string[]
  autoSync: boolean
  syncIntervalMinutes: number
  createdAt: string
  updatedAt: string
}

interface MCPTool {
  id: string
  serverId: string
  serverName: string
  name: string
  description: string | null
  category: string | null
  inputSchema: any
  deferLoading: boolean
  isDeprecated: boolean
  executionCount: number
  avgExecutionTimeMs: number | null
}

const statusConfig: Record<string, { icon: typeof CheckCircle; color: string; label: string }> = {
  CONNECTED: { icon: CheckCircle, color: 'text-green-500', label: 'Connected' },
  DISCONNECTED: { icon: XCircle, color: 'text-gray-500', label: 'Disconnected' },
  PENDING: { icon: Clock, color: 'text-yellow-500', label: 'Pending' },
  ERROR: { icon: AlertTriangle, color: 'text-red-500', label: 'Error' },
  DISABLED: { icon: XCircle, color: 'text-gray-400', label: 'Disabled' },
}

export default function MCPServersPage() {
  const { toast } = useToast()
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false)
  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false)
  const [editingServer, setEditingServer] = useState<MCPServer | null>(null)
  const [selectedServer, setSelectedServer] = useState<string | null>(null)
  const [searchQuery, setSearchQuery] = useState('')
  const [activeTab, setActiveTab] = useState('servers')

  // Form state
  const [formData, setFormData] = useState({
    name: '',
    description: '',
    serverType: 'stdio',
    endpoint: '',
    authType: 'none',
    apiKey: '',
    apiKeyHeader: 'Authorization',
    bearerToken: '',
  })

  // Queries
  const { data: serversData, loading: serversLoading, refetch: refetchServers } = useQuery(GET_MCP_SERVERS)
  const { data: toolsData, loading: toolsLoading } = useQuery(GET_MCP_TOOLS, {
    variables: { serverId: selectedServer },
    skip: !selectedServer && activeTab !== 'tools',
  })
  const { data: searchData, loading: searchLoading } = useQuery(SEARCH_TOOLS, {
    variables: { input: { query: searchQuery, maxResults: 20 } },
    skip: !searchQuery || searchQuery.length < 2,
  })

  // Mutations
  const [createServer] = useMutation(CREATE_MCP_SERVER, {
    onCompleted: () => {
      toast({ title: 'Success', description: 'MCP Server created successfully' })
      setIsCreateDialogOpen(false)
      refetchServers()
      resetForm()
    },
    onError: (error) => {
      toast({ title: 'Error', description: error.message, variant: 'destructive' })
    },
  })

  const [deleteServer] = useMutation(DELETE_MCP_SERVER, {
    onCompleted: () => {
      toast({ title: 'Success', description: 'MCP Server deleted' })
      refetchServers()
    },
    onError: (error) => {
      toast({ title: 'Error', description: error.message, variant: 'destructive' })
    },
  })

  const [connectServer] = useMutation(CONNECT_MCP_SERVER, {
    onCompleted: () => {
      toast({ title: 'Success', description: 'Connected to MCP Server' })
      refetchServers()
      setConnectingServerId(null)
    },
    onError: (error) => {
      toast({ title: 'Error', description: error.message, variant: 'destructive' })
      setConnectingServerId(null)
    },
  })

  const [disconnectServer] = useMutation(DISCONNECT_MCP_SERVER, {
    onCompleted: () => {
      toast({ title: 'Success', description: 'Disconnected from MCP Server' })
      refetchServers()
    },
    onError: (error) => {
      toast({ title: 'Error', description: error.message, variant: 'destructive' })
    },
  })

  const [syncServer, { loading: syncLoading }] = useMutation(SYNC_MCP_SERVER, {
    onCompleted: (data) => {
      const version = data.syncMCPServer
      toast({
        title: 'Sync Complete',
        description: `Version ${version.version}: ${version.changesSummary} (${version.toolCount} tools)`
      })
      refetchServers()
      setSyncingServerId(null)
    },
    onError: (error) => {
      toast({ title: 'Sync Failed', description: error.message, variant: 'destructive' })
      setSyncingServerId(null)
    },
  })

  const [updateServer] = useMutation(UPDATE_MCP_SERVER, {
    onCompleted: () => {
      toast({ title: 'Success', description: 'MCP Server updated successfully' })
      setIsEditDialogOpen(false)
      setEditingServer(null)
      refetchServers()
    },
    onError: (error) => {
      toast({ title: 'Error', description: error.message, variant: 'destructive' })
    },
  })

  const [syncingServerId, setSyncingServerId] = useState<string | null>(null)
  const [connectingServerId, setConnectingServerId] = useState<string | null>(null)

  const handleSyncServer = (serverId: string) => {
    setSyncingServerId(serverId)
    syncServer({ variables: { id: serverId } })
  }

  const handleConnectServer = (serverId: string) => {
    setConnectingServerId(serverId)
    connectServer({ variables: { id: serverId } })
  }

  const resetForm = () => {
    setFormData({
      name: '',
      description: '',
      serverType: 'stdio',
      endpoint: '',
      authType: 'none',
      apiKey: '',
      apiKeyHeader: 'Authorization',
      bearerToken: '',
    })
  }

  const handleCreateServer = () => {
    const input: any = {
      name: formData.name,
      description: formData.description || null,
      serverType: formData.serverType.toUpperCase(),
      endpoint: formData.endpoint,
      authType: formData.authType.toUpperCase(),
    }

    if (formData.authType === 'api_key' && formData.apiKey) {
      input.authConfig = {
        apiKey: formData.apiKey,
        apiKeyHeader: formData.apiKeyHeader,
      }
    }

    if (formData.authType === 'bearer' && formData.bearerToken) {
      input.authConfig = {
        bearerToken: formData.bearerToken,
      }
    }

    createServer({ variables: { input } })
  }

  const handleEditServer = () => {
    if (!editingServer) return

    const input: any = {
      name: formData.name,
      description: formData.description || null,
      endpoint: formData.endpoint,
      authType: formData.authType.toUpperCase(),
    }

    // Only include auth config if new credentials are provided
    if (formData.authType === 'api_key' && formData.apiKey) {
      input.authConfig = {
        apiKey: formData.apiKey,
        apiKeyHeader: formData.apiKeyHeader,
      }
    }

    if (formData.authType === 'bearer' && formData.bearerToken) {
      input.authConfig = {
        bearerToken: formData.bearerToken,
      }
    }

    updateServer({ variables: { id: editingServer.id, input } })
  }

  const servers: MCPServer[] = serversData?.mcpServers || []
  const tools: MCPTool[] = toolsData?.mcpTools || []
  const searchResults = searchData?.searchTools?.tools || []

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold">MCP Gateway</h1>
          <p className="text-muted-foreground">
            Manage MCP servers and discover tools
          </p>
        </div>
        <Dialog open={isCreateDialogOpen} onOpenChange={setIsCreateDialogOpen}>
          <DialogTrigger asChild>
            <Button>
              <Plus className="h-4 w-4 mr-2" />
              Add MCP Server
            </Button>
          </DialogTrigger>
          <DialogContent className="max-w-lg">
            <DialogHeader>
              <DialogTitle>Add MCP Server</DialogTitle>
              <DialogDescription>
                Connect to an MCP server to discover and use its tools
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label htmlFor="name">Name</Label>
                <Input
                  id="name"
                  placeholder="e.g., github-mcp"
                  value={formData.name}
                  onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="description">Description</Label>
                <Textarea
                  id="description"
                  placeholder="Optional description"
                  value={formData.description}
                  onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="serverType">Server Type</Label>
                <Select
                  value={formData.serverType}
                  onValueChange={(v) => setFormData({ ...formData, serverType: v })}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="stdio">Stdio (Local Process)</SelectItem>
                    <SelectItem value="sse">SSE (HTTP)</SelectItem>
                    <SelectItem value="websocket">WebSocket</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label htmlFor="endpoint">
                  {formData.serverType === 'stdio' ? 'Command' : 'Endpoint URL'}
                </Label>
                <Input
                  id="endpoint"
                  placeholder={formData.serverType === 'stdio' ? 'npx -y @modelcontextprotocol/server-github' : 'https://mcp.example.com'}
                  value={formData.endpoint}
                  onChange={(e) => setFormData({ ...formData, endpoint: e.target.value })}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="authType">Authentication</Label>
                <Select
                  value={formData.authType}
                  onValueChange={(v) => setFormData({ ...formData, authType: v })}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="none">None</SelectItem>
                    <SelectItem value="api_key">API Key</SelectItem>
                    <SelectItem value="bearer">Bearer Token</SelectItem>
                    <SelectItem value="basic">Basic Auth</SelectItem>
                    <SelectItem value="oauth2">OAuth2</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              {formData.authType === 'api_key' && (
                <>
                  <div className="space-y-2">
                    <Label htmlFor="apiKey">API Key</Label>
                    <Input
                      id="apiKey"
                      type="password"
                      placeholder="Enter API key"
                      value={formData.apiKey}
                      onChange={(e) => setFormData({ ...formData, apiKey: e.target.value })}
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="apiKeyHeader">Header Name</Label>
                    <Input
                      id="apiKeyHeader"
                      placeholder="Authorization"
                      value={formData.apiKeyHeader}
                      onChange={(e) => setFormData({ ...formData, apiKeyHeader: e.target.value })}
                    />
                  </div>
                </>
              )}
              {formData.authType === 'bearer' && (
                <div className="space-y-2">
                  <Label htmlFor="bearerToken">Bearer Token</Label>
                  <Input
                    id="bearerToken"
                    type="password"
                    placeholder="Enter bearer token (e.g., tvly-your-api-key)"
                    value={formData.bearerToken}
                    onChange={(e) => setFormData({ ...formData, bearerToken: e.target.value })}
                  />
                  <p className="text-xs text-muted-foreground">
                    Token will be sent as: Authorization: Bearer &lt;token&gt;
                  </p>
                </div>
              )}
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => setIsCreateDialogOpen(false)}>
                Cancel
              </Button>
              <Button onClick={handleCreateServer} disabled={!formData.name || !formData.endpoint}>
                Create Server
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        {/* Edit MCP Server Dialog */}
        <Dialog open={isEditDialogOpen} onOpenChange={setIsEditDialogOpen}>
          <DialogContent className="max-w-lg">
            <DialogHeader>
              <DialogTitle>Edit MCP Server</DialogTitle>
              <DialogDescription>
                Update the configuration for this MCP server
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label htmlFor="edit-name">Name</Label>
                <Input
                  id="edit-name"
                  placeholder="e.g., github-mcp"
                  value={formData.name}
                  onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="edit-description">Description</Label>
                <Textarea
                  id="edit-description"
                  placeholder="Optional description"
                  value={formData.description}
                  onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="edit-serverType">Server Type</Label>
                <Select
                  value={formData.serverType}
                  onValueChange={(v) => setFormData({ ...formData, serverType: v })}
                  disabled
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="stdio">Stdio (Local Process)</SelectItem>
                    <SelectItem value="sse">SSE (HTTP)</SelectItem>
                    <SelectItem value="websocket">WebSocket</SelectItem>
                  </SelectContent>
                </Select>
                <p className="text-xs text-muted-foreground">Server type cannot be changed after creation</p>
              </div>
              <div className="space-y-2">
                <Label htmlFor="edit-endpoint">
                  {formData.serverType === 'stdio' ? 'Command' : 'Endpoint URL'}
                </Label>
                <Input
                  id="edit-endpoint"
                  placeholder={formData.serverType === 'stdio' ? 'npx -y @modelcontextprotocol/server-github' : 'https://mcp.example.com'}
                  value={formData.endpoint}
                  onChange={(e) => setFormData({ ...formData, endpoint: e.target.value })}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="edit-authType">Authentication</Label>
                <Select
                  value={formData.authType}
                  onValueChange={(v) => setFormData({ ...formData, authType: v })}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="none">None</SelectItem>
                    <SelectItem value="api_key">API Key</SelectItem>
                    <SelectItem value="bearer">Bearer Token</SelectItem>
                    <SelectItem value="basic">Basic Auth</SelectItem>
                    <SelectItem value="oauth2">OAuth2</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              {formData.authType === 'api_key' && (
                <>
                  <div className="space-y-2">
                    <Label htmlFor="edit-apiKey">API Key</Label>
                    <Input
                      id="edit-apiKey"
                      type="password"
                      placeholder="Leave blank to keep existing"
                      value={formData.apiKey}
                      onChange={(e) => setFormData({ ...formData, apiKey: e.target.value })}
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="edit-apiKeyHeader">Header Name</Label>
                    <Input
                      id="edit-apiKeyHeader"
                      placeholder="Authorization"
                      value={formData.apiKeyHeader}
                      onChange={(e) => setFormData({ ...formData, apiKeyHeader: e.target.value })}
                    />
                  </div>
                </>
              )}
              {formData.authType === 'bearer' && (
                <div className="space-y-2">
                  <Label htmlFor="edit-bearerToken">Bearer Token</Label>
                  <Input
                    id="edit-bearerToken"
                    type="password"
                    placeholder="Leave blank to keep existing"
                    value={formData.bearerToken}
                    onChange={(e) => setFormData({ ...formData, bearerToken: e.target.value })}
                  />
                  <p className="text-xs text-muted-foreground">
                    Token will be sent as: Authorization: Bearer &lt;token&gt;
                  </p>
                </div>
              )}
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => {
                setIsEditDialogOpen(false)
                setEditingServer(null)
              }}>
                Cancel
              </Button>
              <Button onClick={handleEditServer} disabled={!formData.name || !formData.endpoint}>
                Update Server
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      {/* Tabs */}
      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <TabsList>
          <TabsTrigger value="servers">
            <Server className="h-4 w-4 mr-2" />
            Servers ({servers.length})
          </TabsTrigger>
          <TabsTrigger value="tools">
            <Wrench className="h-4 w-4 mr-2" />
            Tools
          </TabsTrigger>
          <TabsTrigger value="search">
            <RefreshCw className="h-4 w-4 mr-2" />
            Search
          </TabsTrigger>
        </TabsList>

        {/* Servers Tab */}
        <TabsContent value="servers" className="space-y-4">
          {serversLoading ? (
            <div className="text-center py-8 text-muted-foreground">Loading servers...</div>
          ) : servers.length === 0 ? (
            <Card>
              <CardContent className="py-12 text-center">
                <Server className="h-12 w-12 mx-auto text-muted-foreground mb-4" />
                <h3 className="text-lg font-medium mb-2">No MCP Servers</h3>
                <p className="text-muted-foreground mb-4">
                  Add an MCP server to start discovering tools
                </p>
                <Button onClick={() => setIsCreateDialogOpen(true)}>
                  <Plus className="h-4 w-4 mr-2" />
                  Add MCP Server
                </Button>
              </CardContent>
            </Card>
          ) : (
            <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
              {servers.map((server) => {
                const normalizedStatus = server.status?.toUpperCase() || 'PENDING'
                const statusInfo = statusConfig[normalizedStatus] || statusConfig.PENDING
                const StatusIcon = statusInfo.icon

                return (
                  <Card key={server.id} className="relative">
                    <CardHeader className="pb-3">
                      <div className="flex items-start justify-between">
                        <div className="flex items-center gap-2">
                          <StatusIcon className={`h-5 w-5 ${statusInfo.color}`} />
                          <div>
                            <CardTitle className="text-base">{server.name}</CardTitle>
                            <CardDescription className="text-xs">
                              {server.serverType?.toLowerCase() || 'unknown'} • {server.authType?.toLowerCase() || 'none'}
                            </CardDescription>
                          </div>
                        </div>
                        <Badge variant={normalizedStatus === 'CONNECTED' ? 'default' : 'secondary'}>
                          {statusInfo.label}
                        </Badge>
                      </div>
                    </CardHeader>
                    <CardContent className="space-y-3">
                      {server.description && (
                        <p className="text-sm text-muted-foreground line-clamp-2">
                          {server.description}
                        </p>
                      )}
                      <div className="flex items-center justify-between text-sm">
                        <span className="text-muted-foreground">Tools</span>
                        <Badge variant="outline">{server.toolCount}</Badge>
                      </div>
                      {server.version && (
                        <div className="flex items-center justify-between text-sm">
                          <span className="text-muted-foreground">Version</span>
                          <span className="font-mono text-xs">{server.version}</span>
                        </div>
                      )}
                      {server.errorMessage && (
                        <p className="text-xs text-red-500 truncate" title={server.errorMessage}>
                          {server.errorMessage}
                        </p>
                      )}
                      <div className="flex gap-2 pt-2">
                        {normalizedStatus === 'CONNECTED' ? (
                          <>
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => handleSyncServer(server.id)}
                              disabled={syncingServerId === server.id}
                            >
                              <RefreshCw className={`h-3 w-3 mr-1 ${syncingServerId === server.id ? 'animate-spin' : ''}`} />
                              {syncingServerId === server.id ? 'Syncing...' : 'Sync'}
                            </Button>
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => disconnectServer({ variables: { id: server.id } })}
                            >
                              <PlugZap className="h-3 w-3 mr-1" />
                              Disconnect
                            </Button>
                          </>
                        ) : (
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => handleConnectServer(server.id)}
                            disabled={connectingServerId === server.id}
                          >
                            <Plug className={`h-3 w-3 mr-1 ${connectingServerId === server.id ? 'animate-spin' : ''}`} />
                            {connectingServerId === server.id ? 'Connecting...' : 'Connect'}
                          </Button>
                        )}
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => {
                            setEditingServer(server)
                            setFormData({
                              name: server.name,
                              description: server.description || '',
                              serverType: server.serverType.toLowerCase(),
                              endpoint: server.endpoint,
                              authType: server.authType.toLowerCase(),
                              apiKey: '',
                              apiKeyHeader: 'Authorization',
                              bearerToken: '',
                            })
                            setIsEditDialogOpen(true)
                          }}
                        >
                          <Settings className="h-3 w-3" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          className="text-red-500 hover:text-red-700"
                          onClick={() => {
                            if (confirm('Are you sure you want to delete this server?')) {
                              deleteServer({ variables: { id: server.id } })
                            }
                          }}
                        >
                          <Trash2 className="h-3 w-3" />
                        </Button>
                      </div>
                    </CardContent>
                  </Card>
                )
              })}
            </div>
          )}
        </TabsContent>

        {/* Tools Tab */}
        <TabsContent value="tools" className="space-y-4">
          <div className="flex gap-4">
            <Select
              value={selectedServer || 'all'}
              onValueChange={(v) => setSelectedServer(v === 'all' ? null : v)}
            >
              <SelectTrigger className="w-64">
                <SelectValue placeholder="Filter by server" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All Servers</SelectItem>
                {servers.map((s) => (
                  <SelectItem key={s.id} value={s.id}>
                    {s.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {toolsLoading ? (
            <div className="text-center py-8 text-muted-foreground">Loading tools...</div>
          ) : tools.length === 0 ? (
            <Card>
              <CardContent className="py-12 text-center">
                <Wrench className="h-12 w-12 mx-auto text-muted-foreground mb-4" />
                <h3 className="text-lg font-medium mb-2">No Tools Found</h3>
                <p className="text-muted-foreground">
                  Connect to an MCP server and sync to discover tools
                </p>
              </CardContent>
            </Card>
          ) : (
            <Card>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Tool</TableHead>
                    <TableHead>Server</TableHead>
                    <TableHead>Category</TableHead>
                    <TableHead className="text-right">Executions</TableHead>
                    <TableHead className="text-right">Avg Time</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {tools.map((tool) => (
                    <TableRow key={tool.id}>
                      <TableCell>
                        <div>
                          <code className="text-sm font-mono">{tool.name}</code>
                          {tool.description && (
                            <p className="text-xs text-muted-foreground line-clamp-1">
                              {tool.description}
                            </p>
                          )}
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge variant="outline">{tool.serverName}</Badge>
                      </TableCell>
                      <TableCell>
                        {tool.category && (
                          <Badge variant="secondary">{tool.category}</Badge>
                        )}
                      </TableCell>
                      <TableCell className="text-right">
                        {tool.executionCount}
                      </TableCell>
                      <TableCell className="text-right">
                        {tool.avgExecutionTimeMs ? `${tool.avgExecutionTimeMs}ms` : '-'}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </Card>
          )}
        </TabsContent>

        {/* Search Tab */}
        <TabsContent value="search" className="space-y-4">
          <div className="flex gap-4">
            <Input
              placeholder="Search for tools... (e.g., 'send message to slack', 'create github issue')"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="max-w-xl"
            />
          </div>

          {searchQuery.length >= 2 && (
            <>
              {searchLoading ? (
                <div className="text-center py-8 text-muted-foreground">Searching...</div>
              ) : searchResults.length === 0 ? (
                <Card>
                  <CardContent className="py-12 text-center">
                    <p className="text-muted-foreground">No tools found matching "{searchQuery}"</p>
                  </CardContent>
                </Card>
              ) : (
                <Card>
                  <CardHeader>
                    <CardTitle className="text-base">
                      Search Results ({searchData?.searchTools?.totalAvailable || 0} tools)
                    </CardTitle>
                  </CardHeader>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Tool</TableHead>
                        <TableHead>Server</TableHead>
                        <TableHead>Category</TableHead>
                        <TableHead className="text-right">Score</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {searchResults.map((result: any) => (
                        <TableRow key={result.tool.id}>
                          <TableCell>
                            <div>
                              <code className="text-sm font-mono">{result.tool.name}</code>
                              {result.tool.description && (
                                <p className="text-xs text-muted-foreground line-clamp-1">
                                  {result.tool.description}
                                </p>
                              )}
                            </div>
                          </TableCell>
                          <TableCell>
                            <Badge variant="outline">{result.tool.serverName}</Badge>
                          </TableCell>
                          <TableCell>
                            {result.tool.category && (
                              <Badge variant="secondary">{result.tool.category}</Badge>
                            )}
                          </TableCell>
                          <TableCell className="text-right">
                            {(result.score * 100).toFixed(0)}%
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </Card>
              )}
            </>
          )}

          {searchQuery.length < 2 && (
            <Card>
              <CardContent className="py-12 text-center">
                <RefreshCw className="h-12 w-12 mx-auto text-muted-foreground mb-4" />
                <h3 className="text-lg font-medium mb-2">Tool Search</h3>
                <p className="text-muted-foreground">
                  Search for tools using natural language queries. <br />
                  Try "send a message" or "read file contents"
                </p>
              </CardContent>
            </Card>
          )}
        </TabsContent>
      </Tabs>
    </div>
  )
}

