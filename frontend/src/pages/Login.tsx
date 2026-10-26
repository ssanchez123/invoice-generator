import { useState } from 'react'
import { Navigate, useNavigate } from 'react-router-dom'
import { api, isAuthenticated } from '../api'

const DEV_TENANT_ID = '00000000-0000-0000-0000-000000000001'

export default function Login() {
  const navigate = useNavigate()
  const [tenantId, setTenantId] = useState(
    import.meta.env.DEV ? import.meta.env.VITE_DEV_TENANT_ID || DEV_TENANT_ID : '',
  )
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  if (isAuthenticated()) return <Navigate to="/" replace />

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setError('')
    setLoading(true)
    try {
      await api.login(tenantId.trim(), email.trim(), password)
      navigate('/', { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'No fue posible iniciar sesión. Intenta de nuevo.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <main className="login-page">
      <section className="login-aside" aria-label="Acerca de InvoiceGen">
        <a className="login-brand" href="/">
          <span className="brand-mark" aria-hidden="true">i</span>
          <strong>invoicegen</strong>
        </a>
        <div className="login-aside-copy">
          <span className="eyebrow eyebrow-light">TU NEGOCIO, EN ORDEN</span>
          <h1>Facturación clara. Operación tranquila.</h1>
          <p>Administra tus clientes, crea facturas y lleva el control de tus pagos desde un solo lugar.</p>
        </div>
        <div className="login-aside-foot">
          <span className="aside-orbit orbit-one" aria-hidden="true" />
          <span className="aside-orbit orbit-two" aria-hidden="true" />
          <span>Una forma simple de mantener tus cuentas al día.</span>
        </div>
      </section>

      <section className="login-panel">
        <div className="login-card">
          <div className="login-mobile-brand">
            <span className="brand-mark" aria-hidden="true">i</span>
            <strong>invoicegen</strong>
          </div>
          <span className="eyebrow">BIENVENIDO DE NUEVO</span>
          <h2>Inicia sesión</h2>
          <p className="login-description">Ingresa los datos de tu espacio de trabajo.</p>

          {error && <div className="alert alert-error" role="alert">{error}</div>}

          <form className="form-stack login-form" onSubmit={handleSubmit}>
            <label className="field">
              <span>Identificador del espacio</span>
              <input
                type="text"
                name="tenantId"
                value={tenantId}
                onChange={event => setTenantId(event.target.value)}
                autoComplete="organization"
                required
                spellCheck={false}
                placeholder="UUID de tu espacio de trabajo"
              />
              <small>Solicítalo al administrador de tu cuenta.</small>
            </label>
            <div className="notice notice-info login-access-note">
              <span className="notice-icon" aria-hidden="true">i</span>
              <p>El registro público está deshabilitado. El acceso se habilita mediante aprovisionamiento controlado por el operador.</p>
            </div>

            <label className="field">
              <span>Correo electrónico</span>
              <input
                type="email"
                name="email"
                value={email}
                onChange={event => setEmail(event.target.value)}
                autoComplete="username"
                autoCapitalize="none"
                spellCheck={false}
                required
                placeholder="nombre@empresa.com"
              />
            </label>

            <div className="field">
              <label htmlFor="login-password">Contraseña</label>
              <span className="password-field">
                <input
                  id="login-password"
                  type={showPassword ? 'text' : 'password'}
                  name="password"
                  value={password}
                  onChange={event => setPassword(event.target.value)}
                  autoComplete="current-password"
                  required
                  placeholder="Tu contraseña"
                />
                <button
                  type="button"
                  className="password-toggle"
                  onClick={() => setShowPassword(value => !value)}
                  aria-label={showPassword ? 'Ocultar contraseña' : 'Mostrar contraseña'}
                  aria-pressed={showPassword}
                >
                  {showPassword ? 'Ocultar' : 'Mostrar'}
                </button>
              </span>
            </div>

            <button className="button button-primary login-submit" type="submit" disabled={loading}>
              {loading ? <><span className="spinner" aria-hidden="true" /> Verificando acceso…</> : 'Entrar a mi cuenta'}
            </button>
          </form>

          {import.meta.env.DEV && (
            <p className="dev-hint">
              Entorno de desarrollo: credenciales de prueba disponibles en <code>LOCAL_SETUP.md</code>.
            </p>
          )}
          <p className="login-legal">Al continuar, accedes a un espacio de trabajo privado y protegido.</p>
        </div>
        <div className="login-footer">© {new Date().getFullYear()} InvoiceGen</div>
      </section>
    </main>
  )
}
