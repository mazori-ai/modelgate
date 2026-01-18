import { useState } from 'react'
import { useMutation } from '@apollo/client'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useToast } from '@/components/ui/use-toast'
import {
  ADD_PROVIDER_API_KEY,
  UPDATE_PROVIDER_API_KEY,
  DELETE_PROVIDER_API_KEY,
} from '@/graphql/operations'
import {
  Plus,
  Trash,
  Edit,
  Check,
  X,
  Activity,
  Clock,
  Loader2,
} from 'lucide-react'
import { cn } from '@/lib/utils'

interface ProviderAPIKey {
  id: string
  provider: string
  name: string | null
  keyPrefix: string
  credentialType: string
  priority: number
  enabled: boolean
  healthScore: number
  successCount: number
  failureCount: number
  rateLimitRemaining: number | null
  rateLimitResetAt: string | null
  requestCount: number
  lastUsedAt: string | null
  createdAt: string
  updatedAt: string
}

interface APIKeyManagerProps {
  provider: string
  providerName: string
  apiKeys: ProviderAPIKey[]
  open: boolean
  onClose: () => void
  onRefetch: () => void
}

function formatRelativeTime(date: string | null): string {
  if (!date) return 'Never'
  const now = new Date()
  const then = new Date(date)
  const diffMs = then.getTime() - now.getTime()
  const diffMinutes = Math.round(diffMs / 60000)

  if (Math.abs(diffMinutes) === 0) return 'just now'
  if (diffMinutes > 0) {
    if (diffMinutes < 60) return `in ${diffMinutes} min`
    if (diffMinutes < 1440) return `in ${Math.round(diffMinutes / 60)} hr`
    return `in ${Math.round(diffMinutes / 1440)} days`
  } else {
    const absDiffMinutes = Math.abs(diffMinutes)
    if (absDiffMinutes < 60) return `${absDiffMinutes} min ago`
    if (absDiffMinutes < 1440) return `${Math.round(absDiffMinutes / 60)} hr ago`
    return `${Math.round(absDiffMinutes / 1440)} days ago`
  }
}

function HealthBadge({ score }: { score: number }) {
  const percentage = Math.round(score * 100)
  const variant =
    score >= 0.8 ? 'default' : score >= 0.5 ? 'secondary' : 'destructive'

  return (
    <Badge variant={variant} className="text-xs">
      {percentage}%
    </Badge>
  )
}

function AddKeyForm({
  provider,
  onSuccess,
}: {
  provider: string
  onSuccess: () => void
}) {
  const { toast } = useToast()
  const isBedrock = provider === 'BEDROCK'
  const [credentialType, setCredentialType] = useState<'api_key' | 'iam_credentials'>(
    isBedrock ? 'iam_credentials' : 'api_key'
  )
  const [formData, setFormData] = useState({
    name: '',
    apiKey: '',
    accessKeyId: '',
    secretAccessKey: '',
    priority: 1,
  })

  const [addKey, { loading }] = useMutation(ADD_PROVIDER_API_KEY, {
    onCompleted: () => {
      toast({ title: 'Credentials added successfully' })
      setFormData({ name: '', apiKey: '', accessKeyId: '', secretAccessKey: '', priority: 1 })
      onSuccess()
    },
    onError: (error) => {
      toast({
        title: 'Failed to add credentials',
        description: error.message,
        variant: 'destructive',
      })
    },
  })

  const handleSubmit = () => {
    // Validation based on credential type
    if (credentialType === 'api_key' && !formData.apiKey) {
      toast({ title: 'API key required', variant: 'destructive' })
      return
    }
    if (credentialType === 'iam_credentials' && (!formData.accessKeyId || !formData.secretAccessKey)) {
      toast({ title: 'Both Access Key ID and Secret Access Key required', variant: 'destructive' })
      return
    }

    addKey({
      variables: {
        input: {
          provider,
          apiKey: credentialType === 'api_key' ? formData.apiKey : null,
          accessKeyId: credentialType === 'iam_credentials' ? formData.accessKeyId : null,
          secretAccessKey: credentialType === 'iam_credentials' ? formData.secretAccessKey : null,
          name: formData.name || null,
          priority: formData.priority,
        },
      },
    })
  }

  return (
    <div className="space-y-3">
      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-2">
          <label className="text-sm font-medium">Name (Optional)</label>
          <Input
            placeholder="Production Key"
            value={formData.name}
            onChange={(e) => setFormData({ ...formData, name: e.target.value })}
          />
        </div>

        <div className="space-y-2">
          <label className="text-sm font-medium">Priority</label>
          <Select
            value={formData.priority.toString()}
            onValueChange={(val) =>
              setFormData({ ...formData, priority: parseInt(val) })
            }
          >
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="1">1 - Primary</SelectItem>
              <SelectItem value="2">2 - Secondary</SelectItem>
              <SelectItem value="3">3 - Tertiary</SelectItem>
              <SelectItem value="4">4 - Backup</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      {isBedrock && (
        <div className="space-y-2">
          <label className="text-sm font-medium">Authentication Type</label>
          <Select
            value={credentialType}
            onValueChange={(val: any) => setCredentialType(val)}
          >
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="iam_credentials">
                ⚡ IAM Credentials - True Streaming (Recommended)
              </SelectItem>
              <SelectItem value="api_key">
                API Key - Simulated Streaming
              </SelectItem>
            </SelectContent>
          </Select>
          <p className="text-xs text-muted-foreground">
            {credentialType === 'iam_credentials'
              ? 'IAM credentials provide true streaming with better performance (AWS recommended)'
              : 'API key uses simulated streaming with higher latency'}
          </p>
        </div>
      )}

      {credentialType === 'api_key' ? (
        <div className="space-y-2">
          <label className="text-sm font-medium">API Key</label>
          <Input
            type="password"
            placeholder="sk-proj-..."
            value={formData.apiKey}
            onChange={(e) => setFormData({ ...formData, apiKey: e.target.value })}
          />
          <p className="text-xs text-muted-foreground">
            Key will be encrypted at rest
          </p>
        </div>
      ) : (
        <>
          <div className="space-y-2">
            <label className="text-sm font-medium">Access Key ID</label>
            <Input
              placeholder="AKIA..."
              value={formData.accessKeyId}
              onChange={(e) => setFormData({ ...formData, accessKeyId: e.target.value })}
            />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">Secret Access Key</label>
            <Input
              type="password"
              placeholder="Enter secret access key"
              value={formData.secretAccessKey}
              onChange={(e) => setFormData({ ...formData, secretAccessKey: e.target.value })}
            />
            <p className="text-xs text-muted-foreground">
              IAM credentials will be encrypted at rest
            </p>
          </div>
        </>
      )}

      <Button onClick={handleSubmit} disabled={loading} className="w-full">
        {loading ? (
          <>
            <Loader2 className="mr-2 h-4 w-4 animate-spin" />
            Adding...
          </>
        ) : (
          <>
            <Plus className="mr-2 h-4 w-4" />
            Add {credentialType === 'iam_credentials' ? 'IAM Credentials' : 'API Key'}
          </>
        )}
      </Button>
    </div>
  )
}

function KeyCard({
  apiKey,
  onUpdate,
}: {
  apiKey: ProviderAPIKey
  onUpdate: () => void
}) {
  const { toast } = useToast()
  const [editing, setEditing] = useState(false)
  const [editData, setEditData] = useState({
    name: apiKey.name || '',
    priority: apiKey.priority,
  })

  const [updateKey, { loading: updating }] = useMutation(UPDATE_PROVIDER_API_KEY, {
    onCompleted: () => {
      toast({ title: 'API key updated' })
      setEditing(false)
      onUpdate()
    },
    onError: (error) => {
      toast({
        title: 'Failed to update key',
        description: error.message,
        variant: 'destructive',
      })
    },
  })

  const [deleteKey, { loading: deleting }] = useMutation(DELETE_PROVIDER_API_KEY, {
    onCompleted: () => {
      toast({ title: 'API key deleted' })
      onUpdate()
    },
    onError: (error) => {
      toast({
        title: 'Failed to delete key',
        description: error.message,
        variant: 'destructive',
      })
    },
  })

  const handleToggleEnabled = () => {
    updateKey({
      variables: {
        input: {
          id: apiKey.id,
          enabled: !apiKey.enabled,
        },
      },
    })
  }

  const handleSave = () => {
    updateKey({
      variables: {
        input: {
          id: apiKey.id,
          name: editData.name || null,
          priority: editData.priority,
        },
      },
    })
  }

  const handleDelete = () => {
    if (confirm(`Delete API key "${apiKey.name || apiKey.keyPrefix}"?`)) {
      deleteKey({ variables: { id: apiKey.id } })
    }
  }

  const successRate =
    apiKey.successCount + apiKey.failureCount > 0
      ? ((apiKey.successCount / (apiKey.successCount + apiKey.failureCount)) * 100).toFixed(1)
      : '0.0'

  return (
    <Card className={cn(!apiKey.enabled && 'opacity-60')}>
      <CardContent className="pt-4">
        <div className="space-y-3">
          {/* Header */}
          <div className="flex items-start justify-between">
            <div className="flex items-center gap-3 flex-1">
              <Switch
                checked={apiKey.enabled}
                onCheckedChange={handleToggleEnabled}
                disabled={updating}
              />
              <div className="flex-1">
                <div className="font-medium">
                  {apiKey.name || 'Unnamed Key'}
                </div>
                <div className="text-xs text-muted-foreground font-mono">
                  {apiKey.keyPrefix}...
                </div>
              </div>
              <Badge variant="outline">P{apiKey.priority}</Badge>
              {apiKey.credentialType === 'iam_credentials' && (
                <Badge variant="default" className="text-xs">⚡ IAM</Badge>
              )}
              {apiKey.credentialType === 'api_key' && (
                <Badge variant="secondary" className="text-xs">🔑 Key</Badge>
              )}
              <HealthBadge score={apiKey.healthScore} />
            </div>
            <div className="flex items-center gap-1">
              <Button
                size="sm"
                variant="ghost"
                onClick={() => setEditing(!editing)}
              >
                <Edit className="h-4 w-4" />
              </Button>
              <Button
                size="sm"
                variant="ghost"
                onClick={handleDelete}
                disabled={deleting}
              >
                <Trash className="h-4 w-4" />
              </Button>
            </div>
          </div>

          {/* Stats */}
          <div className="grid grid-cols-4 gap-3 text-xs">
            <div>
              <div className="text-muted-foreground">Requests</div>
              <div className="font-medium">{apiKey.requestCount.toLocaleString()}</div>
            </div>
            <div>
              <div className="text-muted-foreground">Success Rate</div>
              <div className="font-medium">{successRate}%</div>
            </div>
            <div>
              <div className="text-muted-foreground">Rate Limit</div>
              <div className="font-medium">
                {apiKey.rateLimitRemaining !== null ? (
                  <span
                    className={cn(
                      apiKey.rateLimitRemaining > 100
                        ? 'text-green-600'
                        : apiKey.rateLimitRemaining > 10
                        ? 'text-yellow-600'
                        : 'text-red-600'
                    )}
                  >
                    {apiKey.rateLimitRemaining}
                  </span>
                ) : (
                  <span className="text-muted-foreground">N/A</span>
                )}
              </div>
            </div>
            <div>
              <div className="text-muted-foreground">Last Used</div>
              <div className="font-medium">
                {formatRelativeTime(apiKey.lastUsedAt)}
              </div>
            </div>
          </div>

          {/* Rate Limit Warning */}
          {apiKey.rateLimitResetAt &&
            apiKey.rateLimitRemaining !== null &&
            apiKey.rateLimitRemaining <= 0 && (
              <div className="flex items-center gap-2 text-xs text-amber-600 bg-amber-50 dark:bg-amber-950/20 px-2 py-1 rounded">
                <Clock className="h-3 w-3" />
                Rate limit resets {formatRelativeTime(apiKey.rateLimitResetAt)}
              </div>
            )}

          {/* Edit Form */}
          {editing && (
            <div className="pt-3 border-t space-y-3">
              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-2">
                  <label className="text-sm font-medium">Name</label>
                  <Input
                    placeholder="Production Key"
                    value={editData.name}
                    onChange={(e) =>
                      setEditData({ ...editData, name: e.target.value })
                    }
                  />
                </div>
                <div className="space-y-2">
                  <label className="text-sm font-medium">Priority</label>
                  <Select
                    value={editData.priority.toString()}
                    onValueChange={(val) =>
                      setEditData({ ...editData, priority: parseInt(val) })
                    }
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="1">1 - Primary</SelectItem>
                      <SelectItem value="2">2 - Secondary</SelectItem>
                      <SelectItem value="3">3 - Tertiary</SelectItem>
                      <SelectItem value="4">4 - Backup</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>
              <div className="flex gap-2">
                <Button size="sm" onClick={handleSave} disabled={updating}>
                  <Check className="mr-2 h-4 w-4" />
                  Save
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => setEditing(false)}
                >
                  <X className="mr-2 h-4 w-4" />
                  Cancel
                </Button>
              </div>
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  )
}

export function APIKeyManager({
  provider,
  providerName,
  apiKeys,
  open,
  onClose,
  onRefetch,
}: APIKeyManagerProps) {
  const sortedKeys = [...apiKeys].sort((a, b) => a.priority - b.priority)

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="max-w-4xl max-h-[80vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Manage {providerName} API Keys</DialogTitle>
          <DialogDescription>
            Add multiple API keys with different priorities for automatic failover
            and load balancing
          </DialogDescription>
        </DialogHeader>

        {/* Add New Key */}
        <Card>
          <CardHeader>
            <CardTitle className="text-sm">Add New API Key</CardTitle>
          </CardHeader>
          <CardContent>
            <AddKeyForm provider={provider} onSuccess={onRefetch} />
          </CardContent>
        </Card>

        {/* Existing Keys */}
        <div className="space-y-3">
          <h3 className="text-sm font-medium">
            Existing Keys ({apiKeys.length})
          </h3>

          {apiKeys.length === 0 ? (
            <div className="text-center py-8 text-muted-foreground text-sm">
              No API keys configured. Add one above to get started.
            </div>
          ) : (
            <div className="space-y-2">
              {sortedKeys.map((key) => (
                <KeyCard key={key.id} apiKey={key} onUpdate={onRefetch} />
              ))}
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
