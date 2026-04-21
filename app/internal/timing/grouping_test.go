package timing

import (
	"reflect"
	"testing"

	"osu-daws-app/internal/domain"
	"osu-daws-app/internal/sourcemap"
)

func mkEvent(ss domain.Sampleset, ci int, sound domain.Sound, tick, vol int, ms float64) ConvertedEvent {
	return ConvertedEvent{
		Source: domain.SourceEvent{
			Sampleset: ss, CustomIndex: ci, Sound: sound, Tick: tick, Volume: vol,
		},
		TimeMs: ms,
	}
}

func TestGroupByFinalTime_Empty(t *testing.T) {
	if got := GroupByFinalTime(nil); got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestGroupByFinalTime_SingleEvent(t *testing.T) {
	events := []ConvertedEvent{
		mkEvent(domain.SamplesetDrum, 0, domain.SoundNormal, 0, 40, 1000),
	}
	groups := GroupByFinalTime(events)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	g := groups[0]
	if g.TimeMs != 1000 || g.Volume != 40 || g.CustomIndex != 0 {
		t.Errorf("unexpected group fields: %+v", g)
	}
	if !reflect.DeepEqual(g.Samplesets, []domain.Sampleset{domain.SamplesetDrum}) {
		t.Errorf("Samplesets = %v", g.Samplesets)
	}
	if !reflect.DeepEqual(g.Sounds, []domain.Sound{domain.SoundNormal}) {
		t.Errorf("Sounds = %v", g.Sounds)
	}
}

func TestGroupByFinalTime_MultiSoundSameTimestamp(t *testing.T) {
	events := []ConvertedEvent{
		mkEvent(domain.SamplesetDrum, 0, domain.SoundNormal, 96, 50, 1500.0),
		mkEvent(domain.SamplesetDrum, 0, domain.SoundClap, 96, 50, 1500.0),
		mkEvent(domain.SamplesetDrum, 0, domain.SoundFinish, 96, 50, 1500.0),
	}
	groups := GroupByFinalTime(events)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	g := groups[0]
	if g.TimeMs != 1500 || g.Volume != 50 || g.CustomIndex != 0 {
		t.Errorf("unexpected group fields: %+v", g)
	}
	if !reflect.DeepEqual(g.Samplesets, []domain.Sampleset{domain.SamplesetDrum}) {
		t.Errorf("Samplesets = %v, want [drum]", g.Samplesets)
	}
	want := []domain.Sound{domain.SoundNormal, domain.SoundClap, domain.SoundFinish}
	if !reflect.DeepEqual(g.Sounds, want) {
		t.Errorf("Sounds = %v, want %v", g.Sounds, want)
	}
	if len(g.Events) != 3 {
		t.Errorf("expected 3 events retained, got %d", len(g.Events))
	}

	res := ValidateFinalGroups(groups)
	if !res.OK() {
		t.Errorf("expected valid, got %s", res.Error())
	}
}

func TestGroupByFinalTime_TwoSamplesetsSameTimestamp(t *testing.T) {
	events := []ConvertedEvent{
		mkEvent(domain.SamplesetDrum, 2, domain.SoundNormal, 96, 60, 2000.0),
		mkEvent(domain.SamplesetSoft, 2, domain.SoundClap, 96, 60, 2000.0),
	}
	groups := GroupByFinalTime(events)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	g := groups[0]
	if g.CustomIndex != 2 || g.Volume != 60 {
		t.Errorf("unexpected: %+v", g)
	}
	want := []domain.Sampleset{domain.SamplesetDrum, domain.SamplesetSoft}
	if !reflect.DeepEqual(g.Samplesets, want) {
		t.Errorf("Samplesets = %v, want %v", g.Samplesets, want)
	}

	res := ValidateFinalGroups(groups)
	if !res.OK() {
		t.Errorf("expected valid, got %s", res.Error())
	}
}

func TestGroupByFinalTime_OrderingStability(t *testing.T) {
	events := []ConvertedEvent{
		mkEvent(domain.SamplesetDrum, 0, domain.SoundClap, 192, 50, 2000.0),
		mkEvent(domain.SamplesetDrum, 0, domain.SoundNormal, 0, 50, 0.0),
		mkEvent(domain.SamplesetDrum, 0, domain.SoundNormal, 96, 50, 500.0),
		mkEvent(domain.SamplesetDrum, 0, domain.SoundFinish, 192, 50, 2000.0),
		mkEvent(domain.SamplesetDrum, 0, domain.SoundWhistle, 192, 50, 2000.0),
	}
	groups := GroupByFinalTime(events)
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}
	wantTimes := []int{0, 500, 2000}
	for i, g := range groups {
		if g.TimeMs != wantTimes[i] {
			t.Errorf("group[%d].TimeMs = %d, want %d", i, g.TimeMs, wantTimes[i])
		}
	}

	wantSounds := []domain.Sound{domain.SoundClap, domain.SoundFinish, domain.SoundWhistle}
	if !reflect.DeepEqual(groups[2].Sounds, wantSounds) {
		t.Errorf("Sounds at 2000 = %v, want %v (insertion order)", groups[2].Sounds, wantSounds)
	}
	if len(groups[2].Events) != 3 {
		t.Errorf("expected 3 events at 2000, got %d", len(groups[2].Events))
	}
	for i, tickWant := range []int{192, 192, 192} {
		if groups[2].Events[i].Source.Tick != tickWant {
			t.Errorf("event order broken at %d: %+v", i, groups[2].Events[i])
		}
	}
}

func TestGroupByFinalTime_RoundingCollision(t *testing.T) {
	events := []ConvertedEvent{
		mkEvent(domain.SamplesetDrum, 0, domain.SoundNormal, 0, 40, 999.6),
		mkEvent(domain.SamplesetSoft, 0, domain.SoundClap, 1, 40, 1000.4),
	}
	groups := GroupByFinalTime(events)
	if len(groups) != 1 {
		t.Fatalf("expected rounding collision into 1 group, got %d", len(groups))
	}
	if groups[0].TimeMs != 1000 {
		t.Errorf("TimeMs = %d, want 1000", groups[0].TimeMs)
	}
}

func hasCodeField(res *domain.ValidationResult, code, field string) bool {
	for _, e := range res.Errors {
		if e.Code == code && e.Field == field {
			return true
		}
	}
	return false
}

func TestValidateFinalGroups_Conflicts(t *testing.T) {
	events := []ConvertedEvent{
		mkEvent(domain.SamplesetDrum, 0, domain.SoundNormal, 0, 40, 1000),
		mkEvent(domain.SamplesetSoft, 1, domain.SoundClap, 0, 50, 1000),
		mkEvent(domain.SamplesetNormal, 2, domain.SoundWhistle, 0, 60, 1000),
	}
	groups := GroupByFinalTime(events)
	res := ValidateFinalGroups(groups)
	if res.OK() {
		t.Fatal("expected conflicts")
	}
	if !hasCodeField(res, sourcemap.CodeConflictVolume, "ms[1000]") {
		t.Errorf("missing conflict_volume at ms[1000]: %s", res.Error())
	}
	if !hasCodeField(res, sourcemap.CodeConflictCustomIndex, "ms[1000]") {
		t.Errorf("missing conflict_custom_index at ms[1000]: %s", res.Error())
	}
}
