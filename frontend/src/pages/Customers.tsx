import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, type Customer } from '../api'
import { formatDate } from '../format'

const PAGE_SIZE = 20

export default function Customers() {
  const [customers, setCustomers] = useState<Customer[]>([])
  const [offset, setOffset] = useState(0)
  const [search, setSearch] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showForm, setShowForm] = useState(false)
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)

  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [phone, setPhone] = useState('')
  const [taxId, setTaxId] = useState('')
  const [line1, setLine1] = useState('')
  const [line2, setLine2] = useState('')
  const [city, setCity] = useState('')
  const [state, setState] = useState('')
  const [country, setCountry] = useState('CO')
  const [postcode, setPostcode] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const result = await api.listCustomers({ limit: PAGE_SIZE, offset })
      setCustomers(result.data || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'No fue posible cargar los clientes.')
    } finally {
      setLoading(false)
    }
  }, [offset])

  useEffect(() => { void load() }, [load])

  const visibleCustomers = useMemo(() => {
    const query = search.trim().toLocaleLowerCase('es-CO')
    if (!query) return customers
    return customers.filter(customer =>
      `${customer.name} ${customer.email} ${customer.tax_id}`.toLocaleLowerCase('es-CO').includes(query),
    )
  }, [customers, search])

  const resetForm = () => {
    setName('')
    setEmail('')
    setPhone('')
    setTaxId('')
    setLine1('')
    setLine2('')
    setCity('')
    setState('')
    setPostcode('')
    setCountry('CO')
  }

  const handleCreate = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setError('')
    setSaved(false)
    setSaving(true)
    try {
      await api.createCustomer({
        name: name.trim(),
        email: email.trim() || undefined,
        phone: phone.trim() || undefined,
        tax_id: taxId.trim() || undefined,
        address: {
          line1: line1.trim(),
          line2: line2.trim() || undefined,
          city: city.trim(),
          state: state.trim() || undefined,
          country: country.trim().toUpperCase(),
          postcode: postcode.trim(),
        },
      })
      resetForm()
      setShowForm(false)
      setSaved(true)
      if (offset === 0) await load()
      else setOffset(0)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'No fue posible guardar el cliente.')
    } finally {
      setSaving(false)
    }
  }

  return (
    <section className="page">
      <div className="page-heading">
        <div>
          <span className="eyebrow">DIRECTORIO</span>
          <h1>Clientes</h1>
          <p className="page-description">Mantén los datos de tus clientes listos para facturar.</p>
        </div>
        <button className="button button-primary" onClick={() => { setShowForm(value => !value); setError(''); setSaved(false) }}>
          <span aria-hidden="true">{showForm ? '×' : '＋'}</span> {showForm ? 'Cerrar formulario' : 'Agregar cliente'}
        </button>
      </div>

      {saved && <div className="alert alert-success" role="status">El cliente se guardó correctamente.</div>}
      {error && !showForm && <div className="alert alert-error" role="alert"><span>{error}</span><button className="button button-small button-secondary" onClick={() => void load()}>Reintentar</button></div>}

      {showForm && (
        <section className="card customer-form-card" aria-labelledby="customer-form-heading">
          <div className="card-heading">
            <div><h2 id="customer-form-heading">Nuevo cliente</h2><p>Los campos marcados con * son obligatorios.</p></div>
            <button className="icon-button" type="button" onClick={() => setShowForm(false)} aria-label="Cerrar formulario">×</button>
          </div>
          {error && <div className="alert alert-error" role="alert">{error}</div>}
          <form className="form-stack" onSubmit={handleCreate}>
            <div className="form-section">
              <div className="form-grid form-grid-two">
                <label className="field">
                  <span>Nombre o razón social <span className="required-mark">*</span></span>
                  <input value={name} onChange={event => setName(event.target.value)} required maxLength={160} autoComplete="organization" />
                </label>
                <label className="field">
                  <span>Identificación tributaria</span>
                  <input value={taxId} onChange={event => setTaxId(event.target.value)} maxLength={40} autoComplete="off" placeholder="NIT u otro identificador" />
                </label>
                <label className="field">
                  <span>Correo electrónico</span>
                  <input type="email" value={email} onChange={event => setEmail(event.target.value)} maxLength={254} autoComplete="email" />
                </label>
                <label className="field">
                  <span>Teléfono</span>
                  <input type="tel" value={phone} onChange={event => setPhone(event.target.value)} maxLength={40} autoComplete="tel" />
                </label>
              </div>
            </div>
            <div className="form-section">
              <div className="form-section-heading compact-heading">
                <span className="step-number" aria-hidden="true">⌖</span>
                <div><h2>Dirección</h2><p>La dirección es necesaria para crear facturas.</p></div>
              </div>
              <div className="form-grid form-grid-two">
                <label className="field form-grid-wide">
                  <span>Dirección <span className="required-mark">*</span></span>
                  <input value={line1} onChange={event => setLine1(event.target.value)} required maxLength={200} autoComplete="address-line1" />
                </label>
                <label className="field form-grid-wide">
                  <span>Complemento</span>
                  <input value={line2} onChange={event => setLine2(event.target.value)} maxLength={200} autoComplete="address-line2" />
                </label>
                <label className="field">
                  <span>Ciudad <span className="required-mark">*</span></span>
                  <input value={city} onChange={event => setCity(event.target.value)} required maxLength={100} autoComplete="address-level2" />
                </label>
                <label className="field">
                  <span>Departamento / región</span>
                  <input value={state} onChange={event => setState(event.target.value)} maxLength={100} autoComplete="address-level1" />
                </label>
                <label className="field">
                  <span>Código postal</span>
                  <input value={postcode} onChange={event => setPostcode(event.target.value)} maxLength={20} autoComplete="postal-code" />
                </label>
                <label className="field">
                  <span>País (código ISO) <span className="required-mark">*</span></span>
                  <input value={country} onChange={event => setCountry(event.target.value)} required minLength={2} maxLength={2} autoCapitalize="characters" autoComplete="country" />
                </label>
              </div>
            </div>
            <div className="form-actions">
              <span className="form-action-spacer" />
              <div>
                <button className="button button-quiet" type="button" onClick={() => { resetForm(); setShowForm(false) }}>Cancelar</button>
                <button className="button button-primary" type="submit" disabled={saving}>
                  {saving ? <><span className="spinner" aria-hidden="true" /> Guardando…</> : 'Guardar cliente'}
                </button>
              </div>
            </div>
          </form>
        </section>
      )}

      <section className="card" aria-labelledby="customer-list-heading">
        <div className="card-heading">
          <div><h2 id="customer-list-heading">Directorio de clientes</h2><p>{customers.length} en esta página</p></div>
          <label className="search-field customer-search">
            <span className="visually-hidden">Buscar en esta página</span>
            <span className="search-icon" aria-hidden="true">⌕</span>
            <input type="search" value={search} onChange={event => setSearch(event.target.value)} placeholder="Nombre, correo o NIT" autoComplete="off" />
          </label>
        </div>
        {error && showForm && <p className="inline-warning" role="status">No fue posible actualizar la lista; tus datos se enviaron, vuelve a cargar para comprobar el resultado.</p>}
        {error && !showForm && <div className="alert alert-error" role="alert">{error}</div>}
        {loading ? (
          <div className="loading-state" role="status"><span className="spinner spinner-dark" aria-hidden="true" /> Cargando clientes…</div>
        ) : visibleCustomers.length === 0 ? (
          <div className="empty-state">
            <span className="empty-state-icon" aria-hidden="true">♙</span>
            <h3>{search ? 'No encontramos clientes' : 'Aún no tienes clientes'}</h3>
            <p>{search ? 'Prueba con otro nombre, correo o identificador.' : 'Agrega tus clientes para tenerlos disponibles al crear una factura.'}</p>
            {!search && <button className="button button-secondary" onClick={() => setShowForm(true)}>Agregar el primer cliente</button>}
          </div>
        ) : (
          <>
            <div className="table-scroll">
              <table className="data-table">
                <thead>
                  <tr><th scope="col">Cliente</th><th scope="col">Identificación</th><th scope="col">Teléfono</th><th scope="col">Ciudad</th><th scope="col">Agregado</th></tr>
                </thead>
                <tbody>
                  {visibleCustomers.map(customer => (
                    <tr key={customer.id}>
                      <td><Link className="customer-name customer-name-link" to={`/customers/${customer.id}`}>{customer.name}</Link><span className="table-secondary">{customer.email || 'Sin correo registrado'}</span></td>
                      <td>{customer.tax_id || '—'}</td>
                      <td>{customer.phone || '—'}</td>
                      <td>{customer.address?.city || '—'}</td>
                      <td>{formatDate(customer.created_at)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className="table-mobile-list">
              {visibleCustomers.map(customer => (
                <Link className="mobile-customer-card" key={customer.id} to={`/customers/${customer.id}`}>
                  <strong>{customer.name}</strong>
                  <span>{customer.email || 'Sin correo registrado'}</span>
                  <span>{customer.tax_id || 'Sin identificación tributaria'}</span>
                  <small>{customer.address?.city || '—'} · {formatDate(customer.created_at)}</small>
                </Link>
              ))}
            </div>
            <nav className="pagination" aria-label="Paginación de clientes">
              <span>Clientes ordenados por registro</span>
              <div>
                <button className="button button-secondary button-small" onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))} disabled={offset === 0}>Anterior</button>
                <button className="button button-secondary button-small" onClick={() => setOffset(offset + PAGE_SIZE)} disabled={customers.length < PAGE_SIZE}>Siguiente</button>
              </div>
            </nav>
          </>
        )}
      </section>
    </section>
  )
}
