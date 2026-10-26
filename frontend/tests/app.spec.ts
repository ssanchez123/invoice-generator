import { expect, test, type Page } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'

const customer = {
  id: 'customer-1',
  name: 'Comercial Andina S.A.S.',
  email: 'pagos@example.com',
  phone: '+57 300 000 0000',
  tax_id: '900123456-7',
  address: {
    line1: 'Carrera 10 # 20-30',
    city: 'Bogotá',
    country: 'CO',
    postcode: '110111',
  },
  created_at: '2026-01-15T12:00:00Z',
}

const invoice = {
  id: 'invoice-1',
  number: 'INV-2026-0001',
  customer_id: customer.id,
  status: 'issued' as const,
  issue_date: '2026-10-01T12:00:00Z',
  due_date: '2026-10-30T23:59:59Z',
  currency: 'COP',
  subtotal: 100000,
  tax_total: 19000,
  total: 119000,
  items: [{
    id: 'item-1',
    description: 'Servicios profesionales',
    quantity: 1,
    unit_price: 100000,
    tax_rate_bps: 1900,
    discount_bps: 0,
  }],
  notes: '',
  created_at: '2026-10-01T12:00:00Z',
}

async function seedSession(page: Page) {
  await page.addInitScript(() => {
    sessionStorage.setItem('ig_access_token', 'browser-test-token')
    sessionStorage.setItem('ig_token_expires_at', String(Date.now() + 60 * 60 * 1000))
    sessionStorage.setItem('ig_user', JSON.stringify({
      id: 'user-1',
      tenant_id: 'tenant-1',
      email: 'admin@example.com',
      role: 'admin',
      first_name: 'Ana',
      last_name: 'Gómez',
      full_name: 'Ana Gómez',
    }))
  })
}

async function mockApi(page: Page) {
  let payments: Array<Record<string, unknown>> = []
  let customers = [customer]
  let subscriptions: Array<Record<string, unknown>> = []
  const draftInvoice = {
    ...invoice,
    id: 'invoice-draft',
    number: 'INV-2026-0003',
    status: 'draft' as const,
    issue_date: null,
  }

  await page.route('**/api/v1/**', async route => {
    const request = route.request()
    const url = new URL(request.url())
    const method = request.method()
    const path = url.pathname

    if (path === '/api/v1/auth/login' && method === 'POST') {
      await route.fulfill({
        json: {
          access_token: 'browser-test-token',
          token_type: 'Bearer',
          expires_in: 3600,
          user: {
            id: 'user-1',
            tenant_id: 'tenant-1',
            email: 'admin@example.com',
            role: 'admin',
            first_name: 'Ana',
            last_name: 'Gómez',
            full_name: 'Ana Gómez',
          },
        },
      })
      return
    }

    if (path === '/api/v1/invoices' && method === 'GET') {
      await route.fulfill({ json: { data: [invoice], total: 1 } })
      return
    }
    if (path === '/api/v1/invoices' && method === 'POST') {
      await route.fulfill({ status: 201, json: { ...invoice, id: 'invoice-created', number: 'INV-2026-0002', status: 'draft' } })
      return
    }
    if (path === `/api/v1/invoices/${invoice.id}` && method === 'GET') {
      await route.fulfill({ json: invoice })
      return
    }
    if (path === `/api/v1/invoices/${draftInvoice.id}` && method === 'GET') {
      await route.fulfill({ json: draftInvoice })
      return
    }
    if (path === `/api/v1/invoices/${draftInvoice.id}` && method === 'PATCH') {
      const body = request.postDataJSON() as Record<string, unknown>
      await route.fulfill({ json: { ...draftInvoice, ...body } })
      return
    }
    if (path === '/api/v1/customers' && method === 'GET') {
      await route.fulfill({ json: { data: customers } })
      return
    }
    if (path === '/api/v1/customers' && method === 'POST') {
      const body = request.postDataJSON() as Record<string, unknown>
      const createdCustomer = { ...customer, ...body, id: 'customer-created', created_at: '2026-10-22T12:00:00Z' }
      customers = [createdCustomer, ...customers]
      await route.fulfill({ status: 201, json: createdCustomer })
      return
    }
    if (path === `/api/v1/customers/${customer.id}` && method === 'GET') {
      await route.fulfill({ json: customer })
      return
    }
    if (path === `/api/v1/invoices/${invoice.id}/payments` && method === 'GET') {
      await route.fulfill({ json: { data: payments } })
      return
    }
    if (path === `/api/v1/invoices/${invoice.id}/payments` && method === 'POST') {
      const body = request.postDataJSON() as Record<string, unknown>
      const payment = {
        id: 'payment-1',
        invoice_id: invoice.id,
        amount: body.amount,
        currency: body.currency,
        method: body.method,
        reference: body.reference || '',
        paid_at: '2026-10-02T12:00:00Z',
        created_at: '2026-10-02T12:00:00Z',
      }
      payments = [payment]
      await route.fulfill({ status: 201, json: payment })
      return
    }
    if (path === '/api/v1/subscriptions' && method === 'GET') {
      await route.fulfill({ json: { data: subscriptions, total: subscriptions.length } })
      return
    }
    if (path === '/api/v1/subscriptions' && method === 'POST') {
      const body = request.postDataJSON() as Record<string, unknown>
      const subscription = {
        id: 'subscription-1',
        tenant_id: 'tenant-1',
        customer_id: customer.id,
        plan_items: body.items,
        currency: body.currency,
        frequency: body.frequency,
        next_billing_date: body.next_billing_date,
        billing_day: 15,
        status: 'active',
        created_at: '2026-10-22T12:00:00Z',
        updated_at: '2026-10-22T12:00:00Z',
      }
      subscriptions = [subscription]
      await route.fulfill({ status: 201, json: subscription })
      return
    }
    if (path === '/api/v1/subscriptions/subscription-1' && method === 'DELETE') {
      subscriptions = subscriptions.map(subscription => ({ ...subscription, status: 'cancelled' }))
      await route.fulfill({ json: { status: 'cancelled' } })
      return
    }
    if (path === '/api/v1/audit' && method === 'GET') {
      await route.fulfill({
        json: {
          data: [{
            id: 'audit-1',
            tenant_id: 'tenant-1',
            entity_type: 'invoice',
            entity_id: invoice.id,
            action: 'invoice.issued',
            actor_id: 'user-1',
            payload: { status: 'issued' },
            created_at: '2026-10-01T12:00:00Z',
          }],
          total: 1,
        },
      })
      return
    }
    await route.fulfill({ status: 404, json: { error: 'Mock endpoint not found' } })
  })
}

test('requires authentication before opening a protected route', async ({ page }) => {
  await page.goto('/invoices/new')
  await expect(page).toHaveURL(/\/login$/)
  await expect(page.getByRole('heading', { name: 'Inicia sesión' })).toBeVisible()
})

test('signs in and shows the invoice workspace', async ({ page }) => {
  await mockApi(page)
  await page.goto('/login')
  await page.getByLabel('Identificador del espacio').fill('tenant-1')
  await page.getByLabel('Correo electrónico').fill('admin@example.com')
  await page.getByRole('textbox', { name: 'Contraseña' }).fill('local-test-password')
  await page.getByRole('button', { name: 'Entrar a mi cuenta' }).click()

  await expect(page).toHaveURL(/\/$/)
  await expect(page.getByRole('heading', { name: 'Facturas' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'INV-2026-0001', exact: true })).toBeVisible()
})

test('creates an invoice with integer minor-unit amounts', async ({ page }) => {
  await seedSession(page)
  await mockApi(page)
  await page.goto('/invoices/new')
  await page.getByLabel('Descripción *').fill('Servicio mensual')
  await page.getByLabel('Precio unitario').fill('1250.50')
  await page.getByLabel('Vencimiento *').fill('2026-12-31')
  const createRequest = page.waitForRequest(request =>
    request.url().endsWith('/api/v1/invoices') && request.method() === 'POST',
  )
  await page.getByRole('button', { name: 'Guardar borrador' }).click()
  const request = await createRequest
  const body = request.postDataJSON() as { items: { unit_price: number }[] }

  expect(body.items[0].unit_price).toBe(125050)
  await expect(page).toHaveURL(/\/invoices\/invoice-created$/)
})

test('updates only a draft invoice using integer minor-unit amounts', async ({ page }) => {
  await seedSession(page)
  await mockApi(page)
  await page.goto('/invoices/invoice-draft/edit')
  await expect(page.getByRole('heading', { name: 'Editar factura' })).toBeVisible()
  await page.getByLabel('Precio unitario (COP)').fill('1500.25')
  const updateRequest = page.waitForRequest(request =>
    request.url().endsWith('/api/v1/invoices/invoice-draft') && request.method() === 'PATCH',
  )
  await page.getByRole('button', { name: 'Guardar cambios' }).click()
  const request = await updateRequest
  const body = request.postDataJSON() as { items: { unit_price: number }[] }
  expect(body.items[0].unit_price).toBe(150025)
  await expect(page).toHaveURL(/\/invoices\/invoice-draft$/)
})

test('creates a recurring subscription as a draft-only schedule', async ({ page }) => {
  await seedSession(page)
  await mockApi(page)
  await page.goto('/subscriptions')
  await expect(page.getByRole('heading', { name: 'Suscripciones', exact: true })).toBeVisible()
  await page.getByLabel('Próximo cobro *').fill('2026-12-15')
  await page.getByLabel('Descripción').fill('Servicio mensual')
  await page.getByLabel('Precio unitario (COP)').fill('250000')
  const createRequest = page.waitForRequest(request =>
    request.url().endsWith('/api/v1/subscriptions') && request.method() === 'POST',
  )
  await page.getByRole('button', { name: 'Crear suscripción' }).click()
  const request = await createRequest
  const body = request.postDataJSON() as { items: { unit_price: number }[]; next_billing_date: string }
  expect(body.items[0].unit_price).toBe(25000000)
  expect(body.next_billing_date).toBe('2026-12-15T05:00:00.000Z')
  await expect(page.getByRole('status')).toContainText('generará un borrador')
  await expect(page.getByText('Comercial Andina S.A.S.').last()).toBeVisible()
})

test('admin can inspect the immutable audit log', async ({ page }) => {
  await seedSession(page)
  await mockApi(page)
  await page.goto('/audit')
  await expect(page.getByRole('heading', { name: 'Registro de auditoría' })).toBeVisible()
  await expect(page.getByText('invoice.issued')).toBeVisible()
})

test('records a payment in minor units and refreshes the balance', async ({ page }) => {
  await seedSession(page)
  await mockApi(page)
  await page.goto(`/invoices/${invoice.id}`)
  await expect(page.getByRole('heading', { name: /Factura INV-2026-0001/ })).toBeVisible()
  await page.getByLabel('Monto (COP)').fill('100.00')
  const paymentRequest = page.waitForRequest(request =>
    request.url().endsWith(`/api/v1/invoices/${invoice.id}/payments`) && request.method() === 'POST',
  )
  await page.getByRole('button', { name: 'Registrar pago' }).click()
  const request = await paymentRequest
  const body = request.postDataJSON() as { amount: number }

  expect(body.amount).toBe(10000)
  await expect(page.getByRole('status')).toContainText('El pago se registró correctamente.')
  await expect(page.getByRole('heading', { name: 'Saldo pendiente' })).toBeVisible()
})

test('creates a customer and refreshes the customer directory', async ({ page }) => {
  await seedSession(page)
  await mockApi(page)
  await page.goto('/customers')
  await page.getByRole('button', { name: 'Agregar cliente' }).click()
  await page.getByLabel('Nombre o razón social *').fill('Cliente nuevo S.A.S.')
  await page.getByLabel('Dirección *').fill('Calle 10 # 20-30')
  await page.getByLabel('Ciudad *').fill('Bogotá')
  await page.getByRole('button', { name: 'Guardar cliente' }).click()

  await expect(page.getByText('El cliente se guardó correctamente.', { exact: true })).toBeVisible()
  await expect(page.getByText('Cliente nuevo S.A.S.', { exact: true }).first()).toBeVisible()
})

test('opens a customer profile with its linked invoices', async ({ page }) => {
  await seedSession(page)
  await mockApi(page)
  await page.goto('/customers')
  await page.getByRole('link', { name: 'Comercial Andina S.A.S.', exact: true }).click()

  await expect(page).toHaveURL(/\/customers\/customer-1$/)
  await expect(page.getByRole('heading', { name: 'Comercial Andina S.A.S.', exact: true })).toBeVisible()
  await expect(page.getByRole('link', { name: 'pagos@example.com' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'INV-2026-0001', exact: true })).toBeVisible()
})

test('login page has no automated WCAG A/AA violations', async ({ page }) => {
  await page.goto('/login')
  const results = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'])
    .analyze()
  expect(results.violations).toEqual([])
})

test('invoice list has no automated WCAG A/AA violations', async ({ page }) => {
  await seedSession(page)
  await mockApi(page)
  await page.goto('/')
  await expect(page.getByRole('heading', { name: 'Facturas' })).toBeVisible()
  const results = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'])
    .analyze()
  expect(results.violations).toEqual([])
})

test('new invoice form has no automated WCAG A/AA violations', async ({ page }) => {
  await seedSession(page)
  await mockApi(page)
  await page.goto('/invoices/new')
  await expect(page.getByRole('heading', { name: 'Crear factura' })).toBeVisible()
  const results = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'])
    .analyze()
  expect(results.violations).toEqual([])
})

test('customer directory has no automated WCAG A/AA violations', async ({ page }) => {
  await seedSession(page)
  await mockApi(page)
  await page.goto('/customers')
  await expect(page.getByRole('heading', { name: 'Clientes', exact: true })).toBeVisible()
  const results = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'])
    .analyze()
  expect(results.violations).toEqual([])
})

test('customer detail has no automated WCAG A/AA violations', async ({ page }) => {
  await seedSession(page)
  await mockApi(page)
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto(`/customers/${customer.id}`)
  await expect(page.getByRole('heading', { name: customer.name, exact: true })).toBeVisible()
  const results = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'])
    .analyze()
  expect(results.violations).toEqual([])
})

test('subscriptions page has no automated WCAG A/AA violations', async ({ page }) => {
  await seedSession(page)
  await mockApi(page)
  await page.goto('/subscriptions')
  await expect(page.getByRole('heading', { name: 'Suscripciones', exact: true })).toBeVisible()
  const results = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'])
    .analyze()
  expect(results.violations).toEqual([])
})

test('invoice detail has no automated WCAG A/AA violations', async ({ page }) => {
  await seedSession(page)
  await mockApi(page)
  await page.goto(`/invoices/${invoice.id}`)
  await expect(page.getByRole('heading', { name: /Factura INV-2026-0001/ })).toBeVisible()
  const results = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'])
    .analyze()
  expect(results.violations).toEqual([])
})
