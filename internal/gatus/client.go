// Package gatus is a thin REST client for a Gatus status page
// (https://github.com/TwiN/gatus). The CLI uses it to surface the health of
// MGM services. Built directly against the documented HTTP API.
package gatus

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client talks to a Gatus instance.
type Client struct {
	baseURL   string
	token     string
	http      *http.Client
	userAgent string
}

// New constructs a Client. baseURL is the Gatus root (e.g. https://status.labmgm.org).
// token is optional; pass "" when the instance is unauthenticated.
func New(baseURL, token, userAgent string) *Client {
	return &Client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		token:     token,
		http:      &http.Client{Timeout: 20 * time.Second},
		userAgent: userAgent,
	}
}

// BaseURL returns the configured Gatus root.
func (c *Client) BaseURL() string { return c.baseURL }

// ConditionResult mirrors Gatus' per-condition result.
type ConditionResult struct {
	Condition string `json:"condition"`
	Success   bool   `json:"success"`
}

// Result is one health-check sample.
type Result struct {
	Status            int               `json:"status"`
	Hostname          string            `json:"hostname"`
	IP                string            `json:"ip,omitempty"`
	Duration          int64             `json:"duration"` // nanoseconds
	Errors            []string          `json:"errors,omitempty"`
	ConditionResults  []ConditionResult `json:"conditionResults,omitempty"`
	Success           bool              `json:"success"`
	Timestamp         time.Time         `json:"timestamp"`
	CertificateExpiry int64             `json:"certificateExpiration,omitempty"`
}

// Endpoint groups recent results for a single Gatus endpoint.
type Endpoint struct {
	Name   string   `json:"name"`
	Group  string   `json:"group"`
	Key    string   `json:"key"`
	Result []Result `json:"results"`
}

// LatestResult returns the most recent result, or nil if none.
func (e Endpoint) LatestResult() *Result {
	if len(e.Result) == 0 {
		return nil
	}
	return &e.Result[len(e.Result)-1]
}

// Healthy reports whether the latest result succeeded.
func (e Endpoint) Healthy() bool {
	r := e.LatestResult()
	return r != nil && r.Success
}

// FullName returns "group/name" or just "name" when group is empty.
func (e Endpoint) FullName() string {
	if e.Group == "" {
		return e.Name
	}
	return e.Group + "/" + e.Name
}

// ListEndpoints fetches all endpoint statuses (page=1, large page size).
func (c *Client) ListEndpoints(ctx context.Context) ([]Endpoint, error) {
	q := url.Values{
		"page":     {"1"},
		"pageSize": {"200"},
	}
	var endpoints []Endpoint
	if err := c.do(ctx, http.MethodGet, "/api/v1/endpoints/statuses?"+q.Encode(), &endpoints); err != nil {
		return nil, fmt.Errorf("list endpoints: %w", err)
	}
	return endpoints, nil
}

// GetEndpoint fetches a single endpoint by Gatus key (e.g. "core_api").
func (c *Client) GetEndpoint(ctx context.Context, key string) (*Endpoint, error) {
	var ep Endpoint
	path := fmt.Sprintf("/api/v1/endpoints/%s/statuses", url.PathEscape(key))
	if err := c.do(ctx, http.MethodGet, path, &ep); err != nil {
		return nil, fmt.Errorf("get endpoint %s: %w", key, err)
	}
	return &ep, nil
}

// Uptime returns the uptime ratio (0.0–1.0) over window for the given key.
// Valid windows: 1h, 24h, 7d, 30d.
func (c *Client) Uptime(ctx context.Context, key, window string) (float64, error) {
	path := fmt.Sprintf("/api/v1/endpoints/%s/uptimes/%s", url.PathEscape(key), url.PathEscape(window))
	req, err := c.newRequest(ctx, http.MethodGet, path)
	if err != nil {
		return 0, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return 0, apiError(http.MethodGet, path, resp.StatusCode, body)
	}
	s := strings.TrimSpace(string(body))
	var v float64
	if _, err := fmt.Sscanf(s, "%f", &v); err != nil {
		return 0, fmt.Errorf("parse uptime %q: %w", s, err)
	}
	return v, nil
}

// BadgeURL returns a URL to a Shields-style SVG badge for the endpoint.
// kind is "uptimes/24h", "response-times/24h", or "health".
func (c *Client) BadgeURL(key, kind string) string {
	return fmt.Sprintf("%s/api/v1/endpoints/%s/%s/badge.svg", c.baseURL, url.PathEscape(key), kind)
}

// EndpointPageURL returns the human-friendly page URL for an endpoint.
func (c *Client) EndpointPageURL(key string) string {
	return fmt.Sprintf("%s/endpoints/%s", c.baseURL, url.PathEscape(key))
}

// DeriveKey applies Gatus' key convention: "<group>_<name>", lowercased,
// non-alphanumerics collapsed to dashes. Useful when the user types a name.
func DeriveKey(group, name string) string {
	g := slug(group)
	n := slug(name)
	if g == "" {
		return n
	}
	return g + "_" + n
}

// FindEndpoint returns the endpoint that best matches q. Match precedence:
//  1. exact key
//  2. case-insensitive name
//  3. case-insensitive "group/name"
//  4. case-insensitive substring of name or key
//
// Returns (nil, nil) if no match. Returns ambiguity error if >1 match in tier 4.
func FindEndpoint(endpoints []Endpoint, q string) (*Endpoint, error) {
	if q == "" {
		return nil, nil
	}
	ql := strings.ToLower(q)
	for i := range endpoints {
		if endpoints[i].Key == q {
			return &endpoints[i], nil
		}
	}
	for i := range endpoints {
		if strings.EqualFold(endpoints[i].Name, q) {
			return &endpoints[i], nil
		}
	}
	for i := range endpoints {
		if strings.EqualFold(endpoints[i].FullName(), q) {
			return &endpoints[i], nil
		}
	}
	var matches []*Endpoint
	for i := range endpoints {
		name := strings.ToLower(endpoints[i].Name)
		key := strings.ToLower(endpoints[i].Key)
		if strings.Contains(name, ql) || strings.Contains(key, ql) {
			matches = append(matches, &endpoints[i])
		}
	}
	switch len(matches) {
	case 0:
		return nil, nil
	case 1:
		return matches[0], nil
	default:
		names := make([]string, 0, len(matches))
		for _, m := range matches {
			names = append(names, m.FullName())
		}
		return nil, fmt.Errorf("ambiguous service %q, matches: %s", q, strings.Join(names, ", "))
	}
}

func slug(s string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteRune('-')
				prevDash = true
			}
		}
	}
	out := b.String()
	return strings.Trim(out, "-")
}

// ---------- internals ----------

func (c *Client) newRequest(ctx context.Context, method, path string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return req, nil
}

func (c *Client) do(ctx context.Context, method, path string, out any) error {
	req, err := c.newRequest(ctx, method, path)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return apiError(method, path, resp.StatusCode, body)
	}
	if out == nil || len(body) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
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
		return fmt.Sprintf("gatus %s %s: %d %s", e.Method, e.Path, e.Status, e.Message)
	}
	return fmt.Sprintf("gatus %s %s: %d", e.Method, e.Path, e.Status)
}

func apiError(method, path string, status int, body []byte) error {
	e := &APIError{Method: method, Path: path, Status: status, Raw: string(body)}
	if status == http.StatusNotFound {
		e.Message = "not found"
		return e
	}
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
