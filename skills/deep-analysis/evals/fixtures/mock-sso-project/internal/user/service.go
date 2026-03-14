package user

import (
	"context"
	"errors"

	"github.com/acme/platform/internal/tenant"
)

type Service struct {
	repo      *Repository
	tenantSvc *tenant.Service
}

func NewService(repo *Repository, ts *tenant.Service) *Service {
	return &Service{repo: repo, tenantSvc: ts}
}

func (s *Service) GetByID(ctx context.Context, id string) (*User, error) {
	return s.repo.GetByID(ctx, id)
}

// GetOrCreateByEmail finds a user by email or creates one if it doesn't exist.
// This is used during login to ensure a local user record exists for every
// Keycloak-authenticated user.
func (s *Service) GetOrCreateByEmail(ctx context.Context, email, tenantID string) (*User, error) {
	u, err := s.repo.GetByEmail(ctx, email)
	if err == nil {
		// User exists — update last login and return
		_ = s.repo.UpdateLastLogin(ctx, u.ID)
		return u, nil
	}

	// User doesn't exist — create with default role
	newUser := &User{
		Email:    email,
		Name:     email, // Will be updated from Keycloak profile later
		TenantID: tenantID,
		Role:     "user",
		IsActive: true,
	}
	if err := s.repo.Create(ctx, newUser); err != nil {
		return nil, err
	}

	return s.repo.GetByEmail(ctx, email)
}

func (s *Service) ListByTenant(ctx context.Context, tenantID string) ([]*User, error) {
	// Verify tenant exists
	_, err := s.tenantSvc.GetByID(ctx, tenantID)
	if err != nil {
		return nil, errors.New("tenant not found")
	}
	return s.repo.ListByTenant(ctx, tenantID)
}
