import React, { useState, useEffect } from 'react'
import { useQuery, useMutation } from '@apollo/client'
import {
  Shield,
  Wrench,
  Gauge,
  Box,
  Database,
  Route,
  RefreshCw,
  DollarSign,
  AlertTriangle,
  ChevronRight,
  ChevronDown,
  Plus,
  Trash2,
  Info,
  Zap,
  Lock,
  Eye,
  EyeOff,
  Code,
  ShieldAlert,
  Fingerprint,
  ScanLine,
  FileWarning,
  AlertCircle,
  CheckCircle,
  XCircle,
  Clock,
  Loader2,
  Plug,
} from 'lucide-react'
import {
  GET_ROLE_TOOL_PERMISSIONS,
  SET_TOOL_PERMISSION,
  APPROVE_ALL_PENDING_TOOLS,
  DENY_ALL_PENDING_TOOLS,
  REMOVE_ALL_PENDING_TOOLS,
  DELETE_DISCOVERED_TOOL,
  GET_MCP_SERVERS_WITH_TOOLS,
  SET_MCP_PERMISSION,
  BULK_SET_MCP_VISIBILITY,
} from '@/graphql/operations'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

// Types for all 8 policy sections
interface PromptPolicies {
  structuralSeparation: {
    enabled: boolean
    templateFormat: string
    forbidInstructionsInData: boolean
    markRetrievedAsUntrusted: boolean
  }
  normalization: {
    enabled: boolean
    unicodeNormalization: string
    stripNullBytes: boolean
    removeInvisibleChars: boolean
    detectMixedEncodings: boolean
    rejectSuspiciousEncoding: boolean
  }
  inputBounds: {
    enabled: boolean
    maxPromptLength: number
    maxPromptTokens: number
    maxMessageCount: number
    anomalyThreshold: number
  }
  directInjectionDetection: {
    enabled: boolean
    detectionMethod: string
    sensitivity: string
    onDetection: string
    blockThreshold: number
    patternDetection: {
      detectIgnoreInstructions: boolean
      detectSystemPromptRequests: boolean
      detectRoleConfusion: boolean
      detectJailbreakPhrases: boolean
      detectToolCoercion: boolean
      detectEncodingEvasion: boolean
      customBlockPatterns: string[]
      // Fuzzy matching configuration
      enableFuzzyMatching: boolean
      enableWordMatching: boolean
      enableNormalization: boolean
      fuzzyThreshold: number
      sensitivity: string
      whitelistedPhrases: string[]
    }
  }
  piiPolicy: {
    enabled: boolean
    scanInputs: boolean
    scanOutputs: boolean
    categories: string[]
    onDetection: string
  }
  contentFiltering: {
    enabled: boolean
    blockedCategories: string[]
    onDetection: string
  }
  outputValidation: {
    enabled: boolean
    detectCodeExecution: boolean
    detectSecretLeakage: boolean
    detectPIILeakage: boolean
    onViolation: string
  }
}

interface ToolPolicies {
  allowToolCalling: boolean
  allowedTools: string[]
  blockedTools: string[]
  maxToolCallsPerRequest: number
  requireToolApproval: boolean
}

interface MCPPolicies {
  enabled: boolean
  allowToolSearch: boolean
  auditToolExecution: boolean
}

interface RateLimitPolicy {
  requestsPerMinute: number
  requestsPerHour: number
  requestsPerDay: number
  tokensPerMinute: number
  tokensPerHour: number
  tokensPerDay: number
  costPerDayUSD: number
  costPerMonthUSD: number
  burstLimit: number
}

interface ModelRestrictions {
  allowedModels: string[]
  allowedProviders: string[]
  defaultModel: string
  maxTokensPerRequest: number
}

interface CachingPolicy {
  enabled: boolean
  similarityThreshold: number
  ttlSeconds: number
  maxCacheSize: number
  cacheStreaming: boolean
  cacheToolCalls: boolean // Deprecated: Backend never caches tool calls (time-dependent)
  excludedModels: string[]
  trackSavings: boolean
}

interface RoutingPolicy {
  enabled: boolean
  strategy: string
  allowModelOverride: boolean
  costConfig?: {
    simpleQueryThreshold: number
    complexQueryThreshold: number
    simpleModels: string[]
    complexModels: string[]
  }
  latencyConfig?: {
    maxLatencyMs: number
    preferredModels: string[]
  }
}

interface ResiliencePolicy {
  enabled: boolean
  retryEnabled: boolean
  maxRetries: number
  retryBackoffMs: number
  retryOnTimeout: boolean
  retryOnRateLimit: boolean
  retryOnServerError: boolean
  fallbackEnabled: boolean
  fallbackChain: Array<{
    provider: string
    model: string
    priority: number
  }>
  circuitBreakerEnabled: boolean
  circuitBreakerThreshold: number
  circuitBreakerTimeout: number
  requestTimeoutMs: number
}

interface BudgetPolicy {
  enabled: boolean
  dailyLimitUSD: number
  weeklyLimitUSD: number
  monthlyLimitUSD: number
  maxCostPerRequest: number
  alertThreshold: number
  criticalThreshold: number
  alertWebhook: string
  alertEmails: string[]
  onExceeded: string
}

interface RolePolicy {
  promptPolicies: PromptPolicies
  toolPolicies: ToolPolicies
  mcpPolicies: MCPPolicies
  rateLimitPolicy: RateLimitPolicy
  modelRestrictions: ModelRestrictions
  cachingPolicy: CachingPolicy
  routingPolicy: RoutingPolicy
  resiliencePolicy: ResiliencePolicy
  budgetPolicy: BudgetPolicy
}

interface PolicyEditorAdvancedProps {
  policy: RolePolicy
  onPolicyChange: (policy: RolePolicy) => void
  availableModels: any[]
  readOnly?: boolean
  roleId?: string
}

const defaultPolicy: RolePolicy = {
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
      maxPromptLength: 100000,
      maxPromptTokens: 0,
      maxMessageCount: 100,
      anomalyThreshold: 0.95,
    },
    directInjectionDetection: {
      enabled: true,
      detectionMethod: 'HYBRID',
      sensitivity: 'MEDIUM',
      onDetection: 'BLOCK',
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
      enabled: true,
      scanInputs: true,
      scanOutputs: true,
      categories: ['email', 'phone', 'ssn', 'credit_card'],
      onDetection: 'REDACT',
    },
    contentFiltering: {
      enabled: false,
      blockedCategories: [],
      onDetection: 'BLOCK',
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
    allowToolCalling: true,
    allowedTools: [],
    blockedTools: [],
    maxToolCallsPerRequest: 50,
    requireToolApproval: false,
  },
  mcpPolicies: {
    enabled: false,
    allowToolSearch: true,
    auditToolExecution: true,
  },
  rateLimitPolicy: {
    requestsPerMinute: 60,
    requestsPerHour: 1000,
    requestsPerDay: 10000,
    tokensPerMinute: 100000,
    tokensPerHour: 1000000,
    tokensPerDay: 10000000,
    costPerDayUSD: 100,
    costPerMonthUSD: 1000,
    burstLimit: 10,
  },
  modelRestrictions: {
    allowedModels: [],
    allowedProviders: [],
    defaultModel: '',
    maxTokensPerRequest: 0,
  },
  cachingPolicy: {
    enabled: false,
    similarityThreshold: 0.95,
    ttlSeconds: 3600,
    maxCacheSize: 1000,
    cacheStreaming: false,
    cacheToolCalls: false,
    excludedModels: [],
    trackSavings: true,
  },
  routingPolicy: {
    enabled: false,
    strategy: 'COST',
    allowModelOverride: true,
  },
  resiliencePolicy: {
    enabled: false,
    retryEnabled: true,
    maxRetries: 3,
    retryBackoffMs: 1000,
    retryOnTimeout: true,
    retryOnRateLimit: true,
    retryOnServerError: true,
    fallbackEnabled: false,
    fallbackChain: [],
    circuitBreakerEnabled: false,
    circuitBreakerThreshold: 5,
    circuitBreakerTimeout: 60,
    requestTimeoutMs: 30000,
  },
  budgetPolicy: {
    enabled: false,
    dailyLimitUSD: 0,
    weeklyLimitUSD: 0,
    monthlyLimitUSD: 0,
    maxCostPerRequest: 0,
    alertThreshold: 0.8,
    criticalThreshold: 0.95,
    alertWebhook: '',
    alertEmails: [],
    onExceeded: 'WARN',
  },
}

export function PolicyEditorAdvanced({
  policy: initialPolicy,
  onPolicyChange,
  availableModels,
  readOnly = false,
  roleId,
}: PolicyEditorAdvancedProps) {
  const [policy, setPolicy] = useState<RolePolicy>({
    ...defaultPolicy,
    ...initialPolicy,
  })
  const [activeTab, setActiveTab] = useState('prompt')

  useEffect(() => {
    onPolicyChange(policy)
  }, [policy, onPolicyChange])

  const updatePolicy = <K extends keyof RolePolicy>(
    section: K,
    updates: Partial<RolePolicy[K]>
  ) => {
    setPolicy((prev) => ({
      ...prev,
      [section]: { ...prev[section], ...updates },
    }))
  }

  // Tab configuration with icons and descriptions
  const tabs = [
    { id: 'prompt', label: 'Prompt Security', icon: Shield, color: 'text-purple-500', enterprise: false },
    { id: 'tools', label: 'Tools', icon: Wrench, color: 'text-emerald-500', enterprise: false },
    { id: 'mcp', label: 'MCP', icon: Plug, color: 'text-indigo-500', enterprise: false },
    { id: 'rate', label: 'Rate Limits', icon: Gauge, color: 'text-amber-500', enterprise: false },
    { id: 'models', label: 'Models', icon: Box, color: 'text-blue-500', enterprise: false },
    { id: 'caching', label: 'Caching', icon: Database, color: 'text-cyan-500', enterprise: false },
    { id: 'routing', label: 'Routing', icon: Route, color: 'text-amber-500', enterprise: true },
    { id: 'resilience', label: 'Resilience', icon: RefreshCw, color: 'text-amber-500', enterprise: true },
    { id: 'budget', label: 'Budget', icon: DollarSign, color: 'text-green-500', enterprise: false },
  ]

  return (
    <div className="space-y-6">
      {/* Policy Overview Cards */}
      <div className="grid grid-cols-4 gap-4">
        <PolicyStatusCard
          icon={Shield}
          title="Security"
          enabled={policy.promptPolicies.directInjectionDetection.enabled || policy.promptPolicies.piiPolicy.enabled}
          features={[
            policy.promptPolicies.directInjectionDetection.enabled && 'Injection Detection',
            policy.promptPolicies.piiPolicy.enabled && 'PII Protection',
            policy.promptPolicies.contentFiltering.enabled && 'Content Filtering',
            policy.promptPolicies.outputValidation.enabled && 'Output Validation',
          ].filter(Boolean) as string[]}
          color="purple"
        />
        <PolicyStatusCard
          icon={Gauge}
          title="Rate Limits"
          enabled={true}
          features={[
            `${policy.rateLimitPolicy.requestsPerMinute} req/min`,
            `${(policy.rateLimitPolicy.tokensPerMinute / 1000).toFixed(0)}k tok/min`,
          ]}
          color="amber"
        />
        <PolicyStatusCard
          icon={Database}
          title="Caching"
          enabled={policy.cachingPolicy.enabled}
          features={
            policy.cachingPolicy.enabled
              ? [`${(policy.cachingPolicy.similarityThreshold * 100).toFixed(0)}% threshold`]
              : ['Disabled']
          }
          color="cyan"
        />
        <PolicyStatusCard
          icon={RefreshCw}
          title="Resilience"
          enabled={false}
          features={['Enterprise Feature']}
          color="amber"
          isEnterprise={true}
        />
      </div>

      {/* Main Policy Editor Tabs */}
      <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
        <TabsList className="grid w-full grid-cols-9 h-12">
          {tabs.map((tab) => (
            <TabsTrigger
              key={tab.id}
              value={tab.id}
              className={`flex items-center gap-1.5 text-xs ${tab.enterprise ? 'relative' : ''}`}
            >
              <tab.icon className={`h-4 w-4 ${tab.color}`} />
              <span className="hidden xl:inline">{tab.label}</span>
              {tab.enterprise && (
                <span className="hidden xl:inline text-[10px] text-amber-600">★</span>
              )}
            </TabsTrigger>
          ))}
        </TabsList>

        {/* PROMPT SECURITY TAB */}
        <TabsContent value="prompt" className="mt-6">
          <PromptSecurityEditor
            promptPolicies={policy.promptPolicies}
            onChange={(updates) => updatePolicy('promptPolicies', updates)}
            readOnly={readOnly}
          />
        </TabsContent>

        {/* TOOLS TAB */}
        <TabsContent value="tools" className="mt-6">
          <ToolPoliciesEditor
            toolPolicies={policy.toolPolicies}
            onChange={(updates) => updatePolicy('toolPolicies', updates)}
            readOnly={readOnly}
            roleId={roleId}
          />
        </TabsContent>

        {/* MCP GATEWAY TAB */}
        <TabsContent value="mcp" className="mt-6">
          <MCPPolicyEditor
            mcpPolicies={policy.mcpPolicies}
            onChange={(updates) => updatePolicy('mcpPolicies', updates)}
            readOnly={readOnly}
            roleId={roleId}
          />
        </TabsContent>

        {/* RATE LIMITS TAB */}
        <TabsContent value="rate" className="mt-6">
          <RateLimitEditor
            rateLimitPolicy={policy.rateLimitPolicy}
            onChange={(updates) => updatePolicy('rateLimitPolicy', updates)}
            readOnly={readOnly}
          />
        </TabsContent>

        {/* MODELS TAB */}
        <TabsContent value="models" className="mt-6">
          <ModelRestrictionsEditor
            modelRestrictions={policy.modelRestrictions}
            onChange={(updates) => updatePolicy('modelRestrictions', updates)}
            availableModels={availableModels}
            readOnly={readOnly}
          />
        </TabsContent>

        {/* CACHING TAB */}
        <TabsContent value="caching" className="mt-6">
          <CachingPolicyEditor
            cachingPolicy={policy.cachingPolicy}
            onChange={(updates) => updatePolicy('cachingPolicy', updates)}
            readOnly={readOnly}
          />
        </TabsContent>

        {/* ROUTING TAB */}
        <TabsContent value="routing" className="mt-6">
          <RoutingPolicyEditor
            routingPolicy={policy.routingPolicy}
            onChange={(updates) => updatePolicy('routingPolicy', updates)}
            availableModels={availableModels}
            readOnly={readOnly}
          />
        </TabsContent>

        {/* RESILIENCE TAB */}
        <TabsContent value="resilience" className="mt-6">
          <ResiliencePolicyEditor
            resiliencePolicy={policy.resiliencePolicy}
            onChange={(updates) => updatePolicy('resiliencePolicy', updates)}
            availableModels={availableModels}
            readOnly={readOnly}
          />
        </TabsContent>

        {/* BUDGET TAB */}
        <TabsContent value="budget" className="mt-6">
          <BudgetPolicyEditor
            budgetPolicy={policy.budgetPolicy}
            onChange={(updates) => updatePolicy('budgetPolicy', updates)}
            readOnly={readOnly}
          />
        </TabsContent>
      </Tabs>
    </div>
  )
}

// =============================================================================
// STATUS CARD
// =============================================================================

function PolicyStatusCard({
  icon: Icon,
  title,
  enabled,
  features,
  color,
  isEnterprise = false,
}: {
  icon: any
  title: string
  enabled: boolean
  features: string[]
  color: string
  isEnterprise?: boolean
}) {
  const colorClasses: Record<string, string> = {
    purple: 'border-purple-500/20 bg-purple-500/5',
    amber: 'border-amber-500/20 bg-amber-500/5',
    cyan: 'border-cyan-500/20 bg-cyan-500/5',
    orange: 'border-orange-500/20 bg-orange-500/5',
  }

  return (
    <Card className={`${colorClasses[color]} ${!enabled && !isEnterprise && 'opacity-50'}`}>
      <CardContent className="p-4">
        <div className="flex items-center gap-2 mb-2">
          <Icon className={`h-5 w-5 text-${color}-500`} />
          <span className="font-medium">{title}</span>
          {isEnterprise ? (
            <Badge className="ml-auto text-xs bg-amber-100 text-amber-700 border-amber-200">
              Enterprise
            </Badge>
          ) : enabled ? (
            <Badge variant="success" className="ml-auto text-xs">
              On
            </Badge>
          ) : (
            <Badge variant="secondary" className="ml-auto text-xs">
              Off
            </Badge>
          )}
        </div>
        <div className="space-y-1">
          {features.map((f, i) => (
            <p key={i} className="text-xs text-muted-foreground">
              • {f}
            </p>
          ))}
        </div>
      </CardContent>
    </Card>
  )
}

// =============================================================================
// PROMPT SECURITY EDITOR
// =============================================================================

function PromptSecurityEditor({
  promptPolicies,
  onChange,
  readOnly,
}: {
  promptPolicies: PromptPolicies
  onChange: (updates: Partial<PromptPolicies>) => void
  readOnly: boolean
}) {
  const [expandedSections, setExpandedSections] = useState<Set<string>>(new Set(['injection', 'pii']))

  const toggleSection = (section: string) => {
    setExpandedSections((prev) => {
      const next = new Set(prev)
      if (next.has(section)) next.delete(section)
      else next.add(section)
      return next
    })
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2 mb-4">
        <ShieldAlert className="h-5 w-5 text-purple-500" />
        <h3 className="text-lg font-semibold">Prompt Security (OWASP-Aligned)</h3>
        <Badge variant="outline" className="ml-2">
          Defense-in-Depth
        </Badge>
      </div>

      {/* Injection Detection Section */}
      <CollapsibleSection
        title="Injection Detection"
        icon={ScanLine}
        expanded={expandedSections.has('injection')}
        onToggle={() => toggleSection('injection')}
        enabled={promptPolicies.directInjectionDetection.enabled}
        onEnabledChange={(enabled) =>
          onChange({
            directInjectionDetection: {
              ...promptPolicies.directInjectionDetection,
              enabled,
            },
          })
        }
        readOnly={readOnly}
      >
        <div className="grid grid-cols-2 gap-6">
          <div className="space-y-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">Detection Method</label>
              <Select
                value={promptPolicies.directInjectionDetection.detectionMethod}
                onValueChange={(v) =>
                  onChange({
                    directInjectionDetection: {
                      ...promptPolicies.directInjectionDetection,
                      detectionMethod: v,
                    },
                  })
                }
                disabled={readOnly}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="RULES">Rules Only</SelectItem>
                  <SelectItem value="ML">ML Only</SelectItem>
                  <SelectItem value="HYBRID">Hybrid (Recommended)</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium">Sensitivity</label>
              <Select
                value={promptPolicies.directInjectionDetection.sensitivity}
                onValueChange={(v) =>
                  onChange({
                    directInjectionDetection: {
                      ...promptPolicies.directInjectionDetection,
                      sensitivity: v,
                    },
                  })
                }
                disabled={readOnly}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="LOW">Low (Fewer false positives)</SelectItem>
                  <SelectItem value="MEDIUM">Medium (Balanced)</SelectItem>
                  <SelectItem value="HIGH">High (More catches)</SelectItem>
                  <SelectItem value="PARANOID">Paranoid (Maximum security)</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium">On Detection</label>
              <Select
                value={promptPolicies.directInjectionDetection.onDetection}
                onValueChange={(v) =>
                  onChange({
                    directInjectionDetection: {
                      ...promptPolicies.directInjectionDetection,
                      onDetection: v,
                    },
                  })
                }
                disabled={readOnly}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="BLOCK">Block Request</SelectItem>
                  <SelectItem value="WARN">Allow but Warn</SelectItem>
                  <SelectItem value="LOG">Silent Log</SelectItem>
                  <SelectItem value="QUARANTINE">Quarantine for Review</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium">
                Block Threshold ({(promptPolicies.directInjectionDetection.blockThreshold * 100).toFixed(0)}%)
              </label>
              <input
                type="range"
                min="0.5"
                max="1"
                step="0.05"
                value={promptPolicies.directInjectionDetection.blockThreshold}
                onChange={(e) =>
                  onChange({
                    directInjectionDetection: {
                      ...promptPolicies.directInjectionDetection,
                      blockThreshold: parseFloat(e.target.value),
                    },
                  })
                }
                className="w-full"
                disabled={readOnly}
              />
            </div>
          </div>

          <div className="space-y-3">
            <label className="text-sm font-medium">Pattern Detection</label>
            <div className="space-y-2 rounded-lg border p-3">
              {[
                { key: 'detectIgnoreInstructions', label: 'Ignore Instructions ("ignore previous...")' },
                { key: 'detectSystemPromptRequests', label: 'System Prompt Extraction' },
                { key: 'detectRoleConfusion', label: 'Role Confusion ("you are now...")' },
                { key: 'detectJailbreakPhrases', label: 'Jailbreak Phrases ("DAN mode")' },
                { key: 'detectToolCoercion', label: 'Tool Coercion ("call the admin API")' },
                { key: 'detectEncodingEvasion', label: 'Encoding Evasion (base64, hex, etc.)' },
              ].map((item) => (
                <div key={item.key} className="flex items-center justify-between">
                  <span className="text-sm">{item.label}</span>
                  <Switch
                    checked={
                      promptPolicies.directInjectionDetection.patternDetection[
                        item.key as keyof typeof promptPolicies.directInjectionDetection.patternDetection
                      ] as boolean
                    }
                    onCheckedChange={(checked) =>
                      onChange({
                        directInjectionDetection: {
                          ...promptPolicies.directInjectionDetection,
                          patternDetection: {
                            ...promptPolicies.directInjectionDetection.patternDetection,
                            [item.key]: checked,
                          },
                        },
                      })
                    }
                    disabled={readOnly}
                  />
                </div>
              ))}
            </div>
          </div>

          {/* Fuzzy Matching Configuration */}
          <div className="space-y-3">
            <label className="text-sm font-medium">Fuzzy Matching (Evasion Detection)</label>
            <div className="space-y-3 rounded-lg border p-3 bg-blue-50/30 dark:bg-blue-950/20">
              <p className="text-xs text-muted-foreground">
                Fuzzy matching detects typos, homoglyphs (Cyrillic characters), l33t speak, and word reordering attacks.
              </p>
              {[
                { key: 'enableFuzzyMatching', label: 'Enable Fuzzy Matching (Levenshtein distance)', description: 'Catches typos like "ignor previos instructions"' },
                { key: 'enableNormalization', label: 'Enable Text Normalization', description: 'Catches homoglyphs (Cyrillic) and l33t speak (ign0re)' },
                { key: 'enableWordMatching', label: 'Enable Word-Level Matching', description: 'Catches word reordering attacks' },
              ].map((item) => (
                <div key={item.key} className="flex items-center justify-between">
                  <div>
                    <span className="text-sm">{item.label}</span>
                    <p className="text-xs text-muted-foreground">{item.description}</p>
                  </div>
                  <Switch
                    checked={
                      promptPolicies.directInjectionDetection.patternDetection[
                        item.key as keyof typeof promptPolicies.directInjectionDetection.patternDetection
                      ] as boolean
                    }
                    onCheckedChange={(checked) =>
                      onChange({
                        directInjectionDetection: {
                          ...promptPolicies.directInjectionDetection,
                          patternDetection: {
                            ...promptPolicies.directInjectionDetection.patternDetection,
                            [item.key]: checked,
                          },
                        },
                      })
                    }
                    disabled={readOnly}
                  />
                </div>
              ))}
              
              <div className="grid grid-cols-2 gap-4 pt-2">
                <div className="space-y-2">
                  <label className="text-sm font-medium">Fuzzy Threshold</label>
                  <div className="flex items-center gap-2">
                    <Input
                      type="number"
                      min={0.5}
                      max={1.0}
                      step={0.05}
                      value={promptPolicies.directInjectionDetection.patternDetection.fuzzyThreshold}
                      onChange={(e) =>
                        onChange({
                          directInjectionDetection: {
                            ...promptPolicies.directInjectionDetection,
                            patternDetection: {
                              ...promptPolicies.directInjectionDetection.patternDetection,
                              fuzzyThreshold: parseFloat(e.target.value) || 0.85,
                            },
                          },
                        })
                      }
                      disabled={readOnly}
                      className="w-20"
                    />
                    <span className="text-xs text-muted-foreground">
                      {promptPolicies.directInjectionDetection.patternDetection.fuzzyThreshold >= 0.9 ? 'Strict' : 
                       promptPolicies.directInjectionDetection.patternDetection.fuzzyThreshold >= 0.8 ? 'Balanced' : 'Permissive'}
                    </span>
                  </div>
                  <p className="text-xs text-muted-foreground">0.90 = strict, 0.85 = balanced, 0.75 = permissive</p>
                </div>
                
                <div className="space-y-2">
                  <label className="text-sm font-medium">Sensitivity</label>
                  <Select
                    value={promptPolicies.directInjectionDetection.patternDetection.sensitivity}
                    onValueChange={(value) =>
                      onChange({
                        directInjectionDetection: {
                          ...promptPolicies.directInjectionDetection,
                          patternDetection: {
                            ...promptPolicies.directInjectionDetection.patternDetection,
                            sensitivity: value,
                          },
                        },
                      })
                    }
                    disabled={readOnly}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="LOW">Low (fewer false positives)</SelectItem>
                      <SelectItem value="MEDIUM">Medium (balanced)</SelectItem>
                      <SelectItem value="HIGH">High (stricter)</SelectItem>
                      <SelectItem value="PARANOID">Paranoid (maximum protection)</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>
            </div>
          </div>
        </div>
      </CollapsibleSection>

      {/* PII Policy Section */}
      <CollapsibleSection
        title="PII Protection"
        icon={Fingerprint}
        expanded={expandedSections.has('pii')}
        onToggle={() => toggleSection('pii')}
        enabled={promptPolicies.piiPolicy.enabled}
        onEnabledChange={(enabled) =>
          onChange({
            piiPolicy: { ...promptPolicies.piiPolicy, enabled },
          })
        }
        readOnly={readOnly}
      >
        <div className="grid grid-cols-2 gap-6">
          <div className="space-y-4">
            <div className="space-y-2">
              {[
                { key: 'scanInputs', label: 'Scan Inputs' },
                { key: 'scanOutputs', label: 'Scan Outputs' },
              ].map((item) => (
                <div key={item.key} className="flex items-center justify-between">
                  <span className="text-sm">{item.label}</span>
                  <Switch
                    checked={promptPolicies.piiPolicy[item.key as keyof typeof promptPolicies.piiPolicy] as boolean}
                    onCheckedChange={(checked) =>
                      onChange({
                        piiPolicy: {
                          ...promptPolicies.piiPolicy,
                          [item.key]: checked,
                        },
                      })
                    }
                    disabled={readOnly || !promptPolicies.piiPolicy.enabled}
                  />
                </div>
              ))}
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium">On Detection</label>
              <Select
                value={promptPolicies.piiPolicy.onDetection}
                onValueChange={(v) =>
                  onChange({
                    piiPolicy: { ...promptPolicies.piiPolicy, onDetection: v },
                  })
                }
                disabled={readOnly || !promptPolicies.piiPolicy.enabled}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="BLOCK">Block Request</SelectItem>
                  <SelectItem value="REDACT">Redact PII</SelectItem>
                  <SelectItem value="REWRITE">Rewrite PII</SelectItem>
                  <SelectItem value="WARN">Allow but Warn</SelectItem>
                  <SelectItem value="LOG">Silent Log</SelectItem>
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground mt-1">
                {promptPolicies.piiPolicy.onDetection === 'REWRITE' && 
                  "Transforms PII using deterministic rotation (emails become pseudonyms, phones become repeating digits)"}
                {promptPolicies.piiPolicy.onDetection === 'REDACT' && 
                  "Replaces PII with placeholders like [EMAIL REDACTED]"}
                {promptPolicies.piiPolicy.onDetection === 'BLOCK' && 
                  "Blocks the entire request when PII is detected"}
              </p>
            </div>
          </div>

          <div className="space-y-2">
            <label className="text-sm font-medium">PII Categories to Detect</label>
            <div className="grid grid-cols-2 gap-2 rounded-lg border p-3">
              {['email', 'phone', 'ssn', 'credit_card', 'address', 'name', 'dob', 'ip_address'].map(
                (cat) => (
                  <label key={cat} className="flex items-center gap-2 text-sm">
                    <input
                      type="checkbox"
                      checked={promptPolicies.piiPolicy.categories.includes(cat)}
                      onChange={(e) => {
                        const categories = e.target.checked
                          ? [...promptPolicies.piiPolicy.categories, cat]
                          : promptPolicies.piiPolicy.categories.filter((c) => c !== cat)
                        onChange({
                          piiPolicy: { ...promptPolicies.piiPolicy, categories },
                        })
                      }}
                      disabled={readOnly || !promptPolicies.piiPolicy.enabled}
                      className="rounded"
                    />
                    {cat.replace('_', ' ')}
                  </label>
                )
              )}
            </div>
          </div>
        </div>
      </CollapsibleSection>

      {/* Output Validation Section */}
      <CollapsibleSection
        title="Output Validation"
        icon={FileWarning}
        expanded={expandedSections.has('output')}
        onToggle={() => toggleSection('output')}
        enabled={promptPolicies.outputValidation.enabled}
        onEnabledChange={(enabled) =>
          onChange({
            outputValidation: { ...promptPolicies.outputValidation, enabled },
          })
        }
        readOnly={readOnly}
      >
        <div className="grid grid-cols-2 gap-6">
          <div className="space-y-3">
            <label className="text-sm font-medium">Dangerous Content Detection</label>
            <div className="space-y-2">
              {[
                { key: 'detectCodeExecution', label: 'Executable Code' },
                { key: 'detectSecretLeakage', label: 'Secret/API Key Leakage' },
                { key: 'detectPIILeakage', label: 'PII Leakage' },
              ].map((item) => (
                <div key={item.key} className="flex items-center justify-between">
                  <span className="text-sm">{item.label}</span>
                  <Switch
                    checked={
                      promptPolicies.outputValidation[
                        item.key as keyof typeof promptPolicies.outputValidation
                      ] as boolean
                    }
                    onCheckedChange={(checked) =>
                      onChange({
                        outputValidation: {
                          ...promptPolicies.outputValidation,
                          [item.key]: checked,
                        },
                      })
                    }
                    disabled={readOnly || !promptPolicies.outputValidation.enabled}
                  />
                </div>
              ))}
            </div>
          </div>

          <div className="space-y-2">
            <label className="text-sm font-medium">On Violation</label>
            <Select
              value={promptPolicies.outputValidation.onViolation}
              onValueChange={(v) =>
                onChange({
                  outputValidation: { ...promptPolicies.outputValidation, onViolation: v },
                })
              }
              disabled={readOnly || !promptPolicies.outputValidation.enabled}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="BLOCK">Block Response</SelectItem>
                <SelectItem value="REDACT">Redact Content</SelectItem>
                <SelectItem value="WARN">Return with Warning</SelectItem>
                <SelectItem value="LOG">Silent Log</SelectItem>
                <SelectItem value="REGENERATE">Ask Model to Regenerate</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
      </CollapsibleSection>

      {/* Input Bounds Section */}
      <CollapsibleSection
        title="Input Bounds"
        icon={Gauge}
        expanded={expandedSections.has('bounds')}
        onToggle={() => toggleSection('bounds')}
        enabled={promptPolicies.inputBounds.enabled}
        onEnabledChange={(enabled) =>
          onChange({
            inputBounds: { ...promptPolicies.inputBounds, enabled },
          })
        }
        readOnly={readOnly}
      >
        <div className="grid grid-cols-3 gap-4">
          <div className="space-y-2">
            <label className="text-sm font-medium">Max Prompt Length</label>
            <Input
              type="number"
              value={promptPolicies.inputBounds.maxPromptLength}
              onChange={(e) =>
                onChange({
                  inputBounds: {
                    ...promptPolicies.inputBounds,
                    maxPromptLength: parseInt(e.target.value) || 0,
                  },
                })
              }
              disabled={readOnly || !promptPolicies.inputBounds.enabled}
            />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">Max Message Count</label>
            <Input
              type="number"
              value={promptPolicies.inputBounds.maxMessageCount}
              onChange={(e) =>
                onChange({
                  inputBounds: {
                    ...promptPolicies.inputBounds,
                    maxMessageCount: parseInt(e.target.value) || 0,
                  },
                })
              }
              disabled={readOnly || !promptPolicies.inputBounds.enabled}
            />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">Max Tokens</label>
            <Input
              type="number"
              value={promptPolicies.inputBounds.maxPromptTokens}
              onChange={(e) =>
                onChange({
                  inputBounds: {
                    ...promptPolicies.inputBounds,
                    maxPromptTokens: parseInt(e.target.value) || 0,
                  },
                })
              }
              disabled={readOnly || !promptPolicies.inputBounds.enabled}
            />
            <p className="text-xs text-muted-foreground">0 = unlimited</p>
          </div>
        </div>
      </CollapsibleSection>

      {/* Normalization Section */}
      <CollapsibleSection
        title="Input Normalization"
        icon={Code}
        expanded={expandedSections.has('normalization')}
        onToggle={() => toggleSection('normalization')}
        enabled={promptPolicies.normalization.enabled}
        onEnabledChange={(enabled) =>
          onChange({
            normalization: { ...promptPolicies.normalization, enabled },
          })
        }
        readOnly={readOnly}
      >
        <div className="grid grid-cols-2 gap-4">
          {[
            { key: 'stripNullBytes', label: 'Strip Null Bytes' },
            { key: 'removeInvisibleChars', label: 'Remove Invisible Characters' },
            { key: 'detectMixedEncodings', label: 'Detect Mixed Encodings' },
            { key: 'rejectSuspiciousEncoding', label: 'Reject Suspicious Encoding' },
          ].map((item) => (
            <div key={item.key} className="flex items-center justify-between">
              <span className="text-sm">{item.label}</span>
              <Switch
                checked={
                  promptPolicies.normalization[item.key as keyof typeof promptPolicies.normalization] as boolean
                }
                onCheckedChange={(checked) =>
                  onChange({
                    normalization: {
                      ...promptPolicies.normalization,
                      [item.key]: checked,
                    },
                  })
                }
                disabled={readOnly || !promptPolicies.normalization.enabled}
              />
            </div>
          ))}
        </div>
      </CollapsibleSection>
    </div>
  )
}

// =============================================================================
// TOOL POLICIES EDITOR
// =============================================================================

interface ToolWithPermission {
  tool: {
    id: string
    name: string
    description: string
    category: string | null
    seenCount: number
    lastSeenAt: string
    parameters: Record<string, any>
  }
  status: 'PENDING' | 'ALLOWED' | 'DENIED' | 'REMOVED'
  decidedBy: string | null
  decidedByEmail: string | null
  decidedAt: string | null
  decisionReason: string | null
}

function ToolPoliciesEditor({
  toolPolicies,
  onChange,
  readOnly,
  roleId,
}: {
  toolPolicies: ToolPolicies
  onChange: (updates: Partial<ToolPolicies>) => void
  readOnly: boolean
  roleId?: string
}) {
  const [searchTerm, setSearchTerm] = React.useState('')
  const [statusFilter, setStatusFilter] = React.useState<string>('all')
  const [selectedTool, setSelectedTool] = React.useState<ToolWithPermission | null>(null)

  // Fetch tool permissions for this role
  const { data, loading, refetch } = useQuery(GET_ROLE_TOOL_PERMISSIONS, {
    variables: { roleId: roleId || '' },
    skip: !roleId,
    fetchPolicy: 'network-only',
  })

  // Mutations
  const [setPermission] = useMutation(SET_TOOL_PERMISSION, {
    onCompleted: () => refetch(),
  })
  const [approveAll] = useMutation(APPROVE_ALL_PENDING_TOOLS, {
    onCompleted: () => refetch(),
  })
  const [denyAll] = useMutation(DENY_ALL_PENDING_TOOLS, {
    onCompleted: () => refetch(),
  })
  const [removeAll] = useMutation(REMOVE_ALL_PENDING_TOOLS, {
    onCompleted: () => refetch(),
  })
  const [deleteTool] = useMutation(DELETE_DISCOVERED_TOOL, {
    onCompleted: () => refetch(),
  })

  const tools: ToolWithPermission[] = data?.roleToolPermissions || []

  // Filter tools
  const filteredTools = tools.filter((tp) => {
    if (searchTerm && !tp.tool.name.toLowerCase().includes(searchTerm.toLowerCase()) &&
        !tp.tool.description.toLowerCase().includes(searchTerm.toLowerCase())) {
      return false
    }
    if (statusFilter !== 'all' && tp.status !== statusFilter) {
      return false
    }
    return true
  })

  // Group by category
  const groupedTools = filteredTools.reduce((acc, tp) => {
    const category = tp.tool.category || 'general'
    if (!acc[category]) acc[category] = []
    acc[category].push(tp)
    return acc
  }, {} as Record<string, ToolWithPermission[]>)

  const pendingCount = tools.filter(t => t.status === 'PENDING').length
  const allowedCount = tools.filter(t => t.status === 'ALLOWED').length
  const deniedCount = tools.filter(t => t.status === 'DENIED').length
  const removedCount = tools.filter(t => t.status === 'REMOVED').length

  const handleSetPermission = async (toolId: string, status: 'ALLOWED' | 'DENIED' | 'REMOVED', reason?: string) => {
    if (!roleId) return
    await setPermission({
      variables: {
        input: {
          toolId,
          roleId,
          status,
          reason,
        },
      },
    })
  }

  const statusIcon = (status: string) => {
    switch (status) {
      case 'ALLOWED':
        return <CheckCircle className="h-4 w-4 text-green-500" />
      case 'DENIED':
        return <XCircle className="h-4 w-4 text-red-500" />
      case 'REMOVED':
        return <EyeOff className="h-4 w-4 text-gray-500" />
      default:
        return <Clock className="h-4 w-4 text-amber-500" />
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <Wrench className="h-5 w-5 text-emerald-500" />
          <h3 className="text-lg font-semibold">Tool/Function Calling Policies</h3>
        </div>
      </div>

      {/* Global Settings Card */}
      <Card>
        <CardContent className="p-6 space-y-6">
          <div className="flex items-center justify-between">
            <div>
              <label className="text-sm font-medium">Allow Tool Calling</label>
              <p className="text-xs text-muted-foreground">Enable tool usage (default deny for new tools)</p>
            </div>
            <Switch
              checked={toolPolicies.allowToolCalling}
              onCheckedChange={(checked) => onChange({ allowToolCalling: checked })}
              disabled={readOnly}
            />
          </div>

          <div className="grid grid-cols-2 gap-6">
            <div className="space-y-2">
              <label className="text-sm font-medium">Max Tool Calls per Request</label>
              <Input
                type="number"
                value={toolPolicies.maxToolCallsPerRequest}
                onChange={(e) =>
                  onChange({ maxToolCallsPerRequest: parseInt(e.target.value) || 0 })
                }
                disabled={readOnly || !toolPolicies.allowToolCalling}
              />
            </div>
            <div className="flex items-center justify-between">
              <div>
                <label className="text-sm font-medium">Require Approval</label>
                <p className="text-xs text-muted-foreground">Admin approval for new tools</p>
              </div>
              <Switch
                checked={toolPolicies.requireToolApproval}
                onCheckedChange={(checked) => onChange({ requireToolApproval: checked })}
                disabled={readOnly || !toolPolicies.allowToolCalling}
              />
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Discovered Tools Section */}
      {roleId && (
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <div>
                <CardTitle className="text-base">Discovered Tools</CardTitle>
                <p className="text-sm text-muted-foreground">
                  Tools discovered from requests. Set permissions per tool.
                </p>
              </div>
              <div className="flex items-center gap-2">
                <Badge variant="outline" className="gap-1">
                  <Clock className="h-3 w-3 text-amber-500" />
                  {pendingCount} pending
                </Badge>
                <Badge variant="outline" className="gap-1">
                  <CheckCircle className="h-3 w-3 text-green-500" />
                  {allowedCount} allowed
                </Badge>
                <Badge variant="outline" className="gap-1">
                  <XCircle className="h-3 w-3 text-red-500" />
                  {deniedCount} denied
                </Badge>
                <Badge variant="outline" className="gap-1">
                  <EyeOff className="h-3 w-3 text-gray-500" />
                  {removedCount} removed
                </Badge>
              </div>
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            {/* Filters and Bulk Actions */}
            <div className="flex items-center gap-4">
              <Input
                placeholder="Search tools..."
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                className="max-w-xs"
              />
              <Select value={statusFilter} onValueChange={setStatusFilter}>
                <SelectTrigger className="w-36">
                  <SelectValue placeholder="Filter status" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All Statuses</SelectItem>
                  <SelectItem value="PENDING">Pending</SelectItem>
                  <SelectItem value="ALLOWED">Allowed</SelectItem>
                  <SelectItem value="DENIED">Denied</SelectItem>
                  <SelectItem value="REMOVED">Removed</SelectItem>
                </SelectContent>
              </Select>
              {!readOnly && pendingCount > 0 && (
                <>
                  <Button
                    variant="outline"
                    size="sm"
                    className="ml-auto text-green-600"
                    onClick={() => approveAll({ variables: { roleId } })}
                    title="Allow all pending tools to be used"
                  >
                    <CheckCircle className="h-4 w-4 mr-1" />
                    Approve All
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    className="text-gray-600"
                    onClick={() => removeAll({ variables: { roleId } })}
                    title="Remove all pending tools from requests silently"
                  >
                    <EyeOff className="h-4 w-4 mr-1" />
                    Remove All
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    className="text-red-600"
                    onClick={() => denyAll({ variables: { roleId } })}
                    title="Block requests that use any pending tool"
                  >
                    <XCircle className="h-4 w-4 mr-1" />
                    Deny All
                  </Button>
                </>
              )}
            </div>

            {/* Tool List */}
            {loading ? (
              <div className="flex justify-center py-8">
                <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
              </div>
            ) : tools.length === 0 ? (
              <div className="text-center py-8 text-muted-foreground">
                <AlertCircle className="h-10 w-10 mx-auto mb-2 opacity-50" />
                <p>No tools discovered yet.</p>
                <p className="text-sm">Tools will appear here when requests with tool definitions are made.</p>
              </div>
            ) : (
              <div className="border rounded-lg overflow-hidden">
                <table className="w-full">
                  <thead className="bg-muted/50">
                    <tr>
                      <th className="text-left p-3 text-sm font-medium">Tool</th>
                      <th className="text-left p-3 text-sm font-medium">Description</th>
                      <th className="text-left p-3 text-sm font-medium">Category</th>
                      <th className="text-center p-3 text-sm font-medium">Usage</th>
                      <th className="text-center p-3 text-sm font-medium">Status</th>
                      {!readOnly && <th className="text-center p-3 text-sm font-medium">Actions</th>}
                    </tr>
                  </thead>
                  <tbody>
                    {filteredTools.map((tp) => (
                      <tr key={tp.tool.id} className="border-t hover:bg-muted/30">
                        <td className="p-3">
                          <div className="font-mono text-sm">{tp.tool.name}</div>
                        </td>
                        <td className="p-3">
                          <div className="text-sm text-muted-foreground line-clamp-2 break-words max-w-xs">
                            {tp.tool.description}
                          </div>
                        </td>
                        <td className="p-3">
                          <Badge variant="secondary" className="text-xs">
                            {tp.tool.category || 'general'}
                          </Badge>
                        </td>
                        <td className="p-3 text-center">
                          <span className="text-sm">{tp.tool.seenCount}×</span>
                        </td>
                        <td className="p-3">
                          <div className="flex justify-center items-center gap-1">
                            {statusIcon(tp.status)}
                            <span className="text-sm">{tp.status.toLowerCase()}</span>
                          </div>
                        </td>
                        {!readOnly && (
                          <td className="p-3">
                            <div className="flex justify-center items-center gap-2">
                              <Select
                                value={tp.status}
                                onValueChange={(value) =>
                                  handleSetPermission(tp.tool.id, value as 'ALLOWED' | 'DENIED' | 'REMOVED')
                                }
                                disabled={readOnly}
                              >
                                <SelectTrigger className="w-32">
                                  <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                  <SelectItem value="ALLOWED">
                                    <div className="flex items-center gap-2">
                                      <CheckCircle className="h-3 w-3 text-green-500" />
                                      Allow
                                    </div>
                                  </SelectItem>
                                  <SelectItem value="REMOVED">
                                    <div className="flex items-center gap-2">
                                      <EyeOff className="h-3 w-3 text-gray-500" />
                                      Remove
                                    </div>
                                  </SelectItem>
                                  <SelectItem value="DENIED">
                                    <div className="flex items-center gap-2">
                                      <XCircle className="h-3 w-3 text-red-500" />
                                      Deny
                                    </div>
                                  </SelectItem>
                                </SelectContent>
                              </Select>
                              <Button
                                variant="ghost"
                                size="sm"
                                className="h-7 text-red-600 hover:text-red-700 hover:bg-red-50"
                                onClick={() => {
                                  if (
                                    window.confirm(
                                      `Delete tool "${tp.tool.name}"?\n\nThis will permanently remove the tool from the database and all its permissions. This action cannot be undone.`
                                    )
                                  ) {
                                    deleteTool({ variables: { id: tp.tool.id } })
                                  }
                                }}
                                title="Delete: Permanently remove this tool from the database"
                              >
                                <Trash2 className="h-3.5 w-3.5" />
                              </Button>
                            </div>
                          </td>
                        )}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {!roleId && (
        <div className="text-center py-8 text-muted-foreground border rounded-lg">
          <p>Save this role first to manage tool permissions.</p>
        </div>
      )}
    </div>
  )
}

// =============================================================================
// MCP POLICY EDITOR
// =============================================================================

type MCPToolVisibility = 'DENY' | 'SEARCH' | 'ALLOW'

interface MCPServerWithTools {
  server: {
    id: string
    name: string
    description?: string
    status: string
    toolCount: number
  }
  tools: {
    tool: {
      id: string
      serverId: string
      serverName: string
      name: string
      description?: string
      category?: string
    }
    visibility: MCPToolVisibility
    decidedBy?: string
    decidedAt?: string
  }[]
  stats: {
    totalTools: number
    allowedCount: number
    searchCount: number
    deniedCount: number
  }
}

function MCPPolicyEditor({
  mcpPolicies,
  onChange,
  readOnly,
  roleId,
}: {
  mcpPolicies: MCPPolicies
  onChange: (updates: Partial<MCPPolicies>) => void
  readOnly: boolean
  roleId?: string
}) {
  const [expandedServers, setExpandedServers] = useState<Set<string>>(new Set())

  const { data, loading, refetch } = useQuery(GET_MCP_SERVERS_WITH_TOOLS, {
    variables: { roleId },
    skip: !roleId || !mcpPolicies.enabled,
  })

  const [setPermission] = useMutation(SET_MCP_PERMISSION, {
    onCompleted: () => refetch(),
  })

  const [bulkSetVisibility] = useMutation(BULK_SET_MCP_VISIBILITY, {
    onCompleted: () => refetch(),
  })

  const servers: MCPServerWithTools[] = data?.mcpServersWithTools || []

  const toggleServer = (serverId: string) => {
    const newExpanded = new Set(expandedServers)
    if (newExpanded.has(serverId)) {
      newExpanded.delete(serverId)
    } else {
      newExpanded.add(serverId)
    }
    setExpandedServers(newExpanded)
  }

  const handleVisibilityChange = (serverId: string, toolId: string, visibility: MCPToolVisibility) => {
    setPermission({
      variables: {
        input: { roleId, serverId, toolId, visibility },
      },
    })
  }

  const handleBulkVisibility = (serverId: string, visibility: MCPToolVisibility) => {
    bulkSetVisibility({
      variables: { roleId, serverId, visibility },
    })
  }

  const getVisibilityBadge = (visibility: MCPToolVisibility) => {
    switch (visibility) {
      case 'ALLOW':
        return <Badge className="bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200">Allow</Badge>
      case 'SEARCH':
        return <Badge className="bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200">Search Only</Badge>
      case 'DENY':
        return <Badge className="bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200">Deny</Badge>
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2 mb-4">
        <Plug className="h-5 w-5 text-indigo-500" />
        <h3 className="text-lg font-semibold">MCP Gateway Policy</h3>
      </div>

      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle className="text-base">MCP Gateway Access</CardTitle>
              <CardDescription>Control access to MCP servers and tools</CardDescription>
            </div>
            <Switch
              checked={mcpPolicies.enabled}
              onCheckedChange={(checked) => onChange({ enabled: checked })}
              disabled={readOnly}
            />
          </div>
        </CardHeader>
        {mcpPolicies.enabled && (
          <CardContent className="space-y-6">
            {/* Global Settings */}
            <div className="grid grid-cols-2 gap-4">
              <div className="flex items-center justify-between p-4 border rounded-lg">
                <div>
                  <p className="font-medium">Allow Tool Search</p>
                  <p className="text-sm text-muted-foreground">
                    Enable the tool_search tool for discovery
                  </p>
                </div>
                <Switch
                  checked={mcpPolicies.allowToolSearch}
                  onCheckedChange={(checked) => onChange({ allowToolSearch: checked })}
                  disabled={readOnly}
                />
              </div>
              <div className="flex items-center justify-between p-4 border rounded-lg">
                <div>
                  <p className="font-medium">Log All Tool Calls</p>
                  <p className="text-sm text-muted-foreground">Audit all MCP tool executions</p>
                </div>
                <Switch
                  checked={mcpPolicies.auditToolExecution}
                  onCheckedChange={(checked) => onChange({ auditToolExecution: checked })}
                  disabled={readOnly}
                />
              </div>
            </div>

            {/* Info Box */}
            <div className="flex items-start gap-3 p-4 bg-amber-50 dark:bg-amber-950/20 rounded-lg border border-amber-100 dark:border-amber-900">
              <AlertTriangle className="h-5 w-5 text-amber-500 mt-0.5 flex-shrink-0" />
              <div>
                <p className="font-medium text-amber-800 dark:text-amber-300">Default: Deny All</p>
                <p className="text-sm text-amber-700 dark:text-amber-400 mt-1">
                  All new MCP tools are denied by default. Set visibility for each tool below.
                </p>
              </div>
            </div>

            {/* MCP Servers and Tools */}
            <Card>
              <CardHeader>
                <CardTitle className="text-sm">MCP Tools by Server</CardTitle>
                <CardDescription>
                  <strong>Allow</strong> = Visible in tools/list AND searchable | 
                  <strong> Search</strong> = Only discoverable via tool_search | 
                  <strong> Deny</strong> = Completely hidden
                </CardDescription>
              </CardHeader>
              <CardContent>
                {loading ? (
                  <div className="flex items-center justify-center py-8">
                    <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
                  </div>
                ) : servers.length === 0 ? (
                  <div className="text-center py-8 text-muted-foreground">
                    <Plug className="h-12 w-12 mx-auto mb-4 opacity-20" />
                    <p>No MCP servers connected</p>
                    <p className="text-sm mt-1">Add MCP servers in the MCP Gateway section</p>
                  </div>
                ) : (
                  <div className="space-y-2">
                    {servers.map((serverWithTools) => (
                      <div key={serverWithTools.server.id} className="border rounded-lg">
                        {/* Server Header */}
                        <div
                          className="flex items-center justify-between p-3 cursor-pointer hover:bg-muted/50"
                          onClick={() => toggleServer(serverWithTools.server.id)}
                        >
                          <div className="flex items-center gap-3">
                            {expandedServers.has(serverWithTools.server.id) ? (
                              <ChevronDown className="h-4 w-4" />
                            ) : (
                              <ChevronRight className="h-4 w-4" />
                            )}
                            <Plug className="h-4 w-4 text-indigo-500" />
                            <span className="font-medium">{serverWithTools.server.name}</span>
                            <Badge variant="outline" className="text-xs">
                              {serverWithTools.stats.totalTools} tools
                            </Badge>
                          </div>
                          <div className="flex items-center gap-2">
                            <Badge className="bg-green-100 text-green-800 text-xs">
                              {serverWithTools.stats.allowedCount} allowed
                            </Badge>
                            <Badge className="bg-blue-100 text-blue-800 text-xs">
                              {serverWithTools.stats.searchCount} search
                            </Badge>
                            <Badge className="bg-red-100 text-red-800 text-xs">
                              {serverWithTools.stats.deniedCount} denied
                            </Badge>
                          </div>
                        </div>

                        {/* Tools List */}
                        {expandedServers.has(serverWithTools.server.id) && (
                          <div className="border-t">
                            {/* Bulk Actions */}
                            <div className="flex gap-2 p-2 bg-muted/30 border-b">
                              <Button
                                variant="outline"
                                size="sm"
                                onClick={() => handleBulkVisibility(serverWithTools.server.id, 'ALLOW')}
                                disabled={readOnly}
                              >
                                <CheckCircle className="h-3 w-3 mr-1" />
                                Allow All
                              </Button>
                              <Button
                                variant="outline"
                                size="sm"
                                onClick={() => handleBulkVisibility(serverWithTools.server.id, 'SEARCH')}
                                disabled={readOnly}
                              >
                                <Eye className="h-3 w-3 mr-1" />
                                Search All
                              </Button>
                              <Button
                                variant="outline"
                                size="sm"
                                onClick={() => handleBulkVisibility(serverWithTools.server.id, 'DENY')}
                                disabled={readOnly}
                              >
                                <XCircle className="h-3 w-3 mr-1" />
                                Deny All
                              </Button>
                            </div>

                            {/* Individual Tools */}
                            <div className="divide-y">
                              {serverWithTools.tools.map((toolWithVis) => (
                                <div
                                  key={toolWithVis.tool.id}
                                  className="flex items-start justify-between p-3 hover:bg-muted/30"
                                >
                                  <div className="flex-1 min-w-0 pr-4">
                                    <div className="flex items-center gap-2">
                                      <Wrench className="h-4 w-4 text-muted-foreground flex-shrink-0" />
                                      <span className="font-medium truncate">
                                        {toolWithVis.tool.name}
                                      </span>
                                      {toolWithVis.tool.category && (
                                        <Badge variant="outline" className="text-xs">
                                          {toolWithVis.tool.category}
                                        </Badge>
                                      )}
                                    </div>
                                    {toolWithVis.tool.description && (
                                      <p className="text-sm text-muted-foreground mt-1 ml-6 break-words">
                                        {toolWithVis.tool.description}
                                      </p>
                                    )}
                                  </div>
                                  <div className="flex items-center gap-2 flex-shrink-0">
                                    <Select
                                      value={toolWithVis.visibility}
                                      onValueChange={(value) =>
                                        handleVisibilityChange(
                                          toolWithVis.tool.serverId,
                                          toolWithVis.tool.id,
                                          value as MCPToolVisibility
                                        )
                                      }
                                      disabled={readOnly}
                                    >
                                      <SelectTrigger className="w-32">
                                        <SelectValue />
                                      </SelectTrigger>
                                      <SelectContent>
                                        <SelectItem value="ALLOW">
                                          <div className="flex items-center gap-2">
                                            <CheckCircle className="h-3 w-3 text-green-500" />
                                            Allow
                                          </div>
                                        </SelectItem>
                                        <SelectItem value="SEARCH">
                                          <div className="flex items-center gap-2">
                                            <Eye className="h-3 w-3 text-blue-500" />
                                            Search
                                          </div>
                                        </SelectItem>
                                        <SelectItem value="DENY">
                                          <div className="flex items-center gap-2">
                                            <XCircle className="h-3 w-3 text-red-500" />
                                            Deny
                                          </div>
                                        </SelectItem>
                                      </SelectContent>
                                    </Select>
                                  </div>
                                </div>
                              ))}
                            </div>
                          </div>
                        )}
                      </div>
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>

            {/* Info Box */}
            <div className="flex items-start gap-3 p-4 bg-blue-50 dark:bg-blue-950/20 rounded-lg border border-blue-100 dark:border-blue-900">
              <Info className="h-5 w-5 text-blue-500 mt-0.5 flex-shrink-0" />
              <div>
                <p className="font-medium text-blue-800 dark:text-blue-300">MCP Visibility States</p>
                <ul className="text-sm text-blue-700 dark:text-blue-400 mt-1 space-y-1">
                  <li><strong>Allow</strong>: Tool appears in <code>tools/list</code> AND is searchable via <code>tool_search</code></li>
                  <li><strong>Search</strong>: Tool is hidden from <code>tools/list</code> but discoverable via <code>tool_search</code></li>
                  <li><strong>Deny</strong>: Tool is completely hidden and blocked (default for new tools)</li>
                </ul>
              </div>
            </div>
          </CardContent>
        )}
      </Card>
    </div>
  )
}

// =============================================================================
// RATE LIMIT EDITOR
// =============================================================================

function RateLimitEditor({
  rateLimitPolicy,
  onChange,
  readOnly,
}: {
  rateLimitPolicy: RateLimitPolicy
  onChange: (updates: Partial<RateLimitPolicy>) => void
  readOnly: boolean
}) {
  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2 mb-4">
        <Gauge className="h-5 w-5 text-amber-500" />
        <h3 className="text-lg font-semibold">Rate Limiting</h3>
      </div>

      <div className="grid grid-cols-2 gap-4">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Request Limits</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-3 gap-4">
              <div className="space-y-2">
                <label className="text-sm font-medium">Per Minute</label>
                <Input
                  type="number"
                  value={rateLimitPolicy.requestsPerMinute}
                  onChange={(e) =>
                    onChange({ requestsPerMinute: parseInt(e.target.value) || 0 })
                  }
                  disabled={readOnly}
                />
              </div>
              <div className="space-y-2">
                <label className="text-sm font-medium">Per Hour</label>
                <Input
                  type="number"
                  value={rateLimitPolicy.requestsPerHour}
                  onChange={(e) =>
                    onChange({ requestsPerHour: parseInt(e.target.value) || 0 })
                  }
                  disabled={readOnly}
                />
              </div>
              <div className="space-y-2">
                <label className="text-sm font-medium">Per Day</label>
                <Input
                  type="number"
                  value={rateLimitPolicy.requestsPerDay}
                  onChange={(e) =>
                    onChange({ requestsPerDay: parseInt(e.target.value) || 0 })
                  }
                  disabled={readOnly}
                />
              </div>
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">Burst Limit</label>
              <Input
                type="number"
                value={rateLimitPolicy.burstLimit}
                onChange={(e) => onChange({ burstLimit: parseInt(e.target.value) || 0 })}
                disabled={readOnly}
              />
              <p className="text-xs text-muted-foreground">
                Max concurrent requests allowed in a burst
              </p>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">Token Limits</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-3 gap-4">
              <div className="space-y-2">
                <label className="text-sm font-medium">Per Minute</label>
                <Input
                  type="number"
                  value={rateLimitPolicy.tokensPerMinute}
                  onChange={(e) =>
                    onChange({ tokensPerMinute: parseInt(e.target.value) || 0 })
                  }
                  disabled={readOnly}
                />
              </div>
              <div className="space-y-2">
                <label className="text-sm font-medium">Per Hour</label>
                <Input
                  type="number"
                  value={rateLimitPolicy.tokensPerHour}
                  onChange={(e) =>
                    onChange({ tokensPerHour: parseInt(e.target.value) || 0 })
                  }
                  disabled={readOnly}
                />
              </div>
              <div className="space-y-2">
                <label className="text-sm font-medium">Per Day</label>
                <Input
                  type="number"
                  value={rateLimitPolicy.tokensPerDay}
                  onChange={(e) =>
                    onChange({ tokensPerDay: parseInt(e.target.value) || 0 })
                  }
                  disabled={readOnly}
                />
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Cost Limits</CardTitle>
          <CardDescription>Control spending per time period</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">Per Day (USD)</label>
              <Input
                type="number"
                step="0.01"
                value={rateLimitPolicy.costPerDayUSD}
                onChange={(e) =>
                  onChange({ costPerDayUSD: parseFloat(e.target.value) || 0 })
                }
                disabled={readOnly}
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">Per Month (USD)</label>
              <Input
                type="number"
                step="0.01"
                value={rateLimitPolicy.costPerMonthUSD}
                onChange={(e) =>
                  onChange({ costPerMonthUSD: parseFloat(e.target.value) || 0 })
                }
                disabled={readOnly}
              />
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

// =============================================================================
// MODEL RESTRICTIONS EDITOR
// =============================================================================

function ModelRestrictionsEditor({
  modelRestrictions,
  onChange,
  availableModels,
  readOnly,
}: {
  modelRestrictions: ModelRestrictions
  onChange: (updates: Partial<ModelRestrictions>) => void
  availableModels: any[]
  readOnly: boolean
}) {
  const [expandedProviders, setExpandedProviders] = useState<Set<string>>(new Set())
  const [searchQuery, setSearchQuery] = useState('')

  // Get the allowed models list
  const allowedModels = modelRestrictions.allowedModels

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

  // Get available (unselected) and allowed models grouped by provider
  const availableByProvider: Record<string, any[]> = {}
  const allowedByProvider: Record<string, any[]> = {}

  providers.forEach((provider) => {
    const unselected = modelsByProvider[provider].filter(
      (m: any) => !allowedModels.includes(m.id)
    )
    const selected = modelsByProvider[provider].filter((m: any) =>
      allowedModels.includes(m.id)
    )

    availableByProvider[provider] = filterModels(unselected)
    allowedByProvider[provider] = selected
  })

  const addToAllowed = (modelIds: string[]) => {
    if (readOnly) return
    onChange({ allowedModels: [...allowedModels, ...modelIds] })
  }

  const removeFromAllowed = (modelIds: string[]) => {
    if (readOnly) return
    onChange({ allowedModels: allowedModels.filter((id) => !modelIds.includes(id)) })
  }

  const addAllFromProvider = (provider: string) => {
    const modelIds = availableByProvider[provider].map((m) => m.id)
    addToAllowed(modelIds)
  }

  const removeAllFromProvider = (provider: string) => {
    const modelIds = allowedByProvider[provider].map((m) => m.id)
    removeFromAllowed(modelIds)
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2 mb-4">
        <Box className="h-5 w-5 text-blue-500" />
        <h3 className="text-lg font-semibold">Model Access</h3>
      </div>

      {/* Settings Row */}
      <Card>
        <CardContent className="p-4">
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">Default Model</label>
              <Select
                value={modelRestrictions.defaultModel}
                onValueChange={(v) => onChange({ defaultModel: v })}
                disabled={readOnly}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Select default model" />
                </SelectTrigger>
                <SelectContent>
                  {availableModels.map((m) => (
                    <SelectItem key={m.id} value={m.id}>
                      {m.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium">Max Tokens/Request</label>
              <Input
                type="number"
                value={modelRestrictions.maxTokensPerRequest}
                onChange={(e) =>
                  onChange({ maxTokensPerRequest: parseInt(e.target.value) || 0 })
                }
                disabled={readOnly}
                placeholder="0 = model default"
              />
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Model Selection Split View */}
      <Card>
        <CardContent className="p-4">
          {availableModels.length === 0 ? (
            <p className="py-8 text-center text-muted-foreground">
              No models configured. Add provider API keys and refresh models first.
            </p>
          ) : (
            <div className="grid grid-cols-2 gap-4">
              {/* Available Models Column */}
              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <label className="text-sm font-medium flex items-center gap-2">
                    <Box className="h-4 w-4 text-muted-foreground" />
                    Available Models
                  </label>
                  <Badge variant="secondary">
                    {availableModels.length - allowedModels.length}
                  </Badge>
                </div>
                <Input
                  placeholder="Search models..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="h-8"
                  disabled={readOnly}
                />
                <div className="h-80 overflow-y-auto rounded-lg border bg-muted/30">
                  {providers.map((provider) => {
                    const models = availableByProvider[provider]
                    if (models.length === 0) return null

                    const isExpanded = expandedProviders.has(provider)

                    return (
                      <div key={provider} className="border-b last:border-b-0">
                        {/* Provider Header */}
                        <div
                          className="flex cursor-pointer items-center gap-2 px-3 py-2 hover:bg-muted/50 sticky top-0 bg-background/95 backdrop-blur-sm"
                          onClick={() => toggleProvider(provider)}
                        >
                          {isExpanded ? (
                            <ChevronDown className="h-4 w-4 text-muted-foreground" />
                          ) : (
                            <ChevronRight className="h-4 w-4 text-muted-foreground" />
                          )}
                          <span className="font-medium text-sm">{provider}</span>
                          <Badge variant="outline" className="ml-auto text-xs">
                            {models.length}
                          </Badge>
                          <button
                            className="p-1 hover:bg-primary/10 rounded transition-colors"
                            onClick={(e) => {
                              e.stopPropagation()
                              addAllFromProvider(provider)
                            }}
                            disabled={readOnly}
                            title={`Add all ${provider} models`}
                          >
                            <ChevronRight className="h-4 w-4 text-primary" />
                            <ChevronRight className="h-4 w-4 text-primary -ml-2" />
                          </button>
                        </div>
                        {/* Models List */}
                        {isExpanded && (
                          <div className="bg-muted/20">
                            {models.map((model) => (
                              <div
                                key={model.id}
                                className="flex items-center gap-2 px-3 py-1.5 pl-8 text-sm cursor-pointer hover:bg-primary/10 transition-colors group"
                                onDoubleClick={() => addToAllowed([model.id])}
                                title="Double-click to add"
                              >
                                <span className="truncate flex-1">{model.name}</span>
                                <button
                                  className="opacity-0 group-hover:opacity-100 p-0.5 hover:bg-primary/20 rounded transition-all"
                                  onClick={() => addToAllowed([model.id])}
                                  disabled={readOnly}
                                >
                                  <ChevronRight className="h-3 w-3 text-primary" />
                                </button>
                              </div>
                            ))}
                          </div>
                        )}
                      </div>
                    )
                  })}
                  {providers.every((p) => availableByProvider[p].length === 0) && (
                    <p className="p-4 text-center text-sm text-muted-foreground">
                      {searchQuery ? 'No models match your search' : 'All models have been added'}
                    </p>
                  )}
                </div>
              </div>

              {/* Allowed Models Column */}
              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <label className="text-sm font-medium flex items-center gap-2">
                    <Lock className="h-4 w-4 text-green-500" />
                    Allowed Models
                  </label>
                  <Badge variant="success">
                    {allowedModels.length}
                  </Badge>
                </div>
                <div className="h-8" /> {/* Spacer to align with search box */}
                <div className="h-80 overflow-y-auto rounded-lg border bg-muted/30">
                  {providers.map((provider) => {
                    const models = allowedByProvider[provider]
                    if (models.length === 0) return null

                    const expandKey = provider + '-allowed'
                    const isExpanded = expandedProviders.has(expandKey)

                    return (
                      <div key={provider} className="border-b last:border-b-0">
                        {/* Provider Header */}
                        <div
                          className="flex cursor-pointer items-center gap-2 px-3 py-2 hover:bg-muted/50 sticky top-0 bg-background/95 backdrop-blur-sm"
                          onClick={() => toggleProvider(expandKey)}
                        >
                          <button
                            className="p-1 hover:bg-destructive/10 rounded transition-colors"
                            onClick={(e) => {
                              e.stopPropagation()
                              removeAllFromProvider(provider)
                            }}
                            disabled={readOnly}
                            title={`Remove all ${provider} models`}
                          >
                            <ChevronRight className="h-4 w-4 text-destructive rotate-180" />
                            <ChevronRight className="h-4 w-4 text-destructive rotate-180 -ml-2" />
                          </button>
                          {isExpanded ? (
                            <ChevronDown className="h-4 w-4 text-muted-foreground" />
                          ) : (
                            <ChevronRight className="h-4 w-4 text-muted-foreground" />
                          )}
                          <span className="font-medium text-sm">{provider}</span>
                          <Badge variant="outline" className="ml-auto text-xs">
                            {models.length}
                          </Badge>
                        </div>
                        {/* Models List */}
                        {isExpanded && (
                          <div className="bg-muted/20">
                            {models.map((model) => (
                              <div
                                key={model.id}
                                className="flex items-center gap-2 px-3 py-1.5 pl-8 text-sm cursor-pointer hover:bg-destructive/10 transition-colors group"
                                onDoubleClick={() => removeFromAllowed([model.id])}
                                title="Double-click to remove"
                              >
                                <button
                                  className="opacity-0 group-hover:opacity-100 p-0.5 hover:bg-destructive/20 rounded transition-all"
                                  onClick={() => removeFromAllowed([model.id])}
                                  disabled={readOnly}
                                >
                                  <ChevronRight className="h-3 w-3 text-destructive rotate-180" />
                                </button>
                                <span className="truncate flex-1">{model.name}</span>
                              </div>
                            ))}
                          </div>
                        )}
                      </div>
                    )
                  })}
                  {allowedModels.length === 0 && (
                    <p className="p-4 text-center text-sm text-muted-foreground">
                      No models allowed. Double-click models on the left to add them.
                    </p>
                  )}
                </div>
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Help text */}
      <p className="text-xs text-muted-foreground text-center">
        💡 Double-click a model to move it between lists, or use the arrow buttons to move all models from a provider
      </p>
    </div>
  )
}

// =============================================================================
// CACHING POLICY EDITOR
// =============================================================================

function CachingPolicyEditor({
  cachingPolicy,
  onChange,
  readOnly,
}: {
  cachingPolicy: CachingPolicy
  onChange: (updates: Partial<CachingPolicy>) => void
  readOnly: boolean
}) {
  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2 mb-4">
        <Database className="h-5 w-5 text-cyan-500" />
        <h3 className="text-lg font-semibold">Semantic Caching</h3>
        <Badge variant="outline" className="ml-2">
          {cachingPolicy.enabled ? 'Enabled' : 'Disabled'}
        </Badge>
      </div>

      <Card className={!cachingPolicy.enabled ? 'opacity-60' : ''}>
        <CardContent className="p-6 space-y-6">
          <div className="flex items-center justify-between">
            <div>
              <label className="text-sm font-medium">Enable Semantic Caching</label>
              <p className="text-xs text-muted-foreground">
                Cache similar prompts to reduce costs and latency
              </p>
            </div>
            <Switch
              checked={cachingPolicy.enabled}
              onCheckedChange={(enabled) => onChange({ enabled })}
              disabled={readOnly}
            />
          </div>

          <div className="grid grid-cols-3 gap-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">
                Similarity Threshold ({(cachingPolicy.similarityThreshold * 100).toFixed(0)}%)
              </label>
              <input
                type="range"
                min="0.8"
                max="1"
                step="0.01"
                value={cachingPolicy.similarityThreshold}
                onChange={(e) =>
                  onChange({ similarityThreshold: parseFloat(e.target.value) })
                }
                className="w-full"
                disabled={readOnly || !cachingPolicy.enabled}
              />
              <p className="text-xs text-muted-foreground">Higher = stricter matching</p>
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium">TTL (seconds)</label>
              <Input
                type="number"
                value={cachingPolicy.ttlSeconds}
                onChange={(e) => onChange({ ttlSeconds: parseInt(e.target.value) || 0 })}
                disabled={readOnly || !cachingPolicy.enabled}
              />
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium">Max Cache Size</label>
              <Input
                type="number"
                value={cachingPolicy.maxCacheSize}
                onChange={(e) => onChange({ maxCacheSize: parseInt(e.target.value) || 0 })}
                disabled={readOnly || !cachingPolicy.enabled}
              />
              <p className="text-xs text-muted-foreground">Entries per role</p>
            </div>
          </div>

          <div className="grid grid-cols-3 gap-4">
            <div className="flex items-center justify-between">
              <label className="text-sm font-medium">Cache Streaming</label>
              <Switch
                checked={cachingPolicy.cacheStreaming}
                onCheckedChange={(cacheStreaming) => onChange({ cacheStreaming })}
                disabled={readOnly || !cachingPolicy.enabled}
              />
            </div>

            <div className="flex items-center justify-between">
              <label className="text-sm font-medium">Track Savings</label>
              <Switch
                checked={cachingPolicy.trackSavings}
                onCheckedChange={(trackSavings) => onChange({ trackSavings })}
                disabled={readOnly || !cachingPolicy.enabled}
              />
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

// =============================================================================
// ROUTING POLICY EDITOR
// =============================================================================

function RoutingPolicyEditor({
  routingPolicy,
  onChange,
  availableModels,
  readOnly,
}: {
  routingPolicy: RoutingPolicy
  onChange: (updates: Partial<RoutingPolicy>) => void
  availableModels: any[]
  readOnly: boolean
}) {
  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2 mb-4">
        <Route className="h-5 w-5 text-pink-500" />
        <h3 className="text-lg font-semibold">Intelligent Routing</h3>
        <Badge className="ml-2 bg-amber-100 text-amber-700 border-amber-200">
          Enterprise Feature
        </Badge>
      </div>

      {/* Enterprise Feature Banner */}
      <Card className="border-amber-200 bg-amber-50/50">
        <CardContent className="p-4">
          <div className="flex items-start gap-3">
            <div className="w-10 h-10 rounded-lg bg-amber-100 flex items-center justify-center flex-shrink-0">
              <Route className="h-5 w-5 text-amber-600" />
            </div>
            <div>
              <h4 className="font-semibold text-amber-800">Enterprise Feature</h4>
              <p className="text-sm text-amber-700 mt-1">
                Intelligent routing allows you to automatically route requests to optimal models based on cost, latency, or capability requirements. 
                This feature is available in the Enterprise edition.
              </p>
              <a href="mailto:enterprise@modelgate.io" className="text-sm text-amber-600 hover:text-amber-800 font-medium mt-2 inline-block">
                Contact us for Enterprise →
              </a>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card className={!routingPolicy.enabled ? 'opacity-60' : ''}>
        <CardContent className="p-6 space-y-6">
          <div className="flex items-center justify-between">
            <div>
              <label className="text-sm font-medium">Enable Intelligent Routing</label>
              <p className="text-xs text-muted-foreground">
                Route requests to optimal models based on strategy
              </p>
            </div>
            <Switch
              checked={routingPolicy.enabled}
              onCheckedChange={(enabled) => onChange({ enabled })}
              disabled={readOnly}
            />
          </div>

          <div className="grid grid-cols-2 gap-6">
            <div className="space-y-2">
              <label className="text-sm font-medium">Routing Strategy</label>
              <Select
                value={routingPolicy.strategy}
                onValueChange={(v) => onChange({ strategy: v })}
                disabled={readOnly || !routingPolicy.enabled}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="COST">Cost Optimized</SelectItem>
                  <SelectItem value="LATENCY">Latency Optimized</SelectItem>
                  <SelectItem value="WEIGHTED">Weighted Distribution</SelectItem>
                  <SelectItem value="ROUND_ROBIN">Round Robin</SelectItem>
                  <SelectItem value="CAPABILITY">Capability Based</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="flex items-center justify-between">
              <div>
                <label className="text-sm font-medium">Allow Model Override</label>
                <p className="text-xs text-muted-foreground">
                  Skip routing if model is explicitly specified
                </p>
              </div>
              <Switch
                checked={routingPolicy.allowModelOverride}
                onCheckedChange={(allowModelOverride) => onChange({ allowModelOverride })}
                disabled={readOnly || !routingPolicy.enabled}
              />
            </div>
          </div>

          {routingPolicy.strategy === 'COST' && (
            <Card className="bg-muted/50">
              <CardHeader>
                <CardTitle className="text-sm">Cost Routing Configuration</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <p className="text-xs text-muted-foreground">
                  Route simple queries to cheaper models, complex queries to premium models
                </p>
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <label className="text-sm">Simple Query Threshold</label>
                    <Input
                      type="number"
                      step="0.1"
                      value={routingPolicy.costConfig?.simpleQueryThreshold || 0.3}
                      onChange={(e) =>
                        onChange({
                          costConfig: {
                            ...routingPolicy.costConfig,
                            simpleQueryThreshold: parseFloat(e.target.value) || 0,
                            complexQueryThreshold: routingPolicy.costConfig?.complexQueryThreshold || 0.7,
                            simpleModels: routingPolicy.costConfig?.simpleModels || [],
                            complexModels: routingPolicy.costConfig?.complexModels || [],
                          },
                        })
                      }
                      disabled={readOnly || !routingPolicy.enabled}
                    />
                  </div>
                  <div className="space-y-2">
                    <label className="text-sm">Complex Query Threshold</label>
                    <Input
                      type="number"
                      step="0.1"
                      value={routingPolicy.costConfig?.complexQueryThreshold || 0.7}
                      onChange={(e) =>
                        onChange({
                          costConfig: {
                            ...routingPolicy.costConfig,
                            complexQueryThreshold: parseFloat(e.target.value) || 0,
                            simpleQueryThreshold: routingPolicy.costConfig?.simpleQueryThreshold || 0.3,
                            simpleModels: routingPolicy.costConfig?.simpleModels || [],
                            complexModels: routingPolicy.costConfig?.complexModels || [],
                          },
                        })
                      }
                      disabled={readOnly || !routingPolicy.enabled}
                    />
                  </div>
                </div>
              </CardContent>
            </Card>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

// =============================================================================
// RESILIENCE POLICY EDITOR
// =============================================================================

function ResiliencePolicyEditor({
  resiliencePolicy,
  onChange,
  availableModels,
  readOnly,
}: {
  resiliencePolicy: ResiliencePolicy
  onChange: (updates: Partial<ResiliencePolicy>) => void
  availableModels: any[]
  readOnly: boolean
}) {
  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2 mb-4">
        <RefreshCw className="h-5 w-5 text-orange-500" />
        <h3 className="text-lg font-semibold">Resilience & Failover</h3>
        <Badge className="ml-2 bg-amber-100 text-amber-700 border-amber-200">
          Enterprise Feature
        </Badge>
      </div>

      {/* Enterprise Feature Banner */}
      <Card className="border-amber-200 bg-amber-50/50">
        <CardContent className="p-4">
          <div className="flex items-start gap-3">
            <div className="w-10 h-10 rounded-lg bg-amber-100 flex items-center justify-center flex-shrink-0">
              <RefreshCw className="h-5 w-5 text-amber-600" />
            </div>
            <div>
              <h4 className="font-semibold text-amber-800">Enterprise Feature</h4>
              <p className="text-sm text-amber-700 mt-1">
                Resilience features include automatic retries with exponential backoff, circuit breakers to prevent cascade failures, 
                and fallback chains for high availability. This feature is available in the Enterprise edition.
              </p>
              <a href="mailto:enterprise@modelgate.io" className="text-sm text-amber-600 hover:text-amber-800 font-medium mt-2 inline-block">
                Contact us for Enterprise →
              </a>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card className={!resiliencePolicy.enabled ? 'opacity-60' : ''}>
        <CardContent className="p-6 space-y-6">
          <div className="flex items-center justify-between">
            <div>
              <label className="text-sm font-medium">Enable Resilience Features</label>
              <p className="text-xs text-muted-foreground">
                Automatic retries, fallbacks, and circuit breakers
              </p>
            </div>
            <Switch
              checked={resiliencePolicy.enabled}
              onCheckedChange={(enabled) => onChange({ enabled })}
              disabled={readOnly}
            />
          </div>

          {/* Retry Configuration */}
          <div className="grid grid-cols-2 gap-6">
            <Card className="bg-muted/50">
              <CardHeader>
                <CardTitle className="text-sm flex items-center gap-2">
                  <RefreshCw className="h-4 w-4" />
                  Retry Configuration
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex items-center justify-between">
                  <label className="text-sm">Enable Retries</label>
                  <Switch
                    checked={resiliencePolicy.retryEnabled}
                    onCheckedChange={(retryEnabled) => onChange({ retryEnabled })}
                    disabled={readOnly || !resiliencePolicy.enabled}
                  />
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <label className="text-sm">Max Retries</label>
                    <Input
                      type="number"
                      value={resiliencePolicy.maxRetries}
                      onChange={(e) =>
                        onChange({ maxRetries: parseInt(e.target.value) || 0 })
                      }
                      disabled={readOnly || !resiliencePolicy.enabled || !resiliencePolicy.retryEnabled}
                    />
                  </div>
                  <div className="space-y-2">
                    <label className="text-sm">Backoff (ms)</label>
                    <Input
                      type="number"
                      value={resiliencePolicy.retryBackoffMs}
                      onChange={(e) =>
                        onChange({ retryBackoffMs: parseInt(e.target.value) || 0 })
                      }
                      disabled={readOnly || !resiliencePolicy.enabled || !resiliencePolicy.retryEnabled}
                    />
                  </div>
                </div>
                <div className="space-y-2 text-sm">
                  {[
                    { key: 'retryOnTimeout', label: 'Retry on Timeout' },
                    { key: 'retryOnRateLimit', label: 'Retry on Rate Limit' },
                    { key: 'retryOnServerError', label: 'Retry on 5xx Errors' },
                  ].map((item) => (
                    <div key={item.key} className="flex items-center justify-between">
                      <span>{item.label}</span>
                      <Switch
                        checked={resiliencePolicy[item.key as keyof ResiliencePolicy] as boolean}
                        onCheckedChange={(checked) => onChange({ [item.key]: checked })}
                        disabled={readOnly || !resiliencePolicy.enabled || !resiliencePolicy.retryEnabled}
                      />
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>

            {/* Circuit Breaker */}
            <Card className="bg-muted/50">
              <CardHeader>
                <CardTitle className="text-sm flex items-center gap-2">
                  <Zap className="h-4 w-4" />
                  Circuit Breaker
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex items-center justify-between">
                  <label className="text-sm">Enable Circuit Breaker</label>
                  <Switch
                    checked={resiliencePolicy.circuitBreakerEnabled}
                    onCheckedChange={(circuitBreakerEnabled) =>
                      onChange({ circuitBreakerEnabled })
                    }
                    disabled={readOnly || !resiliencePolicy.enabled}
                  />
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <label className="text-sm">Failure Threshold</label>
                    <Input
                      type="number"
                      value={resiliencePolicy.circuitBreakerThreshold}
                      onChange={(e) =>
                        onChange({ circuitBreakerThreshold: parseInt(e.target.value) || 0 })
                      }
                      disabled={
                        readOnly ||
                        !resiliencePolicy.enabled ||
                        !resiliencePolicy.circuitBreakerEnabled
                      }
                    />
                    <p className="text-xs text-muted-foreground">Failures before open</p>
                  </div>
                  <div className="space-y-2">
                    <label className="text-sm">Recovery Timeout (s)</label>
                    <Input
                      type="number"
                      value={resiliencePolicy.circuitBreakerTimeout}
                      onChange={(e) =>
                        onChange({ circuitBreakerTimeout: parseInt(e.target.value) || 0 })
                      }
                      disabled={
                        readOnly ||
                        !resiliencePolicy.enabled ||
                        !resiliencePolicy.circuitBreakerEnabled
                      }
                    />
                    <p className="text-xs text-muted-foreground">Before half-open</p>
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>

          {/* Timeout */}
          <div className="space-y-2">
            <label className="text-sm font-medium">Request Timeout (ms)</label>
            <Input
              type="number"
              value={resiliencePolicy.requestTimeoutMs}
              onChange={(e) =>
                onChange({ requestTimeoutMs: parseInt(e.target.value) || 0 })
              }
              disabled={readOnly || !resiliencePolicy.enabled}
            />
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

// =============================================================================
// BUDGET POLICY EDITOR
// =============================================================================

function BudgetPolicyEditor({
  budgetPolicy,
  onChange,
  readOnly,
}: {
  budgetPolicy: BudgetPolicy
  onChange: (updates: Partial<BudgetPolicy>) => void
  readOnly: boolean
}) {
  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2 mb-4">
        <DollarSign className="h-5 w-5 text-green-500" />
        <h3 className="text-lg font-semibold">Budget Controls & Alerts</h3>
        <Badge variant="outline" className="ml-2">
          {budgetPolicy.enabled ? 'Enabled' : 'Disabled'}
        </Badge>
      </div>

      <Card className={!budgetPolicy.enabled ? 'opacity-60' : ''}>
        <CardContent className="p-6 space-y-6">
          <div className="flex items-center justify-between">
            <div>
              <label className="text-sm font-medium">Enable Budget Controls</label>
              <p className="text-xs text-muted-foreground">
                Set spending limits and receive alerts
              </p>
            </div>
            <Switch
              checked={budgetPolicy.enabled}
              onCheckedChange={(enabled) => onChange({ enabled })}
              disabled={readOnly}
            />
          </div>

          {/* Spending Limits */}
          <Card className="bg-muted/50">
            <CardHeader>
              <CardTitle className="text-sm">Spending Limits (USD)</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-4 gap-4">
                <div className="space-y-2">
                  <label className="text-sm">Per Request</label>
                  <Input
                    type="number"
                    step="0.01"
                    value={budgetPolicy.maxCostPerRequest}
                    onChange={(e) =>
                      onChange({ maxCostPerRequest: parseFloat(e.target.value) || 0 })
                    }
                    disabled={readOnly || !budgetPolicy.enabled}
                  />
                </div>
                <div className="space-y-2">
                  <label className="text-sm">Daily</label>
                  <Input
                    type="number"
                    step="0.01"
                    value={budgetPolicy.dailyLimitUSD}
                    onChange={(e) =>
                      onChange({ dailyLimitUSD: parseFloat(e.target.value) || 0 })
                    }
                    disabled={readOnly || !budgetPolicy.enabled}
                  />
                </div>
                <div className="space-y-2">
                  <label className="text-sm">Weekly</label>
                  <Input
                    type="number"
                    step="0.01"
                    value={budgetPolicy.weeklyLimitUSD}
                    onChange={(e) =>
                      onChange({ weeklyLimitUSD: parseFloat(e.target.value) || 0 })
                    }
                    disabled={readOnly || !budgetPolicy.enabled}
                  />
                </div>
                <div className="space-y-2">
                  <label className="text-sm">Monthly</label>
                  <Input
                    type="number"
                    step="0.01"
                    value={budgetPolicy.monthlyLimitUSD}
                    onChange={(e) =>
                      onChange({ monthlyLimitUSD: parseFloat(e.target.value) || 0 })
                    }
                    disabled={readOnly || !budgetPolicy.enabled}
                  />
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Alert Thresholds */}
          <div className="grid grid-cols-2 gap-6">
            <Card className="bg-muted/50">
              <CardHeader>
                <CardTitle className="text-sm flex items-center gap-2">
                  <AlertTriangle className="h-4 w-4 text-yellow-500" />
                  Alert Thresholds
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-2">
                  <label className="text-sm">
                    Warning at {(budgetPolicy.alertThreshold * 100).toFixed(0)}%
                  </label>
                  <input
                    type="range"
                    min="0.5"
                    max="1"
                    step="0.05"
                    value={budgetPolicy.alertThreshold}
                    onChange={(e) =>
                      onChange({ alertThreshold: parseFloat(e.target.value) })
                    }
                    className="w-full"
                    disabled={readOnly || !budgetPolicy.enabled}
                  />
                </div>
                <div className="space-y-2">
                  <label className="text-sm">
                    Critical at {(budgetPolicy.criticalThreshold * 100).toFixed(0)}%
                  </label>
                  <input
                    type="range"
                    min="0.5"
                    max="1"
                    step="0.05"
                    value={budgetPolicy.criticalThreshold}
                    onChange={(e) =>
                      onChange({ criticalThreshold: parseFloat(e.target.value) })
                    }
                    className="w-full"
                    disabled={readOnly || !budgetPolicy.enabled}
                  />
                </div>
              </CardContent>
            </Card>

            <Card className="bg-muted/50">
              <CardHeader>
                <CardTitle className="text-sm">On Budget Exceeded</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <Select
                  value={budgetPolicy.onExceeded}
                  onValueChange={(v) => onChange({ onExceeded: v })}
                  disabled={readOnly || !budgetPolicy.enabled}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="BLOCK">Block All Requests</SelectItem>
                    <SelectItem value="WARN">Allow but Warn</SelectItem>
                    <SelectItem value="THROTTLE">Reduce Rate Limit</SelectItem>
                  </SelectContent>
                </Select>
              </CardContent>
            </Card>
          </div>

          {/* Alert Channels */}
          <Card className="bg-muted/50">
            <CardHeader>
              <CardTitle className="text-sm">Alert Channels</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="space-y-2">
                <label className="text-sm">Webhook URL</label>
                <Input
                  placeholder="https://..."
                  value={budgetPolicy.alertWebhook}
                  onChange={(e) => onChange({ alertWebhook: e.target.value })}
                  disabled={readOnly || !budgetPolicy.enabled}
                />
              </div>
              <div className="space-y-2">
                <label className="text-sm">Email Addresses (comma-separated)</label>
                <Input
                  placeholder="admin@example.com, finance@example.com"
                  value={budgetPolicy.alertEmails.join(', ')}
                  onChange={(e) =>
                    onChange({
                      alertEmails: e.target.value
                        .split(',')
                        .map((s) => s.trim())
                        .filter((s) => s),
                    })
                  }
                  disabled={readOnly || !budgetPolicy.enabled}
                />
              </div>
            </CardContent>
          </Card>
        </CardContent>
      </Card>
    </div>
  )
}

// =============================================================================
// COLLAPSIBLE SECTION COMPONENT
// =============================================================================

function CollapsibleSection({
  title,
  icon: Icon,
  expanded,
  onToggle,
  enabled,
  onEnabledChange,
  children,
  readOnly,
}: {
  title: string
  icon: any
  expanded: boolean
  onToggle: () => void
  enabled: boolean
  onEnabledChange: (enabled: boolean) => void
  children: React.ReactNode
  readOnly: boolean
}) {
  return (
    <Card className={!enabled ? 'opacity-70' : ''}>
      <div
        className="flex items-center justify-between p-4 cursor-pointer hover:bg-muted/50"
        onClick={onToggle}
      >
        <div className="flex items-center gap-3">
          {expanded ? (
            <ChevronDown className="h-4 w-4 text-muted-foreground" />
          ) : (
            <ChevronRight className="h-4 w-4 text-muted-foreground" />
          )}
          <Icon className="h-5 w-5 text-primary" />
          <span className="font-medium">{title}</span>
        </div>
        <div className="flex items-center gap-4" onClick={(e) => e.stopPropagation()}>
          <Badge variant={enabled ? 'success' : 'secondary'}>{enabled ? 'On' : 'Off'}</Badge>
          <Switch checked={enabled} onCheckedChange={onEnabledChange} disabled={readOnly} />
        </div>
      </div>
      {expanded && <div className="p-4 pt-0 border-t">{children}</div>}
    </Card>
  )
}

export default PolicyEditorAdvanced

