package workspace

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"unicode"
)

// Slug converts an arbitrary project name into a filesystem-safe,
// lowercase slug: ASCII [a-z0-9] is kept, everything else collapses to
// '-', consecutive dashes collapse to one, and the empty slug falls back
// to "project". Non-ASCII letters are dropped — the random suffix on the
// ID preserves uniqueness.
func Slug(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	prevDash := false
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case r >= 'A' && r <= 'Z':
			b.WriteRune(unicode.ToLower(r))
			prevDash = false
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	s := strings.Trim(b.String(), "-")
	if s == "" {
		return "project"
	}
	return s
}

const randomSuffixLen = 6

// NewID builds a workspace ID by combining the slug of name with a short
// crypto/rand-based suffix, e.g. "my-song-a1b2c3".
func NewID(name string) ID {
	return ID(Slug(name) + "-" + randomSuffix(randomSuffixLen))
}

// randomSuffix returns a lowercase hex string of the given length.
// crypto/rand failure is treated as fatal.
func randomSuffix(n int) string {
	nBytes := (n + 1) / 2
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		panic("workspace: crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(buf)[:n]
}
