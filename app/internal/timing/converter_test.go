package timing

import (
	"math"
	"testing"

	"osu-daws-app/internal/domain"
)

func red(time int, beatLength float64) domain.TimingPoint {
	return domain.TimingPoint{Time: time, BeatLength: beatLength, Meter: 4, Uninherited: true}
}

func green(time int, bl float64) domain.TimingPoint {
	return domain.TimingPoint{Time: time, BeatLength: bl, Meter: 4, Uninherited: false}
}

func smWithTicks(ppq int, ticks ...int) *domain.SourceMap {
	sm := &domain.SourceMap{Meta: domain.SourceMapMeta{PPQ: ppq, TimeSignatureNumerator: 4}}
	for _, t := range ticks {
		sm.Events = append(sm.Events, domain.SourceEvent{
			Sampleset: domain.SamplesetDrum, CustomIndex: 0, Sound: domain.SoundNormal, Tick: t, Volume: 50,
		})
	}
	return sm
}

func closeEnough(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestNewConverterValidation(t *testing.T) {
	valid := smWithTicks(96, 0, 96)
	reds := []domain.TimingPoint{red(0, 500)}

	if _, err := NewConverter(nil, reds, 0); err == nil {
		t.Error("expected error on nil SourceMap")
	}
	if _, err := NewConverter(&domain.SourceMap{}, reds, 0); err == nil {
		t.Error("expected error on zero ppq")
	}
	if _, err := NewConverter(smWithTicks(96), reds, 0); err == nil {
		t.Error("expected error on empty events")
	}
	if _, err := NewConverter(valid, nil, 0); err == nil {
		t.Error("expected error on no red timing points")
	}
	if _, err := NewConverter(valid, []domain.TimingPoint{green(0, -100)}, 0); err == nil {
		t.Error("expected error when only green TPs provided")
	}
	if _, err := NewConverter(valid, []domain.TimingPoint{red(0, 0)}, 0); err == nil {
		t.Error("expected error on zero beatLength")
	}
	if _, err := NewConverter(valid, reds, 1000); err != nil {
		t.Errorf("expected success, got %v", err)
	}
}

func TestSingleSegment(t *testing.T) {
	sm := smWithTicks(96, 0, 48, 96, 192)
	reds := []domain.TimingPoint{red(0, 500)}
	c, err := NewConverter(sm, reds, 1000)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		tick int
		want float64
	}{
		{0, 1000},
		{48, 1250},
		{96, 1500},
		{192, 2000},
	}
	for _, tc := range cases {
		got := c.TickToMs(tc.tick)
		if !closeEnough(got, tc.want) {
			t.Errorf("TickToMs(%d) = %g, want %g", tc.tick, got, tc.want)
		}
	}
}

func TestNonZeroTickOrigin(t *testing.T) {
	sm := smWithTicks(96, 48, 96, 144)
	reds := []domain.TimingPoint{red(0, 500)}
	c, err := NewConverter(sm, reds, 2000)
	if err != nil {
		t.Fatal(err)
	}
	if c.TickOrigin() != 48 {
		t.Fatalf("tickOrigin = %d, want 48", c.TickOrigin())
	}
	cases := map[int]float64{
		48:  2000,
		96:  2250,
		144: 2500,
	}
	for tick, want := range cases {
		got := c.TickToMs(tick)
		if !closeEnough(got, want) {
			t.Errorf("TickToMs(%d) = %g, want %g", tick, got, want)
		}
	}
}

func TestMultipleRedSegments(t *testing.T) {
	sm := smWithTicks(96, 0, 96, 192, 288, 384)
	reds := []domain.TimingPoint{
		red(0, 500),
		red(1000, 250),
	}
	c, err := NewConverter(sm, reds, 0)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		tick int
		want float64
	}{
		{0, 0},
		{96, 500},
		{192, 1000},
		{288, 1250},
		{384, 1500},
	}
	for _, tc := range cases {
		got := c.TickToMs(tc.tick)
		if !closeEnough(got, tc.want) {
			t.Errorf("TickToMs(%d) = %g, want %g", tc.tick, got, tc.want)
		}
	}
}

func TestNoteCrossingTimingChange(t *testing.T) {
	sm := smWithTicks(96, 0, 144)
	reds := []domain.TimingPoint{
		red(0, 500),
		red(500, 250),
	}
	c, err := NewConverter(sm, reds, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := 500.0 + 0.5*250.0
	got := c.TickToMs(144)
	if !closeEnough(got, want) {
		t.Errorf("TickToMs(144) = %g, want %g", got, want)
	}
}

func TestConvertEventsMonotonic(t *testing.T) {
	sm := smWithTicks(96, 0, 24, 96, 120, 192, 384)
	reds := []domain.TimingPoint{
		red(0, 500),
		red(1000, 250),
		red(2000, 125),
	}
	c, err := NewConverter(sm, reds, 1000)
	if err != nil {
		t.Fatal(err)
	}
	events := c.ConvertEvents(sm)
	if len(events) != len(sm.Events) {
		t.Fatalf("got %d events, want %d", len(events), len(sm.Events))
	}
	for i := 1; i < len(events); i++ {
		if events[i].Source.Tick > events[i-1].Source.Tick && events[i].TimeMs <= events[i-1].TimeMs {
			t.Errorf("non-monotonic at i=%d: tick %d->%d, ms %g->%g",
				i, events[i-1].Source.Tick, events[i].Source.Tick, events[i-1].TimeMs, events[i].TimeMs)
		}
	}
}

func TestGreensAreIgnored(t *testing.T) {
	sm := smWithTicks(96, 0, 96)
	reds := []domain.TimingPoint{red(0, 500), green(100, -100), green(300, -50)}
	c, err := NewConverter(sm, reds, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := c.TickToMs(96); !closeEnough(got, 500) {
		t.Errorf("TickToMs(96) = %g, want 500 (greens should not affect conversion)", got)
	}
}

func TestTickToMsIntRounding(t *testing.T) {
	sm := smWithTicks(96, 0, 1)
	reds := []domain.TimingPoint{red(0, 500)}
	c, err := NewConverter(sm, reds, 0)
	if err != nil {
		t.Fatal(err)
	}
	// 1 tick at ppq 96 with 500ms/beat = 500/96 ≈ 5.208 → rounds to 5
	if got := c.TickToMsInt(1); got != 5 {
		t.Errorf("TickToMsInt(1) = %d, want 5", got)
	}
}

func TestStartTimeBeforeFirstRed(t *testing.T) {
	sm := smWithTicks(96, 0, 96)
	reds := []domain.TimingPoint{red(1000, 500)}
	c, err := NewConverter(sm, reds, 200)
	if err != nil {
		t.Fatal(err)
	}
	if got := c.TickToMs(0); !closeEnough(got, 200) {
		t.Errorf("TickToMs(0) = %g, want 200", got)
	}
	if got := c.TickToMs(96); !closeEnough(got, 700) {
		t.Errorf("TickToMs(96) = %g, want 700 (should use first red's beatLength)", got)
	}
}

func TestGoldenFullInput(t *testing.T) {
	sm := &domain.SourceMap{
		Meta: domain.SourceMapMeta{PPQ: 96, TimeSignatureNumerator: 4},
		Events: []domain.SourceEvent{
			{Sampleset: domain.SamplesetDrum, CustomIndex: 0, Sound: domain.SoundNormal, Tick: 0, Volume: 60},
			{Sampleset: domain.SamplesetDrum, CustomIndex: 0, Sound: domain.SoundClap, Tick: 96, Volume: 60},
			{Sampleset: domain.SamplesetSoft, CustomIndex: 0, Sound: domain.SoundNormal, Tick: 192, Volume: 70},
			{Sampleset: domain.SamplesetDrum, CustomIndex: 0, Sound: domain.SoundFinish, Tick: 288, Volume: 80},
			{Sampleset: domain.SamplesetDrum, CustomIndex: 0, Sound: domain.SoundWhistle, Tick: 480, Volume: 90},
		},
	}
	reds := []domain.TimingPoint{
		red(0, 500),
		red(1500, 250),
	}
	c, err := NewConverter(sm, reds, 1000)
	if err != nil {
		t.Fatal(err)
	}
	events := c.ConvertEvents(sm)
	want := []float64{1000, 1500, 1750, 2000, 2500}
	for i, e := range events {
		if !closeEnough(e.TimeMs, want[i]) {
			t.Errorf("event[%d] tick=%d ms=%g, want %g", i, e.Source.Tick, e.TimeMs, want[i])
		}
	}
}
