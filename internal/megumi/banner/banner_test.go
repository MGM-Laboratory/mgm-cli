package banner

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

func TestArtIsRectangular(t *testing.T) {
	w := Width()
	for i, line := range artLines {
		if got := len([]rune(line)); got != w {
			t.Errorf("row %d width = %d, want %d", i, got, w)
		}
	}
}

func TestRenderPlainUnderNoColorProfile(t *testing.T) {
	// ASCII / NoTTY profiles must yield plain text with no ANSI escapes.
	for _, p := range []colorprofile.Profile{colorprofile.NoTTY, colorprofile.Ascii} {
		out := renderColored(p, 0)
		if strings.Contains(out, "\x1b[") {
			t.Errorf("profile %v: expected no ANSI escapes, got some", p)
		}
		// The block glyphs themselves must still be present.
		if !strings.Contains(out, "█") {
			t.Errorf("profile %v: banner glyphs missing", p)
		}
	}
}

func TestRenderColoredUnderTrueColor(t *testing.T) {
	out := renderColored(colorprofile.TrueColor, 0)
	if !strings.Contains(out, "\x1b[") {
		t.Fatal("truecolor render should contain ANSI escapes")
	}
}

func TestRenderInlineCompactWhenNarrow(t *testing.T) {
	out := RenderInline(4, lipgloss.Color("#3a6dc5"), lipgloss.Color("#0f8657"))
	if !strings.Contains(out, "Megumi") {
		t.Errorf("narrow inline render should fall back to a compact wordmark, got %q", out)
	}
	if strings.Contains(out, "█") {
		t.Error("narrow inline render should not include the full block art")
	}
}

func TestRenderInlineFullArtWhenWide(t *testing.T) {
	out := RenderInline(100, lipgloss.Color("#3a6dc5"), lipgloss.Color("#0f8657"))
	if !strings.Contains(out, "█") {
		t.Error("wide inline render should include the full block art")
	}
}
