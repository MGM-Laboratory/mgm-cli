// Package oidc resolves the Megumi Code OIDC configuration from the environment
// and discovers the identity provider's endpoints. Nothing is hardcoded beyond
// documented defaults that point at the lab's Keycloak realm; every value is
// overridable via MEGUMI_* environment variables (see .env.example and the
// backend Keycloak runbook in mgm-cli-backend/docs/keycloak.md).
package oidc

import (
	"os"
	"strings"
)

// Documented defaults. These match the lab deployment but are always overridable
// through the environment so the CLI never ships a hardcoded secret or host that
// can't be repointed.
const (
	DefaultIssuer       = "https://iam.labmgm.org/realms/mgm"
	DefaultClientID     = "mgm-cli"
	DefaultScopes       = "openid profile email offline_access"
	DefaultBaseURL      = "https://cli-api.labmgm.org"
	DefaultRoleAdmin    = "megumi-admin"
	DefaultRoleOperator = "megumi-operator"
)

// Config is the environment-resolved auth configuration for the CLI.
type Config struct {
	// Issuer is the Keycloak realm issuer URL (MEGUMI_OIDC_ISSUER). Endpoints
	// are discovered from "<Issuer>/.well-known/openid-configuration".
	Issuer string
	// ClientID is the public CLI client (MEGUMI_OIDC_CLIENT_ID).
	ClientID string
	// Scopes requested at login (MEGUMI_OIDC_SCOPES). "offline_access" yields a
	// refresh token.
	Scopes []string
	// BaseURL is the backend broker (MEGUMI_BASE_URL), used for the optional
	// /api/v1/me identity enrichment in `mgm whoami`.
	BaseURL string
	// RoleAdmin / RoleOperator are the realm-role or group names that map a user
	// to the admin / operator display role (MEGUMI_ROLE_ADMIN / _OPERATOR).
	RoleAdmin    string
	RoleOperator string
}

// LoadConfig reads the configuration from the environment, applying defaults.
func LoadConfig() Config {
	return Config{
		Issuer:       env("MEGUMI_OIDC_ISSUER", DefaultIssuer),
		ClientID:     env("MEGUMI_OIDC_CLIENT_ID", DefaultClientID),
		Scopes:       strings.Fields(env("MEGUMI_OIDC_SCOPES", DefaultScopes)),
		BaseURL:      env("MEGUMI_BASE_URL", DefaultBaseURL),
		RoleAdmin:    env("MEGUMI_ROLE_ADMIN", DefaultRoleAdmin),
		RoleOperator: env("MEGUMI_ROLE_OPERATOR", DefaultRoleOperator),
	}
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
