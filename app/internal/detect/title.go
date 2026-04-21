package detect

import "strings"

type BeatmapInfo struct {
	Artist  string
	Title   string
	Version string // difficulty name
}

func ParseWindowTitle(title string) *BeatmapInfo {
	title = strings.TrimSpace(title)

	const prefix = "osu!"
	if !strings.HasPrefix(title, prefix) {
		return nil
	}
	rest := strings.TrimLeft(title[len(prefix):], " ")

	if !strings.HasPrefix(rest, "- ") {
		return nil // no beatmap info (main menu)
	}
	rest = rest[2:] // drop "- "

	closeBracket := strings.LastIndex(rest, "]")
	if closeBracket < 0 {
		return nil
	}
	openBracket := -1
	for i := closeBracket - 1; i >= 0; i-- {
		if rest[i] == '[' && (i == 0 || rest[i-1] == ' ') {
			openBracket = i
			break
		}
	}
	if openBracket < 0 {
		return nil
	}
	version := rest[openBracket+1 : closeBracket]

	artistTitle := strings.TrimSpace(rest[:openBracket])

	sepIdx := strings.Index(artistTitle, " - ")
	if sepIdx < 0 {
		return nil
	}
	artist := strings.TrimSpace(artistTitle[:sepIdx])
	songTitle := strings.TrimSpace(artistTitle[sepIdx+3:])

	if artist == "" || songTitle == "" || version == "" {
		return nil
	}

	return &BeatmapInfo{
		Artist:  artist,
		Title:   songTitle,
		Version: version,
	}
}
