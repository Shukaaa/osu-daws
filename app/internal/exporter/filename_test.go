package exporter

import (
	"path/filepath"
	"strings"
	"testing"

	"osu-daws-app/internal/domain"
)

func TestOsuFilename(t *testing.T) {
	cases := []struct {
		name    string
		artist  string
		title   string
		creator string
		diff    string
		want    string
	}{
		{
			name:    "normal metadata",
			artist:  "Hanasaka Yui(CV: M.A.O)",
			title:   "Harumachi Clover",
			creator: "Adachi-Sakura",
			diff:    "osu!daw's HS",
			want:    "Hanasaka Yui(CV M.A.O) - Harumachi Clover (Adachi-Sakura) [osu!daw's HS].osu",
		},
		{
			name:    "characters needing sanitisation",
			artist:  `Art<ist>`,
			title:   `Ti:tle/with"pipes|`,
			creator: `Map?per*`,
			diff:    "Hard",
			want:    "Artist - Titlewithpipes (Mapper) [Hard].osu",
		},
		{
			name:    "missing artist",
			artist:  "",
			title:   "Song",
			creator: "Me",
			diff:    "Easy",
			want:    "Unknown Artist - Song (Me) [Easy].osu",
		},
		{
			name:    "missing title",
			artist:  "Band",
			title:   "",
			creator: "Me",
			diff:    "Normal",
			want:    "Band - Unknown Title (Me) [Normal].osu",
		},
		{
			name:    "missing everything",
			artist:  "",
			title:   "",
			creator: "",
			diff:    "",
			want:    "Unknown Artist - Unknown Title (Unknown) [osu!daw's HS].osu",
		},
		{
			name:    "whitespace-only fields",
			artist:  "   ",
			title:   "\t\t",
			creator: " ",
			diff:    "  ",
			want:    "Unknown Artist - Unknown Title (Unknown) [osu!daw's HS].osu",
		},
		{
			name:    "trailing dots and spaces",
			artist:  "Artist...",
			title:   "Title . .",
			creator: "Mapper.",
			diff:    "Diff",
			want:    "Artist - Title (Mapper) [Diff].osu",
		},
		{
			name:    "unicode preserved",
			artist:  "ヨアソビ",
			title:   "アイドル",
			creator: "Mapper",
			diff:    "Insane",
			want:    "ヨアソビ - アイドル (Mapper) [Insane].osu",
		},
		{
			name:    "difficulty name with special chars",
			artist:  "A",
			title:   "B",
			creator: "C",
			diff:    `My "Special" Diff`,
			want:    "A - B (C) [My Special Diff].osu",
		},
		{
			name:    "extension always .osu",
			artist:  "A",
			title:   "B",
			creator: "C",
			diff:    "D",
			want:    "A - B (C) [D].osu",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := OsuFilename(c.artist, c.title, c.creator, c.diff)
			if got != c.want {
				t.Errorf("\ngot:  %q\nwant: %q", got, c.want)
			}
		})
	}
}

func TestSanitise(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{`hello`, `hello`},
		{`a<b>c`, `abc`},
		{`a:b/c\d`, `abcd`},
		{`"quotes"`, `quotes`},
		{`a  b   c`, `a b c`},
		{`trailing...`, `trailing`},
		{`  spaces  `, `spaces`},
		{"\x00hidden\x01", "hidden"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got := sanitise(c.in)
			if got != c.want {
				t.Errorf("sanitise(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestDefaultExportPath_FullMetadata(t *testing.T) {
	ref := &domain.OsuMap{Metadata: map[string]string{
		"Artist":  "Camellia",
		"Title":   "GHOST",
		"Creator": "Mapper",
	}}
	got := DefaultExportPath(filepath.Join("ws", "exports"), ref)
	want := filepath.Join("ws", "exports",
		"Camellia - GHOST (Mapper) ["+DefaultDifficultyName+"].osu")
	if got != want {
		t.Errorf("\ngot:  %q\nwant: %q", got, want)
	}
}

func TestDefaultExportPath_NilRefUsesPlaceholders(t *testing.T) {
	got := DefaultExportPath("/tmp/exports", nil)
	want := filepath.Join("/tmp/exports",
		"Unknown Artist - Unknown Title (Unknown) ["+DefaultDifficultyName+"].osu")
	if got != want {
		t.Errorf("\ngot:  %q\nwant: %q", got, want)
	}
}

func TestDefaultExportPath_SanitisesMetadata(t *testing.T) {
	ref := &domain.OsuMap{Metadata: map[string]string{
		"Artist":  `A<rt>ist`,
		"Title":   `Ti:tle`,
		"Creator": `Map?per*`,
	}}
	got := DefaultExportPath("out", ref)
	// The filename must contain only sanitised values — the original
	// characters must not survive.
	for _, bad := range []string{"<", ">", ":", "?", "*"} {
		if strings.Contains(filepath.Base(got), bad) {
			t.Errorf("sanitised name still contains %q: %s", bad, got)
		}
	}
	if !strings.HasSuffix(got, ".osu") {
		t.Errorf("got %q, want .osu suffix", got)
	}
}
