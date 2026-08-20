import { Outlet, NavLink } from 'react-router-dom'

export default function App() {
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
      </nav>
      <main className="main-content">
        <Outlet />
      </main>
    </div>
  )
}