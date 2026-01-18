import { useState } from 'react'
import { useQuery } from '@apollo/client'
import { Loader2, Search, Filter, Check, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { GET_AVAILABLE_MODELS } from '@/graphql/operations'
import { providerIcons } from '@/lib/utils'

const PROVIDER_COLORS: Record<string, string> = {
  OPENAI: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200',
  ANTHROPIC: 'bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200',
  GEMINI: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200',
  BEDROCK: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200',
  AZURE_OPENAI: 'bg-cyan-100 text-cyan-800 dark:bg-cyan-900 dark:text-cyan-200',
  OLLAMA: 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200',
  GROQ: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200',
  MISTRAL: 'bg-indigo-100 text-indigo-800 dark:bg-indigo-900 dark:text-indigo-200',
}

const PROVIDER_NAMES: Record<string, string> = {
  OPENAI: 'OpenAI',
  ANTHROPIC: 'Anthropic',
  GEMINI: 'Google Gemini',
  BEDROCK: 'AWS Bedrock',
  AZURE_OPENAI: 'Azure OpenAI',
  OLLAMA: 'Ollama',
  GROQ: 'Groq',
  MISTRAL: 'Mistral AI',
  TOGETHER: 'Together AI',
  COHERE: 'Cohere',
}

export function ModelsPage() {
  const { data, loading, error } = useQuery(GET_AVAILABLE_MODELS, {
    fetchPolicy: 'network-only',
  })
  const [searchQuery, setSearchQuery] = useState('')
  const [selectedProvider, setSelectedProvider] = useState<string>('all')
  const [showOnlyToolSupport, setShowOnlyToolSupport] = useState(false)

  const models = data?.availableModels || []

  // Get unique providers
  const providers = Array.from(new Set(models.map((m: any) => m.provider)))
    .sort()

  // Filter models
  const filteredModels = models.filter((model: any) => {
    const matchesSearch =
      model.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      model.id.toLowerCase().includes(searchQuery.toLowerCase())

    const matchesProvider = selectedProvider === 'all' || model.provider === selectedProvider

    const matchesToolSupport = !showOnlyToolSupport || model.supportsTools

    return matchesSearch && matchesProvider && matchesToolSupport
  })

  // Sort by provider, then by name
  const sortedModels = [...filteredModels].sort((a, b) => {
    if (a.provider !== b.provider) {
      return a.provider.localeCompare(b.provider)
    }
    return a.name.localeCompare(b.name)
  })

  // Group by provider for stats
  const modelsByProvider = models.reduce((acc: any, model: any) => {
    acc[model.provider] = (acc[model.provider] || 0) + 1
    return acc
  }, {})

  const formatCost = (cost: number) => {
    if (cost === 0) return 'Free'
    if (cost < 1) return `$${cost.toFixed(3)}`
    return `$${cost.toFixed(2)}`
  }

  const formatContextLimit = (limit: number) => {
    if (limit >= 1000000) {
      return `${(limit / 1000000).toFixed(1)}M`
    }
    if (limit >= 1000) {
      return `${(limit / 1000).toFixed(0)}K`
    }
    return limit.toString()
  }

  if (loading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (error) {
    return (
      <div className="space-y-4">
        <h1 className="text-3xl font-bold">Models</h1>
        <Card>
          <CardContent className="pt-6">
            <p className="text-destructive">Error loading models: {error.message}</p>
          </CardContent>
        </Card>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold">Models</h1>
        <p className="text-muted-foreground">
          Available models from configured providers
        </p>
      </div>

      {/* Stats Cards */}
      <div className="grid gap-4 md:grid-cols-4">
        <Card>
          <CardHeader className="pb-3">
            <CardDescription>Total Models</CardDescription>
            <CardTitle className="text-3xl">{models.length}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-3">
            <CardDescription>Providers</CardDescription>
            <CardTitle className="text-3xl">{providers.length}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-3">
            <CardDescription>Tool Support</CardDescription>
            <CardTitle className="text-3xl">
              {models.filter((m: any) => m.supportsTools).length}
            </CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-3">
            <CardDescription>Free Models</CardDescription>
            <CardTitle className="text-3xl">
              {models.filter((m: any) => m.inputCostPer1M === 0).length}
            </CardTitle>
          </CardHeader>
        </Card>
      </div>

      {/* Filters */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex flex-col gap-4 md:flex-row md:items-center">
            <div className="flex-1">
              <div className="relative">
                <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
                <Input
                  placeholder="Search models..."
                  className="pl-8"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                />
              </div>
            </div>
            <Select value={selectedProvider} onValueChange={setSelectedProvider}>
              <SelectTrigger className="w-[200px]">
                <SelectValue placeholder="All Providers" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All Providers</SelectItem>
                {(providers as string[]).map((provider: string) => (
                  <SelectItem key={provider} value={provider}>
                    {providerIcons[provider] || '🤖'} {PROVIDER_NAMES[provider] || provider}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button
              variant={showOnlyToolSupport ? 'default' : 'outline'}
              onClick={() => setShowOnlyToolSupport(!showOnlyToolSupport)}
              size="sm"
            >
              <Filter className="mr-2 h-4 w-4" />
              Tool Support Only
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Models Table */}
      <Card>
        <CardHeader>
          <CardTitle>
            {sortedModels.length} {sortedModels.length === 1 ? 'Model' : 'Models'}
          </CardTitle>
          <CardDescription>
            {selectedProvider === 'all'
              ? 'All available models from configured providers'
              : `Models from ${PROVIDER_NAMES[selectedProvider] || selectedProvider}`}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {sortedModels.length === 0 ? (
            <div className="py-8 text-center text-muted-foreground">
              No models found matching your filters
            </div>
          ) : (
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Model</TableHead>
                    <TableHead>Provider</TableHead>
                    <TableHead className="text-center">Context</TableHead>
                    <TableHead className="text-center">Tools</TableHead>
                    <TableHead className="text-right">Input Cost</TableHead>
                    <TableHead className="text-right">Output Cost</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {sortedModels.map((model: any) => (
                    <TableRow key={model.id}>
                      <TableCell>
                        <div>
                          <div className="font-medium">{model.name}</div>
                          <div className="text-xs text-muted-foreground">{model.id}</div>
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge className={PROVIDER_COLORS[model.provider] || ''} variant="secondary">
                          {providerIcons[model.provider] || '🤖'} {PROVIDER_NAMES[model.provider] || model.provider}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-center">
                        <span className="text-sm font-mono">
                          {formatContextLimit(model.contextLimit)}
                        </span>
                      </TableCell>
                      <TableCell className="text-center">
                        {model.supportsTools ? (
                          <Check className="inline h-4 w-4 text-green-600" />
                        ) : (
                          <X className="inline h-4 w-4 text-muted-foreground" />
                        )}
                      </TableCell>
                      <TableCell className="text-right font-mono text-sm">
                        {formatCost(model.inputCostPer1M)}
                        {model.inputCostPer1M > 0 && <span className="text-xs text-muted-foreground">/1M</span>}
                      </TableCell>
                      <TableCell className="text-right font-mono text-sm">
                        {formatCost(model.outputCostPer1M)}
                        {model.outputCostPer1M > 0 && <span className="text-xs text-muted-foreground">/1M</span>}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Provider Breakdown */}
      <Card>
        <CardHeader>
          <CardTitle>Models by Provider</CardTitle>
          <CardDescription>Distribution of models across providers</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid gap-3 md:grid-cols-3">
            {Object.entries(modelsByProvider)
              .sort(([, a]: any, [, b]: any) => (b as number) - (a as number))
              .map(([provider, count]: [string, unknown]) => (
                <div
                  key={provider}
                  className="flex items-center justify-between rounded-lg border p-3"
                >
                  <div className="flex items-center gap-2">
                    <span className="text-xl">{providerIcons[provider] || '🤖'}</span>
                    <span className="font-medium">{PROVIDER_NAMES[provider] || provider}</span>
                  </div>
                  <Badge variant="secondary">{count as number}</Badge>
                </div>
              ))}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
