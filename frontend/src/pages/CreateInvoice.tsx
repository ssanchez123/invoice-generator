import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, type Customer } from '../api'

interface LineItem {
  description: string
  quantity: number
  unit_price: number
  tax_rate_bps: number
}

const CURRENCIES = ['USD', 'EUR', 'COP', 'MXN']
const TAX_RATES = [
  { label: 'Exento (0%)', value: 0 },
  { label: 'IVA 19% (CO)', value: 1900 },
  { label: 'IVA 16% (MX)', value: 1600 },
  { label: 'IVA 21% (ES)', value: 2100 },
  { label: 'VAT 20% (EU)', value: 2000 },
]

export default function CreateInvoice() {
  const navigate = useNavigate()
  const [customers, setCustomers] = useState<Customer[]>([])
  const [customerId, setCustomerId] = useState('')
  const [currency, setCurrency] = useState('USD')
  const [dueDate, setDueDate] = useState('')
  const [notes, setNotes] = useState('')
  const [items, setItems] = useState<LineItem[]>([
    { description: '', quantity: 1, unit_price: 0, tax_rate_bps: 0 },
  ])
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    api.listCustomers().then(res => {
      setCustomers(res.data || [])
      if (res.data && res.data.length > 0) setCustomerId(res.data[0].id)
    }).catch(err => setError(err.message))
  }, [])

  const addItem = () => setItems([...items, { description: '', quantity: 1, unit_price: 0, tax_rate_bps: 0 }])
  const removeItem = (i: number) => setItems(items.filter((_, idx) => idx !== i))
  const updateItem = (i: number, field: keyof LineItem, value: string | number) =>
    setItems(items.map((it, idx) => idx === i ? { ...it, [field]: value } : it))

  const subtotal = items.reduce((sum, it) => sum + it.quantity * it.unit_price, 0)
  const taxTotal = items.reduce((sum, it) => {
    const lineSub = it.quantity * it.unit_price
    return sum + Math.round((lineSub * it.tax_rate_bps) / 10000)
  }, 0)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!customerId) { setError('Selecciona un cliente'); return }
    if (!dueDate) { setError('Selecciona fecha de vencimiento'); return }
    if (items.some(it => !it.description.trim())) { setError('Todos los items necesitan descripción'); return }

    setSaving(true)
    setError('')
    try {
      const inv = await api.createInvoice({
        customer_id: customerId,
        currency,
        due_date: new Date(dueDate + 'T23:59:59').toISOString(),
        items: items.map(it => ({
          description: it.description,
          quantity: it.quantity,
          unit_price: it.unit_price,
          tax_rate_bps: it.tax_rate_bps,
        })),
        notes,
      })
      navigate(`/invoices/${inv.id}`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Error al crear factura')
    } finally {
      setSaving(false)
    }
  }

  const fieldStyle = { padding: '8px 12px', border: '1px solid #ddd', borderRadius: 6, fontSize: 14 }

  return (
    <div>
      <div className="page-header">
        <h2>Nueva Factura</h2>
      </div>
      <div className="card">
        {error && <div style={{ color: '#c00', marginBottom: 16, padding: 12, background: '#fff0f0', borderRadius: 6 }}>{error}</div>}
        <form onSubmit={handleSubmit} style={{ maxWidth: 'none' }}>
          {/* Customer + currency + due date */}
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 16, marginBottom: 24 }}>
            <label>
              <span>Cliente</span>
              <select value={customerId} onChange={e => setCustomerId(e.target.value)} required style={fieldStyle}>
                <option value="">Selecciona...</option>
                {customers.map(c => <option key={c.id} value={c.id}>{c.name}</option>)}
              </select>
            </label>
            <label>
              <span>Moneda</span>
              <select value={currency} onChange={e => setCurrency(e.target.value)} style={fieldStyle}>
                {CURRENCIES.map(c => <option key={c} value={c}>{c}</option>)}
              </select>
            </label>
            <label>
              <span>Fecha de vencimiento</span>
              <input type="date" min={new Date().toISOString().split('T')[0]} value={dueDate} onChange={e => setDueDate(e.target.value)} required style={fieldStyle} />
            </label>
          </div>

          {/* Line items */}
          <h3 style={{ marginTop: 24, marginBottom: 12 }}>Items</h3>
          <div style={{ display: 'grid', gridTemplateColumns: '3fr 80px 140px 160px 40px', gap: 8, marginBottom: 8, fontSize: 12, fontWeight: 600, color: '#666', textTransform: 'uppercase' }}>
            <span>Descripción</span>
            <span>Cantidad</span>
            <span>Precio unit. (cents)</span>
            <span>Tasa de impuesto</span>
            <span></span>
          </div>
          {items.map((item, i) => (
            <div key={i} style={{ display: 'grid', gridTemplateColumns: '3fr 80px 140px 160px 40px', gap: 8, marginBottom: 8, alignItems: 'center' }}>
              <input
                placeholder="Descripción del producto/servicio"
                value={item.description}
                onChange={e => updateItem(i, 'description', e.target.value)}
                style={fieldStyle}
              />
              <input
                type="number"
                min={1}
                value={item.quantity}
                onChange={e => updateItem(i, 'quantity', Number(e.target.value))}
                style={{ ...fieldStyle, width: 80, textAlign: 'center' }}
              />
              <input
                type="number"
                min={0}
                value={item.unit_price}
                onChange={e => updateItem(i, 'unit_price', Number(e.target.value))}
                style={{ ...fieldStyle, width: 140, textAlign: 'right' }}
              />
              <select
                value={item.tax_rate_bps}
                onChange={e => updateItem(i, 'tax_rate_bps', Number(e.target.value))}
                style={{ ...fieldStyle, width: 160 }}
              >
                {TAX_RATES.map(r => <option key={r.value} value={r.value}>{r.label}</option>)}
              </select>
              {items.length > 1 && (
                <button type="button" onClick={() => removeItem(i)} style={{ background: '#cc0000', width: 40, padding: '8px 0' }}>×</button>
              )}
            </div>
          ))}
          <button type="button" onClick={addItem} style={{ background: '#666', marginTop: 4 }}>+ Agregar item</button>

          {/* Totals */}
          <div style={{ marginTop: 24, padding: 16, background: '#f9f9f9', borderRadius: 8 }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', padding: '4px 0' }}>
              <span>Subtotal:</span>
              <span>{(subtotal / 100).toFixed(2)} {currency}</span>
            </div>
            <div style={{ display: 'flex', justifyContent: 'space-between', padding: '4px 0' }}>
              <span>Impuestos:</span>
              <span>{(taxTotal / 100).toFixed(2)} {currency}</span>
            </div>
            <div style={{ display: 'flex', justifyContent: 'space-between', padding: '8px 0 4px', fontWeight: 700, borderTop: '1px solid #ddd', marginTop: 8 }}>
              <span>Total:</span>
              <span>{((subtotal + taxTotal) / 100).toFixed(2)} {currency}</span>
            </div>
          </div>

          {/* Notes */}
          <label style={{ marginTop: 16 }}>
            <span>Notas (opcional)</span>
            <input type="text" value={notes} onChange={e => setNotes(e.target.value)} placeholder="Notas internas..." style={fieldStyle} />
          </label>

          {/* Submit */}
          <div style={{ marginTop: 24, display: 'flex', gap: 12 }}>
            <button type="submit" disabled={saving} style={{ opacity: saving ? 0.6 : 1 }}>
              {saving ? 'Creando...' : 'Crear Factura'}
            </button>
            <button type="button" onClick={() => navigate('/')} style={{ background: '#666' }}>Cancelar</button>
          </div>
        </form>
      </div>
    </div>
  )
}