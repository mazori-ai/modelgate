import { Outlet, useNavigate } from 'react-router-dom'
import { Sidebar } from './Sidebar'
import { AuthGuard } from '../auth/AuthGuard'

interface LayoutProps {
  type: 'admin' | 'tenant' | 'dashboard'
}

export function Layout({ type }: LayoutProps) {
  const navigate = useNavigate()

  const handleLogout = () => {
    localStorage.removeItem('authToken')
    localStorage.removeItem('tenantSlug')
    navigate('/login')
  }

  return (
    <AuthGuard type={type}>
      <div className="min-h-screen bg-background">
        <Sidebar 
          type={type} 
          onLogout={handleLogout} 
        />
        <main className="pl-64">
          <div className="p-8">
            <Outlet />
          </div>
        </main>
      </div>
    </AuthGuard>
  )
}
