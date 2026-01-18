import { Link } from 'react-router-dom'
import {
  Shield, Key, Zap, BarChart3, Globe, ArrowRight, CheckCircle2,
  Code2, Layers, Search, Terminal, Activity, Cpu, Network,
  Lock, Plug, Database, Bot, Sparkles, Crown
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'

export function HomePage() {
  // Key Differentiators - What makes ModelGate unique
  const keyDifferentiators = [
    {
      icon: Shield,
      title: 'RBAC-First Architecture',
      description: 'Built with Role-Based Access Control at its core, not bolted on. Every request flows through a policy engine enforcing 7 policy types: prompt security, tool access, rate limits, model restrictions, and more.',
      iconColor: 'text-violet-600',
      iconBg: 'bg-violet-50',
      highlight: true,
    },
    {
      icon: Globe,
      title: 'Multi-Provider with Local Models',
      description: 'Native support for Ollama and local model servers alongside OpenAI, Anthropic, Gemini, Bedrock, Azure, and more. Run sensitive data on-prem, complex tasks on frontier models—one unified API.',
      iconColor: 'text-pink-600',
      iconBg: 'bg-pink-50',
      highlight: true,
    },
    {
      icon: Plug,
      title: 'Dynamic Tool Discovery (MCP)',
      description: 'Model Context Protocol integration for runtime tool registration. AI agents discover new capabilities without code changes—tools are indexed and permissioned automatically.',
      iconColor: 'text-emerald-600',
      iconBg: 'bg-emerald-50',
      highlight: true,
    },
    {
      icon: Search,
      title: 'Semantic Tool Search',
      description: 'Find tools using natural language, not exact names. Ask for "something to send emails" and discover send_email, compose_message, or notify_user—powered by vector embeddings.',
      iconColor: 'text-cyan-600',
      iconBg: 'bg-cyan-50',
      highlight: true,
    },
    {
      icon: Database,
      title: 'Semantic Response Caching',
      description: 'Intelligent caching that understands meaning, not just exact matches. Similar prompts hit the cache even when worded differently—dramatically reducing API costs.',
      iconColor: 'text-amber-600',
      iconBg: 'bg-amber-50',
      highlight: true,
    },
  ]

  const keyFeatures = [
    {
      icon: Bot,
      title: 'Agent Dashboard',
      description: 'Real-time visibility into AI agent activities, token usage, tool calls, and policy compliance across your applications.',
      iconColor: 'text-blue-600',
      iconBg: 'bg-blue-50',
    },
    {
      icon: Activity,
      title: 'Comprehensive Observability',
      description: 'Real-time metrics, request tracing, cost analytics, and audit logs. Full visibility into your AI infrastructure with Prometheus integration.',
      iconColor: 'text-indigo-600',
      iconBg: 'bg-indigo-50',
    },
    {
      icon: Key,
      title: 'API Key Management',
      description: 'Issue and manage API keys with fine-grained permissions. Assign roles, set expiration dates, and track usage per key.',
      iconColor: 'text-orange-600',
      iconBg: 'bg-orange-50',
    },
  ]

  const capabilities = [
    {
      icon: Shield,
      title: 'Prompt Security',
      description: 'Injection detection, PII auto-redaction, content filtering, and comprehensive audit logs with detailed request tracking',
    },
    {
      icon: Zap,
      title: 'Dual API Support',
      description: 'Native gRPC and OpenAI-compatible HTTP APIs with streaming support for real-time responses',
    },
    {
      icon: BarChart3,
      title: 'Real-time Analytics',
      description: 'Track token usage, costs, performance metrics, and budget alerts in real-time',
    },
    {
      icon: Code2,
      title: 'React Admin UI',
      description: 'Visual policy editor, MCP management, request logs, cost analysis - no YAML editing required',
    },
    {
      icon: Lock,
      title: 'RBAC & Policies',
      description: 'Role-based access control with comprehensive policy types and fine-grained permissions',
    },
    {
      icon: Plug,
      title: 'MCP Integration',
      description: 'Connect to Model Context Protocol servers for dynamic tool discovery and execution',
    },
  ]

  const enterpriseFeatures = [
    {
      title: 'Intelligent Routing',
      description: 'Route requests to optimal models based on cost, latency, or capability requirements',
      icon: Network,
    },
    {
      title: 'Resilience & Failover',
      description: 'Automatic retries, circuit breakers, and fallback chains for high availability',
      icon: Activity,
    },
    {
      title: 'Multi-Tenancy',
      description: 'Complete tenant isolation with separate databases and configurations',
      icon: Database,
    },
    {
      title: 'Advanced Budget Controls',
      description: 'Department-level budgets, alerts, and automatic enforcement',
      icon: BarChart3,
    },
  ]

  const providers = [
    { name: 'Ollama', featured: true },
    { name: 'OpenAI', featured: false },
    { name: 'Anthropic', featured: false },
    { name: 'Gemini', featured: false },
    { name: 'Bedrock', featured: false },
    { name: 'Azure', featured: false },
    { name: 'Groq', featured: false },
    { name: 'Mistral', featured: false },
    { name: 'Together AI', featured: false },
    { name: 'Cohere', featured: false },
  ]

  return (
    <div className="min-h-screen hero-gradient-light">
      {/* Subtle gradient orbs */}
      <div className="gradient-orb-light gradient-orb-light-1" />
      <div className="gradient-orb-light gradient-orb-light-2" />
      
      {/* Navigation */}
      <nav className="fixed top-0 left-0 right-0 z-50 bg-white/80 backdrop-blur-xl border-b border-slate-200/60">
        <div className="max-w-7xl mx-auto px-6 py-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3 animate-fade-in-up">
              <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-emerald-500 to-cyan-500 flex items-center justify-center shadow-lg shadow-emerald-500/20">
                <Shield className="h-5 w-5 text-white" />
              </div>
              <div>
                <h1 className="text-lg font-bold text-slate-900 tracking-tight">ModelGate</h1>
                <p className="text-xs text-slate-500 font-medium">Open Source Edition</p>
              </div>
            </div>
            <div className="flex items-center gap-2 animate-fade-in-up delay-100">
              <a
                href="https://github.com/mazori/modelgate"
                target="_blank"
                rel="noopener noreferrer"
              >
                <Button variant="ghost" size="sm" className="text-slate-600 hover:text-slate-900 hover:bg-slate-100">
                  <Terminal className="h-4 w-4 mr-2" />
                  Docs
                </Button>
              </a>
              <Link to="/dashboard">
                <Button size="sm" className="bg-gradient-to-r from-emerald-500 to-cyan-500 text-white border-0 hover:opacity-90 shadow-md shadow-emerald-500/20">
                  <Key className="h-4 w-4 mr-2" />
                  Dashboard
                </Button>
              </Link>
            </div>
          </div>
        </div>
      </nav>

      {/* Hero Section */}
      <section className="relative pt-32 pb-20 px-6 mesh-gradient-light">
        <div className="max-w-6xl mx-auto relative z-10">
          <div className="max-w-4xl">
            {/* Badge */}
            <div className="animate-fade-in-up inline-flex items-center gap-2 px-4 py-2 rounded-full bg-emerald-50 border border-emerald-200 mb-8">
              <Sparkles className="h-4 w-4 text-emerald-600" />
              <span className="text-sm font-medium text-emerald-700">Policy-Driven LLM & MCP Gateway</span>
              <Badge variant="outline" className="ml-2 text-xs">Open Source</Badge>
            </div>

            {/* Main Headline */}
            <h1 className="animate-fade-in-up delay-100 text-4xl md:text-5xl lg:text-6xl font-display font-bold text-slate-900 mb-6 leading-[1.1] tracking-tight">
              Secure, Observable AI <span className="gradient-text">Gateway</span>
            </h1>

            <p className="animate-fade-in-up delay-200 text-xl text-slate-600 mb-10 leading-relaxed max-w-2xl">
              Self-hosted LLM gateway with intelligent tool orchestration, policy-driven governance, 
              and comprehensive observability. <span className="text-slate-800 font-medium">Free and open source.</span>
            </p>

            {/* CTA Buttons */}
            <div className="animate-fade-in-up delay-300 flex flex-wrap items-center gap-4 mb-16">
              <Link to="/dashboard">
                <Button size="lg" className="bg-gradient-to-r from-emerald-500 to-cyan-500 text-white border-0 hover:opacity-90 shadow-lg shadow-emerald-500/25 h-12 px-8 text-base font-semibold">
                  Open Dashboard
                  <ArrowRight className="ml-2 h-5 w-5" />
                </Button>
              </Link>
              <a href="https://github.com/mazori/modelgate" target="_blank" rel="noopener noreferrer">
                <Button size="lg" variant="outline" className="border-slate-300 text-slate-700 hover:bg-slate-50 hover:border-slate-400 h-12 px-8 text-base font-semibold">
                  <Terminal className="mr-2 h-5 w-5" />
                  View on GitHub
                </Button>
              </a>
            </div>

            {/* Quick Stats */}
            <div className="animate-fade-in-up delay-400 grid grid-cols-2 md:grid-cols-4 gap-4">
              {[
                { value: '10+', label: 'Providers', icon: Network },
                { value: '8', label: 'Policy Types', icon: Shield },
                { value: 'MCP', label: 'Native Support', icon: Plug },
                { value: '100%', label: 'Open Source', icon: Code2 },
              ].map((stat, index) => {
                const Icon = stat.icon
                return (
                  <div 
                    key={stat.label} 
                    className="stat-card-light p-5"
                    style={{ animationDelay: `${400 + index * 100}ms` }}
                  >
                    <div className="flex items-center gap-3 mb-3">
                      <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-emerald-50 to-cyan-50 border border-emerald-100 flex items-center justify-center">
                        <Icon className="h-4 w-4 text-emerald-600" />
                      </div>
                    </div>
                    <div className="text-3xl font-bold gradient-text mb-1">{stat.value}</div>
                    <div className="text-sm font-medium text-slate-600">{stat.label}</div>
                  </div>
                )
              })}
            </div>
          </div>
        </div>
      </section>

      {/* Key Differentiators Section */}
      <section className="relative py-24 px-6 bg-gradient-to-b from-emerald-50/50 to-white">
        <div className="section-divider-light absolute top-0 left-0 right-0" />
        <div className="max-w-6xl mx-auto relative z-10">
          <div className="mb-16 animate-fade-in-up text-center">
            <div className="inline-flex items-center gap-2 px-3 py-1.5 rounded-full bg-emerald-50 border border-emerald-200 mb-4">
              <Sparkles className="h-3.5 w-3.5 text-emerald-600" />
              <span className="text-xs font-medium text-emerald-700 uppercase tracking-wider">Why ModelGate</span>
            </div>
            <h2 className="text-3xl md:text-4xl lg:text-5xl font-display font-bold text-slate-900 mb-4">
              What Sets Us <span className="gradient-text">Apart</span>
            </h2>
            <p className="text-lg text-slate-600 max-w-2xl mx-auto">
              Purpose-built features that make ModelGate the gateway of choice for production AI deployments.
            </p>
          </div>

          <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-6">
            {keyDifferentiators.map((feature, index) => {
              const Icon = feature.icon
              return (
                <div 
                  key={feature.title} 
                  className={`card-light rounded-2xl p-6 animate-fade-in-up relative overflow-hidden ${
                    index === 0 ? 'md:col-span-2 lg:col-span-1' : ''
                  }`}
                  style={{ animationDelay: `${index * 100}ms` }}
                >
                  {/* Subtle gradient accent */}
                  <div className="absolute top-0 right-0 w-32 h-32 bg-gradient-to-br from-emerald-100/50 to-transparent rounded-bl-full" />
                  
                  <div className={`relative w-14 h-14 rounded-xl ${feature.iconBg} border border-current/10 flex items-center justify-center mb-5`}>
                    <Icon className={`h-7 w-7 ${feature.iconColor}`} />
                  </div>

                  <h3 className="relative text-xl font-semibold text-slate-900 mb-3">
                    {feature.title}
                  </h3>

                  <p className="relative text-slate-600 leading-relaxed">
                    {feature.description}
                  </p>
                </div>
              )
            })}
          </div>
        </div>
      </section>

      {/* Additional Features Section */}
      <section className="relative py-24 px-6 bg-slate-50/50">
        <div className="section-divider-light absolute top-0 left-0 right-0" />
        <div className="max-w-6xl mx-auto relative z-10">
          <div className="mb-16 animate-fade-in-up">
            <div className="inline-flex items-center gap-2 px-3 py-1.5 rounded-full bg-violet-50 border border-violet-200 mb-4">
              <Cpu className="h-3.5 w-3.5 text-violet-600" />
              <span className="text-xs font-medium text-violet-700 uppercase tracking-wider">More Features</span>
            </div>
            <h2 className="text-3xl md:text-4xl lg:text-5xl font-display font-bold text-slate-900 mb-4">
              Everything You Need for <span className="gradient-text-alt">AI Governance</span>
            </h2>
            <p className="text-lg text-slate-600 max-w-2xl">
              Comprehensive capabilities for secure, observable, and cost-effective AI operations.
            </p>
          </div>

          <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-6">
            {keyFeatures.map((feature, index) => {
              const Icon = feature.icon
              return (
                <div 
                  key={feature.title} 
                  className="card-light rounded-2xl p-6 animate-fade-in-up"
                  style={{ animationDelay: `${index * 100}ms` }}
                >
                  <div className={`w-12 h-12 rounded-xl ${feature.iconBg} border border-current/10 flex items-center justify-center mb-5`}>
                    <Icon className={`h-6 w-6 ${feature.iconColor}`} />
                  </div>

                  <h3 className="text-xl font-semibold text-slate-900 mb-3">
                    {feature.title}
                  </h3>

                  <p className="text-slate-600 leading-relaxed">
                    {feature.description}
                  </p>
                </div>
              )
            })}
          </div>
        </div>
      </section>

      {/* Enterprise Features Section */}
      <section className="relative py-24 px-6 bg-white">
        <div className="section-divider-light absolute top-0 left-0 right-0" />
        <div className="max-w-6xl mx-auto relative z-10">
          <div className="text-center mb-16 animate-fade-in-up">
            <div className="inline-flex items-center gap-2 px-3 py-1.5 rounded-full bg-amber-50 border border-amber-200 mb-4">
              <Crown className="h-3.5 w-3.5 text-amber-600" />
              <span className="text-xs font-medium text-amber-700 uppercase tracking-wider">Enterprise Edition</span>
            </div>
            <h2 className="text-4xl md:text-5xl font-display font-bold text-slate-900 mb-4">
              Need More Power?
            </h2>
            <p className="text-lg text-slate-600 max-w-2xl mx-auto">
              Enterprise features for organizations requiring advanced routing, resilience, and multi-tenancy.
            </p>
          </div>

          <div className="grid md:grid-cols-2 lg:grid-cols-4 gap-6 animate-fade-in-up delay-200">
            {enterpriseFeatures.map((feature) => {
              const Icon = feature.icon
              return (
                <div 
                  key={feature.title} 
                  className="relative card-light rounded-2xl p-6 border-2 border-amber-100"
                >
                  <Badge className="absolute top-4 right-4 bg-amber-100 text-amber-700 border-amber-200">
                    Enterprise
                  </Badge>
                  <div className="w-12 h-12 rounded-xl bg-amber-50 border border-amber-100 flex items-center justify-center mb-5">
                    <Icon className="h-6 w-6 text-amber-600" />
                  </div>

                  <h3 className="text-lg font-semibold text-slate-900 mb-2">
                    {feature.title}
                  </h3>

                  <p className="text-sm text-slate-600 leading-relaxed">
                    {feature.description}
                  </p>
                </div>
              )
            })}
          </div>

          <div className="text-center mt-12 animate-fade-in-up delay-300">
            <a href="mailto:enterprise@modelgate.io">
              <Button size="lg" variant="outline" className="border-amber-300 text-amber-700 hover:bg-amber-50 hover:border-amber-400">
                <Crown className="mr-2 h-5 w-5" />
                Contact for Enterprise
              </Button>
            </a>
          </div>
        </div>
      </section>

      {/* Additional Capabilities */}
      <section className="relative py-24 px-6 bg-slate-50/50">
        <div className="section-divider-light absolute top-0 left-0 right-0" />
        <div className="max-w-6xl mx-auto relative z-10">
          <div className="mb-16 animate-fade-in-up">
            <div className="inline-flex items-center gap-2 px-3 py-1.5 rounded-full bg-cyan-50 border border-cyan-200 mb-4">
              <Layers className="h-3.5 w-3.5 text-cyan-600" />
              <span className="text-xs font-medium text-cyan-700 uppercase tracking-wider">Capabilities</span>
            </div>
            <h2 className="text-4xl md:text-5xl font-display font-bold text-slate-900 mb-4">
              Built for <span className="gradient-text">Production</span>
            </h2>
            <p className="text-lg text-slate-600 max-w-2xl">
              Secure, observable, and cost-effective access to AI models for your organization.
            </p>
          </div>

          <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-6">
            {capabilities.map((capability, index) => {
              const Icon = capability.icon
              return (
                <div 
                  key={capability.title} 
                  className="card-light rounded-2xl p-6 animate-fade-in-up"
                  style={{ animationDelay: `${index * 100}ms` }}
                >
                  <div className="feature-icon-light w-12 h-12 rounded-xl flex items-center justify-center mb-5">
                    <Icon className="h-6 w-6 text-emerald-600" />
                  </div>
                  <h3 className="text-lg font-semibold text-slate-900 mb-2">
                    {capability.title}
                  </h3>
                  <p className="text-sm text-slate-600 leading-relaxed">
                    {capability.description}
                  </p>
                </div>
              )
            })}
          </div>
        </div>
      </section>

      {/* Integration Example */}
      <section className="relative py-24 px-6 bg-white">
        <div className="section-divider-light absolute top-0 left-0 right-0" />
        <div className="max-w-6xl mx-auto relative z-10">
          <div className="grid lg:grid-cols-2 gap-12 items-center">
            <div className="animate-slide-in-left">
              <div className="inline-flex items-center gap-2 px-3 py-1.5 rounded-full bg-pink-50 border border-pink-200 mb-4">
                <Code2 className="h-3.5 w-3.5 text-pink-600" />
                <span className="text-xs font-medium text-pink-700 uppercase tracking-wider">Integration</span>
              </div>
              <h2 className="text-4xl md:text-5xl font-display font-bold text-slate-900 mb-6">
                OpenAI-Compatible
                <br />
                <span className="gradient-text-alt">Drop-in Replacement</span>
              </h2>
              <p className="text-lg text-slate-600 mb-8 leading-relaxed">
                ModelGate seamlessly integrates with your existing codebase. Just change the base URL 
                and API key — no code changes required.
              </p>
              <div className="space-y-4">
                {[
                  'Same request/response format as OpenAI',
                  'Streaming support with SSE',
                  'Function calling & tool use',
                  'Native gRPC API for high performance',
                  'GraphQL admin API for management',
                ].map((item, index) => (
                  <div 
                    key={item} 
                    className="flex items-start gap-3 animate-fade-in-up"
                    style={{ animationDelay: `${index * 100}ms` }}
                  >
                    <div className="w-5 h-5 rounded-full bg-gradient-to-br from-emerald-500 to-cyan-500 flex items-center justify-center flex-shrink-0 mt-0.5">
                      <CheckCircle2 className="h-3 w-3 text-white" />
                    </div>
                    <span className="text-slate-700">{item}</span>
                  </div>
                ))}
              </div>
            </div>

            <div className="animate-slide-in-right delay-200">
              <div className="code-window-light overflow-hidden">
                <div className="code-header-light px-4 py-3 flex items-center gap-3">
                  <div className="flex gap-2">
                    <div className="w-3 h-3 rounded-full bg-red-500/80"></div>
                    <div className="w-3 h-3 rounded-full bg-yellow-500/80"></div>
                    <div className="w-3 h-3 rounded-full bg-green-500/80"></div>
                  </div>
                  <span className="text-slate-400 text-sm font-mono ml-2">api_example.sh</span>
                </div>
                <div className="p-6 font-mono text-sm overflow-x-auto bg-slate-900">
                  <pre className="text-emerald-400"># Before (OpenAI)</pre>
                  <pre className="text-slate-400 mt-2">curl https://api.openai.com/v1/chat/completions \</pre>
                  <pre className="text-slate-400">  -H "Authorization: Bearer sk-..." \</pre>
                  <pre className="text-slate-500">  ...</pre>
                  <br />
                  <pre className="text-emerald-400"># After (ModelGate) - Same API!</pre>
                  <pre className="text-cyan-400 mt-2">curl http://localhost:8080/v1/chat/completions \</pre>
                  <pre className="text-cyan-400">  -H "Authorization: Bearer mg-..." \</pre>
                  <pre className="text-slate-400">  -H "Content-Type: application/json" \</pre>
                  <pre className="text-slate-400">  -d '&#123;"model": "gpt-4o",</pre>
                  <pre className="text-slate-400">      "messages": [&#123;"role": "user",</pre>
                  <pre className="text-slate-400">                    "content": "Hello!"&#125;]&#125;'</pre>
                </div>
                <div className="p-6 bg-slate-50 border-t border-slate-200">
                  <p className="text-sm text-slate-600 mb-4 font-medium">10 Supported Providers:</p>
                  <div className="flex flex-wrap gap-2">
                    {providers.map((provider) => (
                      <span
                        key={provider.name}
                        className={`px-3 py-1.5 text-xs rounded-lg font-medium transition-all duration-200 ${
                          provider.featured
                            ? 'provider-badge-featured-light text-emerald-700'
                            : 'provider-badge-light text-slate-600 hover:text-slate-800'
                        }`}
                      >
                        {provider.name}
                      </span>
                    ))}
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* CTA Section */}
      <section className="relative py-24 px-6 cta-section-light">
        <div className="max-w-4xl mx-auto text-center relative z-10 animate-fade-in-up">
          <h2 className="text-4xl md:text-5xl font-display font-bold text-white mb-6">
            Ready to Get Started?
          </h2>
          
          <p className="text-xl text-white/90 mb-10 leading-relaxed max-w-2xl mx-auto">
            Deploy ModelGate in minutes. Self-hosted, secure, and completely free.
          </p>
          
          <div className="flex flex-wrap items-center justify-center gap-4 mb-8">
            <Link to="/dashboard">
              <Button size="lg" className="bg-white text-emerald-600 hover:bg-slate-50 shadow-lg h-14 px-10 text-lg font-semibold">
                Open Dashboard
                <ArrowRight className="ml-2 h-5 w-5" />
              </Button>
            </Link>
          </div>
          
          <p className="text-sm text-white/70">
            Self-hosted • Open Source • No vendor lock-in
          </p>
        </div>
      </section>

      {/* Footer */}
      <footer className="relative border-t border-slate-200 py-16 px-6 bg-white">
        <div className="max-w-6xl mx-auto">
          <div className="grid md:grid-cols-4 gap-12 mb-12">
            <div className="md:col-span-2">
              <div className="flex items-center gap-3 mb-6">
                <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-emerald-500 to-cyan-500 flex items-center justify-center">
                  <Shield className="h-5 w-5 text-white" />
                </div>
                <div>
                  <span className="text-xl font-bold text-slate-900">ModelGate</span>
                  <span className="ml-2 text-xs bg-emerald-100 text-emerald-700 px-2 py-0.5 rounded-full">Open Source</span>
                </div>
              </div>
              <p className="text-slate-600 mb-6 max-w-sm leading-relaxed">
                Self-hosted LLM gateway with policy-driven governance, MCP support, 
                and comprehensive observability.
              </p>
              <div className="flex gap-4">
                <a href="https://github.com/mazori/modelgate" target="_blank" rel="noopener noreferrer" className="w-10 h-10 rounded-lg bg-slate-100 border border-slate-200 flex items-center justify-center text-slate-500 hover:text-slate-700 hover:border-slate-300 transition-colors">
                  <Terminal className="h-5 w-5" />
                </a>
                <a href="#" className="w-10 h-10 rounded-lg bg-slate-100 border border-slate-200 flex items-center justify-center text-slate-500 hover:text-slate-700 hover:border-slate-300 transition-colors">
                  <Code2 className="h-5 w-5" />
                </a>
              </div>
            </div>

            <div>
              <h3 className="font-semibold text-slate-900 mb-4">Resources</h3>
              <ul className="space-y-3">
                {['Documentation', 'API Reference', 'Examples', 'Changelog'].map((item) => (
                  <li key={item}>
                    <a href="#" className="text-slate-600 hover:text-emerald-600 transition-colors text-sm">
                      {item}
                    </a>
                  </li>
                ))}
              </ul>
            </div>

            <div>
              <h3 className="font-semibold text-slate-900 mb-4">Community</h3>
              <ul className="space-y-3">
                {['GitHub', 'Discord', 'Twitter', 'Blog'].map((item) => (
                  <li key={item}>
                    <a href="#" className="text-slate-600 hover:text-emerald-600 transition-colors text-sm">
                      {item}
                    </a>
                  </li>
                ))}
              </ul>
            </div>
          </div>

          <div className="section-divider-light mb-8" />

          <div className="flex flex-col md:flex-row items-center justify-between gap-4">
            <p className="text-sm text-slate-500">
              © 2024 ModelGate. MIT License.
            </p>
            <div className="flex gap-6 text-sm text-slate-500">
              <a href="#" className="hover:text-emerald-600 transition-colors">GitHub</a>
              <a href="#" className="hover:text-emerald-600 transition-colors">License</a>
              <a href="#" className="hover:text-emerald-600 transition-colors">Contributing</a>
            </div>
          </div>
        </div>
      </footer>
    </div>
  )
}
