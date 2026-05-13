// Package infisical is a thin REST client for the Infisical API surface the
// CLI needs. Built directly against the documented HTTP API rather than the
// Go SDK so we don't drift with SDK releases.
//
// Auth uses Universal Auth (machine identity client_id/client_secret), which
// returns a short-lived bearer token cached for the lifetime of the process.
package infisical

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client talks to a self-hosted or cloud Infisical instance.
type Client struct {
	hostURL      string
	clientID     string
	clientSecret string
	http         *http.Client

	token     string
	tokenExp  time.Time
	userAgent string
}

// New constructs a Client. hostURL is the base URL (e.g. https://secrets.labmgm.org).
func New(hostURL, clientID, clientSecret, userAgent string) *Client {
	return &Client{
		hostURL:      strings.TrimRight(hostURL, "/"),
		clientID:     clientID,
		clientSecret: clientSecret,
		http:         &http.Client{Timeout: 30 * time.Second},
		userAgent:    userAgent,
	}
}

// ---------- auth ----------

type loginResponse struct {
	AccessToken       string `json:"accessToken"`
	ExpiresIn         int    `json:"expiresIn"`
	AccessTokenMaxTTL int    `json:"accessTokenMaxTTL"`
	TokenType         string `json:"tokenType"`
}

// Login performs Universal Auth login and caches the token. Subsequent calls
// reuse the cached token until 30s before expiry.
func (c *Client) Login(ctx context.Context) error {
	if c.token != "" && time.Now().Add(30*time.Second).Before(c.tokenExp) {
		return nil
	}
	body := map[string]string{
		"clientId":     c.clientID,
		"clientSecret": c.clientSecret,
	}
	var out loginResponse
	if err := c.do(ctx, http.MethodPost, "/api/v1/auth/universal-auth/login", nil, body, &out, false); err != nil {
		return fmt.Errorf("infisical login: %w", err)
	}
	c.token = out.AccessToken
	ttl := out.ExpiresIn
	if ttl <= 0 {
		ttl = 600
	}
	c.tokenExp = time.Now().Add(time.Duration(ttl) * time.Second)
	return nil
}

// Ping verifies credentials & host reachability.
func (c *Client) Ping(ctx context.Context) error {
	return c.Login(ctx)
}

// ---------- projects ----------

type Project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type membershipsResponse struct {
	Memberships []struct {
		Workspace Project `json:"workspace"`
	} `json:"memberships"`
}

// ListProjects returns the projects (workspaces) the identity can access.
func (c *Client) ListProjects(ctx context.Context) ([]Project, error) {
	if err := c.Login(ctx); err != nil {
		return nil, err
	}
	var out membershipsResponse
	if err := c.do(ctx, http.MethodGet, "/api/v2/users/me/memberships", nil, nil, &out, true); err == nil && len(out.Memberships) > 0 {
		ps := make([]Project, 0, len(out.Memberships))
		for _, m := range out.Memberships {
			ps = append(ps, m.Workspace)
		}
		return ps, nil
	}

	// Fallback: machine-identity workspaces endpoint.
	var alt struct {
		Workspaces []Project `json:"workspaces"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v1/workspace", nil, nil, &alt, true); err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	return alt.Workspaces, nil
}

// ---------- environments ----------

type Environment struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type workspaceResponse struct {
	Workspace struct {
		ID           string        `json:"id"`
		Name         string        `json:"name"`
		Environments []Environment `json:"environments"`
	} `json:"workspace"`
}

// ListEnvironments returns the environments configured on a project.
func (c *Client) ListEnvironments(ctx context.Context, projectID string) ([]Environment, error) {
	if err := c.Login(ctx); err != nil {
		return nil, err
	}
	var out workspaceResponse
	path := fmt.Sprintf("/api/v1/workspace/%s", url.PathEscape(projectID))
	if err := c.do(ctx, http.MethodGet, path, nil, nil, &out, true); err != nil {
		return nil, fmt.Errorf("list environments: %w", err)
	}
	return out.Workspace.Environments, nil
}

// ---------- folders ----------

type Folder struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path,omitempty"`
}

type foldersResponse struct {
	Folders []Folder `json:"folders"`
}

// ListFolders lists folders directly under parentPath in env.
func (c *Client) ListFolders(ctx context.Context, projectID, environment, parentPath string) ([]Folder, error) {
	if err := c.Login(ctx); err != nil {
		return nil, err
	}
	if parentPath == "" {
		parentPath = "/"
	}
	q := url.Values{
		"workspaceId": {projectID},
		"environment": {environment},
		"path":        {parentPath},
	}
	var out foldersResponse
	if err := c.do(ctx, http.MethodGet, "/api/v1/folders?"+q.Encode(), nil, nil, &out, true); err != nil {
		return nil, fmt.Errorf("list folders: %w", err)
	}
	return out.Folders, nil
}

// ---------- secrets ----------

type Secret struct {
	ID            string `json:"id,omitempty"`
	SecretKey     string `json:"secretKey"`
	SecretValue   string `json:"secretValue"`
	SecretComment string `json:"secretComment,omitempty"`
	Type          string `json:"type,omitempty"`
	Version       int    `json:"version,omitempty"`
}

type secretsResponse struct {
	Secrets []Secret `json:"secrets"`
}

// ListSecrets returns all secrets at the given path.
func (c *Client) ListSecrets(ctx context.Context, projectID, environment, secretPath string) ([]Secret, error) {
	if err := c.Login(ctx); err != nil {
		return nil, err
	}
	if secretPath == "" {
		secretPath = "/"
	}
	q := url.Values{
		"workspaceId":            {projectID},
		"environment":            {environment},
		"secretPath":             {secretPath},
		"include_imports":        {"true"},
		"expandSecretReferences": {"true"},
	}
	var out secretsResponse
	if err := c.do(ctx, http.MethodGet, "/api/v3/secrets/raw?"+q.Encode(), nil, nil, &out, true); err != nil {
		return nil, fmt.Errorf("list secrets: %w", err)
	}
	return out.Secrets, nil
}

// GetSecret returns one secret by key.
func (c *Client) GetSecret(ctx context.Context, projectID, environment, secretPath, key string) (*Secret, error) {
	if err := c.Login(ctx); err != nil {
		return nil, err
	}
	q := url.Values{
		"workspaceId":            {projectID},
		"environment":            {environment},
		"secretPath":             {secretPath},
		"expandSecretReferences": {"true"},
	}
	var out struct {
		Secret Secret `json:"secret"`
	}
	path := fmt.Sprintf("/api/v3/secrets/raw/%s?%s", url.PathEscape(key), q.Encode())
	if err := c.do(ctx, http.MethodGet, path, nil, nil, &out, true); err != nil {
		return nil, fmt.Errorf("get secret %s: %w", key, err)
	}
	return &out.Secret, nil
}

// CreateSecret adds a new secret.
func (c *Client) CreateSecret(ctx context.Context, projectID, environment, secretPath, key, value, comment string) error {
	if err := c.Login(ctx); err != nil {
		return err
	}
	body := map[string]any{
		"workspaceId":   projectID,
		"environment":   environment,
		"secretPath":    secretPath,
		"secretValue":   value,
		"secretComment": comment,
		"type":          "shared",
	}
	path := fmt.Sprintf("/api/v3/secrets/raw/%s", url.PathEscape(key))
	return c.do(ctx, http.MethodPost, path, nil, body, nil, true)
}

// UpdateSecret replaces a secret's value.
func (c *Client) UpdateSecret(ctx context.Context, projectID, environment, secretPath, key, value string) error {
	if err := c.Login(ctx); err != nil {
		return err
	}
	body := map[string]any{
		"workspaceId": projectID,
		"environment": environment,
		"secretPath":  secretPath,
		"secretValue": value,
		"type":        "shared",
	}
	path := fmt.Sprintf("/api/v3/secrets/raw/%s", url.PathEscape(key))
	return c.do(ctx, http.MethodPatch, path, nil, body, nil, true)
}

// DeleteSecret removes a secret.
func (c *Client) DeleteSecret(ctx context.Context, projectID, environment, secretPath, key string) error {
	if err := c.Login(ctx); err != nil {
		return err
	}
	body := map[string]any{
		"workspaceId": projectID,
		"environment": environment,
		"secretPath":  secretPath,
		"type":        "shared",
	}
	path := fmt.Sprintf("/api/v3/secrets/raw/%s", url.PathEscape(key))
	return c.do(ctx, http.MethodDelete, path, nil, body, nil, true)
}

// Whoami returns identity metadata for the active token, when available.
func (c *Client) Whoami(ctx context.Context) (map[string]any, error) {
	if err := c.Login(ctx); err != nil {
		return nil, err
	}
	var out map[string]any
	if err := c.do(ctx, http.MethodGet, "/api/v1/auth/token", nil, nil, &out, true); err != nil {
		return nil, err
	}
	return out, nil
}

// ---------- internals ----------

func (c *Client) do(ctx context.Context, method, path string, headers map[string]string, in, out any, auth bool) error {
	var body io.Reader
	if in != nil {
		buf, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.hostURL+path, body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	if auth && c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return apiError(method, path, resp.StatusCode, respBody)
	}
	if out == nil || len(respBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("decode %s %s: %w", method, path, err)
	}
	return nil
}

// APIError is returned for non-2xx responses.
type APIError struct {
	Method  string
	Path    string
	Status  int
	Message string
	Raw     string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("infisical %s %s: %d %s", e.Method, e.Path, e.Status, e.Message)
	}
	return fmt.Sprintf("infisical %s %s: %d", e.Method, e.Path, e.Status)
}

func apiError(method, path string, status int, body []byte) error {
	e := &APIError{Method: method, Path: path, Status: status, Raw: string(body)}
	var parsed struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if json.Unmarshal(body, &parsed) == nil {
		if parsed.Message != "" {
			e.Message = parsed.Message
		} else if parsed.Error != "" {
			e.Message = parsed.Error
		}
	}
	return e
}
