package auth

import (
	"context"
	"fmt"

	"github.com/acme/platform/internal/tenant"
	"github.com/acme/platform/internal/user"
)

// Service orchestrates authentication flows. Currently supports only
// email/password login via Keycloak direct grant.
type Service struct {
	keycloak  *KeycloakClient
	userSvc   *user.Service
	tenantSvc *tenant.Service
}

func NewService(kc *KeycloakClient, us *user.Service, ts *tenant.Service) *Service {
	return &Service{
		keycloak:  kc,
		userSvc:   us,
		tenantSvc: ts,
	}
}

type Tokens struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
}

// Login authenticates a user via email/password.
// If tenantID is empty, it resolves the tenant from the user's email domain.
func (s *Service) Login(ctx context.Context, email, password, tenantID string) (*Tokens, error) {
	// Resolve tenant if not provided
	if tenantID == "" {
		t, err := s.tenantSvc.GetByEmailDomain(ctx, email)
		if err != nil {
			return nil, fmt.Errorf("cannot resolve tenant for email %s: %w", email, err)
		}
		tenantID = t.ID
	}

	// Verify tenant exists and is active
	tnt, err := s.tenantSvc.GetByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("tenant lookup failed: %w", err)
	}
	if !tnt.IsActive {
		return nil, fmt.Errorf("tenant %s is inactive", tenantID)
	}

	// Authenticate against Keycloak
	jwt, err := s.keycloak.Authenticate(ctx, email, password)
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Ensure user record exists in our database
	_, err = s.userSvc.GetOrCreateByEmail(ctx, email, tenantID)
	if err != nil {
		return nil, fmt.Errorf("user sync failed: %w", err)
	}

	return &Tokens{
		AccessToken:  jwt.AccessToken,
		RefreshToken: jwt.RefreshToken,
		ExpiresIn:    jwt.ExpiresIn,
	}, nil
}

// Logout invalidates the user's Keycloak session.
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	return s.keycloak.Logout(ctx, refreshToken)
}

// RefreshToken exchanges a refresh token for new tokens.
func (s *Service) RefreshToken(ctx context.Context, refreshToken string) (*Tokens, error) {
	jwt, err := s.keycloak.RefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, fmt.Errorf("token refresh failed: %w", err)
	}
	return &Tokens{
		AccessToken:  jwt.AccessToken,
		RefreshToken: jwt.RefreshToken,
		ExpiresIn:    jwt.ExpiresIn,
	}, nil
}
