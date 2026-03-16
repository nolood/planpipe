# Implementation Design: SAML SSO for Multi-Tenant Platform

## Overview

This document describes the implementation design for adding SAML 2.0 Single Sign-On to the multi-tenant Go backend platform. Keycloak acts as the SAML Service Provider (SP) and identity broker. The existing email/password login flow remains fully functional. SSO configuration is per-tenant, stored in a dedicated `tenant_sso_config` table.

## Architecture

### Current Authentication Flow

```
User -> POST /api/auth/login (email+password)
     -> auth.Handler.Login()
     -> auth.Service.Login()
        -> tenant.Service.GetByEmailDomain() -- resolve tenant
        -> keycloak.Authenticate() -- direct grant (ROPC)
        -> user.Service.GetOrCreateByEmail() -- JIT provision
     -> JWT tokens returned to client
```

### New SSO Authentication Flow

```
User -> GET /api/auth/sso/check?email=user@acme.com
     -> auth.Handler.CheckSSO()
     -> auth.Service.CheckSSO()
        -> tenant.Service.GetByEmailDomain() -- resolve tenant
        -> tenant.Service.GetSSOConfig() -- check if SSO enabled
     -> Returns { sso_enabled: true, initiate_url: "/api/auth/sso/initiate?email=..." }

User -> GET /api/auth/sso/initiate?email=user@acme.com
     -> auth.Handler.InitiateSSO()
     -> auth.Service.InitiateSSO()
        -> tenant.Service.GetByEmailDomain() -- resolve tenant
        -> tenant.Service.GetSSOConfig() -- load SSO config
        -> keycloak.BuildSSORedirectURL() -- construct Keycloak auth URL with IdP hint
     -> HTTP 302 redirect to Keycloak authorization endpoint with kc_idp_hint={idp_alias}

Keycloak -> brokers SAML with tenant's corporate IdP
         -> user authenticates at corporate IdP
         -> Keycloak receives SAML assertion
         -> Keycloak issues authorization code

User -> GET /api/auth/sso/callback?code=xxx&state=yyy&session_state=zzz
     -> auth.Handler.SSOCallback()
     -> auth.Service.HandleSSOCallback()
        -> keycloak.ExchangeCode() -- exchange authorization code for tokens
        -> keycloak.ValidateToken() -- extract claims from access token
        -> tenant.Service.GetByEmailDomain() -- resolve tenant from email claim
        -> user.Service.GetOrCreateByEmailWithKeycloakID() -- JIT provision + link
     -> Returns HTML page that posts tokens to frontend via window.postMessage
        (or redirects to frontend with tokens in fragment)
```

### Flow Diagram

```
  Browser              Go Backend             Keycloak              Corporate IdP
    |                      |                      |                      |
    |-- check SSO -------->|                      |                      |
    |<-- sso_enabled ------|                      |                      |
    |                      |                      |                      |
    |-- initiate SSO ----->|                      |                      |
    |                      |-- build auth URL ---->|                      |
    |<-- 302 redirect -----|                      |                      |
    |                      |                      |                      |
    |-- follow redirect -------------------------------->|               |
    |                      |                      |-- SAML AuthnReq ---->|
    |<-------------------------------------------------- login page ----|
    |-- credentials --------------------------------------------------->|
    |                      |                      |<-- SAML Assertion ---|
    |                      |                      |-- validate, map ---->|
    |<-- 302 callback + code ----------------------|                     |
    |                      |                      |                      |
    |-- callback(code) --->|                      |                      |
    |                      |-- exchange code ----->|                      |
    |                      |<-- JWT tokens --------|                      |
    |                      |-- JIT provision ----->|                      |
    |<-- JWT tokens -------|                      |                      |
```

## Component Design

### 1. Database Schema: `tenant_sso_config` Table

**File:** `migrations/002_sso_config.sql`

```sql
CREATE TABLE tenant_sso_config (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL UNIQUE REFERENCES tenants(id) ON DELETE CASCADE,
    enabled         BOOLEAN NOT NULL DEFAULT false,
    idp_alias       TEXT NOT NULL,          -- Keycloak IdP alias, e.g. "saml-acme"
    idp_display_name TEXT NOT NULL,         -- Human-readable name, e.g. "Acme Corp SSO"
    metadata_url    TEXT,                   -- IdP SAML metadata URL (optional, for auto-config)
    metadata_xml    TEXT,                   -- IdP SAML metadata XML (if URL not available)
    entity_id       TEXT NOT NULL,          -- IdP entity ID from SAML metadata
    sso_url         TEXT NOT NULL,          -- IdP SSO endpoint URL
    certificate     TEXT NOT NULL,          -- IdP X.509 signing certificate (PEM)
    name_id_format  TEXT NOT NULL DEFAULT 'urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress',
    sign_requests   BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tenant_sso_config_tenant_id ON tenant_sso_config(tenant_id);
CREATE INDEX idx_tenant_sso_config_idp_alias ON tenant_sso_config(idp_alias);
```

**Design rationale:** A dedicated table (not JSONB) per user's explicit decision. One-to-one with tenants via `UNIQUE` on `tenant_id`. The `ON DELETE CASCADE` ensures cleanup when a tenant is removed. Fields capture the minimum SAML IdP metadata needed for Keycloak IdP configuration.

### 2. SSO Config Model

**File:** `internal/tenant/sso_config.go` (new file)

```go
package tenant

import "time"

// SSOConfig holds the SAML SSO configuration for a tenant.
// Stored in the tenant_sso_config table. One config per tenant.
type SSOConfig struct {
    ID             string
    TenantID       string
    Enabled        bool
    IdPAlias       string    // Keycloak identity provider alias
    IdPDisplayName string    // Human-readable IdP name
    MetadataURL    string    // Optional: SAML metadata URL for auto-config
    MetadataXML    string    // Optional: raw SAML metadata XML
    EntityID       string    // IdP entity ID
    SSOURL         string    // IdP SSO endpoint
    Certificate    string    // IdP X.509 signing certificate (PEM)
    NameIDFormat   string    // SAML NameID format
    SignRequests   bool      // Whether to sign SAML AuthnRequests
    CreatedAt      time.Time
    UpdatedAt      time.Time
}
```

### 3. SSO Config Repository

**File:** `internal/tenant/sso_repository.go` (new file)

```go
package tenant

// SSORepository handles persistence for tenant SSO configurations.
type SSORepository struct {
    db *pgxpool.Pool
}

func NewSSORepository(db *pgxpool.Pool) *SSORepository { ... }

// GetByTenantID returns the SSO config for a tenant, or nil if none exists.
func (r *SSORepository) GetByTenantID(ctx context.Context, tenantID string) (*SSOConfig, error)

// GetByIdPAlias returns the SSO config matching a Keycloak IdP alias.
func (r *SSORepository) GetByIdPAlias(ctx context.Context, alias string) (*SSOConfig, error)

// Create inserts a new SSO configuration.
func (r *SSORepository) Create(ctx context.Context, cfg *SSOConfig) error

// Update modifies an existing SSO configuration.
func (r *SSORepository) Update(ctx context.Context, cfg *SSOConfig) error

// Delete removes an SSO configuration by tenant ID.
func (r *SSORepository) Delete(ctx context.Context, tenantID string) error
```

### 4. Tenant Service Extension

**File:** `internal/tenant/service.go` (modified)

Add SSO-specific methods to the existing `Service`:

```go
// Extend Service struct to include SSORepository
type Service struct {
    repo    *Repository
    ssoRepo *SSORepository
}

// NewService updated to accept SSORepository
func NewService(repo *Repository, ssoRepo *SSORepository) *Service

// GetSSOConfig returns the SSO configuration for a tenant, or nil if not configured.
func (s *Service) GetSSOConfig(ctx context.Context, tenantID string) (*SSOConfig, error)

// GetSSOConfigByEmailDomain resolves tenant from email, then returns SSO config.
// Returns nil SSOConfig (without error) if the tenant exists but has no SSO.
func (s *Service) GetSSOConfigByEmailDomain(ctx context.Context, email string) (*Tenant, *SSOConfig, error)

// SaveSSOConfig creates or updates SSO configuration for a tenant.
func (s *Service) SaveSSOConfig(ctx context.Context, cfg *SSOConfig) error

// DeleteSSOConfig removes SSO configuration for a tenant.
func (s *Service) DeleteSSOConfig(ctx context.Context, tenantID string) error
```

**Important:** The `NewService` function signature changes, which affects wiring in `cmd/server/main.go`. The `SSORepository` is injected as a dependency.

### 5. Keycloak Client Extension

**File:** `internal/auth/keycloak.go` (modified)

Add methods for Authorization Code flow and IdP management:

```go
// SSOConfig holds Keycloak-specific SSO configuration derived from the app config.
type SSOCallbackConfig struct {
    CallbackURL string // e.g. "https://app.example.com/api/auth/sso/callback"
    FrontendURL string // e.g. "https://app.example.com" for post-auth redirect
}

// BuildSSORedirectURL constructs the Keycloak authorization endpoint URL
// with kc_idp_hint to trigger the specific SAML IdP broker flow.
//
// URL structure:
//   {keycloak_base}/realms/{realm}/protocol/openid-connect/auth
//     ?client_id={client_id}
//     &redirect_uri={callback_url}
//     &response_type=code
//     &scope=openid email profile
//     &kc_idp_hint={idp_alias}
//     &state={csrf_state}
func (kc *KeycloakClient) BuildSSORedirectURL(idpAlias, callbackURL, state string) string

// ExchangeCode exchanges an authorization code for tokens using the
// Keycloak token endpoint (Authorization Code grant).
// This is fundamentally different from the direct grant in Authenticate().
func (kc *KeycloakClient) ExchangeCode(ctx context.Context, code, callbackURL string) (*gocloak.JWT, error)

// --- Keycloak Admin API methods for IdP management ---

// GetAdminToken obtains an admin token for Keycloak Admin REST API operations.
func (kc *KeycloakClient) GetAdminToken(ctx context.Context) (string, error)

// CreateSAMLIdentityProvider creates a SAML identity provider in Keycloak
// for a specific tenant. Uses the Keycloak Admin REST API.
func (kc *KeycloakClient) CreateSAMLIdentityProvider(ctx context.Context, cfg SAMLIdPConfig) error

// UpdateSAMLIdentityProvider updates an existing SAML IdP configuration.
func (kc *KeycloakClient) UpdateSAMLIdentityProvider(ctx context.Context, cfg SAMLIdPConfig) error

// DeleteSAMLIdentityProvider removes a SAML IdP configuration from Keycloak.
func (kc *KeycloakClient) DeleteSAMLIdentityProvider(ctx context.Context, alias string) error

// SAMLIdPConfig holds the parameters needed to create a SAML IdP in Keycloak.
type SAMLIdPConfig struct {
    Alias         string // Unique alias: "saml-{tenant_slug}"
    DisplayName   string
    EntityID      string // IdP entity ID
    SSOURL        string // IdP SSO endpoint
    Certificate   string // IdP signing certificate
    NameIDFormat  string
    SignRequests  bool
}
```

**gocloak vs. direct HTTP:** The `ExchangeCode` method can use gocloak's `GetToken()` with `GrantType: "authorization_code"`. For the Admin API IdP management, gocloak v13.9.0 provides `CreateIdentityProvider` and related methods. If these do not support SAML-specific fields adequately, the implementation falls back to direct HTTP calls to `/admin/realms/{realm}/identity-provider/instances`. The design includes a `keycloakAdminHTTP` fallback struct that wraps `net/http.Client` for this purpose.

**ExchangeCode implementation approach:**

```go
func (kc *KeycloakClient) ExchangeCode(ctx context.Context, code, callbackURL string) (*gocloak.JWT, error) {
    // Option A: Use gocloak's GetToken with grant_type=authorization_code
    opts := gocloak.TokenOptions{
        ClientID:     &kc.clientID,
        ClientSecret: &kc.secret,
        Code:         &code,
        GrantType:    gocloak.StringP("authorization_code"),
        RedirectURI:  &callbackURL,
    }
    token, err := kc.client.GetToken(ctx, kc.realm, opts)
    if err != nil {
        return nil, fmt.Errorf("code exchange failed: %w", err)
    }
    return token, nil
}
```

### 6. Auth Service SSO Methods

**File:** `internal/auth/service.go` (modified)

Add SSO-specific methods alongside the existing `Login()`:

```go
// SSOCheckResponse is returned by CheckSSO to tell the frontend
// whether SSO is available for a given email.
type SSOCheckResponse struct {
    SSOEnabled  bool   `json:"sso_enabled"`
    InitiateURL string `json:"initiate_url,omitempty"` // Only set if SSO enabled
    IdPName     string `json:"idp_name,omitempty"`     // Display name of the IdP
}

// CheckSSO determines if a given email's tenant has SSO configured and enabled.
// This is the first step in the SSO flow -- called before the user decides
// whether to use password or SSO login.
func (s *Service) CheckSSO(ctx context.Context, email string) (*SSOCheckResponse, error) {
    tenant, ssoCfg, err := s.tenantSvc.GetSSOConfigByEmailDomain(ctx, email)
    if err != nil {
        return &SSOCheckResponse{SSOEnabled: false}, nil // Unknown domain = no SSO
    }
    if ssoCfg == nil || !ssoCfg.Enabled {
        return &SSOCheckResponse{SSOEnabled: false}, nil
    }
    return &SSOCheckResponse{
        SSOEnabled:  true,
        InitiateURL: fmt.Sprintf("/api/auth/sso/initiate?email=%s", url.QueryEscape(email)),
        IdPName:     ssoCfg.IdPDisplayName,
    }, nil
}

// InitiateSSO builds the Keycloak redirect URL for SAML SSO and returns it.
// The handler will issue an HTTP 302 redirect to this URL.
func (s *Service) InitiateSSO(ctx context.Context, email string) (redirectURL string, err error) {
    tenant, ssoCfg, err := s.tenantSvc.GetSSOConfigByEmailDomain(ctx, email)
    // ... validate tenant active, SSO enabled ...

    state := generateCSRFState() // crypto/rand based
    // Store state in short-lived cache or encode in a signed cookie

    redirectURL = s.keycloak.BuildSSORedirectURL(
        ssoCfg.IdPAlias,
        s.ssoCallbackURL, // from config
        state,
    )
    return redirectURL, nil
}

// HandleSSOCallback processes the authorization code callback from Keycloak.
// Exchanges the code for tokens, extracts user info, performs JIT provisioning.
func (s *Service) HandleSSOCallback(ctx context.Context, code, state string) (*Tokens, error) {
    // 1. Validate CSRF state
    // 2. Exchange authorization code for tokens
    jwt, err := s.keycloak.ExchangeCode(ctx, code, s.ssoCallbackURL)

    // 3. Validate and extract claims from the access token
    claims, err := s.keycloak.ValidateToken(ctx, jwt.AccessToken)

    // 4. Resolve tenant from email domain
    tenant, err := s.tenantSvc.GetByEmailDomain(ctx, claims.Email)

    // 5. Verify tenant is active and has SSO enabled
    ssoCfg, err := s.tenantSvc.GetSSOConfig(ctx, tenant.ID)
    if ssoCfg == nil || !ssoCfg.Enabled {
        return nil, fmt.Errorf("SSO not enabled for tenant %s", tenant.ID)
    }

    // 6. JIT user provisioning with KeycloakID linking
    user, err := s.userSvc.GetOrCreateByEmailWithKeycloakID(ctx, claims.Email, tenant.ID, claims.Subject)

    // 7. Return tokens
    return &Tokens{
        AccessToken:  jwt.AccessToken,
        RefreshToken: jwt.RefreshToken,
        ExpiresIn:    jwt.ExpiresIn,
    }, nil
}
```

**CSRF state management:** The `state` parameter prevents CSRF in the OAuth2 flow. Two approaches:

- **Signed cookie (preferred for stateless):** Encode `state` + timestamp + HMAC signature in a short-lived cookie set during `InitiateSSO`, validated during `SSOCallback`. No server-side storage needed.
- **In-memory cache (simple, not HA):** Store state in a `sync.Map` with TTL expiry. Simple but lost on restart and not distributed.

The design uses the signed cookie approach for production-readiness without requiring a distributed cache.

### 7. Auth Handler SSO Endpoints

**File:** `internal/auth/handler.go` (modified)

Add three new handler methods:

```go
// CheckSSO handles GET /api/auth/sso/check?email=user@acme.com
// Returns JSON indicating whether SSO is available for the email's tenant.
func (h *Handler) CheckSSO(w http.ResponseWriter, r *http.Request) {
    email := r.URL.Query().Get("email")
    if email == "" {
        httputil.WriteError(w, http.StatusBadRequest, "email parameter required")
        return
    }

    result, err := h.authSvc.CheckSSO(r.Context(), email)
    if err != nil {
        httputil.WriteError(w, http.StatusInternalServerError, "SSO check failed")
        return
    }

    httputil.WriteJSON(w, http.StatusOK, result)
}

// InitiateSSO handles GET /api/auth/sso/initiate?email=user@acme.com
// Redirects the user to Keycloak's SAML IdP broker flow.
func (h *Handler) InitiateSSO(w http.ResponseWriter, r *http.Request) {
    email := r.URL.Query().Get("email")
    if email == "" {
        httputil.WriteError(w, http.StatusBadRequest, "email parameter required")
        return
    }

    redirectURL, stateCookie, err := h.authSvc.InitiateSSO(r.Context(), email)
    if err != nil {
        log.Error().Err(err).Str("email", email).Msg("SSO initiation failed")
        httputil.WriteError(w, http.StatusBadRequest, "SSO not available for this email")
        return
    }

    // Set state cookie for CSRF validation on callback
    http.SetCookie(w, stateCookie)

    http.Redirect(w, r, redirectURL, http.StatusFound)
}

// SSOCallback handles GET /api/auth/sso/callback?code=xxx&state=yyy
// This is the redirect target after Keycloak completes SAML brokering.
func (h *Handler) SSOCallback(w http.ResponseWriter, r *http.Request) {
    code := r.URL.Query().Get("code")
    state := r.URL.Query().Get("state")

    if code == "" {
        // Check for error response from Keycloak
        errorParam := r.URL.Query().Get("error")
        errorDesc := r.URL.Query().Get("error_description")
        log.Warn().Str("error", errorParam).Str("desc", errorDesc).Msg("SSO callback error")
        httputil.WriteError(w, http.StatusBadRequest, "SSO authentication failed")
        return
    }

    // Retrieve and validate state cookie
    stateCookie, err := r.Cookie("sso_state")
    if err != nil || stateCookie.Value == "" {
        httputil.WriteError(w, http.StatusBadRequest, "invalid SSO state")
        return
    }

    tokens, err := h.authSvc.HandleSSOCallback(r.Context(), code, state, stateCookie.Value)
    if err != nil {
        log.Error().Err(err).Msg("SSO callback processing failed")
        httputil.WriteError(w, http.StatusInternalServerError, "SSO authentication failed")
        return
    }

    // Clear state cookie
    http.SetCookie(w, &http.Cookie{Name: "sso_state", MaxAge: -1, Path: "/"})

    // Return tokens to the frontend.
    // Since this is a browser redirect (not XHR), we return an HTML page
    // that communicates tokens to the frontend SPA.
    h.renderSSOCompletePage(w, tokens)
}

// renderSSOCompletePage writes an HTML page that sends tokens to the parent
// window via postMessage, then redirects to the app.
func (h *Handler) renderSSOCompletePage(w http.ResponseWriter, tokens *Tokens) {
    w.Header().Set("Content-Type", "text/html")
    // Template renders: window.opener.postMessage({tokens}, origin); window.close();
    // OR: redirect to frontend URL with tokens in URL fragment
}
```

### 8. User Service Extension

**File:** `internal/user/service.go` (modified)

```go
// GetOrCreateByEmailWithKeycloakID extends GetOrCreateByEmail to also
// set/update the KeycloakID. Used during SSO JIT provisioning.
//
// Behavior:
//   - User exists, has KeycloakID: verify match, update last_login, return
//   - User exists, no KeycloakID: set KeycloakID (account linking), update last_login, return
//   - User does not exist: create with KeycloakID, return
func (s *Service) GetOrCreateByEmailWithKeycloakID(
    ctx context.Context, email, tenantID, keycloakID string,
) (*User, error) {
    u, err := s.repo.GetByEmail(ctx, email)
    if err == nil {
        // User exists
        if u.TenantID != tenantID {
            return nil, fmt.Errorf("email %s belongs to a different tenant", email)
        }
        if u.KeycloakID == "" {
            // Account linking: existing password user doing SSO for the first time
            if err := s.repo.UpdateKeycloakID(ctx, u.ID, keycloakID); err != nil {
                return nil, fmt.Errorf("failed to link Keycloak account: %w", err)
            }
            u.KeycloakID = keycloakID
        } else if u.KeycloakID != keycloakID {
            // KeycloakID mismatch -- reject with clear error
            return nil, fmt.Errorf("keycloak ID mismatch for user %s", email)
        }
        _ = s.repo.UpdateLastLogin(ctx, u.ID)
        return u, nil
    }

    // User doesn't exist -- create with KeycloakID
    newUser := &User{
        Email:      email,
        Name:       email,
        TenantID:   tenantID,
        KeycloakID: keycloakID,
        Role:       "user",
        IsActive:   true,
    }
    if err := s.repo.Create(ctx, newUser); err != nil {
        return nil, err
    }
    return s.repo.GetByEmail(ctx, email)
}
```

**File:** `internal/user/repository.go` (modified)

```go
// UpdateKeycloakID sets the keycloak_id for a user. Used during account linking.
func (r *Repository) UpdateKeycloakID(ctx context.Context, userID, keycloakID string) error {
    _, err := r.db.Exec(ctx,
        `UPDATE users SET keycloak_id = $1, updated_at = NOW() WHERE id = $2`,
        keycloakID, userID,
    )
    return err
}
```

### 9. Configuration Extension

**File:** `internal/config/config.go` (modified)

```go
type Config struct {
    Port        string         `env:"PORT,default=8080"`
    DatabaseURL string         `env:"DATABASE_URL,required"`
    Keycloak    KeycloakConfig `env:",prefix=KEYCLOAK_"`
    SSO         SSOConfig      `env:",prefix=SSO_"`
}

type SSOConfig struct {
    CallbackURL string `env:"CALLBACK_URL,default=http://localhost:8080/api/auth/sso/callback"`
    FrontendURL string `env:"FRONTEND_URL,default=http://localhost:3000"`
    StateSecret string `env:"STATE_SECRET,required"` // HMAC key for state cookie signing
}
```

### 10. Route Registration

**File:** `cmd/server/main.go` (modified)

```go
// Updated wiring
ssoRepo := tenant.NewSSORepository(db)
tenantSvc := tenant.NewService(tenantRepo, ssoRepo)
authSvc := auth.NewService(keycloakClient, userSvc, tenantSvc, cfg.SSO)

// New SSO routes in public group
r.Group(func(r chi.Router) {
    authHandler := auth.NewHandler(authSvc)
    r.Post("/api/auth/login", authHandler.Login)
    r.Post("/api/auth/logout", authHandler.Logout)
    r.Post("/api/auth/refresh", authHandler.RefreshToken)

    // SSO endpoints (public -- unauthenticated users initiate SSO)
    r.Get("/api/auth/sso/check", authHandler.CheckSSO)
    r.Get("/api/auth/sso/initiate", authHandler.InitiateSSO)
    r.Get("/api/auth/sso/callback", authHandler.SSOCallback)
})

// SSO admin endpoints in the admin route group
r.Group(func(r chi.Router) {
    r.Use(authMiddleware.Authenticate)
    r.Use(authMiddleware.RequireRole("admin"))

    // Existing admin routes...

    // SSO configuration management
    r.Get("/api/admin/tenants/{tenantID}/sso", ssoAdminHandler.GetSSOConfig)
    r.Put("/api/admin/tenants/{tenantID}/sso", ssoAdminHandler.SaveSSOConfig)
    r.Delete("/api/admin/tenants/{tenantID}/sso", ssoAdminHandler.DeleteSSOConfig)
})
```

## Edge Case Handling

### Existing Password User Enables SSO

When a user who has been logging in via email/password has their tenant's SSO enabled:

1. User can still log in via `POST /api/auth/login` (dual-auth confirmed).
2. On first SSO login, `GetOrCreateByEmailWithKeycloakID` finds the existing user with no `keycloak_id`.
3. The method sets `keycloak_id` to the Keycloak subject from the brokered SAML assertion.
4. Subsequent SSO logins match on email and verify `keycloak_id` matches.

### SSO Tenant User Tries Password Login

Allowed per dual-auth decision. No changes to `POST /api/auth/login`. The existing `Login()` method continues to work because Keycloak still holds the user's password credentials (or the user was provisioned with a password).

### Email Domain Not Mapped to Any SSO Tenant

`CheckSSO` returns `{sso_enabled: false}`. Frontend presents the normal password login form. No error, no disruption.

### Keycloak/IdP Unavailable

- **CheckSSO:** Still works (reads from local DB, not Keycloak). Returns SSO availability.
- **InitiateSSO:** Builds the redirect URL locally. The user sees a Keycloak error page if Keycloak is down. They can return and use password login.
- **SSOCallback:** If Keycloak is down during code exchange, the callback fails with an error. The user is shown an error page with instructions to try again or use password login.

### SAML Assertion Missing Required Attributes

Keycloak handles SAML assertion validation. If the assertion is missing required attributes (email, typically), Keycloak will not issue a token and will redirect to the callback with an `error` parameter. The `SSOCallback` handler detects the error parameter and returns a clear error message.

### KeycloakID Mismatch

If a user logs in via SSO but their Keycloak subject does not match the stored `keycloak_id`, the system rejects the login with a clear error. This prevents account hijacking if someone has the same email in a different IdP context. An admin must manually resolve the mismatch.

## Keycloak Configuration Requirements

### Client Configuration

The Keycloak client `platform-app` must have:
- **Standard Flow Enabled:** `true` (currently likely only Direct Access Grants)
- **Valid Redirect URIs:** Include `{app_base_url}/api/auth/sso/callback`
- **Web Origins:** Include the frontend origin for CORS

This is a one-time Keycloak admin configuration change, not a code change. Both Standard Flow and Direct Access Grants can be enabled simultaneously without conflict.

### Per-Tenant IdP Configuration

For each tenant with SSO, a SAML Identity Provider is created in Keycloak via the Admin REST API:

```
POST /admin/realms/{realm}/identity-provider/instances
{
    "alias": "saml-{tenant_slug}",
    "displayName": "{tenant_name} SSO",
    "providerId": "saml",
    "enabled": true,
    "config": {
        "entityId": "{idp_entity_id}",
        "singleSignOnServiceUrl": "{idp_sso_url}",
        "signingCertificate": "{idp_certificate}",
        "nameIDPolicyFormat": "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress",
        "wantAuthnRequestsSigned": "true",
        "postBindingAuthnRequest": "true",
        "postBindingResponse": "true",
        "syncMode": "FORCE"
    }
}
```

### Attribute Mappers

Each SAML IdP needs attribute mappers to ensure the email claim propagates correctly:
- **Email mapper:** Maps SAML `email` attribute to Keycloak `email` field
- **Name mapper:** Maps SAML `firstName`/`lastName` to Keycloak profile

### First Broker Login Flow

Use Keycloak's default "first broker login" flow, which handles user linking automatically. This flow:
1. Checks if a Keycloak user with matching email already exists
2. If yes, links the brokered identity to the existing user
3. If no, creates a new Keycloak user

### tenant_id Protocol Mapper

A Keycloak client protocol mapper must be configured to include `tenant_id` in the JWT for brokered logins. Since Keycloak does not inherently know the platform's tenant_id, two approaches:

1. **Email-domain fallback (initial):** The auth middleware already falls back to email-domain-based tenant resolution when `tenant_id` is missing from the JWT. This works without additional Keycloak configuration.
2. **Custom mapper (later optimization):** A Keycloak user attribute `tenant_id` can be populated during first broker login via a custom authenticator, then mapped to the JWT via a protocol mapper.

For MVO, approach 1 is sufficient and requires no Keycloak customization.

## SSO Admin Configuration Flow

Since self-service UI is excluded from scope, SSO configuration is done via admin API:

1. Admin calls `PUT /api/admin/tenants/{tenantID}/sso` with SAML IdP metadata
2. Backend stores config in `tenant_sso_config` table
3. Backend calls Keycloak Admin API to create/update the SAML IdP
4. Backend generates SP metadata (ACS URL, Entity ID) for the tenant to configure their IdP
5. Returns the SP metadata to the admin for communication to the customer

The admin endpoint response includes the Keycloak-generated SP metadata URLs that the enterprise customer needs to configure their IdP:
- ACS URL: `{keycloak_base}/realms/{realm}/broker/{idp_alias}/endpoint`
- SP Entity ID: `{keycloak_base}/realms/{realm}`

## Security Considerations

1. **CSRF protection:** State parameter in OAuth2 flow, validated via signed cookie.
2. **No SAML assertion logging:** The Go backend never sees raw SAML assertions (Keycloak handles SAML protocol). Only JWTs and authorization codes pass through the app.
3. **HTTPS required:** SSO redirects and callbacks must use HTTPS in production. The callback URL in config must be HTTPS.
4. **Token handling:** JWT tokens from SSO flow have the same security properties as password-flow tokens. Same middleware validates them.
5. **IdP certificate validation:** SAML assertion signatures are validated by Keycloak using the certificate stored in the IdP configuration.
6. **State cookie attributes:** HttpOnly, Secure (in production), SameSite=Lax, short MaxAge (5 minutes).

## Testing Strategy

Given zero existing test coverage, the testing plan prioritizes safety:

### Phase 1: Pre-SSO Safety Net
- **Integration test for existing login flow:** Verify `POST /api/auth/login` works with mocked Keycloak, returns correct token structure.
- **Integration test for auth middleware:** Verify JWT validation, context population, tenant resolution fallback.

### Phase 2: SSO Unit Tests
- **CheckSSO:** Test with SSO-enabled tenant, non-SSO tenant, unknown domain.
- **InitiateSSO:** Test redirect URL construction, state generation.
- **HandleSSOCallback:** Test code exchange flow, JIT provisioning paths, account linking, error cases.
- **BuildSSORedirectURL:** Verify URL structure, parameter encoding.
- **ExchangeCode:** Verify grant_type, redirect_uri parameters.

### Phase 3: SSO Integration Tests
- **Full SSO flow with mocked Keycloak:** Initiate -> redirect -> callback -> tokens.
- **Account linking scenarios:** Existing user + first SSO, new user via SSO, keycloak_id mismatch.
- **Error scenarios:** Keycloak unavailable, invalid code, expired state, missing attributes.

### Phase 4: Keycloak Admin API Tests
- **CreateSAMLIdentityProvider:** Verify correct API payload.
- **SAML IdP CRUD operations:** Create, read, update, delete lifecycle.

## Implementation Order

The recommended implementation sequence minimizes risk by building bottom-up:

1. **Migration + SSO config model/repository** -- No existing code touched, pure additions
2. **User service KeycloakID methods** -- Small, isolated addition to user module
3. **Config extension** -- Add SSO config fields
4. **Keycloak client: ExchangeCode** -- New method, does not modify existing methods
5. **Keycloak client: Admin API methods** -- New methods for IdP management
6. **Auth service: CheckSSO** -- Read-only, no side effects
7. **Auth service: InitiateSSO** -- Constructs URL, no mutations
8. **Auth service: HandleSSOCallback** -- The critical path, bringing everything together
9. **Auth handlers: all three SSO endpoints** -- HTTP layer on top of service
10. **Route registration** -- Wire everything together in main.go
11. **SSO admin endpoints** -- Management API for SSO configuration
12. **Existing login flow tests** -- Safety net before any behavioral changes
13. **SSO tests** -- Full test coverage for new code

Steps 1-11 are purely additive. The existing `Login()` flow is not modified. No behavioral changes to existing endpoints.
