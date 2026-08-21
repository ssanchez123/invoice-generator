import { Outlet, NavLink, useNavigate } from 'react-router-dom'
import { logout, getStoredUser } from './api'

export default function App() {
  const navigate = useNavigate()
  const user = getStoredUser()

  const handleLogout = () => {
    logout()
    navigate('/login', { replace: true })
  }

  return (
    <div className="app-layout">
      <nav className="sidebar">
        <h1>InvoiceGen</h1>
        <NavLink to="/" end className={({ isActive }) => isActive ? 'active' : ''}>
          Facturas
        </NavLink>
        <NavLink to="/invoices/new" className={({ isActive }) => isActive ? 'active' : ''}>
          Nueva Factura
        </NavLink>
        <NavLink to="/customers" className={({ isActive }) => isActive ? 'active' : ''}>
          Clientes
        </NavLink>
        <a href="http://localhost:8080/swagger/" target="_blank" rel="noopener noreferrer" className="swagger-link">
          📖 API Docs
        </a>

        {user && (
          <div style={{ marginTop: 'auto', paddingTop: 16, borderTop: '1px solid #e0e0e0' }}>
            <div style={{ fontSize: 13, color: '#888', marginBottom: 8 }}>
              {user.full_name}
            </div>
            <button
              onClick={handleLogout}
              style={{
                width: '100%',
                padding: '6px 12px',
                fontSize: 13,
                border: '1px solid #ddd',
                borderRadius: 4,
                background: 'white',
                cursor: 'pointer',
              }}
            >
              Cerrar sesión
            </button>
          </div>
        )}
      </nav>
      <main className="main-content">
        <Outlet />
      </main>
    </div>
  )
}
