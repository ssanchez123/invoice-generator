import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, type Customer, type Invoice } from '../api'
import { dateAtEndOfLocalDay, localDateValue, parseAmountToMinorUnits } from '../format'

interface EditableItem {
  description: string
  quantity: string
  unitPrice: string
  taxRateBps: number
  discountBps: string
}

function toLocalDate(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '' : localDateValue(date)
}

export default function EditInvoice() {
  const { id = '' } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [invoice, setInvoice] = useState<Invoice | null>(null)
  const [customers, setCustomers] = useState<Customer[]>([])
  const [customerId, setCustomerId] = useState('')
  const [currency, setCurrency] = useState('COP')
  const [dueDate, setDueDate] = useState('')
  const [notes, setNotes] = useState('')
  const [items, setItems] = useState<EditableItem[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    let active = true
    Promise.all([api.getInvoice(id), api.listCustomers({ limit: 100 })]).then(([loadedInvoice, customersResult]) => {
      if (!active) return
      setInvoice(loadedInvoice)
      setCustomers(customersResult.data || [])
      setCustomerId(loadedInvoice.customer_id)
      setCurrency(loadedInvoice.currency)
      setDueDate(toLocalDate(loadedInvoice.due_date))
      setNotes(loadedInvoice.notes || '')
      setItems((loadedInvoice.items || []).map(item => ({
        description: item.description,
        quantity: String(item.quantity),
        unitPrice: (item.unit_price / 100).toFixed(2),
        taxRateBps: item.tax_rate_bps,
        discountBps: String(item.discount_bps),
      })))
    }).catch(err => {
      if (active) setError(err instanceof Error ? err.message : 'No fue posible cargar la factura.')
    }).finally(() => {
      if (active) setLoading(false)
    })
    return () => { active = false }
  }, [id])

  const updateItem = (index: number, field: keyof EditableItem, value: string | number) => {
    setItems(current => current.map((item, itemIndex) => itemIndex === index ? { ...item, [field]: value } : item))
  }

  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!id) return
    setError('')
    if (!items.length || items.some(item => !item.description.trim())) {
      setError('Agrega al menos una línea y completa todas las descripciones.')
      return
    }
    const normalizedItems = items.map(item => {
      const quantity = Number(item.quantity)
      const unitPrice = parseAmountToMinorUnits(item.unitPrice)
      const discountBps = Number(item.discountBps)
      if (!Number.isSafeInteger(quantity) || quantity < 1 || unitPrice === null ||
        !Number.isSafeInteger(discountBps) || discountBps < 0 || discountBps > 10000) return null
      return {
        description: item.description.trim(),
        quantity,
        unit_price: unitPrice,
        tax_rate_bps: item.taxRateBps,
        discount_bps: discountBps,
      }
    })
    if (normalizedItems.some(item => item === null)) {
      setError('Revisa cantidades, precios y descuentos.')
      return
    }
    setSaving(true)
    try {
      const updated = await api.updateInvoice(id, {
        customer_id: customerId,
        currency,
        due_date: dateAtEndOfLocalDay(dueDate),
        items: normalizedItems.filter((item): item is NonNullable<typeof item> => item !== null),
        notes: notes.trim() || undefined,
      })
      navigate(`/invoices/${updated.id}`, { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'No fue posible guardar los cambios.')
    } finally {
      setSaving(false)
    }
  }

  if (loading) return <div className="loading-page" role="status"><span className="spinner spinner-dark" aria-hidden="true" /> Cargando factura…</div>
  if (!invoice) return <section className="card error-page" role="alert"><h1>No pudimos abrir esta factura</h1><p>{error || 'Factura no encontrada.'}</p><Link to="/" className="button button-secondary">Volver a facturas</Link></section>
  if (invoice.status !== 'draft') return <section className="card error-page" role="alert"><h1>La factura ya no es editable</h1><p>Solo se pueden modificar facturas en borrador.</p><Link to={`/invoices/${id}`} className="button button-secondary">Volver al detalle</Link></section>

  return (
    <section className="page page-narrow">
      <div className="page-heading">
        <div><span className="eyebrow">BORRADOR {invoice.number}</span><h1>Editar factura</h1><p className="page-description">Los documentos emitidos permanecen inmutables.</p></div>
        <Link to={`/invoices/${id}`} className="button button-secondary">Cancelar</Link>
      </div>
      {error && <div className="alert alert-error" role="alert">{error}</div>}
      <section className="card form-card">
        <form className="form-stack" onSubmit={submit}>
          <div className="form-grid form-grid-three">
            <label className="field">
              <span>Cliente</span>
              <select value={customerId} onChange={event => setCustomerId(event.target.value)} required>
                {customers.map(customer => <option key={customer.id} value={customer.id}>{customer.name}</option>)}
              </select>
            </label>
            <label className="field">
              <span>Moneda</span>
              <select value={currency} onChange={event => setCurrency(event.target.value)}>
                {['COP', 'USD', 'EUR', 'MXN'].map(code => <option key={code} value={code}>{code}</option>)}
              </select>
            </label>
            <label className="field">
              <span>Vencimiento</span>
              <input type="date" value={dueDate} onChange={event => setDueDate(event.target.value)} required />
            </label>
          </div>
          <div className="form-section">
            <div className="form-section-heading"><span className="step-number" aria-hidden="true">01</span><div><h2>Conceptos</h2><p>Al guardar se recalcularán los totales.</p></div></div>
            <div className="line-items">
              {items.map((item, index) => (
                <fieldset className="line-item" key={index}>
                  <legend className="visually-hidden">Detalle {index + 1}</legend>
                  <div className="line-item-top">
                    <span className="line-item-label">LÍNEA {String(index + 1).padStart(2, '0')}</span>
                    {items.length > 1 && <button className="text-button danger-text" type="button" onClick={() => setItems(current => current.filter((_, i) => i !== index))}>Eliminar</button>}
                  </div>
                  <div className="line-item-grid">
                    <label className="field line-description"><span>Descripción</span><input value={item.description} onChange={event => updateItem(index, 'description', event.target.value)} maxLength={240} required /></label>
                    <label className="field"><span>Cantidad</span><input type="number" min="1" step="1" value={item.quantity} onChange={event => updateItem(index, 'quantity', event.target.value)} required /></label>
                    <label className="field"><span>Precio unitario ({currency})</span><input type="number" min="0" step="0.01" value={item.unitPrice} onChange={event => updateItem(index, 'unitPrice', event.target.value)} required /></label>
                    <label className="field"><span>Impuesto (bps)</span><input type="number" min="0" max="10000" value={item.taxRateBps} onChange={event => updateItem(index, 'taxRateBps', Number(event.target.value))} required /></label>
                    <label className="field"><span>Descuento (bps)</span><input type="number" min="0" max="10000" value={item.discountBps} onChange={event => updateItem(index, 'discountBps', event.target.value)} required /></label>
                  </div>
                </fieldset>
              ))}
            </div>
            <button className="button button-secondary add-line-button" type="button" onClick={() => setItems(current => [...current, { description: '', quantity: '1', unitPrice: '0.00', taxRateBps: 0, discountBps: '0' }])}>
              <span aria-hidden="true">＋</span> Agregar línea
            </button>
          </div>
          <label className="field"><span>Notas internas</span><textarea value={notes} onChange={event => setNotes(event.target.value)} maxLength={1000} rows={3} /></label>
          <div className="form-actions">
            <Link to={`/invoices/${id}`} className="button button-quiet">Cancelar</Link>
            <button className="button button-primary" type="submit" disabled={saving}>{saving ? 'Guardando…' : 'Guardar cambios'}</button>
          </div>
        </form>
      </section>
    </section>
  )
}
