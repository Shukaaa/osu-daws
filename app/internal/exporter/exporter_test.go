package exporter

import (
	"os"
	"strings"
	"testing"

	"osu-daws-app/internal/domain"
	"osu-daws-app/internal/generator"
	"osu-daws-app/internal/osufile"
	"osu-daws-app/internal/timing"
)

func buildReference() *domain.OsuMap {
	m := domain.NewOsuMap()
	m.General["AudioFilename"] = "audio.mp3"
	m.General["PreviewTime"] = "12345"
	m.General["StackLeniency"] = "0.7"
	m.General["Mode"] = "0"

	m.Editor["BeatDivisor"] = "4"

	m.Metadata["Title"] = "Test Song"
	m.Metadata["Artist"] = "Tester"
	m.Metadata["Creator"] = "Mapper"
	m.Metadata["Version"] = "Easy"
	m.Metadata["Tags"] = "osu daws test"
	m.Metadata["BeatmapID"] = "0"
	m.Metadata["BeatmapSetID"] = "-1"

	m.Difficulty["HPDrainRate"] = "5"
	m.Difficulty["CircleSize"] = "4"
	m.Difficulty["OverallDifficulty"] = "5"
	m.Difficulty["ApproachRate"] = "5"
	m.Difficulty["SliderMultiplier"] = "1.4"
	m.Difficulty["SliderTickRate"] = "1"

	m.Events = []string{
		"//Background and Video events",
		`0,0,"bg.jpg",0,0`,
		"//Break Periods",
	}
	return m
}

func TestExport_Golden(t *testing.T) {
	ref := buildReference()

	red := domain.TimingPoint{Time: 0, BeatLength: 500, Meter: 4, SampleSet: 1, SampleIndex: 0, Volume: 70, Uninherited: true}
	groups := []timing.FinalGroup{
		{
			TimeMs: 1000, Volume: 60, CustomIndex: 0,
			Samplesets: []domain.Sampleset{domain.SamplesetDrum},
			Sounds:     []domain.Sound{domain.SoundNormal},
			Events: []timing.ConvertedEvent{{
				Source: domain.SourceEvent{Sampleset: domain.SamplesetDrum, CustomIndex: 0, Sound: domain.SoundNormal, Volume: 60},
				TimeMs: 1000,
			}},
		},
		{
			TimeMs: 2000, Volume: 80, CustomIndex: 2,
			Samplesets: []domain.Sampleset{domain.SamplesetDrum},
			Sounds:     []domain.Sound{domain.SoundNormal, domain.SoundClap},
			Events: []timing.ConvertedEvent{
				{Source: domain.SourceEvent{Sampleset: domain.SamplesetDrum, CustomIndex: 2, Sound: domain.SoundNormal, Volume: 80}, TimeMs: 2000},
				{Source: domain.SourceEvent{Sampleset: domain.SamplesetDrum, CustomIndex: 2, Sound: domain.SoundClap, Volume: 80}, TimeMs: 2000},
			},
		},
	}

	tps := generator.GenerateTimingPoints(groups, []domain.TimingPoint{red})
	hos, hvr := generator.GenerateHitObjects(groups, domain.SamplesetNormal)
	if hvr != nil {
		t.Fatal(hvr.Error())
	}

	got := Export(ref, tps, hos, Options{})

	wantBytes, err := os.ReadFile("testdata/golden_simple.osu")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.TrimRight(strings.ReplaceAll(string(wantBytes), "\r\n", "\n"), "\n")
	gotTrim := strings.TrimRight(got, "\n")

	if gotTrim != want {
		t.Errorf("export mismatch\n=== got ===\n%s\n=== want ===\n%s", gotTrim, want)
	}
}

func TestExport_RoundTripParse(t *testing.T) {
	ref := buildReference()
	red := domain.TimingPoint{Time: 0, BeatLength: 500, Meter: 4, SampleSet: 1, SampleIndex: 0, Volume: 70, Uninherited: true}
	groups := []timing.FinalGroup{
		{
			TimeMs: 1000, Volume: 60, CustomIndex: 0,
			Samplesets: []domain.Sampleset{domain.SamplesetDrum},
			Sounds:     []domain.Sound{domain.SoundNormal},
			Events: []timing.ConvertedEvent{{
				Source: domain.SourceEvent{Sampleset: domain.SamplesetDrum, CustomIndex: 0, Sound: domain.SoundNormal, Volume: 60},
				TimeMs: 1000,
			}},
		},
	}
	tps := generator.GenerateTimingPoints(groups, []domain.TimingPoint{red})
	hos, hvr := generator.GenerateHitObjects(groups, domain.SamplesetNormal)
	if hvr != nil {
		t.Fatal(hvr.Error())
	}
	out := Export(ref, tps, hos, Options{})

	parsed, res := osufile.Parse(strings.NewReader(out))
	if !res.OK() {
		t.Fatalf("exported file failed to parse: %s", res.Error())
	}
	if parsed.Metadata["Title"] != "Test Song" {
		t.Errorf("Title lost: %q", parsed.Metadata["Title"])
	}
	if parsed.Metadata["Version"] != DefaultDifficultyName {
		t.Errorf("Version = %q, want %q", parsed.Metadata["Version"], DefaultDifficultyName)
	}
	if len(parsed.TimingPoints) != 2 {
		t.Errorf("expected 2 TPs, got %d", len(parsed.TimingPoints))
	}
	if len(parsed.HitObjects) != 1 {
		t.Errorf("expected 1 hitobject, got %d", len(parsed.HitObjects))
	}
}

func TestExport_IgnoresReferenceGreensAndHitObjects(t *testing.T) {
	ref, res := osufile.ParseFile("../osufile/testdata/mixed_timing.osu")
	if !res.OK() {
		t.Fatalf("fixture parse errors: %s", res.Error())
	}

	var reds []domain.TimingPoint
	for _, tp := range ref.TimingPoints {
		if tp.IsRed() {
			reds = append(reds, tp)
		}
	}
	if len(reds) == 0 {
		t.Fatal("fixture has no reds")
	}
	if len(ref.HitObjects) == 0 {
		t.Fatal("fixture has no hitobjects")
	}

	groups := []timing.FinalGroup{
		{
			TimeMs: 500, Volume: 50, CustomIndex: 0,
			Samplesets: []domain.Sampleset{domain.SamplesetSoft},
			Sounds:     []domain.Sound{domain.SoundClap},
			Events: []timing.ConvertedEvent{{
				Source: domain.SourceEvent{Sampleset: domain.SamplesetSoft, CustomIndex: 0, Sound: domain.SoundClap, Volume: 50},
				TimeMs: 500,
			}},
		},
	}
	tps := generator.GenerateTimingPoints(groups, reds)
	hos, hvr := generator.GenerateHitObjects(groups, domain.SamplesetNormal)
	if hvr != nil {
		t.Fatal(hvr.Error())
	}

	out := Export(ref, tps, hos, Options{})

	if strings.Contains(out, "1000,-100,4,1,0,60,0,0") {
		t.Error("original green TP 1000,-100 leaked into export")
	}
	if strings.Contains(out, "2000,-80,4,1,0,55,0,0") {
		t.Error("original green TP 2000,-80 leaked into export")
	}
	if strings.Contains(out, "6000,-125,3,2,0,75,0,0") {
		t.Error("original green TP 6000,-125 leaked into export")
	}

	if strings.Contains(out, "100,100,1500,2,0,B|200:200|300:100") {
		t.Error("original slider hitobject leaked into export")
	}
	if strings.Contains(out, "200,100,500,1,2") {
		t.Error("original hit-circle at 500ms leaked into export")
	}

	parsed, res := osufile.Parse(strings.NewReader(out))
	if !res.OK() {
		t.Fatalf("re-parse failed: %s", res.Error())
	}
	if len(parsed.HitObjects) != 1 {
		t.Errorf("expected exactly 1 generated hitobject, got %d", len(parsed.HitObjects))
	}
	if parsed.HitObjects[0].Time != 500 {
		t.Errorf("generated hitobject time = %d, want 500", parsed.HitObjects[0].Time)
	}
	redCount := 0
	greenCount := 0
	for _, tp := range parsed.TimingPoints {
		if tp.IsRed() {
			redCount++
		} else {
			greenCount++
		}
	}
	if redCount != len(reds) {
		t.Errorf("red count = %d, want %d (originals preserved)", redCount, len(reds))
	}
	if greenCount != 1 {
		t.Errorf("green count = %d, want 1 (only generated)", greenCount)
	}
}

func TestExport_DefaultDifficultyName(t *testing.T) {
	ref := buildReference()
	out := Export(ref, nil, nil, Options{})
	if !strings.Contains(out, "Version:"+DefaultDifficultyName+"\n") {
		t.Errorf("expected Version:%s in output, got:\n%s", DefaultDifficultyName, out)
	}
	if strings.Contains(out, "Version:Easy\n") {
		t.Error("reference Version leaked through")
	}
}

func TestExport_DifficultyNameOverride(t *testing.T) {
	ref := buildReference()
	out := Export(ref, nil, nil, Options{DifficultyName: "Hitsounds"})
	if !strings.Contains(out, "Version:Hitsounds\n") {
		t.Errorf("expected Version:Hitsounds in output, got:\n%s", out)
	}
}

func TestExport_NilReferenceErrors(t *testing.T) {
	if err := ExportTo(nil, nil, nil, nil, Options{}); err == nil {
		t.Error("expected error on nil reference")
	}
}
