package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/acme/platform/internal/tenant"
)

type contextKey string

const (
	ContextKeyUserID   contextKey = "user_id"
	ContextKeyTenantID contextKey = "tenant_id"
	ContextKeyRoles    contextKey = "roles"
)

// Middleware handles JWT-based authentication for all protected routes.
// It validates the token against Keycloak and injects user context.
type Middleware struct {
	keycloak  *KeycloakClient
	tenantSvc *tenant.Service
}

func NewMiddleware(kc *KeycloakClient, ts *tenant.Service) *Middleware {
	return &Middleware{
		keycloak:  kc,
		tenantSvc: ts,
	}
}

// Authenticate validates the Bearer token from the Authorization header.
// On success, it populates the request context with user_id, tenant_id, and roles.
// On failure, it returns 401 Unauthorized.
func (m *Middleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if header == "" {
			http.Error(w, "missing authorization header", http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(header, "Bearer ")
		if token == header {
			http.Error(w, "invalid authorization format", http.StatusUnauthorized)
			return
		}

		// Validate token against Keycloak's introspection endpoint
		claims, err := m.keycloak.ValidateToken(r.Context(), token)
		if err != nil {
			log.Warn().Err(err).Msg("token validation failed")
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		// Resolve tenant from the token claims
		tenantID := claims.TenantID
		if tenantID == "" {
			// Fallback: try to resolve tenant from the user's email domain
			t, err := m.tenantSvc.GetByEmailDomain(r.Context(), claims.Email)
			if err != nil {
				log.Error().Err(err).Str("email", claims.Email).Msg("failed to resolve tenant")
				http.Error(w, "tenant resolution failed", http.StatusForbidden)
				return
			}
			tenantID = t.ID
		}

		// Verify that the tenant is active
		tnt, err := m.tenantSvc.GetByID(r.Context(), tenantID)
		if err != nil || !tnt.IsActive {
			http.Error(w, "tenant inactive or not found", http.StatusForbidden)
			return
		}

		ctx := context.WithValue(r.Context(), ContextKeyUserID, claims.Subject)
		ctx = context.WithValue(ctx, ContextKeyTenantID, tenantID)
		ctx = context.WithValue(ctx, ContextKeyRoles, claims.RealmRoles)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole returns middleware that checks if the authenticated user
// has the specified role in their Keycloak realm roles.
func (m *Middleware) RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			roles, ok := r.Context().Value(ContextKeyRoles).([]string)
			if !ok {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			for _, r := range roles {
				if r == role {
					next.ServeHTTP(w, r.WithContext(r.Context()))
					return
				}
			}

			http.Error(w, "insufficient permissions", http.StatusForbidden)
		})
	}
}
