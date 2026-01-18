import { useState } from 'react'
import { useQuery, useMutation } from '@apollo/client'
import { Check, X, Loader2, RefreshCw, Key } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import { GET_PROVIDERS, UPDATE_PROVIDER, REFRESH_PROVIDER_MODELS } from '@/graphql/operations'
import { providerIcons, cn } from '@/lib/utils'
import { useToast } from '@/components/ui/use-toast'
import { APIKeyManager } from '@/components/APIKeyManager'

const PROVIDER_INFO: Record<string, { name: string; description: string; defaultBaseUrl?: string }> = {
  OPENAI: { name: 'OpenAI', description: 'GPT-4, GPT-4o, GPT-3.5 Turbo and more', defaultBaseUrl: 'https://api.openai.com/v1' },
  ANTHROPIC: { name: 'Anthropic', description: 'Claude 3 Opus, Sonnet, Haiku', defaultBaseUrl: 'https://api.anthropic.com/v1' },
  GEMINI: { name: 'Google Gemini', description: 'Gemini Pro, Flash, and Ultra', defaultBaseUrl: 'https://generativelanguage.googleapis.com/v1beta' },
  BEDROCK: { name: 'AWS Bedrock', description: 'Claude, Llama, Titan via AWS' },
  AZURE_OPENAI: { name: 'Azure OpenAI', description: 'OpenAI models on Azure' },
  OLLAMA: { name: 'Ollama', description: 'Local LLM inference', defaultBaseUrl: 'http://localhost:11434' },
  GROQ: { name: 'Groq', description: 'Ultra-fast inference with LPUs', defaultBaseUrl: 'https://api.groq.com/openai/v1' },
  MISTRAL: { name: 'Mistral', description: 'Mistral Large, Medium, Small', defaultBaseUrl: 'https://api.mistral.ai/v1' },
  TOGETHER: { name: 'Together AI', description: 'Open-source models at scale', defaultBaseUrl: 'https://api.together.xyz/v1' },
  COHERE: { name: 'Cohere', description: 'Command models for enterprise', defaultBaseUrl: 'https://api.cohere.com/v2' },
}

export function ProvidersPage() {
  const { data, loading, refetch } = useQuery(GET_PROVIDERS)
  const [updateProvider, { loading: updating }] = useMutation(UPDATE_PROVIDER)
  const [refreshProviderModels, { loading: refreshing }] = useMutation(REFRESH_PROVIDER_MODELS)
  const [editingProvider, setEditingProvider] = useState<string | null>(null)
  const [refreshingProvider, setRefreshingProvider] = useState<string | null>(null)
  const [managingKeysProvider, setManagingKeysProvider] = useState<string | null>(null)
  const [formData, setFormData] = useState<Record<string, any>>({})
  const { toast } = useToast()

  const providers = data?.providers || []

  const handleToggle = async (provider: string, enabled: boolean) => {
    await updateProvider({
      variables: {
        input: {
          provider,
          enabled,
        },
      },
    })
    refetch()
  }

  const handleSave = async (provider: string) => {
    const connSettings = formData[provider]?.connectionSettings
    await updateProvider({
      variables: {
        input: {
          provider,
          enabled: true,
          baseUrl: formData[provider]?.baseUrl,
          region: formData[provider]?.region,
          regionPrefix: formData[provider]?.regionPrefix,
          resourceName: formData[provider]?.resourceName,
          apiVersion: formData[provider]?.apiVersion,
          accessKeyId: formData[provider]?.accessKeyId,
          secretAccessKey: formData[provider]?.secretAccessKey,
          connectionSettings: connSettings ? {
            maxConnections: connSettings.maxConnections,
            maxIdleConnections: connSettings.maxIdleConnections,
            idleTimeoutSec: connSettings.idleTimeoutSec,
            requestTimeoutSec: connSettings.requestTimeoutSec,
            enableHTTP2: connSettings.enableHTTP2,
            enableKeepAlive: connSettings.enableKeepAlive,
          } : undefined,
        },
      },
    })
    setEditingProvider(null)
    setFormData({})
    refetch()
  }

  const handleRefreshModels = async (provider: string) => {
    setRefreshingProvider(provider)
    try {
      const result = await refreshProviderModels({
        variables: {
          provider,
        },
      })

      if (result.data?.refreshProviderModels.success) {
        toast({
          title: 'Models Refreshed',
          description: result.data.refreshProviderModels.message,
        })
      }
    } catch (error: any) {
      toast({
        title: 'Error',
        description: error.message || 'Failed to refresh models',
        variant: 'destructive',
      })
    } finally {
      setRefreshingProvider(null)
    }
  }

  if (loading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">Providers</h1>
        <p className="text-muted-foreground">
          Configure LLM provider API keys and settings
        </p>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        {providers.map((provider: any) => {
          const info = PROVIDER_INFO[provider.provider] || {
            name: provider.provider,
            description: '',
          }
          const icon = providerIcons[provider.provider] || '🤖'
          const isEditing = editingProvider === provider.provider

          return (
            <Card key={provider.provider}>
              <CardHeader>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <span className="text-2xl">{icon}</span>
                    <div>
                      <CardTitle className="text-lg">{info.name}</CardTitle>
                      <CardDescription>{info.description}</CardDescription>
                    </div>
                  </div>
                  <Switch
                    checked={provider.enabled}
                    onCheckedChange={(enabled) => handleToggle(provider.provider, enabled)}
                    disabled={updating}
                  />
                </div>
              </CardHeader>
              <CardContent>
                {isEditing ? (
                  <div className="space-y-4">
                    {/* API Keys Section */}
                    <div className="space-y-2">
                      <label className="text-sm font-medium">API Keys</label>
                      <div className="flex items-center gap-2">
                        <Badge variant={provider.apiKeys?.length > 0 ? "default" : "secondary"}>
                          {provider.apiKeys?.length || 0} {provider.apiKeys?.length === 1 ? 'key' : 'keys'}
                        </Badge>
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => setManagingKeysProvider(provider.provider)}
                        >
                          <Key className="mr-2 h-4 w-4" />
                          Manage Keys
                        </Button>
                      </div>
                      <p className="text-xs text-muted-foreground">
                        Add multiple keys for automatic failover and load balancing
                      </p>
                    </div>

                    {!['BEDROCK', 'AZURE_OPENAI'].includes(provider.provider) && (
                      <div className="space-y-2">
                        <label className="text-sm font-medium">Model URL (Base URL)</label>
                        <Input
                          placeholder={
                            PROVIDER_INFO[provider.provider]?.defaultBaseUrl || 'Enter Base URL'
                          }
                          value={
                            formData[provider.provider]?.baseUrl !== undefined
                              ? formData[provider.provider]?.baseUrl
                              : provider.baseUrl || PROVIDER_INFO[provider.provider]?.defaultBaseUrl || ''
                          }
                          onChange={(e) =>
                            setFormData({
                              ...formData,
                              [provider.provider]: {
                                ...formData[provider.provider],
                                baseUrl: e.target.value,
                              },
                            })
                          }
                        />
                        <p className="text-xs text-muted-foreground">
                          Leave empty to use default: {PROVIDER_INFO[provider.provider]?.defaultBaseUrl}
                        </p>
                      </div>
                    )}

                    {['BEDROCK'].includes(provider.provider) && (
                      <>
                        <div className="space-y-2">
                          <label className="text-sm font-medium">Region</label>
                          <Input
                            placeholder="us-east-1"
                            value={formData[provider.provider]?.region || provider.region || ''}
                            onChange={(e) =>
                              setFormData({
                                ...formData,
                                [provider.provider]: {
                                  ...formData[provider.provider],
                                  region: e.target.value,
                                },
                              })
                            }
                          />
                          <p className="text-xs text-muted-foreground">
                            AWS region for Bedrock API (e.g., us-east-1, us-west-2).
                          </p>
                        </div>
                      </>
                    )}

                    {['AZURE_OPENAI'].includes(provider.provider) && (
                      <>
                        <div className="space-y-2">
                          <label className="text-sm font-medium">Resource Name</label>
                          <Input
                            placeholder="my-openai-resource"
                            value={formData[provider.provider]?.resourceName || provider.resourceName || ''}
                            onChange={(e) =>
                              setFormData({
                                ...formData,
                                [provider.provider]: {
                                  ...formData[provider.provider],
                                  resourceName: e.target.value,
                                },
                              })
                            }
                          />
                        </div>
                        <div className="space-y-2">
                          <label className="text-sm font-medium">API Version</label>
                          <Input
                            placeholder="2024-08-01-preview"
                            value={formData[provider.provider]?.apiVersion || provider.apiVersion || ''}
                            onChange={(e) =>
                              setFormData({
                                ...formData,
                                [provider.provider]: {
                                  ...formData[provider.provider],
                                  apiVersion: e.target.value,
                                },
                              })
                            }
                          />
                        </div>
                      </>
                    )}

                    {/* Connection Settings Section */}
                    <div className="border-t pt-4 mt-4">
                      <h4 className="text-sm font-medium mb-3">Connection Settings</h4>
                      <div className="grid grid-cols-2 gap-3">
                        <div className="space-y-1">
                          <label className="text-xs font-medium">Max Connections</label>
                          <Input
                            type="number"
                            min={1}
                            max={provider.planCeiling?.maxConnections || 50}
                            value={formData[provider.provider]?.connectionSettings?.maxConnections ?? provider.connectionSettings?.maxConnections ?? 10}
                            onChange={(e) =>
                              setFormData({
                                ...formData,
                                [provider.provider]: {
                                  ...formData[provider.provider],
                                  connectionSettings: {
                                    ...formData[provider.provider]?.connectionSettings,
                                    maxConnections: parseInt(e.target.value) || 10,
                                  },
                                },
                              })
                            }
                          />
                          <p className="text-xs text-muted-foreground">
                            Max: {provider.planCeiling?.maxConnections || 50} (plan limit)
                          </p>
                        </div>
                        <div className="space-y-1">
                          <label className="text-xs font-medium">Max Idle Connections</label>
                          <Input
                            type="number"
                            min={1}
                            max={provider.planCeiling?.maxIdleConnections || 25}
                            value={formData[provider.provider]?.connectionSettings?.maxIdleConnections ?? provider.connectionSettings?.maxIdleConnections ?? 5}
                            onChange={(e) =>
                              setFormData({
                                ...formData,
                                [provider.provider]: {
                                  ...formData[provider.provider],
                                  connectionSettings: {
                                    ...formData[provider.provider]?.connectionSettings,
                                    maxIdleConnections: parseInt(e.target.value) || 5,
                                  },
                                },
                              })
                            }
                          />
                          <p className="text-xs text-muted-foreground">
                            Max: {provider.planCeiling?.maxIdleConnections || 25} (plan limit)
                          </p>
                        </div>
                        <div className="space-y-1">
                          <label className="text-xs font-medium">Idle Timeout (sec)</label>
                          <Input
                            type="number"
                            min={10}
                            max={300}
                            value={formData[provider.provider]?.connectionSettings?.idleTimeoutSec ?? provider.connectionSettings?.idleTimeoutSec ?? 90}
                            onChange={(e) =>
                              setFormData({
                                ...formData,
                                [provider.provider]: {
                                  ...formData[provider.provider],
                                  connectionSettings: {
                                    ...formData[provider.provider]?.connectionSettings,
                                    idleTimeoutSec: parseInt(e.target.value) || 90,
                                  },
                                },
                              })
                            }
                          />
                        </div>
                        <div className="space-y-1">
                          <label className="text-xs font-medium">Request Timeout (sec)</label>
                          <Input
                            type="number"
                            min={30}
                            max={600}
                            value={formData[provider.provider]?.connectionSettings?.requestTimeoutSec ?? provider.connectionSettings?.requestTimeoutSec ?? 300}
                            onChange={(e) =>
                              setFormData({
                                ...formData,
                                [provider.provider]: {
                                  ...formData[provider.provider],
                                  connectionSettings: {
                                    ...formData[provider.provider]?.connectionSettings,
                                    requestTimeoutSec: parseInt(e.target.value) || 300,
                                  },
                                },
                              })
                            }
                          />
                        </div>
                      </div>
                      <div className="flex gap-6 mt-3">
                        <div className="flex items-center gap-2">
                          <Switch
                            checked={formData[provider.provider]?.connectionSettings?.enableHTTP2 ?? provider.connectionSettings?.enableHTTP2 ?? true}
                            onCheckedChange={(checked) =>
                              setFormData({
                                ...formData,
                                [provider.provider]: {
                                  ...formData[provider.provider],
                                  connectionSettings: {
                                    ...formData[provider.provider]?.connectionSettings,
                                    enableHTTP2: checked,
                                  },
                                },
                              })
                            }
                          />
                          <label className="text-xs font-medium">Enable HTTP/2</label>
                        </div>
                        <div className="flex items-center gap-2">
                          <Switch
                            checked={formData[provider.provider]?.connectionSettings?.enableKeepAlive ?? provider.connectionSettings?.enableKeepAlive ?? true}
                            onCheckedChange={(checked) =>
                              setFormData({
                                ...formData,
                                [provider.provider]: {
                                  ...formData[provider.provider],
                                  connectionSettings: {
                                    ...formData[provider.provider]?.connectionSettings,
                                    enableKeepAlive: checked,
                                  },
                                },
                              })
                            }
                          />
                          <label className="text-xs font-medium">Keep-Alive</label>
                        </div>
                      </div>
                    </div>

                    <div className="flex gap-2">
                      <Button size="sm" onClick={() => handleSave(provider.provider)} disabled={updating}>
                        <Check className="mr-2 h-4 w-4" />
                        Save
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => {
                          setEditingProvider(null)
                          setFormData({})
                        }}
                      >
                        <X className="mr-2 h-4 w-4" />
                        Cancel
                      </Button>
                    </div>
                  </div>
                ) : (
                  <div className="space-y-3">
                    {/* API Keys Section */}
                    <div className="space-y-2">
                      <div className="flex items-center justify-between">
                        <div className="text-sm font-medium">API Keys</div>
                        <Badge variant={provider.apiKeys?.length > 0 ? "default" : "secondary"}>
                          {provider.apiKeys?.length || 0} {provider.apiKeys?.length === 1 ? 'key' : 'keys'}
                        </Badge>
                      </div>

                      {provider.apiKeys && provider.apiKeys.length > 0 && (
                        <div className="space-y-1">
                          {provider.apiKeys.slice(0, 2).map((key: any) => (
                            <div key={key.id} className="flex items-center gap-2 text-xs text-muted-foreground">
                              <span className={cn(
                                "h-2 w-2 rounded-full",
                                key.enabled ? "bg-green-500" : "bg-gray-300"
                              )} />
                              <span>{key.name || `Key ${key.keyPrefix}`}</span>
                              <Badge variant="outline" className="text-[10px] h-4 px-1">
                                P{key.priority}
                              </Badge>
                              <Badge
                                variant={key.healthScore >= 0.8 ? "default" : key.healthScore >= 0.5 ? "secondary" : "destructive"}
                                className="text-[10px] h-4 px-1"
                              >
                                {Math.round(key.healthScore * 100)}%
                              </Badge>
                            </div>
                          ))}
                          {provider.apiKeys.length > 2 && (
                            <div className="text-xs text-muted-foreground">
                              +{provider.apiKeys.length - 2} more
                            </div>
                          )}
                        </div>
                      )}

                      <div className="flex gap-2">
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => setManagingKeysProvider(provider.provider)}
                        >
                          <Key className="mr-2 h-4 w-4" />
                          Manage Keys
                        </Button>
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => {
                            setEditingProvider(provider.provider)
                            if (!provider.baseUrl && PROVIDER_INFO[provider.provider]?.defaultBaseUrl) {
                              setFormData({
                                ...formData,
                                [provider.provider]: {
                                  ...formData[provider.provider],
                                  baseUrl: PROVIDER_INFO[provider.provider]?.defaultBaseUrl,
                                },
                              })
                            }
                          }}
                        >
                          Configure
                        </Button>
                      </div>
                    </div>

                    {/* Bedrock Streaming Mode */}
                    {provider.provider === 'BEDROCK' && provider.streamingMode && (
                      <div className="text-xs">
                        {provider.streamingMode === 'true_streaming' ? (
                          <span className="inline-flex items-center rounded-full bg-green-50 px-2 py-1 text-xs font-medium text-green-700 ring-1 ring-inset ring-green-600/20">
                            ⚡ True Streaming (~700ms)
                          </span>
                        ) : (
                          <span className="inline-flex items-center rounded-full bg-yellow-50 px-2 py-1 text-xs font-medium text-yellow-800 ring-1 ring-inset ring-yellow-600/20">
                            Simulated Streaming (~2000ms+)
                          </span>
                        )}
                      </div>
                    )}

                    {/* Refresh Models Button */}
                    {provider.enabled && provider.apiKeys && provider.apiKeys.length > 0 && (
                      <Button
                        size="sm"
                        variant="secondary"
                        className="w-full"
                        onClick={() => handleRefreshModels(provider.provider)}
                        disabled={refreshingProvider === provider.provider}
                      >
                        {refreshingProvider === provider.provider ? (
                          <>
                            <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                            Refreshing...
                          </>
                        ) : (
                          <>
                            <RefreshCw className="mr-2 h-4 w-4" />
                            Refresh Models
                          </>
                        )}
                      </Button>
                    )}
                  </div>
                )}
              </CardContent>
            </Card>
          )
        })}
      </div>

      {/* API Key Manager Dialog */}
      {managingKeysProvider && (
        <APIKeyManager
          provider={managingKeysProvider}
          providerName={PROVIDER_INFO[managingKeysProvider]?.name || managingKeysProvider}
          apiKeys={providers.find((p: any) => p.provider === managingKeysProvider)?.apiKeys || []}
          open={!!managingKeysProvider}
          onClose={() => setManagingKeysProvider(null)}
          onRefetch={refetch}
        />
      )}
    </div>
  )
}

