package credstore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"golang.org/x/crypto/hkdf"
)

// The file backend is the fallback when no OS keychain is usable. It writes a
// 0600 file containing AES-256-GCM ciphertext, so credentials are never on disk
// in plaintext. The encryption key is derived from a per-install random salt
// plus host/user context; this is best-effort at-rest obfuscation against a
// casual reader, NOT protection against a determined local attacker (the salt
// lives next to the ciphertext). The OS keychain is strongly preferred and used
// automatically whenever available.
const (
	credFileName = "credentials.enc"
	saltFileName = ".cred-salt"
	hkdfInfo     = "mgm-megumi credstore v1"
)

type fileBackend struct {
	dir      string
	credPath string
	saltPath string
}

func newFileBackend(dir string) (*fileBackend, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create %s: %w", dir, err)
	}
	return &fileBackend{
		dir:      dir,
		credPath: filepath.Join(dir, credFileName),
		saltPath: filepath.Join(dir, saltFileName),
	}, nil
}

func (f *fileBackend) name() string { return "file" }

func (f *fileBackend) get() ([]byte, error) {
	ct, err := os.ReadFile(f.credPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	gcm, err := f.cipher()
	if err != nil {
		return nil, err
	}
	if len(ct) < gcm.NonceSize() {
		return nil, fmt.Errorf("credentials file is corrupt")
	}
	nonce, body := ct[:gcm.NonceSize()], ct[gcm.NonceSize():]
	pt, err := gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt credentials (file may be corrupt or from another machine): %w", err)
	}
	return pt, nil
}

func (f *fileBackend) set(b []byte) error {
	gcm, err := f.cipher()
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	ct := gcm.Seal(nonce, nonce, b, nil)
	return writeFile0600(f.credPath, ct)
}

func (f *fileBackend) clear() error {
	if err := os.Remove(f.credPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// cipher derives the AES-GCM AEAD from the per-install salt and host context.
func (f *fileBackend) cipher() (cipher.AEAD, error) {
	salt, err := f.salt()
	if err != nil {
		return nil, err
	}
	host, _ := os.Hostname()
	context := fmt.Sprintf("%s|%s|uid=%d", host, runtime.GOOS, os.Getuid())

	key := make([]byte, 32)
	r := hkdf.New(sha256.New, salt, []byte(context), []byte(hkdfInfo))
	if _, err := io.ReadFull(r, key); err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// salt loads the per-install random salt, creating it on first use.
func (f *fileBackend) salt() ([]byte, error) {
	if b, err := os.ReadFile(f.saltPath); err == nil {
		if len(b) >= 16 {
			return b, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	salt := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	if err := writeFile0600(f.saltPath, salt); err != nil {
		return nil, err
	}
	return salt, nil
}

func writeFile0600(path string, b []byte) error {
	// Write to a temp file then rename for atomicity, ensuring 0600 throughout.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
