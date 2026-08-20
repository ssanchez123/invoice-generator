package command

import "errors"

var (
	ErrCustomerNotInTenant = errors.New("customer does not belong to tenant")
	ErrInvoiceNotInTenant  = errors.New("invoice does not belong to tenant")
	ErrPaymentNotFound     = errors.New("payment not found")
)
