package workspace

import (
	"strings"
	"testing"

	"osu-daws-app/internal/domain"
)

func mapWith(md map[string]string) *domain.OsuMap {
	m := domain.NewOsuMap()
	for k, v := range md {
		m.Metadata[k] = v
	}
	return m
}

func TestSuggestProjectName_ArtistAndTitle(t *testing.T) {
	got := SuggestProjectName(mapWith(map[string]string{
		"Artist": "bladee",
		"Title":  "shadowface",
	}))
	if got != "bladee - shadowface" {
		t.Errorf("got %q", got)
	}
}

func TestSuggestProjectName_ArtistTitleDifficulty(t *testing.T) {
	got := SuggestProjectName(mapWith(map[string]string{
		"Artist":  "bladee",
		"Title":   "shadowface",
		"Version": "rainworld of angels",
	}))
	want := "bladee - shadowface [rainworld of angels]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSuggestProjectName_MissingArtist(t *testing.T) {
	got := SuggestProjectName(mapWith(map[string]string{
		"Title":   "Easter Pink",
		"Version": "Extra",
	}))
	want := "Easter Pink [Extra]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSuggestProjectName_MissingArtist_NoVersion(t *testing.T) {
	got := SuggestProjectName(mapWith(map[string]string{
		"Title": "Easter Pink",
	}))
	if got != "Easter Pink" {
		t.Errorf("got %q", got)
	}
}

func TestSuggestProjectName_MissingTitle(t *testing.T) {
	got := SuggestProjectName(mapWith(map[string]string{
		"Artist": "fakemink",
	}))
	if got != "fakemink" {
		t.Errorf("got %q", got)
	}
}

func TestSuggestProjectName_MissingBoth(t *testing.T) {
	got := SuggestProjectName(mapWith(map[string]string{
		"Version": "Insane",
	}))
	if got != DefaultProjectName {
		t.Errorf("got %q, want %q", got, DefaultProjectName)
	}
}

func TestSuggestProjectName_NilMap(t *testing.T) {
	if got := SuggestProjectName(nil); got != DefaultProjectName {
		t.Errorf("got %q, want %q", got, DefaultProjectName)
	}
}

func TestSuggestProjectName_NilMetadataSection(t *testing.T) {
	m := &domain.OsuMap{} // Metadata is nil
	if got := SuggestProjectName(m); got != DefaultProjectName {
		t.Errorf("got %q, want %q", got, DefaultProjectName)
	}
}

func TestSuggestProjectName_MalformedWhitespaceAndControl(t *testing.T) {
	got := SuggestProjectName(mapWith(map[string]string{
		"Artist": "  bla\tdee \x00 ",
		"Title":  "\x01 shadow\nface  ",
	}))
	if got != "bla dee - shadow face" {
		t.Errorf("got %q", got)
	}
}

func TestSuggestProjectName_WhitespaceOnlyTreatedAsMissing(t *testing.T) {
	got := SuggestProjectName(mapWith(map[string]string{
		"Artist":  "   ",
		"Title":   "\t\t",
		"Version": " Hard ",
	}))
	if got != DefaultProjectName {
		t.Errorf("got %q, want %q", got, DefaultProjectName)
	}
}

func TestSuggestProjectName_UnicodeFallbacks(t *testing.T) {
	// ASCII key empty → fall back to unicode variant.
	got := SuggestProjectName(mapWith(map[string]string{
		"Artist":        "",
		"ArtistUnicode": "ブリーチ",
		"Title":         "",
		"TitleUnicode":  "青",
	}))
	if got != "ブリーチ - 青" {
		t.Errorf("got %q", got)
	}
}

func TestSuggestProjectName_DropsVersionWhenTooLong(t *testing.T) {
	// Construct a name whose "[Version]" form would exceed the cap.
	longTitle := strings.Repeat("a", 60)
	longVer := strings.Repeat("v", 40)
	got := SuggestProjectName(mapWith(map[string]string{
		"Artist":  "x",
		"Title":   longTitle,
		"Version": longVer,
	}))
	// Should return "x - <longTitle>" without the [version] suffix.
	want := "x - " + longTitle
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
