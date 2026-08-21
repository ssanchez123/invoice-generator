import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, isAuthenticated } from '../api'

const DEV_TENANT_ID = '00000000-0000-0000-0000-000000000001'

export default function Login() {
  const navigate = useNavigate()
  const [tenantId, setTenantId] = useState(DEV_TENANT_ID)
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  // If already logged in, redirect
  if (isAuthenticated()) {
    navigate('/', { replace: true })
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    try {
      await api.login(tenantId, email, password)
      navigate('/', { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Error al iniciar sesión')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={{
      display: 'flex',
      justifyContent: 'center',
      alignItems: 'center',
      minHeight: '100vh',
      background: '#f5f5f5',
    }}>
      <div className="card" style={{ width: 400, padding: 32 }}>
        <h1 style={{ textAlign: 'center', marginBottom: 8 }}>InvoiceGen</h1>
        <p style={{ textAlign: 'center', color: '#888', marginBottom: 24 }}>Inicia sesión para continuar</p>

        {error && (
          <div style={{ color: '#c00', marginBottom: 16, padding: '8px 12px', background: '#fee', borderRadius: 4 }}>
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div>
            <label style={{ display: 'block', marginBottom: 4, fontSize: 14, fontWeight: 500 }}>
              Tenant ID
            </label>
            <input
              type="text"
              value={tenantId}
              onChange={e => setTenantId(e.target.value)}
              style={{ width: '100%', padding: '8px 12px', border: '1px solid #ddd', borderRadius: 4 }}
              placeholder="00000000-0000-0000-0000-000000000001"
            />
          </div>

          <div>
            <label style={{ display: 'block', marginBottom: 4, fontSize: 14, fontWeight: 500 }}>
              Email
            </label>
            <input
              type="email"
              value={email}
              onChange={e => setEmail(e.target.value)}
              required
              style={{ width: '100%', padding: '8px 12px', border: '1px solid #ddd', borderRadius: 4 }}
              placeholder="admin@acme.com"
            />
          </div>

          <div>
            <label style={{ display: 'block', marginBottom: 4, fontSize: 14, fontWeight: 500 }}>
              Contraseña
            </label>
            <input
              type="password"
              value={password}
              onChange={e => setPassword(e.target.value)}
              required
              style={{ width: '100%', padding: '8px 12px', border: '1px solid #ddd', borderRadius: 4 }}
              placeholder="••••••••"
            />
          </div>

          <button
            type="submit"
            disabled={loading}
            className="btn"
            style={{ width: '100%', padding: '10px 16px', fontSize: 16 }}
          >
            {loading ? 'Iniciando sesión...' : 'Iniciar sesión'}
          </button>
        </form>

        <p style={{ textAlign: 'center', marginTop: 16, fontSize: 13, color: '#aaa' }}>
          Demo: <code>admin@acme.com</code> / <code>Password123!</code>
        </p>
      </div>
    </div>
  )
}
