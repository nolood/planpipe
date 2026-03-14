package tenant

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetByID(ctx context.Context, id string) (*Tenant, error) {
	var t Tenant
	err := r.db.QueryRow(ctx,
		`SELECT id, name, slug, email_domain, is_active, plan, created_at, updated_at
		 FROM tenants WHERE id = $1`, id,
	).Scan(&t.ID, &t.Name, &t.Slug, &t.EmailDomain, &t.IsActive, &t.Plan, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
	}
	return &t, nil
}

func (r *Repository) GetBySlug(ctx context.Context, slug string) (*Tenant, error) {
	var t Tenant
	err := r.db.QueryRow(ctx,
		`SELECT id, name, slug, email_domain, is_active, plan, created_at, updated_at
		 FROM tenants WHERE slug = $1`, slug,
	).Scan(&t.ID, &t.Name, &t.Slug, &t.EmailDomain, &t.IsActive, &t.Plan, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
	}
	return &t, nil
}

func (r *Repository) GetByEmailDomain(ctx context.Context, email string) (*Tenant, error) {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid email format: %s", email)
	}
	domain := parts[1]

	var t Tenant
	err := r.db.QueryRow(ctx,
		`SELECT id, name, slug, email_domain, is_active, plan, created_at, updated_at
		 FROM tenants WHERE email_domain = $1`, domain,
	).Scan(&t.ID, &t.Name, &t.Slug, &t.EmailDomain, &t.IsActive, &t.Plan, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("no tenant for domain %s: %w", domain, err)
	}
	return &t, nil
}

func (r *Repository) Update(ctx context.Context, t *Tenant) error {
	_, err := r.db.Exec(ctx,
		`UPDATE tenants SET name=$1, slug=$2, email_domain=$3, is_active=$4, plan=$5, updated_at=NOW()
		 WHERE id=$6`,
		t.Name, t.Slug, t.EmailDomain, t.IsActive, t.Plan, t.ID,
	)
	return err
}

func (r *Repository) List(ctx context.Context) ([]*Tenant, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, name, slug, email_domain, is_active, plan, created_at, updated_at
		 FROM tenants ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tenants []*Tenant
	for rows.Next() {
		var t Tenant
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug, &t.EmailDomain, &t.IsActive, &t.Plan, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tenants = append(tenants, &t)
	}
	return tenants, nil
}
