import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ApiError, api, downloadInvoicePDF, getStoredUser, type Customer, type Invoice, type Payment } from '../api'
import StatusBadge from '../components/StatusBadge'
import { calculateItemTotals, formatDate, formatMoney, parseAmountToMinorUnits } from '../format'

const PAYMENT_METHODS = [
  { value: 'bank_transfer', label: 'Transferencia bancaria' },
  { value: 'credit_card', label: 'Tarjeta' },
  { value: 'cash', label: 'Efectivo' },
  { value: 'other', label: 'Otro' },
]

function itemTotal(item: NonNullable<Invoice['items']>[number]): number {
  if (typeof item.total === 'number') return item.total
  return calculateItemTotals({
    quantity: item.quantity,
    unit_price: item.unit_price,
    tax_rate_bps: item.tax_rate_bps,
    discount_bps: item.discount_bps || 0,
  }).total
}

export default function InvoiceDetail() {
  const { id } = useParams<{ id: string }>()
  const canManage = getStoredUser()?.role === 'admin' || getStoredUser()?.role === 'member'
  const paymentIdempotency = useRef<{ signature: string; key: string } | null>(null)
  const [invoice, setInvoice] = useState<Invoice | null>(null)
  const [customer, setCustomer] = useState<Customer | null>(null)
  const [payments, setPayments] = useState<Payment[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [relatedError, setRelatedError] = useState('')
  const [actionError, setActionError] = useState('')
  const [actionSuccess, setActionSuccess] = useState('')
  const [busy, setBusy] = useState(false)
  const [paymentAmount, setPaymentAmount] = useState('')
  const [paymentMethod, setPaymentMethod] = useState('bank_transfer')
  const [paymentReference, setPaymentReference] = useState('')

  const load = useCallback(async (silent = false) => {
    if (!id) {
      setError('No se encontró el identificador de la factura.')
      setLoading(false)
      return
    }
    if (!silent) setLoading(true)
    setError('')
    setRelatedError('')
    try {
      const loadedInvoice = await api.getInvoice(id)
      setInvoice(loadedInvoice)
      const [customerResult, paymentResult] = await Promise.allSettled([
        api.getCustomer(loadedInvoice.customer_id),
        api.listPayments(id),
      ])
      const relatedErrors: string[] = []
      if (customerResult.status === 'fulfilled') setCustomer(customerResult.value)
      else {
        setCustomer(null)
        relatedErrors.push('No se pudieron cargar los datos del cliente.')
      }
      if (paymentResult.status === 'fulfilled') setPayments(paymentResult.value.data || [])
      else {
        setPayments([])
        relatedErrors.push('No se pudo consultar el historial de pagos.')
      }
      setRelatedError(relatedErrors.join(' '))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'No fue posible cargar la factura.')
    } finally {
      setLoading(false)
    }
  }, [id])

  useEffect(() => { void load() }, [load])

  const totalPaid = useMemo(() => payments.reduce((sum, payment) => sum + payment.amount, 0), [payments])
  const balance = Math.max(0, (invoice?.total || 0) - totalPaid)
  const canIssue = canManage && invoice?.status === 'draft'
  const canCancel = canManage && (invoice?.status === 'draft' || invoice?.status === 'issued')
  const canPay = canManage && (invoice?.status === 'issued' || invoice?.status === 'overdue') && balance > 0

  const handleIssue = async () => {
    if (!id || !window.confirm('¿Marcar esta factura como emitida? Esta acción genera el PDF interno, pero no la envía ni valida ante la DIAN.')) return
    setBusy(true)
    setActionError('')
    setActionSuccess('')
    try {
      await api.issueInvoice(id)
      setActionSuccess('La factura se marcó como emitida. El PDF puede tardar unos momentos en estar disponible.')
      await load(true)
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'No fue posible emitir la factura.')
    } finally {
      setBusy(false)
    }
  }

  const handleCancel = async () => {
    if (!id || !window.confirm('¿Cancelar esta factura? No podrás registrar pagos sobre un documento cancelado.')) return
    setBusy(true)
    setActionError('')
    setActionSuccess('')
    try {
      await api.cancelInvoice(id)
      setActionSuccess('La factura se canceló correctamente.')
      await load(true)
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'No fue posible cancelar la factura.')
    } finally {
      setBusy(false)
    }
  }

  const handlePDFDownload = async () => {
    if (!id) return
    setBusy(true)
    setActionError('')
    setActionSuccess('')
    try {
      await downloadInvoicePDF(id)
    } catch (err) {
      const message = err instanceof ApiError && err.status === 404
        ? 'El PDF todavía se está generando. Espera un momento e inténtalo de nuevo.'
        : err instanceof Error ? err.message : 'No fue posible descargar el PDF.'
      setActionError(message)
    } finally {
      setBusy(false)
    }
  }

  const handlePayment = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!id || !invoice) return
    const amount = parseAmountToMinorUnits(paymentAmount)
    if (amount === null || amount <= 0) {
      setActionError('Ingresa un monto válido con máximo dos decimales.')
      return
    }
    if (amount > balance) {
      setActionError('El monto no puede superar el saldo pendiente.')
      return
    }

    const body = {
      amount,
      currency: invoice.currency,
      method: paymentMethod,
      reference: paymentReference.trim() || undefined,
    }
    const signature = JSON.stringify(body)
    if (paymentIdempotency.current?.signature !== signature) {
      paymentIdempotency.current = { signature, key: crypto.randomUUID() }
    }
    setBusy(true)
    setActionError('')
    setActionSuccess('')
    try {
      await api.recordPayment(id, body, paymentIdempotency.current.key)
      paymentIdempotency.current = null
      setPaymentAmount('')
      setPaymentReference('')
      setActionSuccess('El pago se registró correctamente.')
      await load(true)
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'No fue posible registrar el pago.')
    } finally {
      setBusy(false)
    }
  }

  if (loading) {
    return <div className="loading-page" role="status"><span className="spinner spinner-dark" aria-hidden="true" /> Cargando factura…</div>
  }
  if (error) {
    return (
      <section className="card error-page" role="alert">
        <span className="empty-state-icon" aria-hidden="true">!</span>
        <h1>No pudimos abrir esta factura</h1>
        <p>{error}</p>
        <Link to="/" className="button button-secondary">Volver a facturas</Link>
      </section>
    )
  }
  if (!invoice) return null

  return (
    <section className="page">
      <div className="detail-back-link"><Link to="/">← Volver a facturas</Link></div>
      <div className="page-heading detail-heading">
        <div>
          <span className="eyebrow">DETALLE DEL DOCUMENTO</span>
          <h1>Factura <span className="heading-number">{invoice.number}</span></h1>
          <p className="page-description">Creada el {formatDate(invoice.created_at)}</p>
        </div>
        <div className="heading-actions">
          {canIssue && <Link to={`/invoices/${invoice.id}/edit`} className="button button-secondary">Editar borrador</Link>}
          {invoice.status !== 'draft' && invoice.status !== 'cancelled' && (
            <button className="button button-secondary" onClick={() => void handlePDFDownload()} disabled={busy}>
              <span aria-hidden="true">↓</span> Descargar PDF
            </button>
          )}
          {canCancel && <button className="button button-danger-quiet" onClick={() => void handleCancel()} disabled={busy}>Cancelar factura</button>}
          {canIssue && <button className="button button-primary" onClick={() => void handleIssue()} disabled={busy}>Marcar como emitida</button>}
        </div>
      </div>

      {actionError && <div className="alert alert-error" role="alert">{actionError}</div>}
      {actionSuccess && <div className="alert alert-success" role="status">{actionSuccess}</div>}
      {relatedError && <div className="inline-warning" role="status">{relatedError}</div>}

      <div className="detail-grid">
        <div className="detail-main-column">
          <section className="card document-card" aria-labelledby="document-info-heading">
            <div className="document-card-heading">
              <div>
                <span className="eyebrow">RESUMEN</span>
                <h2 id="document-info-heading">Información de la factura</h2>
              </div>
              <StatusBadge status={invoice.status} />
            </div>
            {invoice.status === 'draft' && (
              <div className="notice notice-warning notice-compact">
                <span className="notice-icon" aria-hidden="true">!</span>
                <p>Al marcarla como emitida, InvoiceGen actualiza el estado y genera una representación PDF. La integración DIAN no está conectada.</p>
              </div>
            )}
            <div className="detail-facts">
              <div><span>Fecha de emisión</span><strong>{formatDate(invoice.issue_date)}</strong></div>
              <div><span>Fecha de vencimiento</span><strong>{formatDate(invoice.due_date)}</strong></div>
              <div><span>Moneda</span><strong>{invoice.currency}</strong></div>
              <div><span>Identificador</span><strong className="mono-text">{invoice.id}</strong></div>
            </div>
          </section>

          <section className="card" aria-labelledby="items-heading">
            <div className="card-heading">
              <div><h2 id="items-heading">Detalle de conceptos</h2><p>{invoice.items?.length || 0} líneas</p></div>
            </div>
            {(invoice.items || []).length === 0 ? (
              <p className="muted-copy">Esta factura no tiene líneas registradas.</p>
            ) : (
              <>
                <div className="table-scroll">
                  <table className="data-table item-table">
                    <thead><tr><th scope="col">Descripción</th><th scope="col" className="align-center">Cant.</th><th scope="col" className="align-right">Precio unitario</th><th scope="col" className="align-right">Impuesto</th><th scope="col" className="align-right">Total</th></tr></thead>
                    <tbody>
                      {(invoice.items || []).map(item => (
                        <tr key={item.id}>
                          <td><strong>{item.description}</strong>{item.discount_bps > 0 && <span className="table-secondary">Descuento: {item.discount_bps / 100}%</span>}</td>
                          <td className="align-center">{item.quantity}</td>
                          <td className="align-right">{formatMoney(item.unit_price, invoice.currency)}</td>
                          <td className="align-right">{item.tax_rate_bps > 0 ? `${item.tax_rate_bps / 100}%` : '—'}</td>
                          <td className="align-right amount-cell">{formatMoney(itemTotal(item), invoice.currency)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                <div className="invoice-totals">
                  <div><span>Subtotal</span><strong>{formatMoney(invoice.subtotal, invoice.currency)}</strong></div>
                  <div><span>Impuestos</span><strong>{formatMoney(invoice.tax_total, invoice.currency)}</strong></div>
                  <div className="grand-total"><span>Total</span><strong>{formatMoney(invoice.total, invoice.currency)}</strong></div>
                </div>
              </>
            )}
          </section>

          <section className="card payment-card" aria-labelledby="payments-heading">
            <div className="card-heading">
              <div><h2 id="payments-heading">Pagos recibidos</h2><p>Registro manual de pagos asociados.</p></div>
              <span className="payment-count">{payments.length}</span>
            </div>
            {payments.length > 0 ? (
              <div className="table-scroll">
                <table className="data-table">
                  <thead><tr><th scope="col">Fecha</th><th scope="col">Método</th><th scope="col">Referencia</th><th scope="col" className="align-right">Monto</th></tr></thead>
                  <tbody>
                    {payments.map(payment => (
                      <tr key={payment.id}>
                        <td>{formatDate(payment.paid_at)}</td>
                        <td>{PAYMENT_METHODS.find(method => method.value === payment.method)?.label || payment.method}</td>
                        <td>{payment.reference || '—'}</td>
                        <td className="align-right amount-cell">{formatMoney(payment.amount, payment.currency)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <div className="empty-inline"><span aria-hidden="true">↗</span><p>Aún no hay pagos registrados.</p></div>
            )}
            {payments.length > 0 && (
              <div className="payment-summary">
                <div><span>Total pagado</span><strong>{formatMoney(totalPaid, invoice.currency)}</strong></div>
                <div><span>Saldo pendiente</span><strong>{formatMoney(balance, invoice.currency)}</strong></div>
              </div>
            )}
            {canPay && (
              <form className="payment-form" onSubmit={handlePayment}>
                <h3>Registrar un pago</h3>
                <div className="payment-form-grid">
                  <label className="field">
                    <span>Monto ({invoice.currency})</span>
                    <input
                      type="number"
                      min="0.01"
                      max={(balance / 100).toFixed(2)}
                      step="0.01"
                      inputMode="decimal"
                      value={paymentAmount}
                      onChange={event => setPaymentAmount(event.target.value)}
                      required
                      aria-describedby="payment-balance-hint"
                    />
                    <small id="payment-balance-hint">Saldo disponible: {formatMoney(balance, invoice.currency)}</small>
                  </label>
                  <label className="field">
                    <span>Método de pago</span>
                    <select value={paymentMethod} onChange={event => setPaymentMethod(event.target.value)}>
                      {PAYMENT_METHODS.map(method => <option key={method.value} value={method.value}>{method.label}</option>)}
                    </select>
                  </label>
                  <label className="field payment-reference-field">
                    <span>Referencia <span className="optional-label">(opcional)</span></span>
                    <input value={paymentReference} onChange={event => setPaymentReference(event.target.value)} maxLength={120} placeholder="Número de transacción o comprobante" />
                  </label>
                  <button className="button button-primary payment-submit" type="submit" disabled={busy}>
                    {busy ? 'Procesando…' : 'Registrar pago'}
                  </button>
                </div>
              </form>
            )}
          </section>

          {invoice.notes && (
            <section className="card" aria-labelledby="notes-heading">
              <div className="card-heading"><div><h2 id="notes-heading">Notas internas</h2></div></div>
              <p className="notes-copy">{invoice.notes}</p>
            </section>
          )}
        </div>

        <aside className="detail-side-column">
          <section className="card customer-detail-card" aria-labelledby="customer-heading">
            <div className="card-heading"><div><h2 id="customer-heading">Cliente</h2></div></div>
            {customer ? (
              <>
                <div className="customer-profile">
                  <span className="customer-avatar" aria-hidden="true">{customer.name.slice(0, 1).toUpperCase()}</span>
                  <div><strong>{customer.name}</strong><span>{customer.tax_id || 'Sin identificación tributaria'}</span></div>
                </div>
                {customer.email && <div className="customer-contact"><span aria-hidden="true">✉</span><a href={`mailto:${customer.email}`}>{customer.email}</a></div>}
                {customer.phone && <div className="customer-contact"><span aria-hidden="true">⌕</span><a href={`tel:${customer.phone}`}>{customer.phone}</a></div>}
                {customer.address?.line1 && <address className="customer-address">{customer.address.line1}{customer.address.line2 ? `, ${customer.address.line2}` : ''}<br />{customer.address.city}{customer.address.state ? `, ${customer.address.state}` : ''}{customer.address.country ? ` · ${customer.address.country}` : ''}</address>}
              </>
            ) : <p className="muted-copy">No hay datos de cliente disponibles.</p>}
          </section>

          <section className="card balance-card" aria-labelledby="balance-heading">
            <span className="eyebrow">RESUMEN DE PAGO</span>
            <h2 id="balance-heading">Saldo pendiente</h2>
            <p className="balance-value">{formatMoney(balance, invoice.currency)}</p>
            <progress
              className="balance-progress"
              aria-label="Porcentaje de factura pagada"
              value={invoice.total > 0 ? Math.min(100, (totalPaid / invoice.total) * 100) : 0}
              max={100}
            />
            <div className="balance-legend"><span>Pagado</span><strong>{invoice.total > 0 ? Math.min(100, Math.round((totalPaid / invoice.total) * 100)) : 0}%</strong></div>
            <div className="balance-line"><span>Total factura</span><strong>{formatMoney(invoice.total, invoice.currency)}</strong></div>
            <div className="balance-line"><span>Pagos recibidos</span><strong>{formatMoney(totalPaid, invoice.currency)}</strong></div>
          </section>
        </aside>
      </div>
    </section>
  )
}
