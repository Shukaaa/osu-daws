package exporter

import (
	"fmt"
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
	return OsuFilenameVersioned(artist, title, creator, diffName, 0)
}

// OsuFilenameVersioned behaves like OsuFilename but appends " v<N>" to
func OsuFilenameVersioned(artist, title, creator, diffName string, version int) string {
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
	if version > 0 {
		diffName = fmt.Sprintf("%s v%d", diffName, version)
	}

	return artist + " - " + title + " (" + creator + ") [" + diffName + "].osu"
}

// DefaultExportPath returns the absolute path where a generated
// hitsound diff should be written by default.
func DefaultExportPath(exportsDir string, ref *domain.OsuMap) string {
	return DefaultExportPathVersioned(exportsDir, ref, 0)
}

// DefaultExportPathVersioned builds the canonical export path and
// appends " v<N>" to the diff name when version > 0.
func DefaultExportPathVersioned(exportsDir string, ref *domain.OsuMap, version int) string {
	var artist, title, creator string
	if ref != nil {
		artist = ref.Metadata["Artist"]
		title = ref.Metadata["Title"]
		creator = ref.Metadata["Creator"]
	}
	return filepath.Join(exportsDir,
		OsuFilenameVersioned(artist, title, creator, DefaultDifficultyName, version))
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
