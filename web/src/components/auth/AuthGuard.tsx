import { useEffect, useState } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'
import { useQuery, gql } from '@apollo/client'
import { Loader2 } from 'lucide-react'

const ME_QUERY = gql`
  query Me {
    me {
      id
      email
      name
      role
    }
  }
`

interface AuthGuardProps {
  children: React.ReactNode
  type: 'admin' | 'tenant' | 'dashboard'
}

export function AuthGuard({ children, type }: AuthGuardProps) {
  const navigate = useNavigate()
  const location = useLocation()
  const [isChecking, setIsChecking] = useState(true)

  const token = localStorage.getItem('authToken')

  // Skip query if no token
  const { data, loading, error } = useQuery(ME_QUERY, {
    skip: !token,
  })

  useEffect(() => {
    // No token - redirect to login
    if (!token) {
      const returnTo = '/dashboard'
      navigate(`/login?returnTo=${encodeURIComponent(returnTo)}`, { replace: true })
      return
    }

    // Query completed
    if (!loading) {
      if (error || !data?.me) {
        // Invalid token - clear and redirect
        localStorage.removeItem('authToken')
        const returnTo = '/dashboard'
        navigate(`/login?returnTo=${encodeURIComponent(returnTo)}`, { replace: true })
      } else {
        // Valid token
        setIsChecking(false)
      }
    }
  }, [token, loading, error, data, navigate, type, location.pathname])

  // Show loading state while checking auth
  if (isChecking || loading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background">
        <div className="text-center">
          <Loader2 className="mx-auto h-8 w-8 animate-spin text-primary" />
          <p className="mt-4 text-sm text-muted-foreground">Verifying authentication...</p>
        </div>
      </div>
    )
  }

  return <>{children}</>
}
