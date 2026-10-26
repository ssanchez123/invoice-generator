import { useCallback, useEffect, useState } from 'react'
import { api, type AuditEntry } from '../api'
import { formatDate } from '../format'

export default function AuditLog() {
  const [entries, setEntries] = useState<AuditEntry[]>([])
  const [entityType, setEntityType] = useState('')
  const [action, setAction] = useState('')
  const [appliedFilters, setAppliedFilters] = useState({ entityType: '', action: '' })
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const result = await api.listAuditEntries({
        entity_type: appliedFilters.entityType || undefined,
        action: appliedFilters.action.trim() || undefined,
        limit: 100,
      })
      setEntries(result.data || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'No fue posible cargar la auditoría.')
    } finally {
      setLoading(false)
    }
  }, [appliedFilters])

  useEffect(() => { void load() }, [load])

  return (
    <section className="page">
      <div className="page-heading">
        <div>
          <span className="eyebrow">CONTROL DEL ESPACIO</span>
          <h1>Registro de auditoría</h1>
          <p className="page-description">Historial de acciones sobre facturas, pagos y suscripciones.</p>
        </div>
      </div>
      <div className="notice notice-info">
        <span className="notice-icon" aria-hidden="true">i</span>
        <p>El registro es de solo lectura. Los eventos de facturación automática identifican acciones del sistema sin usuario asociado.</p>
      </div>
      {error && <div className="alert alert-error" role="alert">{error}</div>}
      <section className="card">
        <form className="form-grid form-grid-three" onSubmit={event => {
          event.preventDefault()
          setAppliedFilters({ entityType, action })
        }}>
          <label className="field">
            <span>Tipo de entidad</span>
            <select value={entityType} onChange={event => setEntityType(event.target.value)}>
              <option value="">Todos</option><option value="invoice">Factura</option><option value="subscription">Suscripción</option><option value="tenant">Espacio</option>
            </select>
          </label>
          <label className="field">
            <span>Acción</span>
            <input value={action} onChange={event => setAction(event.target.value)} maxLength={100} placeholder="Ej. invoice.issued" />
          </label>
          <div className="field audit-filter-action">
            <span aria-hidden="true">&nbsp;</span>
            <button className="button button-secondary" type="submit" disabled={loading}>Aplicar filtros</button>
          </div>
        </form>
      </section>
      <section className="card" aria-labelledby="audit-results-heading">
        <div className="card-heading"><div><h2 id="audit-results-heading">Eventos recientes</h2><p>Se muestran hasta 100 registros.</p></div></div>
        {loading ? <p className="muted-copy" role="status">Cargando historial…</p> : entries.length === 0 ? (
          <p className="muted-copy">No hay eventos para estos filtros.</p>
        ) : (
          <div className="table-scroll">
            <table className="data-table">
              <thead><tr><th scope="col">Fecha</th><th scope="col">Acción</th><th scope="col">Entidad</th><th scope="col">Identificador</th><th scope="col">Actor</th><th scope="col">Detalle</th></tr></thead>
              <tbody>
                {entries.map(entry => (
                  <tr key={entry.id}>
                    <td>{formatDate(entry.created_at)}</td>
                    <td><strong>{entry.action}</strong></td>
                    <td>{entry.entity_type}</td>
                    <td><code className="mono-text">{entry.entity_id}</code></td>
                    <td>{entry.actor_id ? <code className="mono-text">{entry.actor_id}</code> : 'Sistema'}</td>
                    <td><details><summary>Ver datos</summary><pre className="audit-payload">{JSON.stringify(entry.payload, null, 2)}</pre></details></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </section>
  )
}
