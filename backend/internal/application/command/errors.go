package command

import (
	"errors"

	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
)

var (
	ErrCustomerNotInTenant     = errors.New("customer does not belong to tenant")
	ErrInvoiceNotInTenant      = errors.New("invoice does not belong to tenant")
	ErrSubscriptionNotInTenant = errors.New("subscription does not belong to tenant")
	ErrPaymentNotFound         = errors.New("payment not found")

	// Auth errors
	ErrInvalidEmail       = errors.New("invalid email")
	ErrPasswordTooShort   = errors.New("password must be at least 8 characters")
	ErrInvalidRole        = errors.New("invalid user role")
	ErrTenantNotFound     = errors.New("tenant not found")
	ErrEmailAlreadyExists = errors.New("email already registered for this tenant")
)

// Sentinel errors re-exported from domain for convenience.
var (
	ErrInvalidCredentials = entity.ErrInvalidCredentials
)
