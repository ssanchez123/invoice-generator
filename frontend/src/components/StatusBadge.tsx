import type { Invoice } from '../api'

const STATUS_LABELS: Record<Invoice['status'], string> = {
  draft: 'Borrador',
  issued: 'Emitida',
  paid: 'Pagada',
  overdue: 'Vencida',
  cancelled: 'Cancelada',
}

export default function StatusBadge({ status }: { status: Invoice['status'] }) {
  return <span className={`status-badge status-${status}`}>{STATUS_LABELS[status] ?? status}</span>
}
