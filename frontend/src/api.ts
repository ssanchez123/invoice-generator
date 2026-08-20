// API client for the invoice-generator backend.
// All amounts are in minor units (cents). Tax rates in basis points (1900 = 19%).

const API_BASE = '/api/v1'

// Tenant ID for dev — in production this comes from JWT.
// We'll create a seed tenant in the DB with this ID.
const TENANT_ID = '00000000-0000-0000-0000-000000000001'

function headers(extra: Record<string, string> = {}): HeadersInit {
  return {
    'Content-Type': 'application/json',
    'X-Tenant-ID': TENANT_ID,
    'Authorization': 'Bearer dev-token',
    ...extra,
  }
}

async function request<T>(path: string, opts: RequestInit = {}): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...opts,
    headers: headers(opts.headers as Record<string, string>),
  })

  if (!res.ok) {
    let msg = `HTTP ${res.status}`
    try {
      const body = await res.json()
      msg = body.error || msg
    } catch { /* not JSON */ }
    throw new Error(msg)
  }

  if (res.status === 204) return undefined as T
  return res.json()
}

// ========== Types ==========

export interface Invoice {
  id: string
  number: string
  customer_id: string
  status: 'draft' | 'issued' | 'paid' | 'overdue' | 'cancelled'
  issue_date: string | null
  due_date: string
  currency: string
  subtotal: number
  tax_total: number
  total: number
  notes: string
  items?: InvoiceItem[]
  created_at: string
}

export interface InvoiceItem {
  id: string
  description: string
  quantity: number
  unit_price: number
  tax_rate_bps: number
  discount_bps: number
  total: number
}

export interface Customer {
  id: string
  name: string
  email: string
  phone: string
  tax_id: string
  address: {
    line1: string
    line2?: string
    city: string
    state?: string
    postcode: string
    country: string
  }
  created_at: string
}

export interface Payment {
  id: string
  invoice_id: string
  amount: number
  currency: string
  method: string
  reference: string
  paid_at: string
  created_at: string
}

// ========== API ==========

export const api = {
  // Invoices
  listInvoices: (params?: { status?: string; limit?: number; offset?: number }) => {
    const qs = new URLSearchParams()
    if (params?.status) qs.set('status', params.status)
    if (params?.limit) qs.set('limit', String(params.limit))
    if (params?.offset) qs.set('offset', String(params.offset))
    const q = qs.toString()
    return request<{ data: Invoice[]; total: number }>(`/invoices${q ? '?' + q : ''}`)
  },

  getInvoice: (id: string) =>
    request<Invoice>(`/invoices/${id}`),

  createInvoice: (body: {
    customer_id: string
    currency: string
    due_date: string
    items: { description: string; quantity: number; unit_price: number; tax_rate_bps: number; discount_bps?: number }[]
    notes?: string
  }) =>
    request<Invoice>('/invoices', {
      method: 'POST',
      body: JSON.stringify(body),
      headers: { 'Idempotency-Key': crypto.randomUUID() },
    }),

  issueInvoice: (id: string) =>
    request<Invoice>(`/invoices/${id}/issue`, { method: 'POST' }),

  cancelInvoice: (id: string) =>
    request<Invoice>(`/invoices/${id}/cancel`, { method: 'POST' }),

  // Payments
  listPayments: (invoiceId: string) =>
    request<{ data: Payment[] }>(`/invoices/${invoiceId}/payments`),

  recordPayment: (invoiceId: string, body: { amount: number; currency: string; method: string; reference?: string }) =>
    request<Payment>(`/invoices/${invoiceId}/payments`, {
      method: 'POST',
      body: JSON.stringify(body),
      headers: { 'Idempotency-Key': crypto.randomUUID() },
    }),

  // Customers
  listCustomers: () =>
    request<{ data: Customer[] }>('/customers'),

  createCustomer: (body: { name: string; email?: string; phone?: string; tax_id?: string; address?: Partial<Customer['address']> }) =>
    request<Customer>('/customers', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
}