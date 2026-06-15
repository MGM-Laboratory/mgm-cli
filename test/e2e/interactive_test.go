//go:build e2e && (linux || darwin)

package e2e

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
)

// TestE2E_InteractiveStartupAndQuit launches the interactive Megumi TUI under a
// pseudo-terminal (so IsInteractive() is true), authenticated with a seeded API
// code and pointed at the mock broker. It asserts the session starts (branded
// output appears) and then quits cleanly when sent Ctrl+C — i.e. the
// interactive entry path is wired and does not hang or crash.
//
// PTY tests are inherently timing-sensitive; this one is deliberately lenient:
// it waits for a startup marker, drives a quit, and falls back to killing the
// process so a stuck TUI fails as a timeout rather than wedging CI.
func TestE2E_InteractiveStartupAndQuit(t *testing.T) {
	requireBrokerCapable(t)

	broker := newMockBroker(t, "interactive reply")
	_, env := brokerEnv(t, broker)

	cmd := exec.Command(mgmBin, "megumi", "--api-code")
	cmd.Env = append(baseEnv(t), env...)
	// A real terminal type so the TUI initializes its renderer.
	cmd.Env = append(cmd.Env, "TERM=xterm-256color")

	ptmx, err := pty.Start(cmd)
	if err != nil {
		t.Fatalf("pty.Start: %v", err)
	}
	defer func() { _ = ptmx.Close() }()
	_ = pty.Setsize(ptmx, &pty.Winsize{Rows: 40, Cols: 120})

	// Drain the PTY into a buffer in the background.
	var mu sync.Mutex
	var buf bytes.Buffer
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		b := make([]byte, 4096)
		for {
			n, err := ptmx.Read(b)
			if n > 0 {
				mu.Lock()
				buf.Write(b[:n])
				mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()

	snapshot := func() string {
		mu.Lock()
		defer mu.Unlock()
		return buf.String()
	}

	// Wait for a branded startup marker (the credential line or the banner).
	if !waitFor(snapshot, []string{"Megumi", "API code"}, 30*time.Second) {
		_ = ptmx.Close()
		_ = cmd.Process.Kill()
		t.Fatalf("interactive session never produced startup output within timeout.\ngot:\n%s", snapshot())
	}

	// Ask the TUI to quit. Ctrl+C opens/confirms quit in the embedded agent;
	// send it a couple of times, then 'q', spaced out.
	for _, key := range [][]byte{{0x03}, {0x03}, []byte("q")} {
		_, _ = ptmx.Write(key)
		time.Sleep(400 * time.Millisecond)
	}

	if err := waitProcess(cmd, 20*time.Second); err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("interactive session did not exit after quit keys: %v\noutput:\n%s", err, snapshot())
	}

	// Confirm we actually saw branded startup (not just an immediate crash).
	out := snapshot()
	if !strings.Contains(out, "Megumi") && !strings.Contains(out, "API code") {
		t.Errorf("interactive output lacked branded startup markers:\n%s", out)
	}
	if strings.Contains(strings.ToLower(out), "panic:") {
		t.Errorf("interactive session panicked:\n%s", out)
	}
}

// waitFor polls snapshot() until it contains any of the wanted substrings or the
// timeout elapses.
func waitFor(snapshot func() string, wantAny []string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		s := snapshot()
		for _, w := range wantAny {
			if strings.Contains(s, w) {
				return true
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	return false
}

// waitProcess waits up to timeout for cmd to exit, returning an error only on
// timeout. Any exit (including a non-zero status from SIGINT/Ctrl+C) counts as a
// clean shutdown for this test's purposes.
func waitProcess(cmd *exec.Cmd, timeout time.Duration) error {
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return errors.New("process wait timed out")
	}
}
