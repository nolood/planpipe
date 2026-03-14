package user

import "time"

type User struct {
	ID         string
	Email      string
	Name       string
	TenantID   string
	KeycloakID string // Maps to Keycloak's user ID (sub claim)
	Role       string // "user", "admin", "viewer"
	IsActive   bool
	LastLogin  *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
