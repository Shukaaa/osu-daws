package generator

import (
	"reflect"
	"testing"

	"osu-daws-app/internal/domain"
	"osu-daws-app/internal/timing"
)

func fg(ms, vol, ci int, sets []domain.Sampleset) timing.FinalGroup {
	return timing.FinalGroup{
		TimeMs:      ms,
		Volume:      vol,
		CustomIndex: ci,
		Samplesets:  sets,
		Sounds:      []domain.Sound{domain.SoundNormal},
	}
}

func redTP(time int, bl float64) domain.TimingPoint {
	return domain.TimingPoint{
		Time: time, BeatLength: bl, Meter: 4,
		SampleSet: 1, SampleIndex: 0, Volume: 100,
		Uninherited: true, Effects: 0,
	}
}

func greenOnly(tps []domain.TimingPoint) []domain.TimingPoint {
	out := []domain.TimingPoint{}
	for _, tp := range tps {
		if tp.IsGreen() {
			out = append(out, tp)
		}
	}
	return out
}

func redsOnly(tps []domain.TimingPoint) []domain.TimingPoint {
	out := []domain.TimingPoint{}
	for _, tp := range tps {
		if tp.IsRed() {
			out = append(out, tp)
		}
	}
	return out
}

func TestGenerateTimingPoints_FirstStateBeforeAnyRed(t *testing.T) {
	groups := []timing.FinalGroup{
		fg(1000, 60, 2, []domain.Sampleset{domain.SamplesetDrum}),
	}
	reds := []domain.TimingPoint{redTP(2000, 500)}

	out := GenerateTimingPoints(groups, reds)
	if len(out) != 2 {
		t.Fatalf("got %d TPs, want 2", len(out))
	}
	if !out[0].IsGreen() || out[0].Time != 1000 {
		t.Errorf("expected green at 1000, got %+v", out[0])
	}
	if out[0].SampleSet != 0 {
		t.Errorf("green SampleSet = %d, want 0 (inherit)", out[0].SampleSet)
	}
	if out[0].SampleIndex != 2 || out[0].Volume != 60 {
		t.Errorf("green state = %+v", out[0])
	}
	if !out[1].IsRed() {
		t.Errorf("expected red second, got %+v", out[1])
	}
	if out[1].SampleSet != 1 {
		t.Errorf("red SampleSet changed to %d, want 1 (preserved)", out[1].SampleSet)
	}
	if out[1].SampleIndex != 2 || out[1].Volume != 60 {
		t.Errorf("red should adopt index/volume, got %+v", out[1])
	}
}

func TestGenerateTimingPoints_UnchangedStateEmitsNothing(t *testing.T) {
	groups := []timing.FinalGroup{
		fg(1000, 60, 2, []domain.Sampleset{domain.SamplesetDrum}),
		fg(1500, 60, 2, []domain.Sampleset{domain.SamplesetDrum}),
		fg(2000, 60, 2, []domain.Sampleset{domain.SamplesetDrum}),
	}
	reds := []domain.TimingPoint{redTP(0, 500)}

	out := GenerateTimingPoints(groups, reds)
	greens := greenOnly(out)
	if len(greens) != 1 {
		t.Fatalf("expected exactly 1 green line for 3 unchanged-state groups, got %d: %+v", len(greens), greens)
	}
	if greens[0].Time != 1000 {
		t.Errorf("green at %d, want 1000", greens[0].Time)
	}
}

func TestGenerateTimingPoints_SamplesetSwitchDoesNotEmitTP(t *testing.T) {
	// drum -> soft -> drum with identical volume + custom index must NOT
	// produce extra greens. Sampleset selection lives on the hit object.
	groups := []timing.FinalGroup{
		fg(1000, 60, 0, []domain.Sampleset{domain.SamplesetDrum}),
		fg(1500, 60, 0, []domain.Sampleset{domain.SamplesetSoft}),
		fg(2000, 60, 0, []domain.Sampleset{domain.SamplesetDrum}),
		fg(2500, 60, 0, []domain.Sampleset{domain.SamplesetNormal}),
	}
	reds := []domain.TimingPoint{redTP(0, 500)}

	out := GenerateTimingPoints(groups, reds)
	greens := greenOnly(out)
	if len(greens) != 1 {
		t.Fatalf("expected 1 green (only the first state), got %d: %+v", len(greens), greens)
	}
	if greens[0].Time != 1000 || greens[0].Volume != 60 || greens[0].SampleIndex != 0 {
		t.Errorf("unexpected green: %+v", greens[0])
	}
}

func TestGenerateTimingPoints_TwoSamplesetOverlapStillNoExtraTP(t *testing.T) {
	// An overlap that resolves to drum+soft carries its sampleset info in
	// the hit object; it must not cause a TP switch on its own.
	groups := []timing.FinalGroup{
		fg(1000, 60, 0, []domain.Sampleset{domain.SamplesetDrum}),
		fg(1500, 60, 0, []domain.Sampleset{domain.SamplesetDrum, domain.SamplesetSoft}),
		fg(2000, 60, 0, []domain.Sampleset{domain.SamplesetSoft}),
	}
	reds := []domain.TimingPoint{redTP(0, 500)}

	out := GenerateTimingPoints(groups, reds)
	greens := greenOnly(out)
	if len(greens) != 1 {
		t.Fatalf("expected 1 green, got %d: %+v", len(greens), greens)
	}
}

func TestGenerateTimingPoints_VolumeChange(t *testing.T) {
	groups := []timing.FinalGroup{
		fg(1000, 50, 0, []domain.Sampleset{domain.SamplesetDrum}),
		fg(1500, 80, 0, []domain.Sampleset{domain.SamplesetDrum}),
	}
	reds := []domain.TimingPoint{redTP(0, 500)}

	out := GenerateTimingPoints(groups, reds)
	greens := greenOnly(out)
	if len(greens) != 2 {
		t.Fatalf("expected 2 greens on volume change, got %d", len(greens))
	}
	if greens[0].Volume != 50 || greens[1].Volume != 80 {
		t.Errorf("green volumes = [%d,%d], want [50,80]", greens[0].Volume, greens[1].Volume)
	}
}

func TestGenerateTimingPoints_CustomIndexChange(t *testing.T) {
	groups := []timing.FinalGroup{
		fg(1000, 60, 1, []domain.Sampleset{domain.SamplesetSoft}),
		fg(1500, 60, 7, []domain.Sampleset{domain.SamplesetSoft}),
	}
	reds := []domain.TimingPoint{redTP(0, 500)}

	out := GenerateTimingPoints(groups, reds)
	greens := greenOnly(out)
	if len(greens) != 2 {
		t.Fatalf("expected 2 greens on custom index change, got %d", len(greens))
	}
	if greens[0].SampleIndex != 1 || greens[1].SampleIndex != 7 {
		t.Errorf("sample indices = [%d,%d], want [1,7]",
			greens[0].SampleIndex, greens[1].SampleIndex)
	}
}

func TestGenerateTimingPoints_CustomIndexChangeAcrossSamplesetSwitch(t *testing.T) {
	// Sampleset change alone: no TP. Index change: TP. Combined: exactly one TP.
	groups := []timing.FinalGroup{
		fg(1000, 60, 0, []domain.Sampleset{domain.SamplesetDrum}),
		fg(1500, 60, 3, []domain.Sampleset{domain.SamplesetSoft}),
	}
	reds := []domain.TimingPoint{redTP(0, 500)}

	out := GenerateTimingPoints(groups, reds)
	greens := greenOnly(out)
	if len(greens) != 2 {
		t.Fatalf("expected 2 greens (first state + index change), got %d: %+v", len(greens), greens)
	}
	if greens[1].SampleIndex != 3 {
		t.Errorf("second green SampleIndex = %d, want 3", greens[1].SampleIndex)
	}
}

func TestGenerateTimingPoints_RedOnSameTimestampUpdatedInPlace(t *testing.T) {
	groups := []timing.FinalGroup{
		fg(1000, 65, 3, []domain.Sampleset{domain.SamplesetDrum}),
	}
	reds := []domain.TimingPoint{redTP(1000, 500)}

	out := GenerateTimingPoints(groups, reds)
	if len(out) != 1 {
		t.Fatalf("expected a single merged TP, got %d: %+v", len(out), out)
	}
	tp := out[0]
	if !tp.IsRed() {
		t.Errorf("expected red preserved, got %+v", tp)
	}
	if tp.BeatLength != 500 {
		t.Errorf("red BeatLength lost: %g", tp.BeatLength)
	}
	if tp.SampleSet != 1 {
		t.Errorf("red SampleSet changed to %d, want 1 (preserved)", tp.SampleSet)
	}
	if tp.SampleIndex != 3 || tp.Volume != 65 {
		t.Errorf("red index/volume not updated: %+v", tp)
	}
	if len(greenOnly(out)) != 0 {
		t.Error("no green should have been inserted at same ms as a red")
	}
}

func TestGenerateTimingPoints_RedInBetweenKeepsEffectiveState(t *testing.T) {
	groups := []timing.FinalGroup{
		fg(1000, 60, 2, []domain.Sampleset{domain.SamplesetDrum}),
		fg(3000, 60, 2, []domain.Sampleset{domain.SamplesetDrum}),
	}
	reds := []domain.TimingPoint{
		redTP(0, 500),
		redTP(2000, 400),
	}

	out := GenerateTimingPoints(groups, reds)
	greens := greenOnly(out)
	if len(greens) != 1 {
		t.Fatalf("expected 1 green (state stable across middle red), got %d: %+v", len(greens), greens)
	}
	if greens[0].Time != 1000 {
		t.Errorf("green time = %d, want 1000", greens[0].Time)
	}

	midReds := []domain.TimingPoint{}
	for _, tp := range redsOnly(out) {
		if tp.Time == 2000 {
			midReds = append(midReds, tp)
		}
	}
	if len(midReds) != 1 {
		t.Fatalf("expected middle red preserved, got %+v", midReds)
	}
	mid := midReds[0]
	if mid.BeatLength != 400 {
		t.Errorf("middle red BeatLength lost: %g", mid.BeatLength)
	}
	if mid.SampleSet != 1 {
		t.Errorf("middle red SampleSet changed to %d, want 1 (preserved)", mid.SampleSet)
	}
	if mid.SampleIndex != 2 || mid.Volume != 60 {
		t.Errorf("middle red did not adopt index/volume: %+v", mid)
	}
}

func TestGenerateTimingPoints_PreFirstGroupRedKeepsOriginalState(t *testing.T) {
	groups := []timing.FinalGroup{
		fg(5000, 60, 2, []domain.Sampleset{domain.SamplesetDrum}),
	}
	original := redTP(0, 500)
	original.Volume = 77
	original.SampleIndex = 4
	reds := []domain.TimingPoint{original}

	out := GenerateTimingPoints(groups, reds)
	if !out[0].IsRed() || out[0].Time != 0 {
		t.Fatalf("expected red first at 0, got %+v", out[0])
	}
	if out[0].Volume != 77 || out[0].SampleIndex != 4 {
		t.Errorf("pre-first-group red should keep original state, got %+v", out[0])
	}
}

func TestGenerateTimingPoints_GoldenFullFlow(t *testing.T) {
	reds := []domain.TimingPoint{
		redTP(0, 500),
		redTP(4000, 400),
	}
	reds[0].Volume = 100
	reds[0].SampleSet = 1

	groups := []timing.FinalGroup{
		fg(1000, 60, 0, []domain.Sampleset{domain.SamplesetDrum}),
		fg(1500, 60, 0, []domain.Sampleset{domain.SamplesetSoft}),                       // sampleset-only change -> no TP
		fg(2000, 80, 0, []domain.Sampleset{domain.SamplesetDrum}),                       // volume change -> TP
		fg(3000, 80, 2, []domain.Sampleset{domain.SamplesetDrum}),                       // index change -> TP
		fg(4000, 70, 2, []domain.Sampleset{domain.SamplesetSoft, domain.SamplesetDrum}), // volume change on red
	}

	out := GenerateTimingPoints(groups, reds)

	type key struct {
		Time        int
		Kind        string
		SampleSet   int
		SampleIndex int
		Volume      int
		BeatLength  float64
	}
	got := make([]key, len(out))
	for i, tp := range out {
		kind := "green"
		if tp.IsRed() {
			kind = "red"
		}
		got[i] = key{tp.Time, kind, tp.SampleSet, tp.SampleIndex, tp.Volume, tp.BeatLength}
	}

	want := []key{
		{Time: 0, Kind: "red", SampleSet: 1, SampleIndex: 0, Volume: 100, BeatLength: 500},
		{Time: 1000, Kind: "green", SampleSet: 0, SampleIndex: 0, Volume: 60, BeatLength: -100},
		{Time: 2000, Kind: "green", SampleSet: 0, SampleIndex: 0, Volume: 80, BeatLength: -100},
		{Time: 3000, Kind: "green", SampleSet: 0, SampleIndex: 2, Volume: 80, BeatLength: -100},
		{Time: 4000, Kind: "red", SampleSet: 1, SampleIndex: 2, Volume: 70, BeatLength: 400},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("timing points mismatch\ngot:  %+v\nwant: %+v", got, want)
	}
}
