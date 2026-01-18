import { Link, useLocation } from 'react-router-dom'
import { cn } from '@/lib/utils'
import {
  LayoutDashboard,
  Key,
  Users,
  Shield,
  Settings,
  Server,
  Layers,
  FileText,
  DollarSign,
  Activity,
  Bell,
  LogOut,
  Radio,
  ClipboardList,
  Plug,
  Gauge,
  Bot,
} from 'lucide-react'

interface NavItem {
  title: string
  href: string
  icon: React.ElementType
}

interface NavSection {
  title: string
  items: NavItem[]
}

const dashboardNavigation: NavSection[] = [
  {
    title: 'Analytics',
    items: [
      { title: 'Dashboard', href: '/dashboard', icon: LayoutDashboard },
      { title: 'Agent Dashboard', href: '/dashboard/agent-dashboard', icon: Bot },
      { title: 'Advanced Metrics', href: '/dashboard/advanced-metrics', icon: Gauge },
      { title: 'Request Logs', href: '/dashboard/logs', icon: FileText },
      { title: 'Cost Analysis', href: '/dashboard/costs', icon: DollarSign },
      { title: 'Performance', href: '/dashboard/performance', icon: Activity },
    ],
  },
  {
    title: 'Configuration',
    items: [
      { title: 'Providers', href: '/dashboard/providers', icon: Server },
      { title: 'Models', href: '/dashboard/models', icon: Layers },
      { title: 'MCP Gateway', href: '/dashboard/mcp', icon: Plug },
      { title: 'Telemetry', href: '/dashboard/telemetry', icon: Radio },
    ],
  },
  {
    title: 'Access Control',
    items: [
      { title: 'Roles & Policies', href: '/dashboard/roles', icon: Shield },
      { title: 'API Keys', href: '/dashboard/api-keys', icon: Key },
      { title: 'Users', href: '/dashboard/users', icon: Users },
      { title: 'Audit Logs', href: '/dashboard/audit-logs', icon: ClipboardList },
    ],
  },
  {
    title: 'Settings',
    items: [
      { title: 'Budget Alerts', href: '/dashboard/alerts', icon: Bell },
      { title: 'Settings', href: '/dashboard/settings', icon: Settings },
    ],
  },
]

interface SidebarProps {
  type: 'admin' | 'tenant' | 'dashboard'
  tenantSlug?: string
  onLogout: () => void
}

export function Sidebar({ type, onLogout }: SidebarProps) {
  const location = useLocation()
  const navigation = dashboardNavigation

  return (
    <aside className="fixed left-0 top-0 z-40 h-screen w-64 border-r bg-card">
      <div className="flex h-full flex-col">
        {/* Logo */}
        <div className="flex h-16 items-center gap-2 border-b px-6">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary text-primary-foreground">
            <Layers className="h-5 w-5" />
          </div>
          <div className="flex-1 min-w-0">
            <h1 className="font-semibold">ModelGate</h1>
            <p className="text-xs text-muted-foreground">
              Open Source Edition
            </p>
          </div>
        </div>

        {/* Navigation */}
        <nav className="flex-1 space-y-1 overflow-y-auto p-4">
          {navigation.map((section) => (
            <div key={section.title} className="py-2">
              <h2 className="mb-2 px-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                {section.title}
              </h2>
              <div className="space-y-1">
                {section.items.map((item) => {
                  const isActive = location.pathname === item.href || 
                    (item.href === '/dashboard' && location.pathname === '/dashboard')
                  
                  return (
                    <Link
                      key={item.href}
                      to={item.href}
                      className={cn(
                        'flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors',
                        isActive
                          ? 'bg-primary/10 text-primary'
                          : 'text-muted-foreground hover:bg-muted hover:text-foreground'
                      )}
                    >
                      <item.icon className="h-4 w-4" />
                      {item.title}
                    </Link>
                  )
                })}
              </div>
            </div>
          ))}
        </nav>

        {/* Footer */}
        <div className="border-t p-4">
          <button
            onClick={onLogout}
            className="flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
          >
            <LogOut className="h-4 w-4" />
            Logout
          </button>
        </div>
      </div>
    </aside>
  )
}
