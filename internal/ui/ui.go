// Package ui centralises styled output and interactive prompts. All Charm
// huh/lipgloss usage is hidden here so commands stay terse.
package ui

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	warnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
	errStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	keyStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#06B6D4"))
)

// Out and Err are split so callers can redirect.
var (
	Out io.Writer = os.Stdout
	Err io.Writer = os.Stderr
)

// Title prints a styled section header.
func Title(s string) { fmt.Fprintln(Out, titleStyle.Render(s)) }

// Successf, Warnf, Errorf, Infof are sprintf-style status lines.
func Successf(format string, a ...any) { fmt.Fprintln(Out, successStyle.Render(fmt.Sprintf(format, a...))) }
func Warnf(format string, a ...any)    { fmt.Fprintln(Err, warnStyle.Render(fmt.Sprintf(format, a...))) }
func Errorf(format string, a ...any)   { fmt.Fprintln(Err, errStyle.Render(fmt.Sprintf(format, a...))) }
func Infof(format string, a ...any)    { fmt.Fprintln(Out, fmt.Sprintf(format, a...)) }
func Dim(s string) string              { return dimStyle.Render(s) }
func Key(s string) string              { return keyStyle.Render(s) }
func SuccessText(s string) string      { return successStyle.Render(s) }
func WarnText(s string) string         { return warnStyle.Render(s) }
func ErrorText(s string) string        { return errStyle.Render(s) }

// KV prints a key:value line with the key in colour.
func KV(k, v string) {
	fmt.Fprintf(Out, "%s %s\n", keyStyle.Render(k+":"), v)
}

// IsInteractive reports whether stdin & stderr appear to be a terminal,
// used to decide whether to fall back to non-interactive defaults.
func IsInteractive() bool {
	if os.Getenv("MGM_NO_TUI") != "" {
		return false
	}
	if fi, err := os.Stdin.Stat(); err == nil {
		if (fi.Mode() & os.ModeCharDevice) == 0 {
			return false
		}
	}
	return true
}

// PromptString shows a single text input. defaultVal is shown in [...] in the
// title when non-empty.
func PromptString(title, defaultVal string) (string, error) {
	titleText := title
	if defaultVal != "" {
		titleText = fmt.Sprintf("%s [%s]", title, defaultVal)
	} else {
		titleText = fmt.Sprintf("%s [None]", title)
	}
	val := defaultVal
	err := huh.NewInput().
		Title(titleText).
		Value(&val).
		Run()
	if err != nil {
		return "", err
	}
	if val == "" {
		val = defaultVal
	}
	return val, nil
}

// PromptSecret is like PromptString but masked.
func PromptSecret(title, defaultVal string) (string, error) {
	titleText := title
	if defaultVal != "" {
		titleText = fmt.Sprintf("%s [%s]", title, mask(defaultVal))
	} else {
		titleText = fmt.Sprintf("%s [None]", title)
	}
	var val string
	err := huh.NewInput().
		Title(titleText).
		EchoMode(huh.EchoModePassword).
		Value(&val).
		Run()
	if err != nil {
		return "", err
	}
	if val == "" {
		val = defaultVal
	}
	return val, nil
}

// Choice is a labeled value used by SelectOne.
type Choice struct {
	Label string
	Value string
	Hint  string
}

// SelectOne shows a single-select picker. Returns ErrAborted on Ctrl-C.
func SelectOne(title string, choices []Choice) (string, error) {
	if len(choices) == 0 {
		return "", fmt.Errorf("%s: no choices available", title)
	}
	opts := make([]huh.Option[string], 0, len(choices))
	for _, c := range choices {
		label := c.Label
		if c.Hint != "" {
			label = fmt.Sprintf("%s  %s", c.Label, dimStyle.Render(c.Hint))
		}
		opts = append(opts, huh.NewOption(label, c.Value))
	}
	var picked string
	err := huh.NewSelect[string]().
		Title(title).
		Options(opts...).
		Value(&picked).
		Run()
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return "", ErrAborted
		}
		return "", err
	}
	return picked, nil
}

// Confirm prompts for a yes/no answer.
func Confirm(title string, defaultYes bool) (bool, error) {
	val := defaultYes
	err := huh.NewConfirm().
		Title(title).
		Affirmative("Yes").
		Negative("No").
		Value(&val).
		Run()
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return false, ErrAborted
		}
		return false, err
	}
	return val, nil
}

// ErrAborted is returned when the user cancels a prompt.
var ErrAborted = errors.New("aborted by user")

func mask(v string) string {
	if len(v) <= 4 {
		return strings.Repeat("*", len(v))
	}
	return v[:2] + strings.Repeat("*", 6) + v[len(v)-2:]
}
