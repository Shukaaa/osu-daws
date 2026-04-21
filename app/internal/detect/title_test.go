package detect

import "testing"

func TestParseWindowTitle(t *testing.T) {
	cases := []struct {
		name  string
		title string
		want  *BeatmapInfo
	}{
		{
			name:  "main menu",
			title: "osu!",
			want:  nil,
		},
		{
			name:  "empty",
			title: "",
			want:  nil,
		},
		{
			name:  "not osu",
			title: "Notepad",
			want:  nil,
		},
		{
			name:  "simple beatmap",
			title: "osu!  - Camellia - GHOST [Extra]",
			want:  &BeatmapInfo{Artist: "Camellia", Title: "GHOST", Version: "Extra"},
		},
		{
			name:  "single space after osu!",
			title: "osu! - Camellia - GHOST [Extra]",
			want:  &BeatmapInfo{Artist: "Camellia", Title: "GHOST", Version: "Extra"},
		},
		{
			name:  "dash in title",
			title: "osu!  - YOASOBI - Idol - Oshi no Ko [Insane]",
			want:  &BeatmapInfo{Artist: "YOASOBI", Title: "Idol - Oshi no Ko", Version: "Insane"},
		},
		{
			name:  "parentheses in version",
			title: "osu!  - Camellia - GHOST [Lasse's Extra]",
			want:  &BeatmapInfo{Artist: "Camellia", Title: "GHOST", Version: "Lasse's Extra"},
		},
		{
			name:  "featured artist prefix",
			title: "osu!  - bladee - shadowface (feat. Bones) [hard]",
			want:  &BeatmapInfo{Artist: "bladee", Title: "shadowface (feat. Bones)", Version: "hard"},
		},
		{
			name:  "unicode-ish",
			title: "osu!  - ヨアソビ - アイドル [Insane]",
			want:  &BeatmapInfo{Artist: "ヨアソビ", Title: "アイドル", Version: "Insane"},
		},
		{
			name:  "no version bracket",
			title: "osu!  - Camellia - GHOST",
			want:  nil,
		},
		{
			name:  "no artist-title separator",
			title: "osu!  - GHOST [Extra]",
			want:  nil,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ParseWindowTitle(c.title)
			if c.want == nil {
				if got != nil {
					t.Fatalf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil result")
			}
			if got.Artist != c.want.Artist {
				t.Errorf("Artist = %q, want %q", got.Artist, c.want.Artist)
			}
			if got.Title != c.want.Title {
				t.Errorf("Title = %q, want %q", got.Title, c.want.Title)
			}
			if got.Version != c.want.Version {
				t.Errorf("Version = %q, want %q", got.Version, c.want.Version)
			}
		})
	}
}
