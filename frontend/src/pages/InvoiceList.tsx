import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, type Customer, type Invoice } from '../api'
import StatusBadge from '../components/StatusBadge'
import { formatDate, formatMoney } from '../format'

const PAGE_SIZE = 20

export default function InvoiceList() {
  const [invoices, setInvoices] = useState<Invoice[]>([])
  const [customers, setCustomers] = useState<Customer[]>([])
  const [total, setTotal] = useState(0)
  const [offset, setOffset] = useState(0)
  const [statusFilter, setStatusFilter] = useState('')
  const [search, setSearch] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [customerError, setCustomerError] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const result = await api.listInvoices({
        status: statusFilter || undefined,
        limit: PAGE_SIZE,
        offset,
      })
      setInvoices(result.data || [])
      setTotal(result.total || 0)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'No fue posible cargar las facturas.')
    } finally {
      setLoading(false)
    }
  }, [offset, statusFilter])

  useEffect(() => { void load() }, [load])

  useEffect(() => {
    api.listCustomers({ limit: 100 })
      .then(result => {
        setCustomers(result.data || [])
        setCustomerError('')
      })
      .catch(() => setCustomerError('No se pudieron cargar los nombres de clientes.'))
  }, [])

  const customerNames = useMemo(
    () => new Map(customers.map(customer => [customer.id, customer.name])),
    [customers],
  )

  const visibleInvoices = useMemo(() => {
    const normalizedSearch = search.trim().toLocaleLowerCase('es-CO')
    if (!normalizedSearch) return invoices
    return invoices.filter(invoice => {
      const customer = customerNames.get(invoice.customer_id) || ''
      return `${invoice.number} ${customer}`.toLocaleLowerCase('es-CO').includes(normalizedSearch)
    })
  }, [customerNames, invoices, search])

  const changeStatus = (value: string) => {
    setStatusFilter(value)
    setOffset(0)
  }

  const canGoBack = offset > 0
  const canGoForward = offset + PAGE_SIZE < total

  return (
    <section className="page">
      <div className="page-heading">
        <div>
          <span className="eyebrow">CONTROL DE VENTAS</span>
          <h1>Facturas</h1>
          <p className="page-description">Consulta, organiza y da seguimiento a tus documentos.</p>
        </div>
        <Link to="/invoices/new" className="button button-primary">
          <span aria-hidden="true">＋</span> Crear factura
        </Link>
      </div>

      <div className="notice notice-info">
        <span className="notice-icon" aria-hidden="true">i</span>
        <p><strong>Información importante</strong> La emisión en esta plataforma es un estado interno y no representa validación ante la DIAN.</p>
      </div>

      <section className="card invoice-list-card" aria-labelledby="invoice-table-heading">
        <div className="card-heading">
          <div>
            <h2 id="invoice-table-heading">Todos los documentos</h2>
            <p>{statusFilter ? 'Resultados del estado seleccionado' : `${total} ${total === 1 ? 'factura registrada' : 'facturas registradas'}`}</p>
          </div>
          <div className="list-controls">
            <label className="search-field">
              <span className="visually-hidden">Buscar en esta página</span>
              <span className="search-icon" aria-hidden="true">⌕</span>
              <input
                type="search"
                value={search}
                onChange={event => setSearch(event.target.value)}
                placeholder="Buscar en esta página"
                autoComplete="off"
              />
            </label>
            <label className="select-field">
              <span className="visually-hidden">Filtrar por estado</span>
              <select value={statusFilter} onChange={event => changeStatus(event.target.value)}>
                <option value="">Todos los estados</option>
                <option value="draft">Borrador</option>
                <option value="issued">Emitida</option>
                <option value="paid">Pagada</option>
                <option value="overdue">Vencida</option>
                <option value="cancelled">Cancelada</option>
              </select>
            </label>
          </div>
        </div>

        {customerError && <div className="inline-warning" role="status">{customerError}</div>}
        {error && (
          <div className="alert alert-error" role="alert">
            <span>{error}</span>
            <button className="button button-small button-secondary" onClick={() => void load()}>Reintentar</button>
          </div>
        )}

        {loading ? (
          <div className="loading-state" role="status">
            <span className="spinner spinner-dark" aria-hidden="true" />
            <span>Cargando facturas…</span>
          </div>
        ) : !error && visibleInvoices.length === 0 ? (
          <div className="empty-state">
            <span className="empty-state-icon" aria-hidden="true">▤</span>
            <h3>{search ? 'No encontramos coincidencias' : statusFilter ? 'No hay facturas para este estado' : 'Tu primera factura empieza aquí'}</h3>
            <p>{search ? 'Prueba con otro número o nombre de cliente.' : 'Crea una factura para empezar a organizar tus ventas.'}</p>
            {!search && !statusFilter && (
              <Link to="/invoices/new" className="button button-primary">Crear factura</Link>
            )}
          </div>
        ) : (
          <>
            <div className="table-scroll">
              <table className="data-table">
                <thead>
                  <tr>
                    <th scope="col">Número / Cliente</th>
                    <th scope="col">Estado</th>
                    <th scope="col">Emisión</th>
                    <th scope="col">Vencimiento</th>
                    <th scope="col" className="align-right">Total</th>
                    <th scope="col"><span className="visually-hidden">Abrir</span></th>
                  </tr>
                </thead>
                <tbody>
                  {visibleInvoices.map(invoice => (
                    <tr key={invoice.id}>
                      <td>
                        <Link className="invoice-number" to={`/invoices/${invoice.id}`}>{invoice.number}</Link>
                        <span className="table-secondary">{customerNames.get(invoice.customer_id) || 'Cliente'}</span>
                      </td>
                      <td><StatusBadge status={invoice.status} /></td>
                      <td>{formatDate(invoice.issue_date)}</td>
                      <td>{formatDate(invoice.due_date)}</td>
                      <td className="align-right amount-cell">{formatMoney(invoice.total, invoice.currency)}</td>
                      <td className="align-right">
                        <Link className="table-action" to={`/invoices/${invoice.id}`} aria-label={`Ver factura ${invoice.number}`}>
                          Ver <span aria-hidden="true">→</span>
                        </Link>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className="table-mobile-list">
              {visibleInvoices.map(invoice => (
                <Link className="mobile-invoice-card" key={invoice.id} to={`/invoices/${invoice.id}`}>
                  <div className="mobile-invoice-top">
                    <strong>{invoice.number}</strong>
                    <StatusBadge status={invoice.status} />
                  </div>
                  <span className="table-secondary">{customerNames.get(invoice.customer_id) || 'Cliente'}</span>
                  <div className="mobile-invoice-bottom">
                    <span>Vence {formatDate(invoice.due_date)}</span>
                    <strong>{formatMoney(invoice.total, invoice.currency)}</strong>
                  </div>
                </Link>
              ))}
            </div>
            {!statusFilter && total > PAGE_SIZE && (
              <nav className="pagination" aria-label="Paginación de facturas">
                <span>Mostrando {offset + 1}–{Math.min(offset + PAGE_SIZE, total)} de {total}</span>
                <div>
                  <button className="button button-secondary button-small" onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))} disabled={!canGoBack}>
                    Anterior
                  </button>
                  <button className="button button-secondary button-small" onClick={() => setOffset(offset + PAGE_SIZE)} disabled={!canGoForward}>
                    Siguiente
                  </button>
                </div>
              </nav>
            )}
          </>
        )}
      </section>
    </section>
  )
}
