package tenant

import "time"

type Tenant struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Domain    string    `json:"domain"`
	Plan      string    `json:"plan"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TenantSSOConfig struct {
	ID             int64     `json:"id"`
	TenantID       int64     `json:"tenant_id"`
	IdPEntityID    string    `json:"idp_entity_id"`
	IdPSSOURL      string    `json:"idp_sso_url"`
	IdPCertificate string    `json:"idp_certificate"`
	SPEntityID     string    `json:"sp_entity_id"`
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
