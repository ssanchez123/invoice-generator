import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { api, type Customer } from '../api'
import { calculateItemTotals, dateAtEndOfLocalDay, formatMoney, localDateValue, parseAmountToMinorUnits } from '../format'

interface LineItemForm {
  description: string
  quantity: string
  unitPrice: string
  taxRateBps: number
  discountPercent: string
}

interface InvoiceItemInput {
  description: string
  quantity: number
  unit_price: number
  tax_rate_bps: number
  discount_bps: number
}

const INITIAL_ITEM: LineItemForm = {
  description: '',
  quantity: '1',
  unitPrice: '',
  taxRateBps: 1900,
  discountPercent: '0',
}

const TAX_RATES = [
  { label: 'Sin impuesto · 0%', value: 0 },
  { label: 'IVA · 5%', value: 500 },
  { label: 'IVA · 19%', value: 1900 },
]

function lineItemValues(item: LineItemForm) {
  const quantity = Number(item.quantity)
  const unitPrice = parseAmountToMinorUnits(item.unitPrice)
  const discountPercent = Number(item.discountPercent)
  if (
    !Number.isSafeInteger(quantity) ||
    quantity < 1 ||
    unitPrice === null ||
    !Number.isFinite(discountPercent) ||
    discountPercent < 0 ||
    discountPercent > 100
  ) {
    return null
  }
  return {
    quantity,
    unit_price: unitPrice,
    tax_rate_bps: item.taxRateBps,
    discount_bps: Math.round(discountPercent * 100),
  }
}

export default function CreateInvoice() {
  const navigate = useNavigate()
  const idempotency = useRef<{ signature: string; key: string } | null>(null)
  const [customers, setCustomers] = useState<Customer[]>([])
  const [customerId, setCustomerId] = useState('')
  const [currency, setCurrency] = useState('COP')
  const [dueDate, setDueDate] = useState('')
  const [notes, setNotes] = useState('')
  const [items, setItems] = useState<LineItemForm[]>([{ ...INITIAL_ITEM }])
  const [customersLoading, setCustomersLoading] = useState(true)
  const [customersError, setCustomersError] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    api.listCustomers({ limit: 100 }).then(result => {
      const customerList = result.data || []
      setCustomers(customerList)
      if (customerList.length > 0) setCustomerId(customerList[0].id)
    }).catch(err => {
      setCustomersError(err instanceof Error ? err.message : 'No fue posible cargar los clientes.')
    }).finally(() => setCustomersLoading(false))
  }, [])

  const totals = useMemo(() => items.reduce((sum, item) => {
    const values = lineItemValues(item)
    if (!values) return sum
    const lineTotals = calculateItemTotals({
      quantity: values.quantity,
      unit_price: values.unit_price,
      tax_rate_bps: values.tax_rate_bps,
      discount_bps: values.discount_bps,
    })
    return {
      subtotal: sum.subtotal + lineTotals.subtotal,
      tax: sum.tax + lineTotals.tax,
      total: sum.total + lineTotals.total,
    }
  }, { subtotal: 0, tax: 0, total: 0 }), [items])

  const updateItem = (index: number, field: keyof LineItemForm, value: string | number) => {
    setItems(current => current.map((item, itemIndex) =>
      itemIndex === index ? { ...item, [field]: value } : item,
    ))
  }

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setError('')

    if (!customerId) {
      setError('Selecciona un cliente antes de continuar.')
      return
    }
    if (!dueDate) {
      setError('Selecciona una fecha de vencimiento.')
      return
    }

    const normalizedItems: (InvoiceItemInput | null)[] = items.map(item => {
      const values = lineItemValues(item)
      return values ? { description: item.description.trim(), ...values } : null
    })
    if (items.some(item => !item.description.trim())) {
      setError('Agrega una descripción para cada producto o servicio.')
      return
    }
    if (normalizedItems.some(item => item === null)) {
      setError('Revisa cantidades, precios y descuentos. Usa precios con máximo dos decimales.')
      return
    }
    const invoiceItems = normalizedItems.filter((item): item is InvoiceItemInput => item !== null)
    if (invoiceItems.some(item => !Number.isSafeInteger(item.quantity * item.unit_price))) {
      setError('El total de una línea supera el monto permitido.')
      return
    }

    const body = {
      customer_id: customerId,
      currency,
      due_date: dateAtEndOfLocalDay(dueDate),
      items: invoiceItems,
      notes: notes.trim() || undefined,
    }
    const signature = JSON.stringify(body)
    if (idempotency.current?.signature !== signature) {
      idempotency.current = { signature, key: crypto.randomUUID() }
    }

    setSaving(true)
    try {
      const invoice = await api.createInvoice(body, idempotency.current.key)
      idempotency.current = null
      navigate(`/invoices/${invoice.id}`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'No fue posible crear la factura.')
    } finally {
      setSaving(false)
    }
  }

  return (
    <section className="page page-narrow">
      <div className="page-heading">
        <div>
          <span className="eyebrow">DOCUMENTO NUEVO</span>
          <h1>Crear factura</h1>
          <p className="page-description">Completa los datos para guardar un nuevo borrador.</p>
        </div>
        <Link to="/" className="button button-secondary">Cancelar</Link>
      </div>

      <div className="notice notice-warning">
        <span className="notice-icon" aria-hidden="true">!</span>
        <p>Los impuestos son valores de cálculo genéricos. Esta factura todavía no se transmite ni se valida ante la DIAN.</p>
      </div>

      <section className="card form-card">
        {error && <div className="alert alert-error" role="alert">{error}</div>}
        {customersError && <div className="alert alert-error" role="alert">{customersError}</div>}

        {!customersLoading && customers.length === 0 && !customersError ? (
          <div className="empty-state">
            <span className="empty-state-icon" aria-hidden="true">♙</span>
            <h2>Primero agrega un cliente</h2>
            <p>Necesitas asociar la factura a un cliente de tu espacio de trabajo.</p>
            <Link to="/customers" className="button button-primary">Ir a clientes</Link>
          </div>
        ) : (
          <form className="form-stack" onSubmit={handleSubmit}>
            <div className="form-section">
              <div className="form-section-heading">
                <span className="step-number" aria-hidden="true">01</span>
                <div><h2>Información general</h2><p>Asocia el documento y define sus condiciones.</p></div>
              </div>
              <div className="form-grid form-grid-three">
                <label className="field">
                  <span>Cliente <span className="required-mark">*</span></span>
                  <select value={customerId} onChange={event => setCustomerId(event.target.value)} required disabled={customersLoading}>
                    <option value="">{customersLoading ? 'Cargando clientes…' : 'Selecciona un cliente'}</option>
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
                  <span>Vencimiento <span className="required-mark">*</span></span>
                  <input
                    type="date"
                    value={dueDate}
                    min={localDateValue()}
                    onChange={event => setDueDate(event.target.value)}
                    required
                  />
                </label>
              </div>
            </div>

            <div className="form-section">
              <div className="form-section-heading">
                <span className="step-number" aria-hidden="true">02</span>
                <div><h2>Productos y servicios</h2><p>Los valores se calculan en unidades menores de la moneda seleccionada.</p></div>
              </div>

              <div className="line-items">
                {items.map((item, index) => (
                  <fieldset className="line-item" key={index}>
                    <legend className="visually-hidden">Detalle {index + 1}</legend>
                    <div className="line-item-top">
                      <span className="line-item-label">LÍNEA {String(index + 1).padStart(2, '0')}</span>
                      {items.length > 1 && (
                        <button
                          className="text-button danger-text"
                          type="button"
                          onClick={() => setItems(current => current.filter((_, itemIndex) => itemIndex !== index))}
                          aria-label={`Eliminar línea ${index + 1}`}
                        >
                          Eliminar
                        </button>
                      )}
                    </div>
                    <div className="line-item-grid">
                      <label className="field line-description">
                        <span>Descripción <span className="required-mark">*</span></span>
                        <input
                          value={item.description}
                          onChange={event => updateItem(index, 'description', event.target.value)}
                          required
                          maxLength={240}
                          placeholder="Ej. Consultoría mensual"
                        />
                      </label>
                      <label className="field">
                        <span>Cantidad</span>
                        <input
                          type="number"
                          min="1"
                          step="1"
                          inputMode="numeric"
                          value={item.quantity}
                          onChange={event => updateItem(index, 'quantity', event.target.value)}
                          required
                        />
                      </label>
                      <label className="field">
                        <span>Precio unitario</span>
                        <span className="input-with-prefix"><span>{currency}</span>
                          <input
                            type="number"
                            min="0"
                            step="0.01"
                            inputMode="decimal"
                            value={item.unitPrice}
                            onChange={event => updateItem(index, 'unitPrice', event.target.value)}
                            required
                          />
                        </span>
                      </label>
                      <label className="field">
                        <span>Impuesto</span>
                        <select
                          value={item.taxRateBps}
                          onChange={event => updateItem(index, 'taxRateBps', Number(event.target.value))}
                        >
                          {TAX_RATES.map(rate => <option key={rate.value} value={rate.value}>{rate.label}</option>)}
                        </select>
                      </label>
                      <label className="field">
                        <span>Descuento (%)</span>
                        <input
                          type="number"
                          min="0"
                          max="100"
                          step="0.01"
                          value={item.discountPercent}
                          onChange={event => updateItem(index, 'discountPercent', event.target.value)}
                        />
                      </label>
                    </div>
                  </fieldset>
                ))}
              </div>
              <button className="button button-secondary add-line-button" type="button" onClick={() => setItems(current => [...current, { ...INITIAL_ITEM }])}>
                <span aria-hidden="true">＋</span> Agregar línea
              </button>
            </div>

            <div className="form-grid form-grid-two">
              <label className="field">
                <span>Notas internas <span className="optional-label">(opcional)</span></span>
                <textarea value={notes} onChange={event => setNotes(event.target.value)} rows={3} maxLength={1000} placeholder="Información de referencia para tu equipo" />
                <small>No aparecerán como validación ni soporte fiscal.</small>
              </label>
              <div className="totals-panel" aria-live="polite" aria-label="Resumen de valores">
                <div><span>Subtotal</span><strong>{formatMoney(totals.subtotal, currency)}</strong></div>
                <div><span>Impuestos</span><strong>{formatMoney(totals.tax, currency)}</strong></div>
                <div className="grand-total"><span>Total estimado</span><strong>{formatMoney(totals.total, currency)}</strong></div>
              </div>
            </div>

            <div className="form-actions">
              <p><span className="required-mark">*</span> Campos obligatorios</p>
              <div>
                <Link to="/" className="button button-quiet">Cancelar</Link>
                <button className="button button-primary" type="submit" disabled={saving || customersLoading || customers.length === 0}>
                  {saving ? <><span className="spinner" aria-hidden="true" /> Guardando…</> : 'Guardar borrador'}
                </button>
              </div>
            </div>
          </form>
        )}
      </section>
    </section>
  )
}
