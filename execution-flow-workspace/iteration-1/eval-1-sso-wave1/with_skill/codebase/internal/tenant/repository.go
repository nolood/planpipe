package tenant

import "context"

type Repository struct {
	db interface{}
}

func NewRepository(db interface{}) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindByID(ctx context.Context, id int64) (*Tenant, error) {
	// placeholder
	return nil, nil
}

func (r *Repository) FindByDomain(ctx context.Context, domain string) (*Tenant, error) {
	// placeholder
	return nil, nil
}
