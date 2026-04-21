package osufile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"osu-daws-app/internal/domain"
)

func TestParseSimple(t *testing.T) {
	m, res := ParseFile("testdata/simple.osu")
	if !res.OK() {
		t.Fatalf("unexpected errors: %s", res.Error())
	}
	if m == nil {
		t.Fatal("expected non-nil map")
	}

	if got := m.General["AudioFilename"]; got != "audio.mp3" {
		t.Errorf("General.AudioFilename = %q, want audio.mp3", got)
	}
	if got := m.Metadata["Title"]; got != "Test Song" {
		t.Errorf("Metadata.Title = %q, want \"Test Song\"", got)
	}
	if got := m.Difficulty["ApproachRate"]; got != "5" {
		t.Errorf("Difficulty.ApproachRate = %q, want 5", got)
	}
	if len(m.Events) == 0 {
		t.Error("expected at least one event line preserved")
	}
	if len(m.TimingPoints) != 1 {
		t.Fatalf("expected 1 timing point, got %d", len(m.TimingPoints))
	}
	tp := m.TimingPoints[0]
	if !tp.IsRed() {
		t.Errorf("expected red timing point, got %+v", tp)
	}
	if tp.Time != 0 || tp.BeatLength != 500 || tp.Meter != 4 || tp.Volume != 70 {
		t.Errorf("unexpected timing point: %+v", tp)
	}
	if len(m.HitObjects) != 1 {
		t.Fatalf("expected 1 hit object, got %d", len(m.HitObjects))
	}
	ho := m.HitObjects[0]
	if ho.X != 256 || ho.Y != 192 || ho.Time != 0 || ho.Type != 1 {
		t.Errorf("unexpected hit object: %+v", ho)
	}
	if ho.HitSample != "0:0:0:0:" {
		t.Errorf("expected HitSample \"0:0:0:0:\", got %q", ho.HitSample)
	}

	reds := RedTimingPoints(m)
	if len(reds) != 1 {
		t.Errorf("RedTimingPoints = %d, want 1", len(reds))
	}
	greens := GreenTimingPoints(m)
	if len(greens) != 0 {
		t.Errorf("GreenTimingPoints = %d, want 0", len(greens))
	}
}

func TestParseMixedTiming(t *testing.T) {
	m, res := ParseFile("testdata/mixed_timing.osu")
	if !res.OK() {
		t.Fatalf("unexpected errors: %s", res.Error())
	}
	if len(m.TimingPoints) != 5 {
		t.Fatalf("expected 5 timing points, got %d", len(m.TimingPoints))
	}

	reds := RedTimingPoints(m)
	greens := GreenTimingPoints(m)
	if len(reds) != 2 {
		t.Errorf("RedTimingPoints = %d, want 2", len(reds))
	}
	if len(greens) != 3 {
		t.Errorf("GreenTimingPoints = %d, want 3", len(greens))
	}

	for _, tp := range reds {
		if tp.BeatLength <= 0 {
			t.Errorf("red TP has non-positive beatLength: %+v", tp)
		}
		if !tp.Uninherited {
			t.Errorf("red TP Uninherited=false: %+v", tp)
		}
	}
	for _, tp := range greens {
		if tp.BeatLength >= 0 {
			t.Errorf("green TP has non-negative beatLength: %+v", tp)
		}
		if tp.Uninherited {
			t.Errorf("green TP Uninherited=true: %+v", tp)
		}
	}

	redTimes := []int{reds[0].Time, reds[1].Time}
	if redTimes[0] != 0 || redTimes[1] != 4000 {
		t.Errorf("red timing times = %v, want [0 4000]", redTimes)
	}

	if len(m.HitObjects) != 3 {
		t.Fatalf("expected 3 hit objects, got %d", len(m.HitObjects))
	}
	slider := m.HitObjects[2]
	if slider.Type&2 == 0 {
		t.Errorf("expected slider bit set on third hitobject type=%d", slider.Type)
	}
	if slider.ObjectParams == "" {
		t.Error("expected slider ObjectParams to be populated")
	}
	if slider.HitSample != "0:0:0:0:" {
		t.Errorf("expected slider HitSample \"0:0:0:0:\", got %q", slider.HitSample)
	}
}

func TestParseMalformedTiming(t *testing.T) {
	m, res := ParseFile("testdata/malformed_timing.osu")
	if res.OK() {
		t.Fatal("expected validation errors")
	}
	if m == nil {
		t.Fatal("expected partial map despite malformed lines")
	}

	found := 0
	for _, e := range res.Errors {
		if e.Code == CodeInvalidTimingPoint {
			found++
		}
	}
	if found != 2 {
		t.Errorf("expected 2 invalid_timing_point errors, got %d (%s)", found, res.Error())
	}

	if len(m.TimingPoints) != 2 {
		t.Errorf("expected 2 parsed timing points (malformed skipped), got %d", len(m.TimingPoints))
	}
	for _, tp := range m.TimingPoints {
		if !tp.IsRed() {
			t.Errorf("expected surviving TP to be red, got %+v", tp)
		}
	}
}

func TestParseMissingHeader(t *testing.T) {
	r := strings.NewReader("[General]\nAudioFilename: a.mp3\n")
	m, res := Parse(r)
	if res.OK() {
		t.Fatal("expected invalid_osu_header error")
	}
	if m != nil {
		t.Errorf("expected nil map, got %+v", m)
	}
	if res.Errors[0].Code != CodeInvalidOsuHeader {
		t.Errorf("expected %s, got %s", CodeInvalidOsuHeader, res.Errors[0].Code)
	}
}

func TestParseEmptyInput(t *testing.T) {
	_, res := Parse(strings.NewReader(""))
	if res.OK() {
		t.Fatal("expected error on empty input")
	}
	if res.Errors[0].Code != CodeInvalidOsuHeader {
		t.Errorf("expected %s, got %s", CodeInvalidOsuHeader, res.Errors[0].Code)
	}
}

func TestParseBOMHeader(t *testing.T) {
	r := strings.NewReader("\ufeffosu file format v14\n\n[General]\nAudioFilename:a.mp3\n")
	m, res := Parse(r)
	if !res.OK() {
		t.Fatalf("unexpected errors: %s", res.Error())
	}
	if m.General["AudioFilename"] != "a.mp3" {
		t.Errorf("expected AudioFilename parsed, got %q", m.General["AudioFilename"])
	}
}

func TestParseRealMaps(t *testing.T) {
	dir := "testdata/real"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("no real map directory: %v", err)
		return
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(e.Name()), ".osu") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	if len(files) == 0 {
		t.Skip("no real .osu fixtures present in testdata/real")
		return
	}

	for _, path := range files {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			m, res := ParseFile(path)
			if m == nil {
				t.Fatalf("parse returned nil OsuMap: %s", res.Error())
			}
			for _, e := range res.Errors {
				t.Logf("validation: %s", e.Error())
			}

			reds := RedTimingPoints(m)
			if len(reds) == 0 {
				t.Errorf("expected at least one red timing point in real map %s", path)
			}
			assertNonEmpty(t, m.Metadata, "Title")
			assertNonEmpty(t, m.Metadata, "Version")
		})
	}
}

func assertNonEmpty(t *testing.T, section map[string]string, key string) {
	t.Helper()
	if v, ok := section[key]; !ok || v == "" {
		t.Errorf("expected %s to be non-empty", key)
	}
}

var _ = domain.TimingPoint{}
