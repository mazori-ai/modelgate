import { useState } from 'react'
import { useQuery, useMutation } from '@apollo/client'
import { Plus, Key, Copy, Check, Trash2, Edit2, AlertTriangle, Clock, User } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { GET_API_KEYS, GET_ROLES, GET_GROUPS, CREATE_API_KEY, DELETE_API_KEY } from '@/graphql/operations'
import { formatDate } from '@/lib/utils'

export function APIKeysPage() {
  const [showCreate, setShowCreate] = useState(false)
  const [newSecret, setNewSecret] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  const { data, loading, refetch } = useQuery(GET_API_KEYS)
  const { data: rolesData } = useQuery(GET_ROLES)
  const { data: groupsData } = useQuery(GET_GROUPS)

  const [createAPIKey] = useMutation(CREATE_API_KEY, {
    onCompleted: (data) => {
      setNewSecret(data.createAPIKey.secret)
      refetch()
    },
  })

  const [deleteAPIKey] = useMutation(DELETE_API_KEY, {
    onCompleted: () => refetch(),
  })

  const apiKeys = data?.apiKeys || []
  const roles = rolesData?.roles || []
  const groups = groupsData?.groups || []

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">API Keys</h1>
          <p className="text-muted-foreground">
            Manage API keys for programmatic access
          </p>
        </div>
        <Button onClick={() => setShowCreate(true)}>
          <Plus className="mr-2 h-4 w-4" />
          New API Key
        </Button>
      </div>

      <Card>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Key Prefix</TableHead>
              <TableHead>Role / Group</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Created By</TableHead>
              <TableHead>Last Used</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading ? (
              <TableRow>
                <TableCell colSpan={7} className="text-center py-8 text-muted-foreground">
                  Loading...
                </TableCell>
              </TableRow>
            ) : apiKeys.length === 0 ? (
              <TableRow>
                <TableCell colSpan={7} className="text-center py-8 text-muted-foreground">
                  No API keys yet. Create one to start using the API.
                </TableCell>
              </TableRow>
            ) : (
              apiKeys.map((key: any) => {
                const isExpired = key.isExpired || (key.expiresAt && new Date(key.expiresAt) < new Date())
                const isRevoked = key.revoked

                return (
                  <TableRow key={key.id} className={isExpired || isRevoked ? 'opacity-60' : ''}>
                    <TableCell>
                      <div className="flex items-center gap-2">
                        <Key className="h-4 w-4 text-muted-foreground" />
                        <span className="font-medium">{key.name}</span>
                      </div>
                    </TableCell>
                    <TableCell>
                      <code className="rounded bg-muted px-2 py-1 text-sm">
                        {key.keyPrefix}...
                      </code>
                    </TableCell>
                    <TableCell>
                      {key.role ? (
                        <Badge variant="outline">{key.role.name}</Badge>
                      ) : key.group ? (
                        <Badge className="bg-purple-500/20 text-purple-400">{key.group.name}</Badge>
                      ) : (
                        <span className="text-muted-foreground">—</span>
                      )}
                    </TableCell>
                    <TableCell>
                      {isRevoked ? (
                        <Badge className="bg-red-500/20 text-red-400">
                          <AlertTriangle className="h-3 w-3 mr-1" />
                          Revoked
                        </Badge>
                      ) : isExpired ? (
                        <Badge className="bg-orange-500/20 text-orange-400">
                          <Clock className="h-3 w-3 mr-1" />
                          Expired
                        </Badge>
                      ) : key.expiresAt ? (
                        <div className="text-sm">
                          <Badge className="bg-green-500/20 text-green-400">Active</Badge>
                          <div className="text-xs text-muted-foreground mt-1">
                            Expires: {formatDate(key.expiresAt)}
                          </div>
                        </div>
                      ) : (
                        <Badge className="bg-green-500/20 text-green-400">Active</Badge>
                      )}
                    </TableCell>
                    <TableCell>
                      {key.createdByEmail ? (
                        <div className="flex items-center gap-1 text-sm">
                          <User className="h-3 w-3 text-muted-foreground" />
                          <span>{key.createdByEmail}</span>
                        </div>
                      ) : (
                        <span className="text-muted-foreground text-sm">—</span>
                      )}
                      <div className="text-xs text-muted-foreground">
                        {formatDate(key.createdAt)}
                      </div>
                    </TableCell>
                    <TableCell className="text-muted-foreground text-sm">
                      {key.lastUsedAt ? formatDate(key.lastUsedAt) : 'Never'}
                    </TableCell>
                    <TableCell className="text-right">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => {
                          if (confirm(`Delete API key "${key.name}"?`)) {
                            deleteAPIKey({ variables: { id: key.id } })
                          }
                        }}
                      >
                        <Trash2 className="h-4 w-4 text-destructive" />
                      </Button>
                    </TableCell>
                  </TableRow>
                )
              })
            )}
          </TableBody>
        </Table>
      </Card>

      <CreateAPIKeyDialog
        open={showCreate}
        onOpenChange={setShowCreate}
        roles={roles}
        groups={groups}
        onSubmit={(data) => createAPIKey({ variables: { input: data } })}
      />

      {/* Secret Display Dialog */}
      <Dialog open={!!newSecret} onOpenChange={() => setNewSecret(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>API Key Created</DialogTitle>
            <DialogDescription>
              Copy your API key now. You won't be able to see it again!
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="flex items-center gap-2">
              <code className="flex-1 rounded bg-muted px-4 py-3 text-sm font-mono break-all">
                {newSecret}
              </code>
              <Button
                variant="outline"
                size="icon"
                onClick={() => copyToClipboard(newSecret || '')}
              >
                {copied ? (
                  <Check className="h-4 w-4 text-green-500" />
                ) : (
                  <Copy className="h-4 w-4" />
                )}
              </Button>
            </div>
            <p className="text-sm text-destructive">
              ⚠️ Save this key securely. It will not be shown again.
            </p>
          </div>
          <DialogFooter>
            <Button onClick={() => setNewSecret(null)}>Done</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function CreateAPIKeyDialog({
  open,
  onOpenChange,
  roles,
  groups,
  onSubmit,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  roles: any[]
  groups: any[]
  onSubmit: (data: any) => void
}) {
  const [name, setName] = useState('')
  const [roleId, setRoleId] = useState('')
  const [groupId, setGroupId] = useState('')
  const [hasExpiry, setHasExpiry] = useState(false)
  const [expiryDate, setExpiryDate] = useState('')

  const handleSubmit = () => {
    const input: any = {
      name,
    }

    // Add role or group (mutually exclusive)
    if (roleId) {
      input.roleId = roleId
    } else if (groupId) {
      input.groupId = groupId
    }

    // Add expiry date if set
    if (hasExpiry && expiryDate) {
      input.expiresAt = new Date(expiryDate).toISOString()
    }

    onSubmit(input)

    // Reset form
    setName('')
    setRoleId('')
    setGroupId('')
    setHasExpiry(false)
    setExpiryDate('')
    onOpenChange(false)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Create API Key</DialogTitle>
          <DialogDescription>Create a new API key for programmatic access</DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-4">
          <div className="space-y-2">
            <label className="text-sm font-medium">Name *</label>
            <Input
              placeholder="e.g., Production App"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">Role</label>
            <Select
              value={roleId}
              onValueChange={(value) => {
                setRoleId(value)
                if (value) setGroupId('') // Clear group if role is selected
              }}
              disabled={!!groupId}
            >
              <SelectTrigger>
                <SelectValue placeholder="Select a role (optional)" />
              </SelectTrigger>
              <SelectContent>
                {roles.map((role: any) => (
                  <SelectItem key={role.id} value={role.id}>
                    {role.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <p className="text-xs text-muted-foreground">
              Assign a role to control what this API key can access.
            </p>
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">Group</label>
            <Select
              value={groupId}
              onValueChange={(value) => {
                setGroupId(value)
                if (value) setRoleId('') // Clear role if group is selected
              }}
              disabled={!!roleId}
            >
              <SelectTrigger>
                <SelectValue placeholder="Select a group (optional)" />
              </SelectTrigger>
              <SelectContent>
                {groups.map((group: any) => (
                  <SelectItem key={group.id} value={group.id}>
                    {group.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <p className="text-xs text-muted-foreground">
              Assign a group to inherit multiple role permissions.
            </p>
          </div>
          <div className="space-y-2">
            <div className="flex items-center gap-2">
              <input
                type="checkbox"
                id="hasExpiry"
                checked={hasExpiry}
                onChange={(e) => setHasExpiry(e.target.checked)}
                className="h-4 w-4 rounded border-gray-300"
              />
              <label htmlFor="hasExpiry" className="text-sm font-medium">
                Set expiry date
              </label>
            </div>
            {hasExpiry && (
              <Input
                type="datetime-local"
                value={expiryDate}
                onChange={(e) => setExpiryDate(e.target.value)}
                placeholder="Select expiry date and time"
              />
            )}
            <p className="text-xs text-muted-foreground">
              By default, keys never expire. Set an expiry date if needed.
            </p>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={handleSubmit} disabled={!name || (!roleId && !groupId)}>
            Create Key
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

