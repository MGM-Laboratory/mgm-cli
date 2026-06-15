package styles

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/exp/charmtone"
)

// MGM brand palette (DESIGN_SYSTEM.md). The closed palette is blue/yellow/red/
// green — no purple, teal, pink, or orange.
var (
	brandBlue   = lipgloss.Color("#3a6dc5")
	brandYellow = lipgloss.Color("#f7bf33")
	brandRed    = lipgloss.Color("#f94141")
	brandGreen  = lipgloss.Color("#0f8657")
	brandOnFill = lipgloss.Color("#ffffff") // text on a brand fill
)

// ThemeForProvider returns the Styles associated with the given provider ID.
// Megumi Code always uses the MGM brand theme; the default branch covers our
// single "megumi" provider.
func ThemeForProvider(providerID string) Styles {
	return MegumiTheme()
}

// MegumiTheme is the MGM-branded dark theme: brand colors on neutral greys, with
// no purple anywhere (replacing Charm's Charple/Mauve). Neutral fg/bg greys are
// kept for terminal legibility; only the brand and status slots are brand colors.
func MegumiTheme() Styles {
	return quickStyle(quickStyleOpts{
		primary:   brandBlue,
		secondary: brandYellow,
		accent:    brandGreen,
		keyword:   brandBlue,

		fgBase:       charmtone.Sash,
		fgMoreSubtle: charmtone.Squid,
		fgSubtle:     charmtone.Smoke,
		fgMostSubtle: charmtone.Oyster,

		onPrimary: brandOnFill,

		bgBase:         charmtone.Pepper,
		bgLeastVisible: charmtone.BBQ,
		bgLessVisible:  charmtone.Char,
		bgMostVisible:  charmtone.Iron,

		separator: charmtone.Char,

		destructive:       brandRed,
		error:             brandRed,
		warningSubtle:     brandYellow,
		warning:           brandYellow,
		denied:            brandRed,
		busy:              brandYellow,
		info:              brandBlue,
		infoMoreSubtle:    brandBlue,
		infoMostSubtle:    brandBlue,
		success:           brandGreen,
		successMoreSubtle: brandGreen,
		successMostSubtle: brandGreen,
	})
}

// CharmtonePantera previously returned Charm's purple Charmtone theme. Megumi
// Code uses one brand theme everywhere, so this now delegates to MegumiTheme to
// keep any remaining call sites (and the logo example) brand-consistent.
func CharmtonePantera() Styles { return MegumiTheme() }

// HypercrushObsidiana likewise delegates to the brand theme.
func HypercrushObsidiana() Styles { return MegumiTheme() }
