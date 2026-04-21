package sourcemap

import (
	"testing"

	"osu-daws-app/internal/domain"
)

func ev(ss domain.Sampleset, ci int, sound domain.Sound, tick, vol int) domain.SourceEvent {
	return domain.SourceEvent{Sampleset: ss, CustomIndex: ci, Sound: sound, Tick: tick, Volume: vol}
}

func TestGroupByTick(t *testing.T) {
	events := []domain.SourceEvent{
		ev(domain.SamplesetDrum, 0, domain.SoundNormal, 96, 40),
		ev(domain.SamplesetDrum, 0, domain.SoundClap, 96, 40),
		ev(domain.SamplesetSoft, 0, domain.SoundNormal, 0, 50),
		ev(domain.SamplesetDrum, 0, domain.SoundNormal, 192, 60),
	}
	groups := GroupByTick(events)
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}
	if groups[0].Tick != 0 || groups[1].Tick != 96 || groups[2].Tick != 192 {
		t.Fatalf("groups not sorted by tick: %+v", groups)
	}
	if len(groups[1].Events) != 2 {
		t.Fatalf("expected 2 events at tick 96, got %d", len(groups[1].Events))
	}
}

func TestGroupByTickEmpty(t *testing.T) {
	if got := GroupByTick(nil); got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func hasCodeAt(res *domain.ValidationResult, code, field string) bool {
	for _, e := range res.Errors {
		if e.Code == code && e.Field == field {
			return true
		}
	}
	return false
}

func TestValidateTickGroups(t *testing.T) {
	cases := []struct {
		name       string
		events     []domain.SourceEvent
		wantOK     bool
		wantCodes  []string
		wantFields []string
	}{
		{
			name: "single event",
			events: []domain.SourceEvent{
				ev(domain.SamplesetDrum, 0, domain.SoundNormal, 0, 40),
			},
			wantOK: true,
		},
		{
			name: "multiple sounds same tick same volume same index",
			events: []domain.SourceEvent{
				ev(domain.SamplesetDrum, 0, domain.SoundNormal, 96, 40),
				ev(domain.SamplesetDrum, 0, domain.SoundClap, 96, 40),
				ev(domain.SamplesetDrum, 0, domain.SoundWhistle, 96, 40),
				ev(domain.SamplesetDrum, 0, domain.SoundFinish, 96, 40),
			},
			wantOK: true,
		},
		{
			name: "valid 2-sampleset overlap",
			events: []domain.SourceEvent{
				ev(domain.SamplesetDrum, 2, domain.SoundNormal, 96, 40),
				ev(domain.SamplesetSoft, 2, domain.SoundClap, 96, 40),
			},
			wantOK: true,
		},
		{
			name: "conflicting volumes",
			events: []domain.SourceEvent{
				ev(domain.SamplesetDrum, 0, domain.SoundNormal, 96, 40),
				ev(domain.SamplesetDrum, 0, domain.SoundClap, 96, 50),
			},
			wantOK:     false,
			wantCodes:  []string{CodeConflictVolume},
			wantFields: []string{"tick[96]"},
		},
		{
			name: "conflicting custom indices",
			events: []domain.SourceEvent{
				ev(domain.SamplesetDrum, 0, domain.SoundNormal, 96, 40),
				ev(domain.SamplesetDrum, 3, domain.SoundClap, 96, 40),
			},
			wantOK:     false,
			wantCodes:  []string{CodeConflictCustomIndex},
			wantFields: []string{"tick[96]"},
		},
		{
			name: "three samplesets on one tick is no longer a sourcemap-level conflict",
			events: []domain.SourceEvent{
				ev(domain.SamplesetDrum, 0, domain.SoundNormal, 96, 40),
				ev(domain.SamplesetSoft, 0, domain.SoundClap, 96, 40),
				ev(domain.SamplesetNormal, 0, domain.SoundWhistle, 96, 40),
			},
			wantOK: true,
		},
		{
			name: "multiple conflicts on one tick",
			events: []domain.SourceEvent{
				ev(domain.SamplesetDrum, 0, domain.SoundNormal, 96, 40),
				ev(domain.SamplesetSoft, 1, domain.SoundClap, 96, 50),
				ev(domain.SamplesetNormal, 2, domain.SoundWhistle, 96, 60),
			},
			wantOK: false,
			wantCodes: []string{
				CodeConflictVolume,
				CodeConflictCustomIndex,
			},
			wantFields: []string{"tick[96]", "tick[96]"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			groups := GroupByTick(c.events)
			res := ValidateTickGroups(groups)
			if res.OK() != c.wantOK {
				t.Fatalf("OK=%v want=%v errors=%s", res.OK(), c.wantOK, res.Error())
			}
			for i, code := range c.wantCodes {
				if !hasCodeAt(res, code, c.wantFields[i]) {
					t.Errorf("missing expected error code=%s field=%s (got %s)",
						code, c.wantFields[i], res.Error())
				}
			}
		})
	}
}

func TestValidateTickGroupsIndependentTicks(t *testing.T) {
	events := []domain.SourceEvent{
		ev(domain.SamplesetDrum, 0, domain.SoundNormal, 0, 40),
		ev(domain.SamplesetDrum, 0, domain.SoundClap, 0, 50), // conflict at 0
		ev(domain.SamplesetDrum, 0, domain.SoundNormal, 96, 40),
		ev(domain.SamplesetSoft, 0, domain.SoundClap, 96, 40), // valid overlap at 96
	}
	res := ValidateTickGroups(GroupByTick(events))
	if res.OK() {
		t.Fatal("expected conflict")
	}
	if !hasCodeAt(res, CodeConflictVolume, "tick[0]") {
		t.Errorf("expected volume conflict at tick[0], got %s", res.Error())
	}
	if hasCodeAt(res, CodeConflictVolume, "tick[96]") {
		t.Errorf("unexpected conflict at tick[96]: %s", res.Error())
	}
}

func TestTickGroupAccessors(t *testing.T) {
	g := TickGroup{Tick: 10, Events: []domain.SourceEvent{
		ev(domain.SamplesetDrum, 2, domain.SoundNormal, 10, 40),
		ev(domain.SamplesetSoft, 2, domain.SoundClap, 10, 40),
		ev(domain.SamplesetDrum, 2, domain.SoundFinish, 10, 40),
	}}
	if got := g.Volumes(); len(got) != 1 || got[0] != 40 {
		t.Errorf("Volumes() = %v", got)
	}
	if got := g.CustomIndices(); len(got) != 1 || got[0] != 2 {
		t.Errorf("CustomIndices() = %v", got)
	}
	ss := g.Samplesets()
	if len(ss) != 2 || ss[0] != domain.SamplesetDrum || ss[1] != domain.SamplesetSoft {
		t.Errorf("Samplesets() = %v", ss)
	}
}
