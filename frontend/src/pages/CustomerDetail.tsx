import { useCallback, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api, type Customer, type Invoice } from '../api'
import StatusBadge from '../components/StatusBadge'
import { formatDate, formatMoney } from '../format'

const PAGE_SIZE = 20

export default function CustomerDetail() {
  const { id } = useParams<{ id: string }>()
  const [customer, setCustomer] = useState<Customer | null>(null)
  const [invoices, setInvoices] = useState<Invoice[]>([])
  const [invoiceTotal, setInvoiceTotal] = useState(0)
  const [offset, setOffset] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [invoiceError, setInvoiceError] = useState('')

  const load = useCallback(async () => {
    if (!id) {
      setError('No se encontró el identificador del cliente.')
      setLoading(false)
      return
    }

    setLoading(true)
    setError('')
    setInvoiceError('')
    const [customerResult, invoicesResult] = await Promise.allSettled([
      api.getCustomer(id),
      api.listInvoices({ customer_id: id, limit: PAGE_SIZE, offset }),
    ])

    if (customerResult.status === 'fulfilled') {
      setCustomer(customerResult.value)
    } else {
      setCustomer(null)
      setError(customerResult.reason instanceof Error
        ? customerResult.reason.message
        : 'No fue posible cargar el cliente.')
    }

    if (invoicesResult.status === 'fulfilled') {
      setInvoices(invoicesResult.value.data || [])
      setInvoiceTotal(invoicesResult.value.total || 0)
    } else {
      setInvoices([])
      setInvoiceTotal(0)
      setInvoiceError(invoicesResult.reason instanceof Error
        ? invoicesResult.reason.message
        : 'No fue posible cargar las facturas del cliente.')
    }
    setLoading(false)
  }, [id, offset])

  useEffect(() => { void load() }, [load])

  return (
    <section className="page">
      {customer && (
        <div className="detail-back-link">
          <Link to="/customers">← Volver al directorio de clientes</Link>
        </div>
      )}

      {loading && !customer && (
        <div className="loading-state" role="status">
          <span className="spinner spinner-dark" aria-hidden="true" />
          Cargando cliente…
        </div>
      )}

      {error && !customer && (
        <div className="alert alert-error" role="alert">
          <span>{error}</span>
          <button className="button button-small button-secondary" onClick={() => void load()}>Reintentar</button>
        </div>
      )}

      {customer && (
        <>
          <div className="page-heading">
            <div>
              <span className="eyebrow">PERFIL DEL CLIENTE</span>
              <h1>{customer.name}</h1>
              <p className="page-description">Información de contacto y documentos asociados.</p>
            </div>
          </div>

          <div className="customer-detail-grid">
            <section className="card" aria-labelledby="customer-contact-heading">
              <div className="card-heading">
                <div><h2 id="customer-contact-heading">Contacto</h2><p>Datos registrados del cliente.</p></div>
              </div>
              <dl className="customer-facts">
                <div><dt>Identificación tributaria</dt><dd>{customer.tax_id || 'No registrada'}</dd></div>
                <div>
                  <dt>Correo electrónico</dt>
                  <dd>{customer.email
                    ? <a href={`mailto:${customer.email}`}>{customer.email}</a>
                    : 'No registrado'}</dd>
                </div>
                <div>
                  <dt>Teléfono</dt>
                  <dd>{customer.phone
                    ? <a href={`tel:${customer.phone}`}>{customer.phone}</a>
                    : 'No registrado'}</dd>
                </div>
                <div><dt>Agregado</dt><dd>{formatDate(customer.created_at)}</dd></div>
              </dl>
            </section>

            <section className="card" aria-labelledby="customer-address-heading">
              <div className="card-heading">
                <div><h2 id="customer-address-heading">Dirección</h2><p>Domicilio registrado.</p></div>
              </div>
              <address className="customer-detail-address">
                <span>{customer.address?.line1 || 'Sin dirección registrada'}</span>
                {customer.address?.line2 && <span>{customer.address.line2}</span>}
                <span>{[customer.address?.city, customer.address?.state].filter(Boolean).join(', ') || '—'}</span>
                <span>{[customer.address?.postcode, customer.address?.country].filter(Boolean).join(' · ') || '—'}</span>
              </address>
            </section>
          </div>

          <section className="card invoice-list-card" aria-labelledby="customer-invoices-heading">
            <div className="card-heading">
              <div>
                <h2 id="customer-invoices-heading">Facturas asociadas</h2>
                <p>{invoiceTotal} {invoiceTotal === 1 ? 'factura' : 'facturas'}</p>
              </div>
            </div>

            {invoiceError && (
              <div className="alert alert-error" role="alert">
                <span>{invoiceError}</span>
                <button className="button button-small button-secondary" onClick={() => void load()}>Reintentar</button>
              </div>
            )}

            {loading ? (
              <div className="loading-state" role="status">
                <span className="spinner spinner-dark" aria-hidden="true" />
                Cargando facturas…
              </div>
            ) : !invoiceError && invoices.length === 0 ? (
              <div className="empty-state">
                <span className="empty-state-icon" aria-hidden="true">▤</span>
                <h3>Aún no hay facturas</h3>
                <p>Las facturas asociadas a este cliente aparecerán aquí.</p>
              </div>
            ) : !invoiceError && invoices.length > 0 ? (
              <>
                <div className="table-scroll">
                  <table className="data-table">
                    <thead>
                      <tr>
                        <th scope="col">Factura</th>
                        <th scope="col">Estado</th>
                        <th scope="col">Emisión</th>
                        <th scope="col">Vencimiento</th>
                        <th scope="col" className="align-right">Total</th>
                      </tr>
                    </thead>
                    <tbody>
                      {invoices.map(invoice => (
                        <tr key={invoice.id}>
                          <td><Link className="invoice-number" to={`/invoices/${invoice.id}`}>{invoice.number}</Link></td>
                          <td><StatusBadge status={invoice.status} /></td>
                          <td>{formatDate(invoice.issue_date)}</td>
                          <td>{formatDate(invoice.due_date)}</td>
                          <td className="align-right amount-cell">{formatMoney(invoice.total, invoice.currency)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                <div className="table-mobile-list">
                  {invoices.map(invoice => (
                    <Link className="mobile-invoice-card" key={invoice.id} to={`/invoices/${invoice.id}`}>
                      <div className="mobile-invoice-top">
                        <strong>{invoice.number}</strong>
                        <StatusBadge status={invoice.status} />
                      </div>
                      <div className="mobile-invoice-bottom">
                        <span>Vence {formatDate(invoice.due_date)}</span>
                        <strong>{formatMoney(invoice.total, invoice.currency)}</strong>
                      </div>
                    </Link>
                  ))}
                </div>
                {invoiceTotal > PAGE_SIZE && (
                  <nav className="pagination" aria-label="Paginación de facturas del cliente">
                    <span>Mostrando {offset + 1}–{Math.min(offset + PAGE_SIZE, invoiceTotal)} de {invoiceTotal}</span>
                    <div>
                      <button className="button button-secondary button-small" onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))} disabled={offset === 0}>
                        Anterior
                      </button>
                      <button className="button button-secondary button-small" onClick={() => setOffset(offset + PAGE_SIZE)} disabled={offset + PAGE_SIZE >= invoiceTotal}>
                        Siguiente
                      </button>
                    </div>
                  </nav>
                )}
              </>
            ) : null}
          </section>
        </>
      )}
    </section>
  )
}
