import { useEffect, useState, useCallback } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api, type Invoice, type Payment, type Customer } from '../api'

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

const PAYMENT_METHODS = ['bank_transfer', 'credit_card', 'cash', 'crypto']

function formatDate(s: string | null): string {
  if (!s) return '—'
  return new Date(s).toLocaleDateString('es-CO', { day: '2-digit', month: 'short', year: 'numeric' })
}

function formatMoney(cents: number, currency: string): string {
  return `${(cents / 100).toFixed(2)} ${currency}`
}

export default function InvoiceDetail() {
  const { id } = useParams<{ id: string }>()
  const [invoice, setInvoice] = useState<Invoice | null>(null)
  const [customer, setCustomer] = useState<Customer | null>(null)
  const [payments, setPayments] = useState<Payment[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [actionError, setActionError] = useState('')
  const [busy, setBusy] = useState(false)

  // Payment form
  const [payAmount, setPayAmount] = useState('')
  const [payMethod, setPayMethod] = useState('bank_transfer')
  const [payRef, setPayRef] = useState('')

  const load = useCallback(async () => {
    if (!id) return
    setLoading(true)
    setError('')
    try {
      const inv = await api.getInvoice(id)
      setInvoice(inv)
      if (inv.customer_id) {
        try {
          const c = await api.listCustomers()
          const found = c.data.find(cust => cust.id === inv.customer_id)
          if (found) setCustomer(found)
        } catch { /* non-critical */ }
      }
      try {
        const payRes = await api.listPayments(id)
        setPayments(payRes.data || [])
      } catch { /* non-critical */ }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Error al cargar factura')
    } finally {
      setLoading(false)
    }
  }, [id])

  useEffect(() => { load() }, [load])

  const handleIssue = async () => {
    if (!id) return
    setBusy(true); setActionError('')
    try {
      await api.issueInvoice(id)
      await load()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Error al emitir')
    } finally { setBusy(false) }
  }

  const handleCancel = async () => {
    if (!id) return
    setBusy(true); setActionError('')
    try {
      await api.cancelInvoice(id)
      await load()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Error al cancelar')
    } finally { setBusy(false) }
  }

  const handlePayment = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!id || !invoice) return
    setBusy(true); setActionError('')
    try {
      await api.recordPayment(id, {
        amount: Number(payAmount),
        currency: invoice.currency,
        method: payMethod,
        reference: payRef,
      })
      setPayAmount('')
      setPayRef('')
      await load()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Error al registrar pago')
    } finally { setBusy(false) }
  }

  if (loading) return <div className="card"><p>Cargando...</p></div>
  if (error) return <div className="card"><p style={{ color: '#c00' }}>{error}</p></div>
  if (!invoice) return <div className="card"><p>Factura no encontrada</p></div>

  const canIssue = invoice.status === 'draft'
  const canCancel = invoice.status === 'draft' || invoice.status === 'issued'
  const canPay = invoice.status === 'issued' || invoice.status === 'overdue'
  const totalPaid = payments.reduce((sum, p) => sum + p.amount, 0)
  const balance = invoice.total - totalPaid

  return (
    <div>
      <div className="page-header">
        <h2>Factura {invoice.number}</h2>
        <Link to="/" className="btn">← Volver</Link>
      </div>

      {actionError && <div style={{ color: '#c00', marginBottom: 16, padding: 12, background: '#fff0f0', borderRadius: 6 }}>{actionError}</div>}

      {/* Status + actions */}
      <div className="card" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div style={{ display: 'flex', gap: 24, alignItems: 'center' }}>
          <span className="badge" style={{ background: STATUS_COLORS[invoice.status] || '#eee', fontSize: 14, padding: '4px 12px' }}>
            {STATUS_LABELS[invoice.status] || invoice.status}
          </span>
          <span style={{ color: '#666', fontSize: 14 }}>Vence: {formatDate(invoice.due_date)}</span>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          {canIssue && <button onClick={handleIssue} disabled={busy} style={{ background: '#0066cc' }}>Emitir</button>}
          {canCancel && <button onClick={handleCancel} disabled={busy} style={{ background: '#cc0000' }}>Cancelar</button>}
        </div>
      </div>

      {/* Customer */}
      {customer && (
        <div className="card">
          <h3 style={{ marginBottom: 12, fontSize: 14, textTransform: 'uppercase', color: '#666' }}>Cliente</h3>
          <div style={{ fontSize: 16, fontWeight: 600 }}>{customer.name}</div>
          {customer.email && <div style={{ color: '#666' }}>{customer.email}</div>}
          {customer.tax_id && <div style={{ color: '#999' }}>Tax ID: {customer.tax_id}</div>}
        </div>
      )}

      {/* Items */}
      <div className="card">
        <h3 style={{ marginBottom: 12, fontSize: 14, textTransform: 'uppercase', color: '#666' }}>Items</h3>
        {(invoice.items || []).length === 0 ? (
          <p style={{ color: '#999' }}>Sin items</p>
        ) : (
          <table>
            <thead>
              <tr>
                <th>Descripción</th>
                <th style={{ textAlign: 'center' }}>Cant.</th>
                <th style={{ textAlign: 'right' }}>Precio unit.</th>
                <th style={{ textAlign: 'center' }}>Imp.</th>
                <th style={{ textAlign: 'right' }}>Total</th>
              </tr>
            </thead>
            <tbody>
              {(invoice.items || []).map(item => (
                <tr key={item.id}>
                  <td>{item.description}</td>
                  <td style={{ textAlign: 'center' }}>{item.quantity}</td>
                  <td style={{ textAlign: 'right' }}>{formatMoney(item.unit_price, invoice.currency)}</td>
                  <td style={{ textAlign: 'center' }}>{item.tax_rate_bps > 0 ? `${item.tax_rate_bps / 100}%` : '—'}</td>
                  <td style={{ textAlign: 'right' }}>{formatMoney(item.total, invoice.currency)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
        <div style={{ marginTop: 16, paddingTop: 16, borderTop: '1px solid #eee' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', padding: '4px 0' }}>
            <span>Subtotal:</span><span>{formatMoney(invoice.subtotal, invoice.currency)}</span>
          </div>
          <div style={{ display: 'flex', justifyContent: 'space-between', padding: '4px 0' }}>
            <span>Impuestos:</span><span>{formatMoney(invoice.tax_total, invoice.currency)}</span>
          </div>
          <div style={{ display: 'flex', justifyContent: 'space-between', padding: '8px 0 4px', fontWeight: 700, borderTop: '1px solid #ddd', marginTop: 8 }}>
            <span>Total:</span><span>{formatMoney(invoice.total, invoice.currency)}</span>
          </div>
        </div>
      </div>

      {/* Payments */}
      <div className="card">
        <h3 style={{ marginBottom: 12, fontSize: 14, textTransform: 'uppercase', color: '#666' }}>Pagos</h3>
        {payments.length > 0 ? (
          <table>
            <thead>
              <tr>
                <th>Fecha</th>
                <th>Método</th>
                <th>Referencia</th>
                <th style={{ textAlign: 'right' }}>Monto</th>
              </tr>
            </thead>
            <tbody>
              {payments.map(p => (
                <tr key={p.id}>
                  <td>{formatDate(p.paid_at)}</td>
                  <td>{p.method}</td>
                  <td>{p.reference || '—'}</td>
                  <td style={{ textAlign: 'right' }}>{formatMoney(p.amount, p.currency)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : (
          <p style={{ color: '#999' }}>Sin pagos registrados</p>
        )}
        {payments.length > 0 && (
          <div style={{ marginTop: 12, paddingTop: 12, borderTop: '1px solid #eee' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', padding: '4px 0' }}>
              <span>Total pagado:</span><span style={{ fontWeight: 600 }}>{formatMoney(totalPaid, invoice.currency)}</span>
            </div>
            <div style={{ display: 'flex', justifyContent: 'space-between', padding: '4px 0' }}>
              <span>Saldo:</span><span style={{ fontWeight: 600, color: balance > 0 ? '#c00' : '#070' }}>{formatMoney(balance, invoice.currency)}</span>
            </div>
          </div>
        )}

        {/* Payment form */}
        {canPay && balance > 0 && (
          <form onSubmit={handlePayment} style={{ marginTop: 16, paddingTop: 16, borderTop: '1px solid #eee', display: 'flex', gap: 8, flexWrap: 'wrap', maxWidth: 'none' }}>
            <input
              type="number"
              placeholder={`Monto (cents) — saldo: ${balance}`}
              value={payAmount}
              onChange={e => setPayAmount(e.target.value)}
              required
              min={1}
              max={balance}
              style={{ flex: '1 1 200px', padding: '8px 12px', border: '1px solid #ddd', borderRadius: 6, fontSize: 14 }}
            />
            <select value={payMethod} onChange={e => setPayMethod(e.target.value)} style={{ flex: '0 1 150px', padding: '8px 12px', border: '1px solid #ddd', borderRadius: 6, fontSize: 14 }}>
              {PAYMENT_METHODS.map(m => <option key={m} value={m}>{m}</option>)}
            </select>
            <input
              type="text"
              placeholder="Referencia (opcional)"
              value={payRef}
              onChange={e => setPayRef(e.target.value)}
              style={{ flex: '1 1 200px', padding: '8px 12px', border: '1px solid #ddd', borderRadius: 6, fontSize: 14 }}
            />
            <button type="submit" disabled={busy} style={{ flex: '0 0 auto' }}>Registrar pago</button>
          </form>
        )}
      </div>

      {/* Notes */}
      {invoice.notes && (
        <div className="card">
          <h3 style={{ marginBottom: 8, fontSize: 14, textTransform: 'uppercase', color: '#666' }}>Notas</h3>
          <p>{invoice.notes}</p>
        </div>
      )}
    </div>
  )
}