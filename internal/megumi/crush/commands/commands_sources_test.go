package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MGM-Laboratory/mgm-cli/internal/megumi/crush/config"
)

// Under Megumi the global-config dir and the data directory resolve to the same
// place (~/.mgm/megumi); buildCommandSources must surface Megumi-branded paths
// (including project-local .megumi/commands) and must not list any directory
// twice.
func TestBuildCommandSourcesMegumiPathsAndDedup(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CRUSH_GLOBAL_CONFIG", home) // Megumi points this at ~/.mgm/megumi
	cfg := &config.Config{Options: &config.Options{DataDirectory: home}}

	counts := map[string]int{}
	for _, s := range buildCommandSources(cfg) {
		counts[s.path]++
	}

	for p, n := range counts {
		if n > 1 {
			t.Errorf("path %q appears %d times; want deduped", p, n)
		}
	}

	globalCmds := filepath.Join(home, "commands")
	if counts[globalCmds] == 0 {
		t.Errorf("expected global commands dir %q in sources", globalCmds)
	}

	if wd, err := os.Getwd(); err == nil {
		projectCmds := filepath.Join(wd, ".megumi", "commands")
		if counts[projectCmds] == 0 {
			t.Errorf("expected project-local source %q", projectCmds)
		}
	}
}
