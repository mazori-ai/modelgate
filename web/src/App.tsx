import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { ApolloProvider } from '@apollo/client'
import { client } from './graphql/client'
import { Layout } from './components/layout/Layout'
import { LoginPage } from './pages/Login'
import { HomePage } from './pages/Home'
import { Toaster } from './components/ui/toaster'

// Dashboard pages
import { DashboardPage } from './pages/tenant/Dashboard'
import { RolesPage } from './pages/tenant/Roles'
import { APIKeysPage } from './pages/tenant/APIKeys'
import { ProvidersPage } from './pages/tenant/Providers'
import { ModelsPage } from './pages/tenant/Models'
import UsersPage from './pages/tenant/Users'
import SettingsPage from './pages/tenant/Settings'
import TelemetryPage from './pages/tenant/Telemetry'
import RequestLogsPage from './pages/tenant/RequestLogs'
import CostAnalysisPage from './pages/tenant/CostAnalysis'
import AuditLogsPage from './pages/tenant/AuditLogs'
import MCPServersPage from './pages/tenant/MCPServers'
import AdvancedMetricsPage from './pages/tenant/AdvancedMetrics'
import { AgentDashboardPage } from './pages/tenant/AgentDashboardGraphQL'

// Placeholder for pages not yet implemented
const PlaceholderPage = ({ title }: { title: string }) => (
  <div className="space-y-4">
    <h1 className="text-3xl font-bold">{title}</h1>
    <p className="text-muted-foreground">This page is coming soon.</p>
  </div>
)

function App() {
  return (
    <ApolloProvider client={client}>
      <BrowserRouter>
        <Routes>
          {/* Public routes */}
          <Route path="/" element={<HomePage />} />
          <Route path="/login" element={<LoginPage />} />

          {/* Dashboard routes (single tenant - no tenant slug needed) */}
          <Route path="/dashboard" element={<Layout type="dashboard" />}>
            <Route index element={<DashboardPage />} />
            <Route path="agent-dashboard" element={<AgentDashboardPage />} />
            <Route path="logs" element={<RequestLogsPage />} />
            <Route path="costs" element={<CostAnalysisPage />} />
            <Route path="performance" element={<PlaceholderPage title="Performance" />} />
            <Route path="advanced-metrics" element={<AdvancedMetricsPage />} />
            <Route path="providers" element={<ProvidersPage />} />
            <Route path="models" element={<ModelsPage />} />
            <Route path="roles" element={<RolesPage />} />
            <Route path="api-keys" element={<APIKeysPage />} />
            <Route path="users" element={<UsersPage />} />
            <Route path="audit-logs" element={<AuditLogsPage />} />
            <Route path="telemetry" element={<TelemetryPage />} />
            <Route path="mcp" element={<MCPServersPage />} />
            <Route path="alerts" element={<PlaceholderPage title="Budget Alerts" />} />
            <Route path="settings" element={<SettingsPage />} />
          </Route>

          {/* Legacy tenant routes - redirect to dashboard */}
          <Route path="/tenant/*" element={<Navigate to="/dashboard" replace />} />
          <Route path="/admin/*" element={<Navigate to="/dashboard" replace />} />

          {/* Fallback redirect */}
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
        <Toaster />
      </BrowserRouter>
    </ApolloProvider>
  )
}

export default App
