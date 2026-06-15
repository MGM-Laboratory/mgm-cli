package auth

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/MGM-Laboratory/mgm-cli/internal/megumi/oidc"
)

// makeJWT builds an unsigned (alg=none) JWT with the given claims payload. The
// CLI never verifies the signature, so a stub signature is fine for tests.
func makeJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	body, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(body)
	return header + "." + payload + ".sig"
}

func TestParseIdentityRoleMapping(t *testing.T) {
	cfg := oidc.Config{RoleAdmin: "megumi-admin", RoleOperator: "megumi-operator"}

	cases := []struct {
		name     string
		claims   map[string]any
		wantRole string
		wantName string
	}{
		{
			name: "admin via realm role",
			claims: map[string]any{
				"sub": "u1", "email": "a@x.io", "name": "Admin A",
				"realm_access": map[string]any{"roles": []string{"megumi-admin", "default-roles"}},
			},
			wantRole: RoleAdmin, wantName: "Admin A",
		},
		{
			name: "operator via realm role",
			claims: map[string]any{
				"sub": "u2", "email": "o@x.io", "name": "Op O",
				"realm_access": map[string]any{"roles": []string{"megumi-operator"}},
			},
			wantRole: RoleOperator, wantName: "Op O",
		},
		{
			name: "operator via group path trailing segment",
			claims: map[string]any{
				"sub": "u3", "email": "g@x.io", "preferred_username": "guser",
				"groups": []string{"/teams/megumi-operator"},
			},
			wantRole: RoleOperator, wantName: "guser",
		},
		{
			name: "admin wins over operator",
			claims: map[string]any{
				"sub":          "u4",
				"realm_access": map[string]any{"roles": []string{"megumi-operator", "megumi-admin"}},
			},
			wantRole: RoleAdmin,
		},
		{
			name:     "member when no roles",
			claims:   map[string]any{"sub": "u5", "email": "m@x.io"},
			wantRole: RoleMember,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tok := makeJWT(t, tc.claims)
			id, err := ParseIdentity(tok, cfg)
			if err != nil {
				t.Fatalf("ParseIdentity: %v", err)
			}
			if id.Role != tc.wantRole {
				t.Errorf("Role = %q, want %q", id.Role, tc.wantRole)
			}
			if tc.wantName != "" && id.Name != tc.wantName {
				t.Errorf("Name = %q, want %q", id.Name, tc.wantName)
			}
		})
	}
}

func TestParseIdentityHonorsCustomRoleNames(t *testing.T) {
	cfg := oidc.Config{RoleAdmin: "boss", RoleOperator: "helper"}
	tok := makeJWT(t, map[string]any{
		"sub":          "u",
		"realm_access": map[string]any{"roles": []string{"boss"}},
	})
	id, err := ParseIdentity(tok, cfg)
	if err != nil {
		t.Fatalf("ParseIdentity: %v", err)
	}
	if id.Role != RoleAdmin {
		t.Errorf("Role = %q, want %q (custom admin role name)", id.Role, RoleAdmin)
	}
}

func TestParseIdentityRejectsNonJWT(t *testing.T) {
	if _, err := ParseIdentity("not-a-jwt", oidc.Config{}); err == nil {
		t.Fatal("expected error for non-JWT token")
	}
}
