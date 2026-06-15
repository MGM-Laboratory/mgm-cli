package credstore

import (
	"errors"

	"github.com/zalando/go-keyring"
)

// The keychain backend stores the JSON credential blob as a single secret. The
// library is pure-Go (no cgo): macOS Keychain via /usr/bin/security, Windows
// Credential Manager via wincred, Linux Secret Service via D-Bus/libsecret.
const (
	keychainService = "mgm-megumi"
	keychainUser    = "credentials"
	keychainProbe   = "__probe__"
)

type keychainBackend struct{}

func (keychainBackend) name() string { return "keychain" }

func (keychainBackend) get() ([]byte, error) {
	s, err := keyring.Get(keychainService, keychainUser)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return []byte(s), nil
}

func (keychainBackend) set(b []byte) error {
	return keyring.Set(keychainService, keychainUser, string(b))
}

func (keychainBackend) clear() error {
	if err := keyring.Delete(keychainService, keychainUser); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return err
	}
	return nil
}

// keychainUsable probes whether the OS keychain is operational by writing and
// deleting a sentinel value. On platforms without a secret service (e.g. a
// headless Linux box with no D-Bus) this returns false so callers fall back to
// the encrypted file.
func keychainUsable() bool {
	if err := keyring.Set(keychainService, keychainProbe, "1"); err != nil {
		return false
	}
	_ = keyring.Delete(keychainService, keychainProbe)
	return true
}
