//go:build e2e

// Package e2e holds black-box, binary-driven tests for the `mgm` CLI. They
// build (or reuse) a real `mgm` binary and exercise it as a subprocess, so they
// validate the shipped behavior end to end: argument handling, the auth →
// broker → model pipeline, branding, and graceful error paths.
//
// These tests are guarded by the `e2e` build tag so they never run during the
// fast unit pass (`go test ./...`). Run them with:
//
//	go test -tags e2e ./test/e2e/...
//
// CI builds the binary once and passes its path via MGM_BIN; locally, TestMain
// builds a throwaway binary automatically.
package e2e

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

// mgmBin is the absolute path to the `mgm` binary under test, resolved once in
// TestMain.
var mgmBin string

func TestMain(m *testing.M) {
	bin, cleanup, err := resolveBinary()
	if err != nil {
		fmt.Fprintln(os.Stderr, "e2e: failed to obtain mgm binary:", err)
		os.Exit(1)
	}
	mgmBin = bin
	code := m.Run()
	if cleanup != nil {
		cleanup()
	}
	os.Exit(code)
}

// resolveBinary returns the binary to test. If MGM_BIN is set (CI builds it
// once with the right ldflags) it is used as-is; otherwise a throwaway binary
// is built into a temp dir and a cleanup func is returned.
func resolveBinary() (string, func(), error) {
	if b := os.Getenv("MGM_BIN"); b != "" {
		abs, err := filepath.Abs(b)
		if err != nil {
			return "", nil, err
		}
		if _, err := os.Stat(abs); err != nil {
			return "", nil, fmt.Errorf("MGM_BIN=%s: %w", b, err)
		}
		return abs, nil, nil
	}

	dir, err := os.MkdirTemp("", "mgm-e2e-bin")
	if err != nil {
		return "", nil, err
	}
	name := "mgm"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	out := filepath.Join(dir, name)

	// Build from the module root (two levels up from test/e2e).
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		return "", nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build",
		"-ldflags", "-X github.com/MGM-Laboratory/mgm-cli/internal/version.Version=e2e-test",
		"-o", out, "./cmd/mgm")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if outBytes, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		return "", nil, fmt.Errorf("go build failed: %v\n%s", err, outBytes)
	}
	return out, func() { os.RemoveAll(dir) }, nil
}

// runResult captures the outcome of one CLI invocation.
type runResult struct {
	Stdout   string
	Stderr   string
	Combined string
	ExitCode int
	TimedOut bool
}

// run executes `mgm <args...>` with the given extra environment and a generous
// timeout. stdin is closed (EOF) so non-interactive paths never block waiting
// for input. The harness always returns a result (never fatals) so individual
// tests decide what success means.
func run(t *testing.T, env []string, args ...string) runResult {
	t.Helper()
	return runWithTimeout(t, 60*time.Second, nil, env, args...)
}

// runWithTimeout is run() with a caller-chosen timeout and optional stdin bytes.
func runWithTimeout(t *testing.T, timeout time.Duration, stdin []byte, env []string, args ...string) runResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, mgmBin, args...)
	cmd.Env = append(baseEnv(t), env...)
	// Always attach an io.Reader (empty by default) so exec wires the child's
	// stdin to a pipe rather than /dev/null. A pipe is NOT a character device,
	// so the CLI correctly detects a non-interactive (piped/redirected) session
	// — matching real `... | mgm megumi -p` usage and avoiding a spurious TTY
	// prompt.
	if stdin == nil {
		stdin = []byte{}
	}
	cmd.Stdin = bytes.NewReader(stdin)

	var stdout, stderr bytes.Buffer
	combined := &syncBuffer{}
	cmd.Stdout = &teeWriter{dst: &stdout, shared: combined}
	cmd.Stderr = &teeWriter{dst: &stderr, shared: combined}

	err := cmd.Run()

	res := runResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Combined: combined.String(),
		ExitCode: 0,
	}
	if ctx.Err() == context.DeadlineExceeded {
		res.TimedOut = true
		res.ExitCode = -1
		return res
	}
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			res.ExitCode = ee.ExitCode()
		} else {
			res.ExitCode = -1
			res.Stderr += "\n[harness] " + err.Error()
		}
	}
	return res
}

// baseEnv builds a hermetic environment for a CLI invocation: an isolated
// home/config tree and disabled telemetry, so tests never touch the developer's
// real ~/.mgm or phone home. Per-test env entries are appended by callers and
// win on duplicate keys (Go uses the last value).
func baseEnv(t *testing.T) []string {
	t.Helper()
	tmp := t.TempDir()
	megHome := filepath.Join(tmp, "mgm", "megumi")
	if err := os.MkdirAll(megHome, 0o700); err != nil {
		t.Fatalf("mkdir megumi home: %v", err)
	}
	env := []string{
		"HOME=" + tmp,
		"USERPROFILE=" + tmp, // Windows
		"XDG_CONFIG_HOME=" + filepath.Join(tmp, ".config"),
		"XDG_CACHE_HOME=" + filepath.Join(tmp, ".cache"),
		"XDG_DATA_HOME=" + filepath.Join(tmp, ".local", "share"),
		"MGM_HOME=" + filepath.Join(tmp, "mgm"),
		"MEGUMI_HOME=" + megHome,
		"MEGUMI_CRED_STORE=file", // avoid the OS keychain in CI
		"DO_NOT_TRACK=1",
		"CRUSH_DISABLE_METRICS=1",
		"NO_COLOR=1",
	}
	// Preserve PATH/SystemRoot so `go`, the shell, and Windows DLLs resolve.
	for _, k := range []string{"PATH", "Path", "SystemRoot", "TEMP", "TMP", "ComSpec", "PATHEXT", "GOROOT", "GOPATH", "GOCACHE"} {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	return env
}

// syncBuffer is a bytes.Buffer guarded by a mutex, so stdout and stderr copier
// goroutines can both append to the merged "combined" view safely.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// teeWriter duplicates writes to a per-stream buffer and the shared merged
// buffer so a test can read a stream on its own or merged with the other (for
// order-insensitive asserts). The per-stream buffer is only ever written by a
// single goroutine; the shared buffer is synchronized.
type teeWriter struct {
	dst    *bytes.Buffer
	shared *syncBuffer
}

func (w *teeWriter) Write(p []byte) (int, error) {
	w.dst.Write(p)
	return w.shared.Write(p)
}
