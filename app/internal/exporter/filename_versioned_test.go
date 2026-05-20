package exporter

import (
	"path/filepath"
	"strings"
	"testing"

	"osu-daws-app/internal/domain"
)

func TestOsuFilenameVersioned(t *testing.T) {
	cases := []struct {
		name    string
		version int
		wantHas string
	}{
		{"no version", 0, "[" + DefaultDifficultyName + "]"},
		{"v1", 1, "[" + DefaultDifficultyName + " v1]"},
		{"v42", 42, "[" + DefaultDifficultyName + " v42]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := OsuFilenameVersioned("A", "T", "C", DefaultDifficultyName, tc.version)
			if !strings.Contains(got, tc.wantHas) {
				t.Errorf("\ngot:  %q\nwant contains: %q", got, tc.wantHas)
			}
			if !strings.HasSuffix(got, ".osu") {
				t.Errorf("missing .osu extension: %s", got)
			}
		})
	}
}

func TestDefaultExportPathVersioned(t *testing.T) {
	ref := &domain.OsuMap{Metadata: map[string]string{
		"Artist": "Camellia", "Title": "GHOST", "Creator": "Mapper",
	}}
	got := DefaultExportPathVersioned(filepath.Join("ws", "exports"), ref, 3)
	want := filepath.Join("ws", "exports",
		"Camellia - GHOST (Mapper) ["+DefaultDifficultyName+" v3].osu")
	if got != want {
		t.Errorf("\ngot:  %q\nwant: %q", got, want)
	}
}

func TestOptions_DiffNameWithVersion(t *testing.T) {
	cases := []struct {
		opts Options
		want string
	}{
		{Options{}, DefaultDifficultyName},
		{Options{Version: 0}, DefaultDifficultyName},
		{Options{Version: 1}, DefaultDifficultyName + " v1"},
		{Options{DifficultyName: "Custom", Version: 2}, "Custom v2"},
		{Options{DifficultyName: "Custom"}, "Custom"},
	}
	for _, tc := range cases {
		if got := tc.opts.diffName(); got != tc.want {
			t.Errorf("%+v: got %q, want %q", tc.opts, got, tc.want)
		}
	}
}
