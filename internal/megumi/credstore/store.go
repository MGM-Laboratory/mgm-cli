// Package credstore persists Megumi Code credentials under ~/.mgm/megumi. It is
// the shared home for both CLI auth methods (mgm account and, later, a Megumi
// API code). Credentials are stored in the OS keychain when one is usable, and
// otherwise in a 0600 AES-256-GCM encrypted file. Plaintext credentials are
// never written to disk.
package credstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Method identifies which Megumi auth method a record represents.
type Method string

const (
	// MethodAccount is a Keycloak mgm-account login (OIDC tokens).
	MethodAccount Method = "account"
	// MethodAPICode is a Megumi API code. Reserved for a later phase; the record
	// shape supports it so the store stays the shared home for both methods.
	MethodAPICode Method = "api_code"
)

// Credentials is the persisted record shared by `mgm auth` and `mgm megumi`.
type Credentials struct {
	Method Method `json:"method"`

	// OIDC tokens (Method == MethodAccount).
	AccessToken  string    `json:"access_token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	IDToken      string    `json:"id_token,omitempty"`
	TokenType    string    `json:"token_type,omitempty"`
	Expiry       time.Time `json:"expiry,omitempty"`

	// Megumi API code (Method == MethodAPICode). Reserved for a later phase.
	APICode string `json:"api_code,omitempty"`

	// Identity metadata captured at login so whoami/status render offline.
	Issuer  string `json:"issuer,omitempty"`
	Subject string `json:"subject,omitempty"`
	Email   string `json:"email,omitempty"`
	Name    string `json:"name,omitempty"`
	Role    string `json:"role,omitempty"`

	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// ErrNotFound is returned by Load when no credentials are stored.
var ErrNotFound = errors.New("no Megumi credentials stored")

// backend abstracts the concrete secret store (keychain or encrypted file).
type backend interface {
	get() ([]byte, error) // returns ErrNotFound when absent
	set([]byte) error
	clear() error
	name() string
}

// Store reads and writes Credentials through a backend.
type Store struct {
	be backend
}

// New selects a backend. By default the OS keychain is used when it is usable,
// otherwise a 0600 encrypted file under ~/.mgm/megumi. MEGUMI_CRED_STORE may be
// set to "keychain" or "file" to force a specific backend (e.g. "file" on a
// headless server or in tests).
func New() (*Store, error) {
	d, err := storeDir()
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MEGUMI_CRED_STORE"))) {
	case "file":
		return fileStore(d)
	case "keychain":
		return &Store{be: keychainBackend{}}, nil
	case "", "auto":
		if keychainUsable() {
			return &Store{be: keychainBackend{}}, nil
		}
		return fileStore(d)
	default:
		return nil, fmt.Errorf("invalid MEGUMI_CRED_STORE (want auto|keychain|file)")
	}
}

func fileStore(dir string) (*Store, error) {
	fb, err := newFileBackend(dir)
	if err != nil {
		return nil, err
	}
	return &Store{be: fb}, nil
}

// Backend returns the active backend name ("keychain" or "file") for display.
func (s *Store) Backend() string { return s.be.name() }

// Load returns the stored credentials, or ErrNotFound when none exist.
func (s *Store) Load() (*Credentials, error) {
	b, err := s.be.get()
	if err != nil {
		return nil, err
	}
	var c Credentials
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("decode stored credentials: %w", err)
	}
	return &c, nil
}

// Save persists creds, stamping UpdatedAt.
func (s *Store) Save(c *Credentials) error {
	c.UpdatedAt = time.Now().UTC()
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return s.be.set(b)
}

// Clear removes any stored credentials. It is a no-op when none exist.
func (s *Store) Clear() error { return s.be.clear() }

// storeDir is ~/.mgm/megumi, overridable via MEGUMI_HOME (used by tests).
func storeDir() (string, error) {
	if v := strings.TrimSpace(os.Getenv("MEGUMI_HOME")); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home directory: %w", err)
	}
	return filepath.Join(home, ".mgm", "megumi"), nil
}
