import { useCallback, useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, getStoredUser, type Customer, type InvoiceItem, type Subscription } from '../api'
import { dateAtStartOfColombiaDay, formatDate, formatMoney, parseAmountToMinorUnits } from '../format'

interface PlanItemForm {
  description: string
  quantity: string
  unitPrice: string
  taxRateBps: number
  discountPercent: string
}

const INITIAL_ITEM: PlanItemForm = {
  description: '',
  quantity: '1',
  unitPrice: '',
  taxRateBps: 1900,
  discountPercent: '0',
}

const FREQUENCIES: { value: Subscription['frequency']; label: string }[] = [
  { value: 'monthly', label: 'Mensual' },
  { value: 'yearly', label: 'Anual' },
  { value: 'weekly', label: 'Semanal' },
  { value: 'daily', label: 'Diaria' },
]

function planItemValues(item: PlanItemForm): Omit<InvoiceItem, 'id' | 'total'> | null {
  const quantity = Number(item.quantity)
  const unitPrice = parseAmountToMinorUnits(item.unitPrice)
  const discountPercent = Number(item.discountPercent)
  if (
    !Number.isSafeInteger(quantity) || quantity < 1 ||
    unitPrice === null || !Number.isFinite(discountPercent) ||
    discountPercent < 0 || discountPercent > 100
  ) return null
  return {
    description: item.description.trim(),
    quantity,
    unit_price: unitPrice,
    tax_rate_bps: item.taxRateBps,
    discount_bps: Math.round(discountPercent * 100),
  }
}

export default function Subscriptions() {
  const idempotency = useRef<{ signature: string; key: string } | null>(null)
  const role = getStoredUser()?.role
  const canManage = role === 'admin' || role === 'member'
  const [customers, setCustomers] = useState<Customer[]>([])
  const [subscriptions, setSubscriptions] = useState<Subscription[]>([])
  const [customerId, setCustomerId] = useState('')
  const [currency, setCurrency] = useState('COP')
  const [frequency, setFrequency] = useState<Subscription['frequency']>('monthly')
  const [nextDate, setNextDate] = useState('')
  const [items, setItems] = useState<PlanItemForm[]>([{ ...INITIAL_ITEM }])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')

  const load = useCallback(async () => {
    setError('')
    try {
      const [customerResult, subscriptionResult] = await Promise.all([
        api.listCustomers({ limit: 100 }),
        api.listSubscriptions({ limit: 100 }),
      ])
      const customerList = customerResult.data || []
      setCustomers(customerList)
      setCustomerId(current => current || customerList[0]?.id || '')
      setSubscriptions(subscriptionResult.data || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'No fue posible cargar las suscripciones.')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void load() }, [load])

  const updateItem = (index: number, field: keyof PlanItemForm, value: string | number) => {
    setItems(current => current.map((item, itemIndex) =>
      itemIndex === index ? { ...item, [field]: value } : item,
    ))
  }

  const create = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setError('')
    setNotice('')
    if (!customerId || !nextDate) {
      setError('Selecciona un cliente y la fecha del próximo cobro.')
      return
    }
    const planItems = items.map(planItemValues)
    if (planItems.some(item => !item || !item.description)) {
      setError('Completa la descripción, cantidad y precio de cada línea.')
      return
    }
    setSaving(true)
    try {
      const body = {
        customer_id: customerId,
        currency,
        frequency,
        next_billing_date: dateAtStartOfColombiaDay(nextDate),
        items: planItems.filter((item): item is NonNullable<typeof item> => item !== null),
      }
      const signature = JSON.stringify(body)
      if (idempotency.current?.signature !== signature) {
        idempotency.current = { signature, key: crypto.randomUUID() }
      }
      await api.createSubscription(body, idempotency.current.key)
      idempotency.current = null
      setItems([{ ...INITIAL_ITEM }])
      setNextDate('')
      setNotice('Suscripción creada. Cada vencimiento generará un borrador que debes revisar y emitir manualmente.')
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'No fue posible crear la suscripción.')
    } finally {
      setSaving(false)
    }
  }

  const cancel = async (subscription: Subscription) => {
    if (!window.confirm('¿Cancelar esta suscripción? No se generarán nuevos borradores.')) return
    setError('')
    setNotice('')
    try {
      await api.cancelSubscription(subscription.id)
      setNotice('Suscripción cancelada.')
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'No fue posible cancelar la suscripción.')
    }
  }

  return (
    <section className="page page-narrow">
      <div className="page-heading">
        <div>
          <span className="eyebrow">FACTURACIÓN RECURRENTE</span>
          <h1>Suscripciones</h1>
          <p className="page-description">Programa borradores de factura para revisar antes de emitir.</p>
        </div>
      </div>

      <div className="notice notice-warning">
        <span className="notice-icon" aria-hidden="true">!</span>
        <p>Los vencimientos crean facturas en estado borrador, con 30 días para vencimiento. No se emiten ni transmiten a la DIAN automáticamente.</p>
      </div>
      {error && <div className="alert alert-error" role="alert">{error}</div>}
      {notice && <div className="alert alert-success" role="status">{notice}</div>}

      {!canManage && <div className="notice notice-info"><span className="notice-icon" aria-hidden="true">i</span><p>Tu perfil puede consultar las suscripciones, pero no crear ni cancelar.</p></div>}
      {canManage && <section className="card form-card">
        <form className="form-stack" onSubmit={create}>
          <div className="form-section">
            <div className="form-section-heading">
              <span className="step-number" aria-hidden="true">01</span>
              <div><h2>Configuración</h2><p>Define el cliente, la periodicidad y la próxima fecha.</p></div>
            </div>
            <div className="form-grid form-grid-three">
              <label className="field">
                <span>Cliente <span className="required-mark">*</span></span>
                <select value={customerId} onChange={event => setCustomerId(event.target.value)} required>
                  <option value="">Selecciona un cliente</option>
                  {customers.map(customer => <option key={customer.id} value={customer.id}>{customer.name}</option>)}
                </select>
              </label>
              <label className="field">
                <span>Frecuencia</span>
                <select value={frequency} onChange={event => setFrequency(event.target.value as Subscription['frequency'])}>
                  {FREQUENCIES.map(option => <option key={option.value} value={option.value}>{option.label}</option>)}
                </select>
              </label>
              <label className="field">
                <span>Próximo cobro <span className="required-mark">*</span></span>
                <input type="date" value={nextDate} onChange={event => setNextDate(event.target.value)} required />
              </label>
            </div>
            <label className="field subscription-currency-field">
              <span>Moneda</span>
              <select value={currency} onChange={event => setCurrency(event.target.value)}>
                {['COP', 'USD', 'EUR', 'MXN'].map(code => <option key={code} value={code}>{code}</option>)}
              </select>
            </label>
          </div>

          <div className="form-section">
            <div className="form-section-heading">
              <span className="step-number" aria-hidden="true">02</span>
              <div><h2>Detalle recurrente</h2><p>Estos conceptos se copiarán en cada borrador.</p></div>
            </div>
            <div className="line-items">
              {items.map((item, index) => (
                <fieldset className="line-item" key={index}>
                  <legend className="visually-hidden">Detalle {index + 1}</legend>
                  <div className="line-item-top">
                    <span className="line-item-label">LÍNEA {String(index + 1).padStart(2, '0')}</span>
                    {items.length > 1 && (
                      <button className="text-button danger-text" type="button" onClick={() => setItems(current => current.filter((_, i) => i !== index))}>
                        Eliminar
                      </button>
                    )}
                  </div>
                  <div className="line-item-grid">
                    <label className="field line-description">
                      <span>Descripción</span>
                      <input value={item.description} onChange={event => updateItem(index, 'description', event.target.value)} maxLength={240} required />
                    </label>
                    <label className="field">
                      <span>Cantidad</span>
                      <input type="number" min="1" step="1" value={item.quantity} onChange={event => updateItem(index, 'quantity', event.target.value)} required />
                    </label>
                    <label className="field">
                      <span>Precio unitario ({currency})</span>
                      <input type="number" min="0" step="0.01" value={item.unitPrice} onChange={event => updateItem(index, 'unitPrice', event.target.value)} required />
                    </label>
                    <label className="field">
                      <span>IVA</span>
                      <select value={item.taxRateBps} onChange={event => updateItem(index, 'taxRateBps', Number(event.target.value))}>
                        <option value={0}>0%</option><option value={500}>5%</option><option value={1900}>19%</option>
                      </select>
                    </label>
                    <label className="field">
                      <span>Descuento (%)</span>
                      <input type="number" min="0" max="100" step="0.01" value={item.discountPercent} onChange={event => updateItem(index, 'discountPercent', event.target.value)} />
                    </label>
                  </div>
                </fieldset>
              ))}
            </div>
            <button className="button button-secondary add-line-button" type="button" onClick={() => setItems(current => [...current, { ...INITIAL_ITEM }])}>
              <span aria-hidden="true">＋</span> Agregar línea
            </button>
          </div>
          <div className="form-actions">
            <Link to="/" className="button button-quiet">Volver</Link>
            <button className="button button-primary" type="submit" disabled={saving || loading || customers.length === 0}>
              {saving ? 'Guardando…' : 'Crear suscripción'}
            </button>
          </div>
        </form>
      </section>}

      <section className="card" aria-labelledby="subscriptions-list-heading">
        <div className="card-heading"><div><h2 id="subscriptions-list-heading">Suscripciones registradas</h2><p>{subscriptions.length} en este espacio de trabajo</p></div></div>
        {loading ? <p className="muted-copy">Cargando suscripciones…</p> : subscriptions.length === 0 ? (
          <p className="muted-copy subscription-empty-copy">Todavía no hay suscripciones.</p>
        ) : (
          <div className="table-scroll">
            <table className="data-table">
              <thead><tr><th scope="col">Cliente</th><th scope="col">Frecuencia</th><th scope="col">Próximo borrador</th><th scope="col">Estado</th><th scope="col"><span className="visually-hidden">Acciones</span></th></tr></thead>
              <tbody>
                {subscriptions.map(subscription => {
                  const customer = customers.find(entry => entry.id === subscription.customer_id)
                  const total = subscription.plan_items.reduce((sum, item) => sum + item.unit_price * item.quantity, 0)
                  return (
                    <tr key={subscription.id}>
                      <td><strong>{customer?.name || 'Cliente'}</strong><span className="table-secondary">{formatMoney(total, subscription.currency)} base</span></td>
                      <td>{FREQUENCIES.find(option => option.value === subscription.frequency)?.label || subscription.frequency}</td>
                      <td>{formatDate(subscription.next_billing_date)}</td>
                      <td>{subscription.status === 'active' ? 'Activa' : subscription.status === 'paused' ? 'Pausada' : 'Cancelada'}</td>
                      <td className="align-right">
                        {canManage && subscription.status === 'active' && <button className="text-button danger-text" type="button" onClick={() => void cancel(subscription)}>Cancelar</button>}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </section>
  )
}
