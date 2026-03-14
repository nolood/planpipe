package auth

import (
	"context"
	"fmt"

	"github.com/Nerzal/gocloak/v13"

	"github.com/acme/platform/internal/config"
)

// KeycloakClient wraps the GoCloak library to provide authentication
// operations against a Keycloak instance. Currently supports only
// direct grant (password) authentication.
type KeycloakClient struct {
	client   *gocloak.GoCloak
	realm    string
	clientID string
	secret   string
}

func NewKeycloakClient(cfg config.KeycloakConfig) *KeycloakClient {
	client := gocloak.NewClient(cfg.BaseURL)
	return &KeycloakClient{
		client:   client,
		realm:    cfg.Realm,
		clientID: cfg.ClientID,
		secret:   cfg.ClientSecret,
	}
}

// TokenClaims holds the parsed claims from a validated Keycloak JWT.
type TokenClaims struct {
	Subject    string   // Keycloak user ID
	Email      string
	TenantID   string   // Custom claim: tenant_id
	RealmRoles []string // Keycloak realm-level roles
}

// Authenticate performs a direct grant (password) login against Keycloak.
// Returns access token, refresh token, and expiry information.
func (kc *KeycloakClient) Authenticate(ctx context.Context, username, password string) (*gocloak.JWT, error) {
	token, err := kc.client.Login(ctx, kc.clientID, kc.secret, kc.realm, username, password)
	if err != nil {
		return nil, fmt.Errorf("keycloak login failed: %w", err)
	}
	return token, nil
}

// ValidateToken introspects a token and returns parsed claims.
// Uses Keycloak's token introspection endpoint.
func (kc *KeycloakClient) ValidateToken(ctx context.Context, accessToken string) (*TokenClaims, error) {
	result, err := kc.client.RetrospectToken(ctx, accessToken, kc.clientID, kc.secret, kc.realm)
	if err != nil {
		return nil, fmt.Errorf("token introspection failed: %w", err)
	}

	if !*result.Active {
		return nil, fmt.Errorf("token is not active")
	}

	// Decode the token to extract claims
	_, claims, err := kc.client.DecodeAccessToken(ctx, accessToken, kc.realm)
	if err != nil {
		return nil, fmt.Errorf("token decode failed: %w", err)
	}

	tokenClaims := &TokenClaims{
		Subject: getStringClaim(claims, "sub"),
		Email:   getStringClaim(claims, "email"),
	}

	// Extract custom tenant_id claim if present
	if tid, ok := (*claims)["tenant_id"].(string); ok {
		tokenClaims.TenantID = tid
	}

	// Extract realm roles
	if realmAccess, ok := (*claims)["realm_access"].(map[string]interface{}); ok {
		if roles, ok := realmAccess["roles"].([]interface{}); ok {
			for _, role := range roles {
				if r, ok := role.(string); ok {
					tokenClaims.RealmRoles = append(tokenClaims.RealmRoles, r)
				}
			}
		}
	}

	return tokenClaims, nil
}

// Logout invalidates a refresh token in Keycloak.
func (kc *KeycloakClient) Logout(ctx context.Context, refreshToken string) error {
	return kc.client.Logout(ctx, kc.clientID, kc.secret, kc.realm, refreshToken)
}

// RefreshToken exchanges a refresh token for a new token pair.
func (kc *KeycloakClient) RefreshToken(ctx context.Context, refreshToken string) (*gocloak.JWT, error) {
	token, err := kc.client.RefreshToken(ctx, refreshToken, kc.clientID, kc.secret, kc.realm)
	if err != nil {
		return nil, fmt.Errorf("token refresh failed: %w", err)
	}
	return token, nil
}

func getStringClaim(claims *map[string]interface{}, key string) string {
	if v, ok := (*claims)[key].(string); ok {
		return v
	}
	return ""
}
