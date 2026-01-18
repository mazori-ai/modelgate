import { useState } from 'react'
import { useNavigate, useSearchParams, Link } from 'react-router-dom'
import { useMutation } from '@apollo/client'
import { Shield, Loader2, ArrowLeft } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { LOGIN } from '@/graphql/operations'

export function LoginPage() {
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')

  const returnTo = searchParams.get('returnTo') || '/dashboard'

  const [login, { loading }] = useMutation(LOGIN, {
    onCompleted: (data) => {
      localStorage.setItem('authToken', data.login.token)
      navigate(returnTo)
    },
    onError: (err) => {
      setError(err.message || 'Invalid credentials')
    },
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    login({
      variables: {
        input: { email, password },
      },
    })
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-slate-50 via-white to-slate-50 flex items-center justify-center p-6">
      <div className="w-full max-w-md">
        <div className="mb-8">
          <Link to="/" className="inline-flex items-center gap-2 text-slate-600 hover:text-slate-900 transition-colors">
            <ArrowLeft className="h-4 w-4" />
            Back to Home
          </Link>
        </div>

        <Card>
          <CardHeader className="text-center">
            <div className="mx-auto w-12 h-12 rounded-lg flex items-center justify-center mb-4 bg-gradient-to-br from-emerald-500 to-cyan-500">
              <Shield className="h-6 w-6 text-white" />
            </div>
            <CardTitle className="text-2xl">Sign In</CardTitle>
            <CardDescription>
              Sign in to access the ModelGate dashboard
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSubmit} className="space-y-4">
              {error && (
                <div className="p-3 rounded-lg bg-red-50 border border-red-200 text-red-700 text-sm">
                  {error}
                </div>
              )}

              <div className="space-y-2">
                <Label htmlFor="email">Email *</Label>
                <Input
                  id="email"
                  type="email"
                  placeholder="admin@modelgate.local"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  required
                  autoComplete="email"
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="password">Password *</Label>
                <Input
                  id="password"
                  type="password"
                  placeholder="••••••••"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  autoComplete="current-password"
                />
              </div>

              <Button
                type="submit"
                className="w-full bg-gradient-to-r from-emerald-500 to-cyan-500 hover:from-emerald-600 hover:to-cyan-600"
                disabled={loading}
              >
                {loading ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    Signing in...
                  </>
                ) : (
                  'Sign In'
                )}
              </Button>
            </form>

            <div className="mt-6 pt-4 border-t border-slate-200">
              <p className="text-xs text-center text-slate-500">
                Default credentials: admin@modelgate.local / admin123
              </p>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
