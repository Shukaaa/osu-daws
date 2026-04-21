package ui

import (
	"strings"

	"osu-daws-app/internal/osufile"
	"osu-daws-app/internal/workspace"
)

// AutoProjectName tracks whether the name field should still be
// auto-filled from a reference .osu file. Once the user types a value
// that differs from the last auto-suggestion, further suggestions are
// suppressed.
type AutoProjectName struct {
	current  string
	lastAuto string
	edited   bool
}

func (a *AutoProjectName) Current() string { return a.current }

func (a *AutoProjectName) Edited() bool { return a.edited }

// UserTyped records a change emitted by the entry widget.
func (a *AutoProjectName) UserTyped(text string) {
	a.current = text
	if text != a.lastAuto {
		a.edited = true
	}
}

// Suggest returns (newText, true) when `suggestion` should be pushed
// into the entry widget, or ("", false) when the user has taken over
// the field or the suggestion is not usable.
func (a *AutoProjectName) Suggest(suggestion string) (string, bool) {
	if a.edited {
		return "", false
	}
	suggestion = strings.TrimSpace(suggestion)
	if suggestion == "" || suggestion == a.current {
		return "", false
	}
	a.lastAuto = suggestion
	a.current = suggestion
	return suggestion, true
}

// suggestNameFromReference parses the .osu file at path and returns a
// best-effort project name. Returns "" when no usable name can be derived.
func suggestNameFromReference(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	m, _ := osufile.ParseFile(path)
	if m == nil {
		return ""
	}
	return workspace.SuggestProjectName(m)
}
