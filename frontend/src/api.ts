const API_BASE = (import.meta.env.VITE_API_BASE_URL || '/api/v1').replace(/\/+$/, '')

const TOKEN_KEY = 'ig_access_token'
const USER_KEY = 'ig_user'
const EXPIRY_KEY = 'ig_token_expires_at'

export interface AuthUser {
  id: string
  tenant_id: string
  email: string
  role: string
  first_name: string
  last_name: string
  full_name: string
}

interface LoginResponse {
  access_token: string
  token_type: string
  expires_in: number
  user: AuthUser
}

export interface InvoiceItem {
  id: string
  description: string
  quantity: number
  unit_price: number
  tax_rate_bps: number
  discount_bps: number
  total?: number
}

export interface Invoice {
  id: string
  number: string
  customer_id: string
  status: 'draft' | 'issued' | 'paid' | 'overdue' | 'cancelled'
  issue_date?: string | null
  due_date: string
  currency: string
  subtotal: number
  tax_total: number
  total: number
  notes?: string
  metadata?: Record<string, unknown>
  items?: InvoiceItem[]
  created_at: string
  updated_at?: string
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

export interface Subscription {
  id: string
  tenant_id: string
  customer_id: string
  plan_items: InvoiceItem[]
  currency: string
  frequency: 'daily' | 'weekly' | 'monthly' | 'yearly'
  next_billing_date: string
  billing_day: number
  status: 'active' | 'paused' | 'cancelled'
  created_at: string
  updated_at: string
}

export interface AuditEntry {
  id: string
  tenant_id: string
  entity_type: string
  entity_id: string
  action: string
  actor_id: string
  payload: Record<string, unknown>
  created_at: string
}

export class ApiError extends Error {
  constructor(message: string, public readonly status: number) {
    super(message)
    this.name = 'ApiError'
  }
}

function clearToken(): void {
  sessionStorage.removeItem(TOKEN_KEY)
  sessionStorage.removeItem(USER_KEY)
  sessionStorage.removeItem(EXPIRY_KEY)
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(USER_KEY)
}

function getToken(): string | null {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(USER_KEY)
  const token = sessionStorage.getItem(TOKEN_KEY)
  if (!token) return null

  const expiresAt = Number(sessionStorage.getItem(EXPIRY_KEY))
  if (!Number.isFinite(expiresAt) || expiresAt <= Date.now()) {
    clearToken()
    return null
  }

  return token
}

export function getStoredUser(): AuthUser | null {
  if (!getToken()) return null
  const raw = sessionStorage.getItem(USER_KEY)
  if (!raw) return null

  try {
    const user: unknown = JSON.parse(raw)
    if (isAuthUser(user)) {
      return user
    }
  } catch {
    clearToken()
  }
  clearToken()
  return null
}

function isAuthUser(user: unknown): user is AuthUser {
  return typeof user === 'object' && user !== null &&
    'id' in user && typeof user.id === 'string' &&
    'tenant_id' in user && typeof user.tenant_id === 'string' &&
    'email' in user && typeof user.email === 'string' &&
    'role' in user && typeof user.role === 'string' &&
    'first_name' in user && typeof user.first_name === 'string' &&
    'last_name' in user && typeof user.last_name === 'string' &&
    'full_name' in user && typeof user.full_name === 'string'
}

function isLoginResponse(response: unknown): response is LoginResponse {
  return typeof response === 'object' && response !== null &&
    'access_token' in response && typeof response.access_token === 'string' && response.access_token.trim().length > 0 &&
    'token_type' in response && typeof response.token_type === 'string' && response.token_type.trim().length > 0 &&
    'expires_in' in response && typeof response.expires_in === 'number' && Number.isFinite(response.expires_in) && response.expires_in > 0 &&
    'user' in response && isAuthUser(response.user)
}

export function isAuthenticated(): boolean {
  return getToken() !== null
}

export function logout(): void {
  clearToken()
}

function saveSession(response: LoginResponse): void {
  sessionStorage.setItem(TOKEN_KEY, response.access_token)
  sessionStorage.setItem(USER_KEY, JSON.stringify(response.user))
  sessionStorage.setItem(
    EXPIRY_KEY,
    String(Date.now() + response.expires_in * 1000),
  )
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(USER_KEY)
}

function requestHeaders(extra?: HeadersInit, hasBody = false): Headers {
  const headers = new Headers(extra)
  headers.set('Accept', 'application/json')
  if (hasBody && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }

  const token = getToken()
  if (token) headers.set('Authorization', `Bearer ${token}`)
  return headers
}

async function errorMessage(response: Response): Promise<string> {
  try {
    const body: unknown = await response.json()
    if (typeof body === 'object' && body !== null && 'error' in body && typeof body.error === 'string') {
      return body.error
    }
  } catch {
    // Fall through to the status-based message for non-JSON error responses.
  }
  return `Error de solicitud (${response.status})`
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: requestHeaders(options.headers, Boolean(options.body)),
  })

  if (response.status === 401) {
    clearToken()
    if (window.location.pathname !== '/login' && path !== '/auth/login') {
      window.location.assign('/login')
    }
  }

  if (!response.ok) {
    throw new ApiError(await errorMessage(response), response.status)
  }
  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}

export async function downloadInvoicePDF(invoiceId: string): Promise<void> {
  const headers = requestHeaders()
  headers.set('Accept', 'application/pdf')
  const response = await fetch(`${API_BASE}/invoices/${encodeURIComponent(invoiceId)}/pdf`, {
    headers,
  })

  if (response.status === 401) {
    clearToken()
    if (window.location.pathname !== '/login') window.location.assign('/login')
  }
  if (!response.ok) {
    throw new ApiError(await errorMessage(response), response.status)
  }

  const blob = await response.blob()
  const disposition = response.headers.get('Content-Disposition')
  const filename = disposition?.match(/filename="?([^";]+)"?/i)?.[1] ?? `invoice-${invoiceId}.pdf`
  const objectUrl = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = objectUrl
  link.download = filename
  link.click()
  window.setTimeout(() => URL.revokeObjectURL(objectUrl), 1000)
}

export const api = {
  login: async (tenantId: string, email: string, password: string): Promise<LoginResponse> => {
    const response: unknown = await request<unknown>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ tenant_id: tenantId, email, password }),
    })
    if (!isLoginResponse(response)) {
      throw new ApiError('La respuesta de autenticación no tiene un formato válido.', 502)
    }
    saveSession(response)
    return response
  },

  listInvoices: (params: { status?: string; customer_id?: string; limit?: number; offset?: number } = {}) => {
    const query = new URLSearchParams()
    if (params.status) query.set('status', params.status)
    if (params.customer_id) query.set('customer_id', params.customer_id)
    if (params.limit !== undefined) query.set('limit', String(params.limit))
    if (params.offset !== undefined) query.set('offset', String(params.offset))
    const suffix = query.size > 0 ? `?${query.toString()}` : ''
    return request<{ data: Invoice[]; total: number }>(`/invoices${suffix}`)
  },

  getInvoice: (id: string) => request<Invoice>(`/invoices/${encodeURIComponent(id)}`),

  createInvoice: (
    body: {
      customer_id: string
      currency: string
      due_date: string
      items: {
        description: string
        quantity: number
        unit_price: number
        tax_rate_bps: number
        discount_bps?: number
      }[]
      notes?: string
    },
    idempotencyKey: string = crypto.randomUUID(),
  ) =>
    request<Invoice>('/invoices', {
      method: 'POST',
      body: JSON.stringify(body),
      headers: { 'Idempotency-Key': idempotencyKey },
    }),

  issueInvoice: (id: string) =>
    request<Invoice>(`/invoices/${encodeURIComponent(id)}/issue`, { method: 'POST' }),

  updateInvoice: (
    id: string,
    body: {
      customer_id: string
      currency: string
      due_date: string
      items: Omit<InvoiceItem, 'id' | 'total'>[]
      notes?: string
    },
  ) =>
    request<Invoice>(`/invoices/${encodeURIComponent(id)}`, {
      method: 'PATCH',
      body: JSON.stringify(body),
    }),

  cancelInvoice: (id: string) =>
    request<{ status: string }>(`/invoices/${encodeURIComponent(id)}/cancel`, { method: 'POST' }),

  listPayments: (invoiceId: string) =>
    request<{ data: Payment[] }>(`/invoices/${encodeURIComponent(invoiceId)}/payments`),

  recordPayment: (
    invoiceId: string,
    body: { amount: number; currency: string; method: string; reference?: string },
    idempotencyKey: string = crypto.randomUUID(),
  ) =>
    request<Payment>(`/invoices/${encodeURIComponent(invoiceId)}/payments`, {
      method: 'POST',
      body: JSON.stringify(body),
      headers: { 'Idempotency-Key': idempotencyKey },
    }),

  listCustomers: (params: { limit?: number; offset?: number } = {}) => {
    const query = new URLSearchParams()
    if (params.limit !== undefined) query.set('limit', String(params.limit))
    if (params.offset !== undefined) query.set('offset', String(params.offset))
    const suffix = query.size > 0 ? `?${query.toString()}` : ''
    return request<{ data: Customer[] }>(`/customers${suffix}`)
  },

  getCustomer: (id: string) => request<Customer>(`/customers/${encodeURIComponent(id)}`),

  createCustomer: (body: {
    name: string
    email?: string
    phone?: string
    tax_id?: string
    address?: Partial<Customer['address']>
  }) =>
    request<Customer>('/customers', {
      method: 'POST',
      body: JSON.stringify(body),
    }),

  listSubscriptions: (params: { status?: string; limit?: number; offset?: number } = {}) => {
    const query = new URLSearchParams()
    if (params.status) query.set('status', params.status)
    if (params.limit !== undefined) query.set('limit', String(params.limit))
    if (params.offset !== undefined) query.set('offset', String(params.offset))
    const suffix = query.size > 0 ? `?${query.toString()}` : ''
    return request<{ data: Subscription[]; total: number }>(`/subscriptions${suffix}`)
  },

  createSubscription: (
    body: {
      customer_id: string
      currency: string
      frequency: Subscription['frequency']
      next_billing_date: string
      items: Omit<InvoiceItem, 'id' | 'total'>[]
    },
    idempotencyKey: string = crypto.randomUUID(),
  ) =>
    request<Subscription>('/subscriptions', {
      method: 'POST',
      body: JSON.stringify(body),
      headers: { 'Idempotency-Key': idempotencyKey },
    }),

  cancelSubscription: (id: string, idempotencyKey: string = crypto.randomUUID()) =>
    request<{ status: string }>(`/subscriptions/${encodeURIComponent(id)}`, {
      method: 'DELETE',
      headers: { 'Idempotency-Key': idempotencyKey },
    }),

  listAuditEntries: (params: { entity_type?: string; entity_id?: string; action?: string; limit?: number; offset?: number } = {}) => {
    const query = new URLSearchParams()
    if (params.entity_type) query.set('entity_type', params.entity_type)
    if (params.entity_id) query.set('entity_id', params.entity_id)
    if (params.action) query.set('action', params.action)
    if (params.limit !== undefined) query.set('limit', String(params.limit))
    if (params.offset !== undefined) query.set('offset', String(params.offset))
    const suffix = query.size > 0 ? `?${query.toString()}` : ''
    return request<{ data: AuditEntry[]; total: number }>(`/audit${suffix}`)
  },
}
