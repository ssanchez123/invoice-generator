import type { InvoiceItem } from './api'

export function formatMoney(amountMinor: number, currency: string): string {
  const safeAmount = Number.isFinite(amountMinor) ? amountMinor : 0
  try {
    return new Intl.NumberFormat('es-CO', {
      style: 'currency',
      currency: currency || 'COP',
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    }).format(safeAmount / 100)
  } catch {
    return `${(safeAmount / 100).toFixed(2)} ${currency || 'COP'}`
  }
}

export function formatDate(value?: string | null): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime()) || date.getUTCFullYear() < 2) return '—'
  return new Intl.DateTimeFormat('es-CO', {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
  }).format(date)
}

export function localDateValue(date = new Date()): string {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

export function dateAtEndOfLocalDay(value: string): string {
  return new Date(`${value}T23:59:59`).toISOString()
}

export function dateAtStartOfColombiaDay(value: string): string {
  return new Date(`${value}T00:00:00-05:00`).toISOString()
}

export function parseAmountToMinorUnits(value: string): number | null {
  if (!/^\d+(?:\.\d{0,2})?$/.test(value)) return null
  const [whole, fraction = ''] = value.split('.')
  const amount = Number(whole) * 100 + Number(fraction.padEnd(2, '0'))
  return Number.isSafeInteger(amount) ? amount : null
}

export function calculateItemTotals(item: Pick<
  InvoiceItem,
  'quantity' | 'unit_price' | 'tax_rate_bps' | 'discount_bps'
>): { subtotal: number; tax: number; total: number } {
  const gross = item.quantity * item.unit_price
  const discount = Math.round((gross * item.discount_bps) / 10_000)
  const subtotal = gross - discount
  const tax = Math.round((subtotal * item.tax_rate_bps) / 10_000)
  return { subtotal, tax, total: subtotal + tax }
}
