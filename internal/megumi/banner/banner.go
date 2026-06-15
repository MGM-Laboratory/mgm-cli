// Package banner renders the Megumi Code wordmark with the MGM brand palette and
// full terminal-capability degradation:
//
//   - truecolor + TTY        → smooth color-cycling animation
//   - 256/16-color           → stepped static brand gradient
//   - NO_COLOR / dumb / CI / non-TTY → static plain text, no animation
//
// NO_COLOR and FORCE_COLOR are honored via charmbracelet/colorprofile.Detect.
// The package is shared by the pre-launch welcome (mgm) and the in-session
// header (the embedded agent).
package banner

import (
	"image/color"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/mattn/go-isatty"
)

// Brand palette (DESIGN_SYSTEM.md). Closed set; no purple/teal/pink/orange.
var (
	brandBlue   = lipgloss.Color("#3a6dc5")
	brandYellow = lipgloss.Color("#f7bf33")
	brandRed    = lipgloss.Color("#f94141")
	brandGreen  = lipgloss.Color("#0f8657")
)

// brandStops is the cycle order; the first color repeats at the end so a ramp
// built from it wraps seamlessly for animation.
var brandStops = []color.Color{brandBlue, brandGreen, brandYellow, brandRed, brandBlue}

// Profile detects the active terminal color profile for stdout (honors
// NO_COLOR / FORCE_COLOR).
func Profile() colorprofile.Profile {
	return colorprofile.Detect(os.Stdout, os.Environ())
}

// animatable reports whether stdout can show the cycling animation.
func animatable() bool {
	return isatty.IsTerminal(os.Stdout.Fd()) && Profile() == colorprofile.TrueColor
}

// Show writes the banner to w, animating when the terminal supports it and
// otherwise rendering a static (possibly plain) banner.
func Show(w io.Writer) {
	if animatable() {
		animate(w)
		return
	}
	io.WriteString(w, RenderStatic()+"\n")
}

// RenderStatic returns the banner as a static string, colored per the detected
// profile (a horizontal brand gradient), or plain text with no ANSI when the
// terminal has no color support.
func RenderStatic() string {
	return renderColored(Profile(), 0)
}

// renderColored colors every glyph by a horizontal brand ramp shifted by offset
// (offset drives the animation; 0 is the static frame). Colors are downsampled
// to the profile via Convert; when Convert yields nil (ASCII/NoTTY) the glyph is
// emitted plain.
func renderColored(profile colorprofile.Profile, offset int) string {
	width := Width()
	ramp := lipgloss.Blend1D(width, brandStops...)

	var b strings.Builder
	for row, line := range artLines {
		if row > 0 {
			b.WriteByte('\n')
		}
		for col, r := range []rune(line) {
			cc := profile.Convert(ramp[(col+offset)%width])
			if cc == nil {
				b.WriteRune(r)
				continue
			}
			b.WriteString(lipgloss.NewStyle().Foreground(cc).Render(string(r)))
		}
	}
	return b.String()
}

// animate loops a color-cycling render for a short, bounded duration, then
// leaves a settled static frame in place.
func animate(w io.Writer) {
	const (
		fps    = 18
		frames = 30 // ~1.7s
	)
	width := Width()
	up := len(artLines)
	for f := 0; f < frames; f++ {
		io.WriteString(w, renderColored(colorprofile.TrueColor, f%width))
		io.WriteString(w, "\n")
		if f < frames-1 {
			time.Sleep(time.Second / fps)
			// Move the cursor back to the top of the banner to redraw in place.
			io.WriteString(w, "\x1b["+strconv.Itoa(up)+"A\r")
		}
	}
}

// RenderInline returns the wordmark for in-session use (e.g. the agent header),
// colored with a horizontal gradient between cA and cB. When width is too narrow
// for the full art, a compact single-line "Megumi" wordmark is returned instead.
// Downsampling is left to the surrounding program's color-profile writer.
func RenderInline(width int, cA, cB color.Color) string {
	if width > 0 && width < Width() {
		return lipgloss.NewStyle().Bold(true).Foreground(cA).Render("Megumi")
	}
	aw := Width()
	ramp := lipgloss.Blend1D(aw, cA, cB)
	var b strings.Builder
	for row, line := range artLines {
		if row > 0 {
			b.WriteByte('\n')
		}
		for col, r := range []rune(line) {
			b.WriteString(lipgloss.NewStyle().Foreground(ramp[col%aw]).Render(string(r)))
		}
	}
	return b.String()
}
