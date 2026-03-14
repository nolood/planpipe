package auth

import "time"

// Session represents an active user session. Currently not persisted
// server-side — sessions are managed entirely by Keycloak.
// This struct exists for potential future use (session tracking, audit).
type Session struct {
	ID        string
	UserID    string
	TenantID  string
	CreatedAt time.Time
	ExpiresAt time.Time
}
