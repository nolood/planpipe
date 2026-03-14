package tenant

import "time"

// Tenant represents an organization using the platform.
// Each tenant has its own user pool and configuration.
type Tenant struct {
	ID          string
	Name        string
	Slug        string // URL-safe identifier, used in subdomains
	EmailDomain string // e.g. "acme.com" — used to auto-resolve tenant during login
	IsActive    bool
	Plan        string // "free", "pro", "enterprise"
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NOTE: There is currently no per-tenant configuration storage.
// All tenant-specific settings (plan, active status) are columns
// on the tenants table. If we need arbitrary key-value config
// (e.g., feature flags, integration settings), we'd need a
// tenant_settings table or a JSONB column.
