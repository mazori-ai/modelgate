import { useState } from 'react'
import { useQuery, useMutation } from '@apollo/client'
import { Plus, Shield, Edit2, Trash2, Lock, ChevronRight, ChevronDown, ChevronsRight, ChevronsLeft, Database, Route, RefreshCw, DollarSign } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
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
import { GET_ROLES, GET_GROUPS, CREATE_ROLE, UPDATE_ROLE_POLICY, DELETE_ROLE, CREATE_GROUP, DELETE_GROUP, GET_AVAILABLE_MODELS } from '@/graphql/operations'
import { PolicyEditorAdvanced } from '@/components/policies/PolicyEditorAdvanced'

interface Role {
  id: string
  name: string
  description: string
  isDefault: boolean
  isSystem: boolean
  policy?: {
    promptPolicies?: {
      piiPolicy?: {
        enabled: boolean
        scanInputs: boolean
        scanOutputs: boolean
        categories: string[]
        onDetection: string
      }
      contentFiltering?: {
        enabled: boolean
        blockedCategories: string[]
        onDetection: string
      }
      directInjectionDetection?: {
        enabled: boolean
        sensitivity: string
        onDetection: string
      }
      inputBounds?: {
        maxPromptLength: number
        maxMessageCount: number
      }
    }
    toolPolicies?: {
      allowToolCalling: boolean
      allowedTools: string[]
      blockedTools: string[]
      maxToolCallsPerRequest: number
      requireToolApproval: boolean
    }
    rateLimitPolicy?: {
      requestsPerMinute: number
      requestsPerHour: number
      requestsPerDay: number
      tokensPerMinute: number
      tokensPerHour: number
      tokensPerDay: number
    }
    modelRestrictions?: {
      allowedModels: string[]
      allowedProviders: string[]
      defaultModel: string
      maxTokensPerRequest: number
    }
    cachingPolicy?: {
      enabled: boolean
      similarityThreshold: number
      ttlSeconds: number
      maxCacheSize: number
      cacheStreaming: boolean
      cacheToolCalls: boolean // Deprecated: Backend never caches tool calls (time-dependent)
      excludedModels: string[]
      excludedPatterns: string[]
      trackSavings: boolean
    }
    routingPolicy?: {
      enabled: boolean
      strategy: string
      allowModelOverride: boolean
    }
    resiliencePolicy?: {
      enabled: boolean
      retryEnabled: boolean
      maxRetries: number
      retryBackoffMs: number
      retryBackoffMax: number
      retryJitter: boolean
      retryOnTimeout: boolean
      retryOnRateLimit: boolean
      retryOnServerError: boolean
      retryableErrors: string[]
      fallbackEnabled: boolean
      fallbackChain: any[]
      circuitBreakerEnabled: boolean
      circuitBreakerThreshold: number
      circuitBreakerTimeout: number
      requestTimeoutMs: number
    }
    budgetPolicy?: {
      enabled: boolean
      dailyLimitUSD: number
      weeklyLimitUSD: number
      monthlyLimitUSD: number
      maxCostPerRequest: number
      alertThreshold: number
      criticalThreshold: number
      alertWebhook: string
      alertEmails: string[]
      alertSlack: string
      onExceeded: string
      softLimitEnabled: boolean
      softLimitBuffer: number
    }
  }
}

interface Group {
  id: string
  name: string
  description: string
  roles: Array<{ id: string; name: string }>
}

export function RolesPage() {
  const [activeTab, setActiveTab] = useState('roles')
  const [showCreateRole, setShowCreateRole] = useState(false)
  const [showCreateGroup, setShowCreateGroup] = useState(false)
  const [editingRole, setEditingRole] = useState<Role | null>(null)
  
  const { data: rolesData, loading: rolesLoading, refetch: refetchRoles } = useQuery(GET_ROLES)
  const { data: groupsData, loading: groupsLoading, refetch: refetchGroups } = useQuery(GET_GROUPS)
  const { data: modelsData } = useQuery(GET_AVAILABLE_MODELS)

  const [createRole] = useMutation(CREATE_ROLE, {
    onCompleted: () => {
      setShowCreateRole(false)
      refetchRoles()
    },
  })

  const [createGroup] = useMutation(CREATE_GROUP, {
    onCompleted: () => {
      setShowCreateGroup(false)
      refetchGroups()
    },
  })

  const [updateRolePolicy] = useMutation(UPDATE_ROLE_POLICY, {
    onCompleted: () => {
      setEditingRole(null)
      refetchRoles()
    },
  })

  const [deleteRole] = useMutation(DELETE_ROLE, {
    onCompleted: () => refetchRoles(),
  })

  const [deleteGroup] = useMutation(DELETE_GROUP, {
    onCompleted: () => refetchGroups(),
  })

  const roles: Role[] = rolesData?.roles || []
  const groups: Group[] = groupsData?.groups || []
  const availableModels = modelsData?.availableModels || []

  const policyBadges = (role: Role) => {
    const badges = []
    if (role.policy?.promptPolicies) badges.push({ label: 'Prompt', color: 'purple' as const, icon: Shield })
    if (role.policy?.toolPolicies?.allowToolCalling) badges.push({ label: 'Tools', color: 'success' as const })
    if (role.policy?.rateLimitPolicy?.requestsPerMinute) badges.push({ label: 'Rate Limit', color: 'warning' as const })
    if (role.policy?.modelRestrictions?.allowedModels?.length) badges.push({ label: 'Models', color: 'pink' as const })
    // Extended policies
    if (role.policy?.cachingPolicy?.enabled) badges.push({ label: 'Caching', color: 'secondary' as const, icon: Database })
    if (role.policy?.routingPolicy?.enabled) badges.push({ label: 'Routing', color: 'secondary' as const, icon: Route })
    if (role.policy?.resiliencePolicy?.enabled) badges.push({ label: 'Resilience', color: 'secondary' as const, icon: RefreshCw })
    if (role.policy?.budgetPolicy?.enabled) badges.push({ label: 'Budget', color: 'secondary' as const, icon: DollarSign })
    return badges
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">Roles & Policies</h1>
          <p className="text-muted-foreground">
            Manage access control with roles, groups, and policies
          </p>
        </div>
      </div>

      {/* Tabs */}
      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <TabsList className="grid w-full max-w-md grid-cols-2">
          <TabsTrigger value="roles">Roles</TabsTrigger>
          <TabsTrigger value="groups">Groups</TabsTrigger>
        </TabsList>

        {/* Roles Tab */}
        <TabsContent value="roles" className="space-y-4">
          <div className="flex justify-between">
            <p className="text-sm text-muted-foreground">
              Roles define what API keys can do. Each role has policies for prompt safety, tools, rate limiting, and model access.
            </p>
            <Button onClick={() => setShowCreateRole(true)}>
              <Plus className="mr-2 h-4 w-4" />
              New Role
            </Button>
          </div>

          <Card>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Role</TableHead>
                  <TableHead>Description</TableHead>
                  <TableHead>Policies</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {rolesLoading ? (
                  <TableRow>
                    <TableCell colSpan={5} className="text-center py-8 text-muted-foreground">
                      Loading roles...
                    </TableCell>
                  </TableRow>
                ) : roles.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={5} className="text-center py-8 text-muted-foreground">
                      No roles found. Create your first role to get started.
                    </TableCell>
                  </TableRow>
                ) : (
                  roles.map((role) => (
                    <TableRow key={role.id}>
                      <TableCell>
                        <div className="flex items-center gap-2">
                          <Shield className="h-4 w-4 text-primary" />
                          <span className="font-medium">{role.name}</span>
                        </div>
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {role.description || '—'}
                      </TableCell>
                      <TableCell>
                        <div className="flex flex-wrap gap-1">
                          {policyBadges(role).map((badge) => (
                            <Badge key={badge.label} variant={badge.color}>
                              {badge.label}
                            </Badge>
                          ))}
                          {policyBadges(role).length === 0 && (
                            <span className="text-sm text-muted-foreground">No policies</span>
                          )}
                        </div>
                      </TableCell>
                      <TableCell>
                        {role.isDefault && <Badge variant="success">Default</Badge>}
                        {role.isSystem && <Badge variant="secondary">System</Badge>}
                      </TableCell>
                      <TableCell className="text-right">
                        <div className="flex justify-end gap-2">
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => setEditingRole(role)}
                          >
                            <Edit2 className="h-4 w-4" />
                          </Button>
                          {!role.isSystem && (
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => {
                                if (confirm('Delete this role?')) {
                                  deleteRole({ variables: { id: role.id } })
                                }
                              }}
                            >
                              <Trash2 className="h-4 w-4 text-destructive" />
                            </Button>
                          )}
                          {role.isSystem && (
                            <Lock className="h-4 w-4 text-muted-foreground" />
                          )}
                        </div>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </Card>
        </TabsContent>

        {/* Groups Tab */}
        <TabsContent value="groups" className="space-y-4">
          <div className="flex justify-between">
            <p className="text-sm text-muted-foreground">
              Groups combine multiple roles. API keys assigned to a group inherit permissions from all roles.
            </p>
            <Button onClick={() => setShowCreateGroup(true)}>
              <Plus className="mr-2 h-4 w-4" />
              New Group
            </Button>
          </div>

          <Card>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Group</TableHead>
                  <TableHead>Description</TableHead>
                  <TableHead>Roles</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {groupsLoading ? (
                  <TableRow>
                    <TableCell colSpan={4} className="text-center py-8 text-muted-foreground">
                      Loading groups...
                    </TableCell>
                  </TableRow>
                ) : groups.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={4} className="text-center py-8 text-muted-foreground">
                      No groups found. Create a group to combine multiple roles.
                    </TableCell>
                  </TableRow>
                ) : (
                  groups.map((group) => (
                    <TableRow key={group.id}>
                      <TableCell className="font-medium">{group.name}</TableCell>
                      <TableCell className="text-muted-foreground">
                        {group.description || '—'}
                      </TableCell>
                      <TableCell>
                        <div className="flex flex-wrap gap-1">
                          {group.roles.map((role) => (
                            <Badge key={role.id} variant="outline">
                              {role.name}
                            </Badge>
                          ))}
                        </div>
                      </TableCell>
                      <TableCell className="text-right">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => {
                            if (confirm('Delete this group?')) {
                              deleteGroup({ variables: { id: group.id } })
                            }
                          }}
                        >
                          <Trash2 className="h-4 w-4 text-destructive" />
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </Card>
        </TabsContent>
      </Tabs>

      {/* Create Role Dialog */}
      <CreateRoleDialog
        open={showCreateRole}
        onOpenChange={setShowCreateRole}
        onSubmit={(data) => createRole({ variables: { input: data } })}
        availableModels={availableModels}
      />

      {/* Create Group Dialog */}
      <CreateGroupDialog
        open={showCreateGroup}
        onOpenChange={setShowCreateGroup}
        onSubmit={(data) => createGroup({ variables: { input: data } })}
        roles={roles}
      />

      {/* Edit Role Policy Dialog */}
      {editingRole && (
        <EditPolicyDialog
          role={editingRole}
          open={!!editingRole}
          onOpenChange={(open) => !open && setEditingRole(null)}
          onSubmit={(policy) => updateRolePolicy({ 
            variables: { roleId: editingRole.id, input: policy } 
          })}
          availableModels={availableModels}
        />
      )}
    </div>
  )
}

// Create Role Dialog
function CreateRoleDialog({ 
  open, 
  onOpenChange, 
  onSubmit,
  availableModels,
}: { 
  open: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (data: any) => void
  availableModels: any[]
}) {
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [isDefault, setIsDefault] = useState(false)

  const handleSubmit = () => {
    onSubmit({
      name,
      description,
      isDefault,
      policy: {
        promptPolicies: {
          inputBounds: {
            enabled: true,
            maxPromptLength: 100000,
          },
        },
        toolPolicies: {
          allowToolCalling: true,
          allowedTools: [],
          blockedTools: [],
        },
        rateLimitPolicy: {
          requestsPerMinute: 60,
          tokensPerMinute: 100000,
          tokensPerDay: 0,
        },
        modelRestrictions: {
          allowedModels: [],
          allowedProviders: [],
          maxTokensPerRequest: 0,
        },
      },
    })
    setName('')
    setDescription('')
    setIsDefault(false)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Create New Role</DialogTitle>
          <DialogDescription>
            Create a role and configure its policies after creation.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-4">
          <div className="space-y-2">
            <label className="text-sm font-medium">Role Name *</label>
            <Input
              placeholder="e.g., analyst, developer"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">Description</label>
            <Input
              placeholder="Brief description of this role"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </div>
          <div className="flex items-center justify-between">
            <div>
              <label className="text-sm font-medium">Set as Default</label>
              <p className="text-xs text-muted-foreground">
                New API keys will use this role
              </p>
            </div>
            <Switch checked={isDefault} onCheckedChange={setIsDefault} />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={handleSubmit} disabled={!name}>
            Create Role
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// Create Group Dialog
function CreateGroupDialog({
  open,
  onOpenChange,
  onSubmit,
  roles,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (data: any) => void
  roles: Role[]
}) {
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [selectedRoles, setSelectedRoles] = useState<string[]>([])

  const handleSubmit = () => {
    onSubmit({
      name,
      description,
      roleIds: selectedRoles,
    })
    setName('')
    setDescription('')
    setSelectedRoles([])
  }

  const toggleRole = (roleId: string) => {
    setSelectedRoles((prev) =>
      prev.includes(roleId)
        ? prev.filter((id) => id !== roleId)
        : [...prev, roleId]
    )
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Create New Group</DialogTitle>
          <DialogDescription>
            Combine multiple roles into a group for flexible permissions.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-4">
          <div className="space-y-2">
            <label className="text-sm font-medium">Group Name *</label>
            <Input
              placeholder="e.g., engineering-team"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">Description</label>
            <Input
              placeholder="Brief description of this group"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">Select Roles *</label>
            <div className="max-h-48 space-y-2 overflow-y-auto rounded-lg border p-2">
              {roles.map((role) => (
                <label
                  key={role.id}
                  className="flex cursor-pointer items-center gap-3 rounded-md p-2 hover:bg-muted"
                >
                  <input
                    type="checkbox"
                    checked={selectedRoles.includes(role.id)}
                    onChange={() => toggleRole(role.id)}
                    className="rounded"
                  />
                  <div>
                    <div className="font-medium">{role.name}</div>
                    {role.description && (
                      <div className="text-xs text-muted-foreground">
                        {role.description}
                      </div>
                    )}
                  </div>
                </label>
              ))}
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={handleSubmit} disabled={!name || selectedRoles.length === 0}>
            Create Group
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// Edit Policy Dialog - Uses the advanced 8-tab policy editor
function EditPolicyDialog({
  role,
  open,
  onOpenChange,
  onSubmit,
  availableModels,
}: {
  role: Role
  open: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (policy: any) => void
  availableModels: any[]
}) {
  // Convert role policy to the comprehensive format
  const initialPolicy = {
    promptPolicies: {
      structuralSeparation: {
        enabled: false,
        templateFormat: 'XML',
        forbidInstructionsInData: true,
        markRetrievedAsUntrusted: true,
      },
      normalization: {
        enabled: true,
        unicodeNormalization: 'NFKC',
        stripNullBytes: true,
        removeInvisibleChars: true,
        detectMixedEncodings: true,
        rejectSuspiciousEncoding: false,
      },
      inputBounds: {
        enabled: true,
        maxPromptLength: role.policy?.promptPolicies?.inputBounds?.maxPromptLength || 100000,
        maxPromptTokens: 0,
        maxMessageCount: role.policy?.promptPolicies?.inputBounds?.maxMessageCount || 100,
        anomalyThreshold: 0.95,
      },
      directInjectionDetection: {
        enabled: role.policy?.promptPolicies?.directInjectionDetection?.enabled ?? true,
        detectionMethod: 'HYBRID',
        sensitivity: role.policy?.promptPolicies?.directInjectionDetection?.sensitivity || 'MEDIUM',
        onDetection: role.policy?.promptPolicies?.directInjectionDetection?.onDetection || 'BLOCK',
        blockThreshold: 0.85,
        patternDetection: {
          detectIgnoreInstructions: true,
          detectSystemPromptRequests: true,
          detectRoleConfusion: true,
          detectJailbreakPhrases: true,
          detectToolCoercion: true,
          detectEncodingEvasion: true,
          customBlockPatterns: [],
          // Fuzzy matching defaults
          enableFuzzyMatching: true,
          enableWordMatching: true,
          enableNormalization: true,
          fuzzyThreshold: 0.85,
          sensitivity: 'MEDIUM',
          whitelistedPhrases: [],
        },
      },
      piiPolicy: {
        enabled: role.policy?.promptPolicies?.piiPolicy?.enabled ?? false,
        scanInputs: role.policy?.promptPolicies?.piiPolicy?.scanInputs ?? true,
        scanOutputs: role.policy?.promptPolicies?.piiPolicy?.scanOutputs ?? true,
        categories: role.policy?.promptPolicies?.piiPolicy?.categories || ['email', 'phone', 'ssn', 'credit_card'],
        onDetection: role.policy?.promptPolicies?.piiPolicy?.onDetection || 'REDACT',
      },
      contentFiltering: {
        enabled: role.policy?.promptPolicies?.contentFiltering?.enabled ?? false,
        blockedCategories: role.policy?.promptPolicies?.contentFiltering?.blockedCategories || [],
        onDetection: role.policy?.promptPolicies?.contentFiltering?.onDetection || 'BLOCK',
      },
      outputValidation: {
        enabled: true,
        detectCodeExecution: true,
        detectSecretLeakage: true,
        detectPIILeakage: true,
        onViolation: 'REDACT',
      },
    },
    toolPolicies: {
      allowToolCalling: role.policy?.toolPolicies?.allowToolCalling ?? true,
      allowedTools: role.policy?.toolPolicies?.allowedTools || [],
      blockedTools: role.policy?.toolPolicies?.blockedTools || [],
      maxToolCallsPerRequest: role.policy?.toolPolicies?.maxToolCallsPerRequest || 50,
      requireToolApproval: role.policy?.toolPolicies?.requireToolApproval ?? false,
    },
    rateLimitPolicy: {
      requestsPerMinute: role.policy?.rateLimitPolicy?.requestsPerMinute || 60,
      requestsPerHour: 1000,
      requestsPerDay: role.policy?.rateLimitPolicy?.tokensPerDay ? 10000 : 0,
      tokensPerMinute: role.policy?.rateLimitPolicy?.tokensPerMinute || 100000,
      tokensPerHour: 1000000,
      tokensPerDay: role.policy?.rateLimitPolicy?.tokensPerDay || 0,
      costPerDayUSD: 100,
      costPerMonthUSD: 1000,
      burstLimit: 10,
    },
    modelRestrictions: {
      allowedModels: role.policy?.modelRestrictions?.allowedModels || [],
      allowedProviders: role.policy?.modelRestrictions?.allowedProviders || [],
      defaultModel: '',
      maxTokensPerRequest: role.policy?.modelRestrictions?.maxTokensPerRequest || 0,
    },
    cachingPolicy: {
      enabled: role.policy?.cachingPolicy?.enabled ?? false,
      similarityThreshold: role.policy?.cachingPolicy?.similarityThreshold ?? 0.95,
      ttlSeconds: role.policy?.cachingPolicy?.ttlSeconds ?? 3600,
      maxCacheSize: role.policy?.cachingPolicy?.maxCacheSize ?? 1000,
      cacheStreaming: role.policy?.cachingPolicy?.cacheStreaming ?? false,
      cacheToolCalls: role.policy?.cachingPolicy?.cacheToolCalls ?? false,
      excludedModels: role.policy?.cachingPolicy?.excludedModels || [],
      excludedPatterns: role.policy?.cachingPolicy?.excludedPatterns || [],
      trackSavings: role.policy?.cachingPolicy?.trackSavings ?? true,
    },
    routingPolicy: {
      enabled: role.policy?.routingPolicy?.enabled ?? false,
      strategy: role.policy?.routingPolicy?.strategy || 'COST',
      allowModelOverride: role.policy?.routingPolicy?.allowModelOverride ?? true,
    },
    resiliencePolicy: {
      enabled: role.policy?.resiliencePolicy?.enabled ?? false,
      retryEnabled: role.policy?.resiliencePolicy?.retryEnabled ?? true,
      maxRetries: role.policy?.resiliencePolicy?.maxRetries ?? 3,
      retryBackoffMs: role.policy?.resiliencePolicy?.retryBackoffMs ?? 1000,
      retryBackoffMax: role.policy?.resiliencePolicy?.retryBackoffMax ?? 30000,
      retryJitter: role.policy?.resiliencePolicy?.retryJitter ?? true,
      retryOnTimeout: role.policy?.resiliencePolicy?.retryOnTimeout ?? true,
      retryOnRateLimit: role.policy?.resiliencePolicy?.retryOnRateLimit ?? true,
      retryOnServerError: role.policy?.resiliencePolicy?.retryOnServerError ?? true,
      retryableErrors: role.policy?.resiliencePolicy?.retryableErrors || [],
      fallbackEnabled: role.policy?.resiliencePolicy?.fallbackEnabled ?? false,
      fallbackChain: role.policy?.resiliencePolicy?.fallbackChain || [],
      circuitBreakerEnabled: role.policy?.resiliencePolicy?.circuitBreakerEnabled ?? false,
      circuitBreakerThreshold: role.policy?.resiliencePolicy?.circuitBreakerThreshold ?? 5,
      circuitBreakerTimeout: role.policy?.resiliencePolicy?.circuitBreakerTimeout ?? 60,
      requestTimeoutMs: role.policy?.resiliencePolicy?.requestTimeoutMs ?? 30000,
    },
    budgetPolicy: {
      enabled: role.policy?.budgetPolicy?.enabled ?? false,
      dailyLimitUSD: role.policy?.budgetPolicy?.dailyLimitUSD ?? 0,
      weeklyLimitUSD: role.policy?.budgetPolicy?.weeklyLimitUSD ?? 0,
      monthlyLimitUSD: role.policy?.budgetPolicy?.monthlyLimitUSD ?? 0,
      maxCostPerRequest: role.policy?.budgetPolicy?.maxCostPerRequest ?? 0,
      alertThreshold: role.policy?.budgetPolicy?.alertThreshold ?? 0.8,
      criticalThreshold: role.policy?.budgetPolicy?.criticalThreshold ?? 0.95,
      alertWebhook: role.policy?.budgetPolicy?.alertWebhook ?? '',
      alertEmails: role.policy?.budgetPolicy?.alertEmails || [],
      alertSlack: role.policy?.budgetPolicy?.alertSlack ?? '',
      onExceeded: role.policy?.budgetPolicy?.onExceeded || 'WARN',
      softLimitEnabled: role.policy?.budgetPolicy?.softLimitEnabled ?? false,
      softLimitBuffer: role.policy?.budgetPolicy?.softLimitBuffer ?? 0,
    },
    mcpPolicies: {
      enabled: (role.policy as any)?.mcpPolicies?.enabled || false,
      allowToolSearch: (role.policy as any)?.mcpPolicies?.allowToolSearch || false,
      auditToolExecution: (role.policy as any)?.mcpPolicies?.auditToolExecution || false,
    },
  }

  const [currentPolicy, setCurrentPolicy] = useState(initialPolicy)

  const handlePolicyChange = (updatedPolicy: any) => {
    setCurrentPolicy(updatedPolicy)
  }

  const handleSubmit = () => {
    // Use the clean nested structure
    onSubmit({
      promptPolicies: {
        piiPolicy: {
          enabled: currentPolicy.promptPolicies.piiPolicy.enabled,
          scanInputs: currentPolicy.promptPolicies.piiPolicy.scanInputs,
          scanOutputs: currentPolicy.promptPolicies.piiPolicy.scanOutputs,
          categories: currentPolicy.promptPolicies.piiPolicy.categories,
          onDetection: currentPolicy.promptPolicies.piiPolicy.onDetection,
        },
        contentFiltering: {
          enabled: currentPolicy.promptPolicies.contentFiltering.enabled,
          blockedCategories: currentPolicy.promptPolicies.contentFiltering.blockedCategories,
          onDetection: currentPolicy.promptPolicies.contentFiltering.onDetection,
        },
        directInjectionDetection: {
          enabled: currentPolicy.promptPolicies.directInjectionDetection.enabled,
          sensitivity: currentPolicy.promptPolicies.directInjectionDetection.sensitivity,
          onDetection: currentPolicy.promptPolicies.directInjectionDetection.onDetection,
        },
        inputBounds: {
          enabled: currentPolicy.promptPolicies.inputBounds.enabled,
          maxPromptLength: currentPolicy.promptPolicies.inputBounds.maxPromptLength,
          maxMessageCount: currentPolicy.promptPolicies.inputBounds.maxMessageCount,
        },
      },
      toolPolicies: {
        allowToolCalling: currentPolicy.toolPolicies.allowToolCalling,
        allowedTools: currentPolicy.toolPolicies.allowedTools,
        blockedTools: currentPolicy.toolPolicies.blockedTools,
        maxToolCallsPerRequest: currentPolicy.toolPolicies.maxToolCallsPerRequest,
        requireToolApproval: currentPolicy.toolPolicies.requireToolApproval,
      },
      rateLimitPolicy: {
        requestsPerMinute: currentPolicy.rateLimitPolicy.requestsPerMinute,
        requestsPerHour: currentPolicy.rateLimitPolicy.requestsPerHour,
        requestsPerDay: currentPolicy.rateLimitPolicy.requestsPerDay,
        tokensPerMinute: currentPolicy.rateLimitPolicy.tokensPerMinute,
        tokensPerHour: currentPolicy.rateLimitPolicy.tokensPerHour,
        tokensPerDay: currentPolicy.rateLimitPolicy.tokensPerDay,
      },
      modelRestrictions: {
        allowedModels: currentPolicy.modelRestrictions.allowedModels,
        allowedProviders: currentPolicy.modelRestrictions.allowedProviders,
        defaultModel: currentPolicy.modelRestrictions.defaultModel,
        maxTokensPerRequest: currentPolicy.modelRestrictions.maxTokensPerRequest,
      },
      mcpPolicies: {
        enabled: currentPolicy.mcpPolicies.enabled,
        allowToolSearch: currentPolicy.mcpPolicies.allowToolSearch,
        auditToolExecution: currentPolicy.mcpPolicies.auditToolExecution,
      },
      cachingPolicy: {
        enabled: currentPolicy.cachingPolicy.enabled,
        similarityThreshold: currentPolicy.cachingPolicy.similarityThreshold,
        ttlSeconds: currentPolicy.cachingPolicy.ttlSeconds,
        maxCacheSize: currentPolicy.cachingPolicy.maxCacheSize,
        cacheStreaming: currentPolicy.cachingPolicy.cacheStreaming,
        cacheToolCalls: currentPolicy.cachingPolicy.cacheToolCalls,
        excludedModels: currentPolicy.cachingPolicy.excludedModels,
        excludedPatterns: currentPolicy.cachingPolicy.excludedPatterns,
        trackSavings: currentPolicy.cachingPolicy.trackSavings,
      },
      routingPolicy: {
        enabled: currentPolicy.routingPolicy.enabled,
        strategy: currentPolicy.routingPolicy.strategy || null,
        allowModelOverride: currentPolicy.routingPolicy.allowModelOverride,
      },
      resiliencePolicy: {
        enabled: currentPolicy.resiliencePolicy.enabled,
        retryEnabled: currentPolicy.resiliencePolicy.retryEnabled,
        maxRetries: currentPolicy.resiliencePolicy.maxRetries,
        retryBackoffMs: currentPolicy.resiliencePolicy.retryBackoffMs,
        retryBackoffMax: currentPolicy.resiliencePolicy.retryBackoffMax,
        retryJitter: currentPolicy.resiliencePolicy.retryJitter,
        retryOnTimeout: currentPolicy.resiliencePolicy.retryOnTimeout,
        retryOnRateLimit: currentPolicy.resiliencePolicy.retryOnRateLimit,
        retryOnServerError: currentPolicy.resiliencePolicy.retryOnServerError,
        retryableErrors: currentPolicy.resiliencePolicy.retryableErrors,
        fallbackEnabled: currentPolicy.resiliencePolicy.fallbackEnabled,
        fallbackChain: currentPolicy.resiliencePolicy.fallbackChain,
        circuitBreakerEnabled: currentPolicy.resiliencePolicy.circuitBreakerEnabled,
        circuitBreakerThreshold: currentPolicy.resiliencePolicy.circuitBreakerThreshold,
        circuitBreakerTimeout: currentPolicy.resiliencePolicy.circuitBreakerTimeout,
        requestTimeoutMs: currentPolicy.resiliencePolicy.requestTimeoutMs,
      },
      budgetPolicy: {
        enabled: currentPolicy.budgetPolicy.enabled,
        dailyLimitUSD: currentPolicy.budgetPolicy.dailyLimitUSD,
        weeklyLimitUSD: currentPolicy.budgetPolicy.weeklyLimitUSD,
        monthlyLimitUSD: currentPolicy.budgetPolicy.monthlyLimitUSD,
        maxCostPerRequest: currentPolicy.budgetPolicy.maxCostPerRequest,
        alertThreshold: currentPolicy.budgetPolicy.alertThreshold,
        criticalThreshold: currentPolicy.budgetPolicy.criticalThreshold,
        alertWebhook: currentPolicy.budgetPolicy.alertWebhook,
        alertEmails: currentPolicy.budgetPolicy.alertEmails,
        alertSlack: currentPolicy.budgetPolicy.alertSlack,
        onExceeded: currentPolicy.budgetPolicy.onExceeded || null,
        softLimitEnabled: currentPolicy.budgetPolicy.softLimitEnabled,
        softLimitBuffer: currentPolicy.budgetPolicy.softLimitBuffer,
      },
    })
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-6xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Edit Policy - {role.name}</DialogTitle>
          <DialogDescription>
            Configure comprehensive policies for this role including security, rate limits, caching, and more
          </DialogDescription>
        </DialogHeader>

        <div className="mt-4">
          <PolicyEditorAdvanced
            policy={currentPolicy}
            onPolicyChange={handlePolicyChange}
            availableModels={availableModels}
            readOnly={false}
            roleId={role.id}
          />
        </div>

        <DialogFooter className="mt-6">
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={handleSubmit}>Save Policy</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}


// Model Selector Component with Split View
function ModelSelector({
  selectedModels,
  setSelectedModels,
  availableModels,
  maxTokensPerRequest,
  setMaxTokensPerRequest,
}: {
  selectedModels: string[]
  setSelectedModels: (models: string[] | ((prev: string[]) => string[])) => void
  availableModels: any[]
  maxTokensPerRequest: number
  setMaxTokensPerRequest: (tokens: number) => void
}) {
  const [expandedProviders, setExpandedProviders] = useState<Set<string>>(new Set())
  const [searchQuery, setSearchQuery] = useState('')

  // Group models by provider
  const modelsByProvider = availableModels.reduce((acc, model) => {
    if (!acc[model.provider]) {
      acc[model.provider] = []
    }
    acc[model.provider].push(model)
    return acc
  }, {} as Record<string, any[]>)

  const providers = Object.keys(modelsByProvider).sort()

  const toggleProvider = (provider: string) => {
    setExpandedProviders((prev) => {
      const next = new Set(prev)
      if (next.has(provider)) {
        next.delete(provider)
      } else {
        next.add(provider)
      }
      return next
    })
  }

  // Filter models based on search query
  const filterModels = (models: any[]) => {
    if (!searchQuery.trim()) return models
    const query = searchQuery.toLowerCase()
    return models.filter((m) =>
      m.name.toLowerCase().includes(query) ||
      m.id.toLowerCase().includes(query)
    )
  }

  // Get available (unselected) and selected models grouped by provider
  const availableByProvider: Record<string, any[]> = {}
  const selectedByProvider: Record<string, any[]> = {}

  providers.forEach((provider) => {
    const unselected = modelsByProvider[provider].filter(
      (m: any) => !selectedModels.includes(m.id)
    )
    const selected = modelsByProvider[provider].filter((m: any) =>
      selectedModels.includes(m.id)
    )

    availableByProvider[provider] = filterModels(unselected)
    selectedByProvider[provider] = selected
  })

  const moveToSelected = (modelIds: string[]) => {
    setSelectedModels((prev: string[]) => [...prev, ...modelIds])
  }

  const moveToAvailable = (modelIds: string[]) => {
    setSelectedModels((prev: string[]) => prev.filter((id) => !modelIds.includes(id)))
  }

  const moveAllFromProvider = (provider: string) => {
    const modelIds = availableByProvider[provider].map((m) => m.id)
    moveToSelected(modelIds)
  }

  const removeAllFromProvider = (provider: string) => {
    const modelIds = selectedByProvider[provider].map((m) => m.id)
    moveToAvailable(modelIds)
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Model Access</CardTitle>
        <CardDescription>Select which models this role can access</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {availableModels.length === 0 ? (
          <p className="py-8 text-center text-muted-foreground">
            No models configured. Add provider API keys and refresh models first.
          </p>
        ) : (
          <>
            <div className="grid grid-cols-2 gap-4">
              {/* Available Models */}
              <div className="space-y-2">
                <label className="text-sm font-medium">Available Models</label>
                <Input
                  placeholder="Search models..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="mb-2"
                />
                <div className="h-96 overflow-y-auto rounded-lg border p-2">
                  {providers.map((provider) => {
                    const models = availableByProvider[provider]
                    if (models.length === 0) return null

                    return (
                      <div key={provider} className="mb-2">
                        <div
                          className="flex cursor-pointer items-center gap-2 rounded-md p-2 hover:bg-muted"
                          onClick={() => toggleProvider(provider)}
                        >
                          {expandedProviders.has(provider) ? (
                            <ChevronDown className="h-4 w-4" />
                          ) : (
                            <ChevronRight className="h-4 w-4" />
                          )}
                          <span className="font-medium">{provider}</span>
                          <Badge variant="secondary" className="ml-auto">
                            {models.length}
                          </Badge>
                          <Button
                            size="sm"
                            variant="ghost"
                            className="h-6 px-2"
                            onClick={(e) => {
                              e.stopPropagation()
                              moveAllFromProvider(provider)
                            }}
                          >
                            <ChevronsRight className="h-3 w-3" />
                          </Button>
                        </div>
                        {expandedProviders.has(provider) && (
                          <div className="ml-6 space-y-1">
                            {models.map((model) => (
                              <div
                                key={model.id}
                                className="flex cursor-pointer items-center gap-2 rounded-md p-2 text-sm hover:bg-muted"
                                onClick={() => moveToSelected([model.id])}
                              >
                                <ChevronRight className="h-3 w-3" />
                                <span>{model.name}</span>
                              </div>
                            ))}
                          </div>
                        )}
                      </div>
                    )
                  })}
                </div>
              </div>

              {/* Selected Models */}
              <div className="space-y-2">
                <label className="text-sm font-medium">Selected Models</label>
                <div className="h-[430px] overflow-y-auto rounded-lg border p-2">
                  {providers.map((provider) => {
                    const models = selectedByProvider[provider]
                    if (models.length === 0) return null

                    return (
                      <div key={provider} className="mb-2">
                        <div
                          className="flex cursor-pointer items-center gap-2 rounded-md p-2 hover:bg-muted"
                          onClick={() => toggleProvider(provider + '-selected')}
                        >
                          <Button
                            size="sm"
                            variant="ghost"
                            className="h-6 px-2"
                            onClick={(e) => {
                              e.stopPropagation()
                              removeAllFromProvider(provider)
                            }}
                          >
                            <ChevronsLeft className="h-3 w-3" />
                          </Button>
                          {expandedProviders.has(provider + '-selected') ? (
                            <ChevronDown className="h-4 w-4" />
                          ) : (
                            <ChevronRight className="h-4 w-4" />
                          )}
                          <span className="font-medium">{provider}</span>
                          <Badge variant="secondary" className="ml-auto">
                            {models.length}
                          </Badge>
                        </div>
                        {expandedProviders.has(provider + '-selected') && (
                          <div className="ml-6 space-y-1">
                            {models.map((model) => (
                              <div
                                key={model.id}
                                className="flex cursor-pointer items-center gap-2 rounded-md p-2 text-sm hover:bg-muted"
                                onClick={() => moveToAvailable([model.id])}
                              >
                                <span>{model.name}</span>
                                <ChevronRight className="ml-auto h-3 w-3 rotate-180" />
                              </div>
                            ))}
                          </div>
                        )}
                      </div>
                    )
                  })}
                  {selectedModels.length === 0 && (
                    <p className="py-8 text-center text-sm text-muted-foreground">
                      No models selected. Role will not have access to any models.
                    </p>
                  )}
                </div>
              </div>
            </div>
          </>
        )}

        <div className="space-y-2">
          <label className="text-sm font-medium">Max Tokens/Request</label>
          <Input
            type="number"
            value={maxTokensPerRequest}
            onChange={(e) => setMaxTokensPerRequest(parseInt(e.target.value) || 0)}
          />
          <p className="text-xs text-muted-foreground">0 = use model default</p>
        </div>
      </CardContent>
    </Card>
  )
}
