import { useEffect, useState, useCallback } from 'react'
import { api, type Customer } from '../api'

function formatDate(s: string): string {
  return new Date(s).toLocaleDateString('es-CO', { day: '2-digit', month: 'short', year: 'numeric' })
}

export default function Customers() {
  const [customers, setCustomers] = useState<Customer[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showForm, setShowForm] = useState(false)

  // Form state
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [phone, setPhone] = useState('')
  const [taxId, setTaxId] = useState('')
  const [line1, setLine1] = useState('')
  const [city, setCity] = useState('')
  const [country, setCountry] = useState('CO')
  const [postcode, setPostcode] = useState('')
  const [saving, setSaving] = useState(false)

  const load = useCallback(async () => {
    setLoading(true); setError('')
    try {
      const res = await api.listCustomers()
      setCustomers(res.data || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Error al cargar clientes')
    } finally { setLoading(false) }
  }, [])

  useEffect(() => { load() }, [load])

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) { setError('Nombre es obligatorio'); return }
    if (!line1.trim() || !city.trim()) { setError('Dirección y ciudad son obligatorias'); return }
    setSaving(true); setError('')
    try {
      await api.createCustomer({
        name,
        email: email || undefined,
        phone: phone || undefined,
        tax_id: taxId || undefined,
        address: { line1, city, country, postcode },
      })
      setName(''); setEmail(''); setPhone(''); setTaxId(''); setLine1(''); setCity(''); setPostcode('')
      setShowForm(false)
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Error al crear cliente')
    } finally { setSaving(false) }
  }

  const fieldStyle = { padding: '8px 12px', border: '1px solid #ddd', borderRadius: 6, fontSize: 14 }

  return (
    <div>
      <div className="page-header">
        <h2>Clientes</h2>
        <button onClick={() => setShowForm(!showForm)} className="btn">
          {showForm ? 'Cancelar' : '+ Nuevo Cliente'}
        </button>
      </div>

      {error && <div style={{ color: '#c00', marginBottom: 16 }}>{error}</div>}

      {showForm && (
        <div className="card">
          <form onSubmit={handleCreate} style={{ maxWidth: 'none' }}>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
              <label><span>Nombre *</span><input value={name} onChange={e => setName(e.target.value)} required style={fieldStyle} /></label>
              <label><span>Email</span><input type="email" value={email} onChange={e => setEmail(e.target.value)} style={fieldStyle} /></label>
              <label><span>Teléfono</span><input value={phone} onChange={e => setPhone(e.target.value)} style={fieldStyle} /></label>
              <label><span>Tax ID (NIT/RFC/EIN)</span><input value={taxId} onChange={e => setTaxId(e.target.value)} style={fieldStyle} /></label>
              <label><span>Dirección *</span><input value={line1} onChange={e => setLine1(e.target.value)} required style={fieldStyle} /></label>
              <label><span>Ciudad *</span><input value={city} onChange={e => setCity(e.target.value)} required style={fieldStyle} /></label>
              <label><span>Código postal</span><input value={postcode} onChange={e => setPostcode(e.target.value)} style={fieldStyle} /></label>
              <label><span>País</span><input value={country} onChange={e => setCountry(e.target.value)} maxLength={2} style={fieldStyle} /></label>
            </div>
            <div style={{ marginTop: 16 }}>
              <button type="submit" disabled={saving} style={{ opacity: saving ? 0.6 : 1 }}>
                {saving ? 'Guardando...' : 'Guardar Cliente'}
              </button>
            </div>
          </form>
        </div>
      )}

      <div className="card">
        {loading ? (
          <p>Cargando...</p>
        ) : customers.length === 0 ? (
          <p style={{ color: '#999', textAlign: 'center', padding: '32px' }}>
            No hay clientes. Crea uno para poder facturar.
          </p>
        ) : (
          <table>
            <thead>
              <tr>
                <th>Nombre</th>
                <th>Email</th>
                <th>Tax ID</th>
                <th>Creado</th>
              </tr>
            </thead>
            <tbody>
              {customers.map(c => (
                <tr key={c.id}>
                  <td style={{ fontWeight: 600 }}>{c.name}</td>
                  <td>{c.email || '—'}</td>
                  <td>{c.tax_id || '—'}</td>
                  <td>{formatDate(c.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}