// Package config handles persistent CLI configuration stored in ~/.mgm/config.
//
// Format is TOML with one section per profile. The "default" profile is used
// unless the user passes --profile or sets MGM_PROFILE.
//
//	[default]
//	host_url       = "https://secrets.labmgm.org"
//	client_id      = "..."
//	client_secret  = "..."
//	default_project_id  = ""
//	default_environment = "dev"
//	default_folder      = "/"
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/viper"
)

const (
	DefaultHostURL  = "https://secrets.labmgm.org"
	DefaultGatusURL = "https://status.labmgm.org"
	DefaultProfile  = "default"
)

// Profile is the persisted credentials/defaults for one named profile.
// Fields are grouped by namespace: env (Infisical) and status (Gatus).
type Profile struct {
	// env / Infisical
	HostURL            string `mapstructure:"host_url"`
	ClientID           string `mapstructure:"client_id"`
	ClientSecret       string `mapstructure:"client_secret"`
	DefaultProjectID   string `mapstructure:"default_project_id"`
	DefaultEnvironment string `mapstructure:"default_environment"`
	DefaultFolder      string `mapstructure:"default_folder"`

	// status / Gatus
	GatusURL   string `mapstructure:"gatus_url"`
	GatusToken string `mapstructure:"gatus_token"`
}

// IsConfigured returns true when there's enough to authenticate to Infisical.
func (p Profile) IsConfigured() bool {
	return p.ClientID != "" && p.ClientSecret != "" && p.HostURL != ""
}

// HasGatus returns true when a Gatus URL is set (token is optional).
func (p Profile) HasGatus() bool {
	return p.GatusURL != ""
}

// Manager loads and saves the on-disk config file.
type Manager struct {
	path    string
	profile string
	v       *viper.Viper
}

// New returns a Manager pointing at ~/.mgm/config (or $MGM_CONFIG when set).
// profile may be empty, in which case the "default" profile is used.
func New(profile string) (*Manager, error) {
	if profile == "" {
		profile = os.Getenv("MGM_PROFILE")
	}
	if profile == "" {
		profile = DefaultProfile
	}

	path, err := configPath()
	if err != nil {
		return nil, err
	}

	v := viper.New()
	v.SetConfigType("toml")
	v.SetConfigFile(path)

	if _, statErr := os.Stat(path); statErr == nil {
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("read config %s: %w", path, err)
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return nil, statErr
	}

	return &Manager{path: path, profile: profile, v: v}, nil
}

// Path returns the config file path on disk.
func (m *Manager) Path() string { return m.path }

// Profile returns the active profile name.
func (m *Manager) ProfileName() string { return m.profile }

// Load returns the active profile, applying environment overrides on top of
// what's stored in the file. Missing fields are returned as empty strings,
// caller is responsible for prompting if IsConfigured() is false.
func (m *Manager) Load() Profile {
	sub := m.v.Sub(m.profile)
	var p Profile
	if sub != nil {
		_ = sub.Unmarshal(&p)
	}

	// Environment overrides (useful in CI).
	if v := os.Getenv("MGM_HOST_URL"); v != "" {
		p.HostURL = v
	}
	if v := os.Getenv("MGM_CLIENT_ID"); v != "" {
		p.ClientID = v
	}
	if v := os.Getenv("MGM_CLIENT_SECRET"); v != "" {
		p.ClientSecret = v
	}
	if v := os.Getenv("MGM_PROJECT_ID"); v != "" {
		p.DefaultProjectID = v
	}
	if v := os.Getenv("MGM_ENVIRONMENT"); v != "" {
		p.DefaultEnvironment = v
	}
	if v := os.Getenv("MGM_FOLDER"); v != "" {
		p.DefaultFolder = v
	}
	if v := os.Getenv("MGM_GATUS_URL"); v != "" {
		p.GatusURL = v
	}
	if v := os.Getenv("MGM_GATUS_TOKEN"); v != "" {
		p.GatusToken = v
	}

	if p.HostURL == "" {
		p.HostURL = DefaultHostURL
	}
	if p.GatusURL == "" {
		p.GatusURL = DefaultGatusURL
	}
	if p.DefaultFolder == "" {
		p.DefaultFolder = "/"
	}
	return p
}

// Save persists p under the active profile, creating ~/.mgm if needed.
// Existing other profiles are preserved.
func (m *Manager) Save(p Profile) error {
	if err := os.MkdirAll(filepath.Dir(m.path), 0o700); err != nil {
		return err
	}

	m.v.Set(m.profile+".host_url", p.HostURL)
	m.v.Set(m.profile+".client_id", p.ClientID)
	m.v.Set(m.profile+".client_secret", p.ClientSecret)
	m.v.Set(m.profile+".default_project_id", p.DefaultProjectID)
	m.v.Set(m.profile+".default_environment", p.DefaultEnvironment)
	m.v.Set(m.profile+".default_folder", p.DefaultFolder)
	m.v.Set(m.profile+".gatus_url", p.GatusURL)
	m.v.Set(m.profile+".gatus_token", p.GatusToken)

	if err := m.v.WriteConfigAs(m.path); err != nil {
		return fmt.Errorf("write config %s: %w", m.path, err)
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(m.path, 0o600)
	}
	return nil
}

// Profiles returns the names of all profiles in the config file.
func (m *Manager) Profiles() []string {
	keys := m.v.AllKeys()
	seen := map[string]struct{}{}
	var out []string
	for _, k := range keys {
		section := strings.SplitN(k, ".", 2)[0]
		if _, ok := seen[section]; ok {
			continue
		}
		seen[section] = struct{}{}
		out = append(out, section)
	}
	return out
}

func configPath() (string, error) {
	if v := os.Getenv("MGM_CONFIG"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home directory: %w", err)
	}
	return filepath.Join(home, ".mgm", "config"), nil
}
