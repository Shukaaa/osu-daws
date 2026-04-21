package workspace

import (
	"strings"
	"unicode"

	"osu-daws-app/internal/domain"
)

// DefaultProjectName is returned when the map lacks enough metadata.
const DefaultProjectName = "New Workspace"

// maxSuggestedNameLen caps how long "Artist - Title [Version]" can get
// in runes before the difficulty is dropped.
const maxSuggestedNameLen = 80

// SuggestProjectName builds a readable default workspace name from the
// [Metadata] section of a parsed .osu map. Falls back to
// DefaultProjectName when nothing usable is available.
func SuggestProjectName(m *domain.OsuMap) string {
	if m == nil {
		return DefaultProjectName
	}

	artist := pickMetadata(m.Metadata, "Artist", "ArtistUnicode")
	title := pickMetadata(m.Metadata, "Title", "TitleUnicode")
	version := pickMetadata(m.Metadata, "Version")

	switch {
	case artist != "" && title != "":
		base := artist + " - " + title
		if version != "" {
			full := base + " [" + version + "]"
			if runeLen(full) <= maxSuggestedNameLen {
				return full
			}
		}
		return base

	case title != "":
		if version != "" {
			candidate := title + " [" + version + "]"
			if runeLen(candidate) <= maxSuggestedNameLen {
				return candidate
			}
		}
		return title

	case artist != "":
		return artist
	}

	return DefaultProjectName
}

// pickMetadata returns the first non-empty cleaned value for the given
// keys. Prefers the ASCII key over the unicode variant.
func pickMetadata(meta map[string]string, keys ...string) string {
	if meta == nil {
		return ""
	}
	for _, k := range keys {
		if raw, ok := meta[k]; ok {
			if v := cleanMetaValue(raw); v != "" {
				return v
			}
		}
	}
	return ""
}

// cleanMetaValue trims surrounding whitespace, drops control runes and
// collapses internal whitespace runs to a single space.
func cleanMetaValue(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		switch {
		case unicode.IsSpace(r):
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
		case unicode.IsControl(r):
		default:
			b.WriteRune(r)
			prevSpace = false
		}
	}
	return strings.TrimSpace(b.String())
}

func runeLen(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}
