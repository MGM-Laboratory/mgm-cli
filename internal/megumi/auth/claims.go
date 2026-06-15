package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/MGM-Laboratory/mgm-cli/internal/megumi/oidc"
)

// Display roles. The backend (via JWKS) is authoritative for access decisions;
// the CLI only decodes claims to *show* who is logged in.
const (
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleMember   = "member"
)

// Identity is the human-facing identity derived from an access token's claims.
type Identity struct {
	Subject string `json:"subject"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Role    string `json:"role"`
}

// rawClaims is the subset of Keycloak access-token claims the CLI reads.
type rawClaims struct {
	Subject           string   `json:"sub"`
	Email             string   `json:"email"`
	Name              string   `json:"name"`
	PreferredUsername string   `json:"preferred_username"`
	Groups            []string `json:"groups"`
	RealmAccess       struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
}

// ParseIdentity decodes the (unverified) JWT payload of an access token and maps
// it to a display Identity using the configured role names. Signature
// verification is intentionally NOT performed here — that is the backend's job
// via JWKS. This is for display only.
func ParseIdentity(accessToken string, cfg oidc.Config) (Identity, error) {
	claims, err := decodePayload(accessToken)
	if err != nil {
		return Identity{}, err
	}
	name := claims.Name
	if name == "" {
		name = claims.PreferredUsername
	}
	return Identity{
		Subject: claims.Subject,
		Email:   claims.Email,
		Name:    name,
		Role:    resolveRole(claims, cfg),
	}, nil
}

// resolveRole matches the configured admin/operator names against the token's
// realm roles and group names (a group path's trailing segment counts). Admin
// and operator have equal rights; everyone else is a member.
func resolveRole(c rawClaims, cfg oidc.Config) string {
	has := func(target string) bool {
		for _, r := range c.RealmAccess.Roles {
			if r == target {
				return true
			}
		}
		for _, g := range c.Groups {
			if g == target || trailingSegment(g) == target {
				return true
			}
		}
		return false
	}
	switch {
	case has(cfg.RoleAdmin):
		return RoleAdmin
	case has(cfg.RoleOperator):
		return RoleOperator
	default:
		return RoleMember
	}
}

func trailingSegment(s string) string {
	s = strings.TrimRight(s, "/")
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}

// decodePayload base64url-decodes the JWT payload segment without verifying the
// signature.
func decodePayload(token string) (rawClaims, error) {
	var c rawClaims
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return c, fmt.Errorf("not a JWT (expected 3 segments)")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return c, fmt.Errorf("decode JWT payload: %w", err)
	}
	if err := json.Unmarshal(payload, &c); err != nil {
		return c, fmt.Errorf("parse JWT claims: %w", err)
	}
	return c, nil
}
