package tenant

import (
	"context"
	"fmt"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetByID(ctx context.Context, id string) (*Tenant, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) GetBySlug(ctx context.Context, slug string) (*Tenant, error) {
	return s.repo.GetBySlug(ctx, slug)
}

// GetByEmailDomain resolves a tenant from a user's email address.
// Used during login when tenant_id is not explicitly provided.
func (s *Service) GetByEmailDomain(ctx context.Context, email string) (*Tenant, error) {
	t, err := s.repo.GetByEmailDomain(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("tenant resolution by email failed: %w", err)
	}
	return t, nil
}

func (s *Service) Update(ctx context.Context, t *Tenant) error {
	return s.repo.Update(ctx, t)
}

func (s *Service) List(ctx context.Context) ([]*Tenant, error) {
	return s.repo.List(ctx)
}
