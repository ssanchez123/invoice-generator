import { useEffect, useState, useCallback } from 'react'
import { Link } from 'react-router-dom'
import { api, type Invoice } from '../api'

const STATUS_LABELS: Record<string, string> = {
  draft: 'Borrador',
  issued: 'Emitida',
  paid: 'Pagada',
  overdue: 'Vencida',
  cancelled: 'Cancelada',
}

const STATUS_COLORS: Record<string, string> = {
  draft: '#e0e0e0',
  issued: '#cce5ff',
  paid: '#d4edda',
  overdue: '#f8d7da',
  cancelled: '#f0f0f0',
}

function formatDate(s: string | null): string {
  if (!s) return '—'
  return new Date(s).toLocaleDateString('es-CO', { day: '2-digit', month: 'short', year: 'numeric' })
}

function formatMoney(cents: number, currency: string): string {
  return `${(cents / 100).toFixed(2)} ${currency}`
}

export default function InvoiceList() {
  const [invoices, setInvoices] = useState<Invoice[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [statusFilter, setStatusFilter] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const res = await api.listInvoices(statusFilter ? { status: statusFilter } : undefined)
      setInvoices(res.data || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Error al cargar facturas')
    } finally {
      setLoading(false)
    }
  }, [statusFilter])

  useEffect(() => { load() }, [load])

  return (
    <div>
      <div className="page-header">
        <h2>Facturas</h2>
        <Link to="/invoices/new" className="btn">+ Nueva Factura</Link>
      </div>

      <div style={{ marginBottom: 16, display: 'flex', gap: 8, alignItems: 'center' }}>
        <label style={{ flexDirection: 'row', alignItems: 'center', gap: 8 }}>
          <span>Filtrar por estado:</span>
          <select value={statusFilter} onChange={e => setStatusFilter(e.target.value)} style={{ minWidth: 120 }}>
            <option value="">Todas</option>
            <option value="draft">Borrador</option>
            <option value="issued">Emitida</option>
            <option value="paid">Pagada</option>
            <option value="overdue">Vencida</option>
            <option value="cancelled">Cancelada</option>
          </select>
        </label>
      </div>

      {error && <div style={{ color: '#c00', marginBottom: 16 }}>{error}</div>}

      <div className="card">
        {loading ? (
          <p>Cargando...</p>
        ) : invoices.length === 0 ? (
          <p style={{ color: '#999', textAlign: 'center', padding: '32px' }}>
            No hay facturas todavía. Crea la primera.
          </p>
        ) : (
          <table>
            <thead>
              <tr>
                <th>Número</th>
                <th>Estado</th>
                <th>Fecha emisión</th>
                <th>Fecha vencimiento</th>
                <th style={{ textAlign: 'right' }}>Total</th>
              </tr>
            </thead>
            <tbody>
              {invoices.map(inv => (
                <tr key={inv.id}>
                  <td><Link to={`/invoices/${inv.id}`} style={{ fontWeight: 600 }}>{inv.number}</Link></td>
                  <td>
                    <span className="badge" style={{ background: STATUS_COLORS[inv.status] || '#eee' }}>
                      {STATUS_LABELS[inv.status] || inv.status}
                    </span>
                  </td>
                  <td>{formatDate(inv.issue_date)}</td>
                  <td>{formatDate(inv.due_date)}</td>
                  <td style={{ textAlign: 'right' }}>{formatMoney(inv.total, inv.currency)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}