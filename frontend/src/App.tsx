import { Link, NavLink, Outlet, useNavigate } from 'react-router-dom'
import { getStoredUser, logout } from './api'

export default function App() {
  const navigate = useNavigate()
  const user = getStoredUser()

  const handleLogout = () => {
    logout()
    navigate('/login', { replace: true })
  }

  return (
    <div className="app-shell">
      <a className="skip-link" href="#main-content">Saltar al contenido</a>

      <aside className="sidebar" aria-label="Navegación principal">
        <Link className="brand" to="/">
          <span className="brand-mark" aria-hidden="true">i</span>
          <span>
            <strong>invoicegen</strong>
            <small>Gestión de facturas</small>
          </span>
        </Link>

        <div className="nav-section-label">ESPACIO DE TRABAJO</div>
        <nav className="primary-nav">
          <NavLink to="/" end className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
            <span className="nav-icon" aria-hidden="true">▤</span>
            <span>Facturas</span>
          </NavLink>
          <NavLink to="/invoices/new" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
            <span className="nav-icon" aria-hidden="true">＋</span>
            <span>Nueva factura</span>
          </NavLink>
          <NavLink to="/customers" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
            <span className="nav-icon" aria-hidden="true">♙</span>
            <span>Clientes</span>
          </NavLink>
          <NavLink to="/subscriptions" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
            <span className="nav-icon" aria-hidden="true">↻</span>
            <span>Suscripciones</span>
          </NavLink>
          {user?.role === 'admin' && (
            <NavLink to="/audit" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
              <span className="nav-icon" aria-hidden="true">◷</span>
              <span>Auditoría</span>
            </NavLink>
          )}
        </nav>

        <a className="api-docs-link" href="/swagger/" target="_blank" rel="noopener noreferrer">
          <span aria-hidden="true">↗</span> Documentación API
        </a>

        <div className="sidebar-account">
          <div className="avatar" aria-hidden="true">
            {(user?.first_name?.[0] || user?.full_name?.[0] || 'U').toUpperCase()}
          </div>
          <div className="account-copy">
            <strong>{user?.full_name || 'Usuario'}</strong>
            <span>{user?.email || ''}</span>
          </div>
          <button className="icon-button logout-button" onClick={handleLogout} aria-label="Cerrar sesión" title="Cerrar sesión">
            <span aria-hidden="true">↪</span>
          </button>
        </div>
      </aside>

      <div className="workspace">
        <header className="topbar">
          <div className="topbar-context">
            <span className="topbar-kicker">FACTURACIÓN</span>
            <span className="topbar-divider" aria-hidden="true">/</span>
            <span>Panel de trabajo</span>
          </div>
          <div className="topbar-account">
            <span className="status-dot" aria-hidden="true" />
            <span>Sesión activa</span>
          </div>
        </header>
        <main id="main-content" className="main-content" tabIndex={-1}>
          <Outlet />
          <footer className="app-footer">
            <span>InvoiceGen</span>
            <span>La emisión interna no equivale a validación electrónica DIAN.</span>
          </footer>
        </main>
      </div>
    </div>
  )
}
