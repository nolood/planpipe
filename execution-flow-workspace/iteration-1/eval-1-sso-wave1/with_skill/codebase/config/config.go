package config

import "os"

type SSOConfig struct {
	KeycloakURL           string
	KeycloakRealm         string
	KeycloakAdminUser     string
	KeycloakAdminPassword string
	SAMLCallbackURL       string
	SPEntityID            string
}

type Config struct {
	DatabaseURL string
	ServerAddr  string
	JWTSecret   string
	SSO         SSOConfig
}

func Load() *Config {
	return &Config{
		DatabaseURL: getEnv("DATABASE_URL", "postgres://localhost:5432/app"),
		ServerAddr:  getEnv("SERVER_ADDR", ":8080"),
		JWTSecret:   getEnv("JWT_SECRET", "dev-secret"),
		SSO: SSOConfig{
			KeycloakURL:           getEnv("KEYCLOAK_URL", "http://localhost:8180"),
			KeycloakRealm:         getEnv("KEYCLOAK_REALM", "master"),
			KeycloakAdminUser:     getEnv("KEYCLOAK_ADMIN_USER", "admin"),
			KeycloakAdminPassword: getEnv("KEYCLOAK_ADMIN_PASSWORD", "admin"),
			SAMLCallbackURL:       getEnv("SAML_CALLBACK_URL", "http://localhost:8080/auth/sso/callback"),
			SPEntityID:            getEnv("SP_ENTITY_ID", "multi-tenant-app"),
		},
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
