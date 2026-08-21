import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import App from './App'
import Login from './pages/Login'
import InvoiceList from './pages/InvoiceList'
import InvoiceDetail from './pages/InvoiceDetail'
import CreateInvoice from './pages/CreateInvoice'
import Customers from './pages/Customers'
import { isAuthenticated } from './api'
import './index.css'

// ProtectedRoute wraps routes that require authentication
function ProtectedRoute({ children }: { children: React.ReactNode }) {
  if (!isAuthenticated()) {
    return <Navigate to="/login" replace />
  }
  return <>{children}</>
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="/" element={<ProtectedRoute><App /></ProtectedRoute>}>
          <Route index element={<InvoiceList />} />
          <Route path="invoices/new" element={<CreateInvoice />} />
          <Route path="invoices/:id" element={<InvoiceDetail />} />
          <Route path="customers" element={<Customers />} />
        </Route>
      </Routes>
    </BrowserRouter>
  </React.StrictMode>,
)
