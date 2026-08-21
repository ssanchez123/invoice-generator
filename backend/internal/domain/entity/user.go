package entity

import (
	"errors"
	"strings"
	"time"
)

// UserRole represents the role of a user within a tenant.
type UserRole string

const (
	UserRoleAdmin  UserRole = "admin"
	UserRoleMember UserRole = "member"
	UserRoleViewer UserRole = "viewer"
)

// User represents an authenticated user within a tenant.
type User struct {
	ID           string
	TenantID     string
	Email        string
	PasswordHash string
	Role         UserRole
	FirstName    string
	LastName     string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// FullName returns the user's full name, or email if names are empty.
func (u *User) FullName() string {
	first := strings.TrimSpace(u.FirstName)
	last := strings.TrimSpace(u.LastName)
	if first == "" && last == "" {
		return u.Email
	}
	if last == "" {
		return first
	}
	return first + " " + last
}

// HasRole checks if the user has a specific role.
func (u *User) HasRole(role UserRole) bool {
	return u.Role == role
}

// IsAdmin returns true if the user is an admin.
func (u *User) IsAdmin() bool {
	return u.Role == UserRoleAdmin
}

// CanCreateInvoices returns true if the user can create invoices (admin or member).
func (u *User) CanCreateInvoices() bool {
	return u.Role == UserRoleAdmin || u.Role == UserRoleMember
}

// Sentinel errors for auth domain.
var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrEmailAlreadyExists = errors.New("email already registered for this tenant")
	ErrInvalidEmail       = errors.New("invalid email format")
	ErrInvalidRole        = errors.New("invalid user role")
)
