package permission

import (
	"testing"
)

func TestModeNextCycles(t *testing.T) {
	if got := ModeNormal.Next(); got != ModeAcceptEdits {
		t.Errorf("normal.Next() = %v, want accept-edits", got)
	}
	if got := ModeAcceptEdits.Next(); got != ModePlan {
		t.Errorf("accept-edits.Next() = %v, want plan", got)
	}
	if got := ModePlan.Next(); got != ModeNormal {
		t.Errorf("plan.Next() = %v, want normal", got)
	}
}

func TestSetModeRoundTrip(t *testing.T) {
	s := NewPermissionService(t.TempDir(), false, nil)
	if s.Mode() != ModeNormal {
		t.Errorf("default mode = %v, want normal", s.Mode())
	}
	s.SetMode(ModePlan)
	if s.Mode() != ModePlan {
		t.Errorf("mode = %v, want plan", s.Mode())
	}
}

// req is a helper that issues a permission Request for a tool. The mode/skip
// short-circuit paths under test all resolve synchronously without a waiter,
// so the call never blocks.
func req(t *testing.T, s Service, tool string) bool {
	t.Helper()
	granted, err := s.Request(t.Context(), CreatePermissionRequest{
		SessionID:  "sess",
		ToolCallID: "call-" + tool,
		ToolName:   tool,
		Action:     "write",
		Path:       t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Request(%s): %v", tool, err)
	}
	return granted
}

func TestPlanModeBlocksMutatingTools(t *testing.T) {
	s := NewPermissionService(t.TempDir(), false, nil)
	s.SetMode(ModePlan)
	for _, tool := range []string{"edit", "write", "multiedit", "bash", "download"} {
		if req(t, s, tool) {
			t.Errorf("plan mode: %s should be blocked, but it was granted", tool)
		}
	}
}

func TestAcceptEditsAutoApprovesEditsOnly(t *testing.T) {
	s := NewPermissionService(t.TempDir(), false, nil)
	s.SetMode(ModeAcceptEdits)
	for _, tool := range []string{"edit", "write", "multiedit"} {
		if !req(t, s, tool) {
			t.Errorf("accept-edits: %s should be auto-approved", tool)
		}
	}
}

func TestSkipRequestsOverridesMode(t *testing.T) {
	s := NewPermissionService(t.TempDir(), true /* skip */, nil)
	s.SetMode(ModePlan)
	// Yolo (skip) wins over plan mode: even a mutating tool is granted.
	if !req(t, s, "bash") {
		t.Error("skip=true should grant even in plan mode")
	}
}
