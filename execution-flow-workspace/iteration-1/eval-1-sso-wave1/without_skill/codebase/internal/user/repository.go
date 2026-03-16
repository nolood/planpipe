package user

import "context"

type Repository struct {
	db interface{}
}

func NewRepository(db interface{}) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindByEmail(ctx context.Context, email string) (*User, error) {
	// placeholder
	return nil, nil
}

func (r *Repository) FindByID(ctx context.Context, id int64) (*User, error) {
	// placeholder
	return nil, nil
}

func (r *Repository) Create(ctx context.Context, u *User) error {
	// placeholder
	return nil
}

func (r *Repository) Update(ctx context.Context, u *User) error {
	// placeholder
	return nil
}
