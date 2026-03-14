package user

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetByID(ctx context.Context, id string) (*User, error) {
	var u User
	err := r.db.QueryRow(ctx,
		`SELECT id, email, name, tenant_id, keycloak_id, role, is_active, last_login, created_at, updated_at
		 FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &u.Name, &u.TenantID, &u.KeycloakID, &u.Role, &u.IsActive, &u.LastLogin, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return &u, nil
}

func (r *Repository) GetByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	err := r.db.QueryRow(ctx,
		`SELECT id, email, name, tenant_id, keycloak_id, role, is_active, last_login, created_at, updated_at
		 FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Email, &u.Name, &u.TenantID, &u.KeycloakID, &u.Role, &u.IsActive, &u.LastLogin, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return &u, nil
}

func (r *Repository) GetByKeycloakID(ctx context.Context, keycloakID string) (*User, error) {
	var u User
	err := r.db.QueryRow(ctx,
		`SELECT id, email, name, tenant_id, keycloak_id, role, is_active, last_login, created_at, updated_at
		 FROM users WHERE keycloak_id = $1`, keycloakID,
	).Scan(&u.ID, &u.Email, &u.Name, &u.TenantID, &u.KeycloakID, &u.Role, &u.IsActive, &u.LastLogin, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return &u, nil
}

func (r *Repository) Create(ctx context.Context, u *User) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO users (id, email, name, tenant_id, keycloak_id, role, is_active, created_at, updated_at)
		 VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, true, NOW(), NOW())`,
		u.Email, u.Name, u.TenantID, u.KeycloakID, u.Role,
	)
	return err
}

func (r *Repository) UpdateLastLogin(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET last_login = NOW() WHERE id = $1`, id)
	return err
}

func (r *Repository) ListByTenant(ctx context.Context, tenantID string) ([]*User, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, email, name, tenant_id, keycloak_id, role, is_active, last_login, created_at, updated_at
		 FROM users WHERE tenant_id = $1 ORDER BY email`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.TenantID, &u.KeycloakID, &u.Role, &u.IsActive, &u.LastLogin, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, &u)
	}
	return users, nil
}
