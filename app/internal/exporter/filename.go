package exporter

import (
	"path/filepath"
	"strings"

	"osu-daws-app/internal/domain"
)

const invalidChars = `<>:"/\|?*` + "\x00"

// OsuFilename builds an osu!-style .osu filename from map metadata:
//
//	<Artist> - <Title> (<Creator>) [<DifficultyName>].osu
//
// All fields are sanitised for filesystem safety. Missing fields are
// replaced by sensible fallbacks so the result is always a usable name.
func OsuFilename(artist, title, creator, diffName string) string {
	artist = sanitise(artist)
	title = sanitise(title)
	creator = sanitise(creator)
	diffName = sanitise(diffName)

	if artist == "" {
		artist = "Unknown Artist"
	}
	if title == "" {
		title = "Unknown Title"
	}
	if creator == "" {
		creator = "Unknown"
	}
	if diffName == "" {
		diffName = DefaultDifficultyName
	}

	return artist + " - " + title + " (" + creator + ") [" + diffName + "].osu"
}

// DefaultExportPath returns the absolute path where a generated
// hitsound diff should be written by default:
//
//	<exportsDir>/<Artist> - <Title> (<Creator>) [<DefaultDifficultyName>].osu
//
// Fields come from ref.Metadata; any missing field falls back to the
// same placeholders OsuFilename uses, so the returned path is always
// a usable filename.
//
// ref may be nil — in that case the full placeholder filename is used.
// exportsDir must be set by the caller; the helper performs no I/O.
func DefaultExportPath(exportsDir string, ref *domain.OsuMap) string {
	var artist, title, creator string
	if ref != nil {
		artist = ref.Metadata["Artist"]
		title = ref.Metadata["Title"]
		creator = ref.Metadata["Creator"]
	}
	return filepath.Join(exportsDir,
		OsuFilename(artist, title, creator, DefaultDifficultyName))
}

func sanitise(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		if strings.ContainsRune(invalidChars, r) || r < 0x20 {
			continue
		}
		if r == ' ' || r == '\t' {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	return strings.TrimRight(strings.TrimSpace(b.String()), ". ")
}
