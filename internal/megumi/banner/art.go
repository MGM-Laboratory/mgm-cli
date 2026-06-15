package banner

// artLines is the "MEGUMI" wordmark in a simple, fixed-width block font (each
// glyph is a 5x5 cell separated by one space, so every row is exactly 35 cells
// wide and trivially aligned). Kept deliberately plain so it renders the same on
// any terminal; color is applied separately and degrades per capability.
var artLines = []string{
	"█   █ █████ █████ █   █ █   █ █████",
	"██ ██ █     █     █   █ ██ ██   █  ",
	"█ █ █ ████  █  ██ █   █ █ █ █   █  ",
	"█   █ █     █   █ █   █ █   █   █  ",
	"█   █ █████ █████ █████ █   █ █████",
}

// Art returns the static (uncolored) MEGUMI banner as a single string.
func Art() string {
	out := ""
	for i, l := range artLines {
		if i > 0 {
			out += "\n"
		}
		out += l
	}
	return out
}

// Width is the fixed render width of the banner in cells.
func Width() int { return len([]rune(artLines[0])) }
