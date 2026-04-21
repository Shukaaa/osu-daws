package generator

import (
	"reflect"
	"testing"

	"osu-daws-app/internal/domain"
	"osu-daws-app/internal/timing"
)

func mkEvent(ss domain.Sampleset, snd domain.Sound, ci, vol int, ms float64) timing.ConvertedEvent {
	return timing.ConvertedEvent{
		Source: domain.SourceEvent{Sampleset: ss, CustomIndex: ci, Sound: snd, Tick: 0, Volume: vol},
		TimeMs: ms,
	}
}

func groupFromEvents(ms int, evs ...timing.ConvertedEvent) timing.FinalGroup {
	groups := timing.GroupByFinalTime(evs)
	for _, g := range groups {
		if g.TimeMs == ms {
			return g
		}
	}
	return timing.FinalGroup{TimeMs: ms}
}

func TestGenerateHitObject_DefaultSamplesetRules(t *testing.T) {
	cases := []struct {
		name       string
		defaultSet domain.Sampleset
		events     []timing.ConvertedEvent
		wantHS     string
	}{
		{
			name:       "default=soft, single soft-hitnormal -> auto/auto",
			defaultSet: domain.SamplesetSoft,
			events: []timing.ConvertedEvent{
				mkEvent(domain.SamplesetSoft, domain.SoundNormal, 0, 50, 0),
			},
			wantHS: "0:0:0:0:",
		},
		{
			name:       "default=soft, single drum-hitnormal -> drum/auto",
			defaultSet: domain.SamplesetSoft,
			events: []timing.ConvertedEvent{
				mkEvent(domain.SamplesetDrum, domain.SoundNormal, 0, 50, 0),
			},
			wantHS: "3:0:0:0:",
		},
		{
			name:       "default=soft, single drum-hitclap -> auto/drum",
			defaultSet: domain.SamplesetSoft,
			events: []timing.ConvertedEvent{
				mkEvent(domain.SamplesetDrum, domain.SoundClap, 0, 50, 0),
			},
			wantHS: "0:3:0:0:",
		},
		{
			name:       "default=soft, soft-hitnormal + drum-hitclap -> auto/drum",
			defaultSet: domain.SamplesetSoft,
			events: []timing.ConvertedEvent{
				mkEvent(domain.SamplesetSoft, domain.SoundNormal, 0, 50, 0),
				mkEvent(domain.SamplesetDrum, domain.SoundClap, 0, 50, 0),
			},
			wantHS: "0:3:0:0:",
		},
		{
			name:       "default=soft, drum-hitnormal + soft-hitclap -> drum/soft",
			defaultSet: domain.SamplesetSoft,
			events: []timing.ConvertedEvent{
				mkEvent(domain.SamplesetDrum, domain.SoundNormal, 0, 50, 0),
				mkEvent(domain.SamplesetSoft, domain.SoundClap, 0, 50, 0),
			},
			wantHS: "3:2:0:0:",
		},
		{
			name:       "default=drum, drum-hitnormal + drum-hitclap -> auto/auto",
			defaultSet: domain.SamplesetDrum,
			events: []timing.ConvertedEvent{
				mkEvent(domain.SamplesetDrum, domain.SoundNormal, 0, 50, 0),
				mkEvent(domain.SamplesetDrum, domain.SoundClap, 0, 50, 0),
			},
			wantHS: "0:0:0:0:",
		},
		{
			name:       "default=soft, drum-hitnormal + drum-hitclap -> drum/auto",
			defaultSet: domain.SamplesetSoft,
			events: []timing.ConvertedEvent{
				mkEvent(domain.SamplesetDrum, domain.SoundNormal, 0, 50, 0),
				mkEvent(domain.SamplesetDrum, domain.SoundClap, 0, 50, 0),
			},
			wantHS: "3:0:0:0:",
		},
		{
			name:       "default=soft, drum-hitnormal + normal-hitclap -> drum/normal",
			defaultSet: domain.SamplesetSoft,
			events: []timing.ConvertedEvent{
				mkEvent(domain.SamplesetDrum, domain.SoundNormal, 0, 50, 0),
				mkEvent(domain.SamplesetNormal, domain.SoundClap, 0, 50, 0),
			},
			wantHS: "3:1:0:0:",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := groupFromEvents(0, c.events...)
			ho, verr := GenerateHitObject(g, c.defaultSet)
			if verr != nil {
				t.Fatalf("unexpected validation error: %s", verr.Error())
			}
			if ho.HitSample != c.wantHS {
				t.Errorf("HitSample = %q, want %q", ho.HitSample, c.wantHS)
			}
		})
	}
}

func TestGenerateHitObject_ValidationFailures(t *testing.T) {
	cases := []struct {
		name       string
		defaultSet domain.Sampleset
		events     []timing.ConvertedEvent
		wantCode   string
	}{
		{
			name:       "two non-default hitnormals",
			defaultSet: domain.SamplesetSoft,
			events: []timing.ConvertedEvent{
				mkEvent(domain.SamplesetDrum, domain.SoundNormal, 0, 50, 0),
				mkEvent(domain.SamplesetNormal, domain.SoundNormal, 0, 50, 0),
			},
			wantCode: CodeMultipleNonDefaultNormals,
		},
		{
			name:       "multiple non-default addition-only samplesets",
			defaultSet: domain.SamplesetSoft,
			events: []timing.ConvertedEvent{
				mkEvent(domain.SamplesetDrum, domain.SoundClap, 0, 50, 0),
				mkEvent(domain.SamplesetNormal, domain.SoundWhistle, 0, 50, 0),
			},
			wantCode: CodeMultipleNonDefaultAdditions,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := groupFromEvents(0, c.events...)
			_, verr := GenerateHitObject(g, c.defaultSet)
			if verr == nil {
				t.Fatal("expected validation error")
			}
			if verr.Code != c.wantCode {
				t.Errorf("Code = %q, want %q", verr.Code, c.wantCode)
			}
			if verr.Field != "ms[0]" {
				t.Errorf("Field = %q, want ms[0]", verr.Field)
			}
		})
	}
}

func TestGenerateHitObject_FixedPosition(t *testing.T) {
	g := groupFromEvents(0, mkEvent(domain.SamplesetSoft, domain.SoundNormal, 0, 40, 0))
	ho, _ := GenerateHitObject(g, domain.SamplesetSoft)
	if ho.X != DefaultX || ho.Y != DefaultY {
		t.Errorf("position = (%d,%d), want (%d,%d)", ho.X, ho.Y, DefaultX, DefaultY)
	}
	if ho.Type != HitObjectType {
		t.Errorf("Type = %d, want %d (circle)", ho.Type, HitObjectType)
	}
}

func TestGenerateHitObject_HitSoundBitmask(t *testing.T) {
	g := groupFromEvents(0,
		mkEvent(domain.SamplesetSoft, domain.SoundNormal, 0, 50, 0),
		mkEvent(domain.SamplesetSoft, domain.SoundClap, 0, 50, 0),
		mkEvent(domain.SamplesetSoft, domain.SoundFinish, 0, 50, 0),
	)
	ho, verr := GenerateHitObject(g, domain.SamplesetSoft)
	if verr != nil {
		t.Fatal(verr.Error())
	}
	want := HitSoundNormal | HitSoundClap | HitSoundFinish
	if ho.HitSound != want {
		t.Errorf("HitSound = %d, want %d", ho.HitSound, want)
	}
}

func TestGenerateHitObject_VolumeNotInHitSample(t *testing.T) {
	// Even though the event carries volume=55, the HitSample must write 0
	// so that the timing point's volume is used at runtime.
	g := groupFromEvents(500, mkEvent(domain.SamplesetDrum, domain.SoundNormal, 9, 55, 500))
	ho, verr := GenerateHitObject(g, domain.SamplesetSoft)
	if verr != nil {
		t.Fatal(verr.Error())
	}
	if ho.HitSample != "3:0:9:0:" {
		t.Errorf("HitSample = %q, want %q (volume must be 0)", ho.HitSample, "3:0:9:0:")
	}
	if ho.Time != 500 {
		t.Errorf("Time = %d, want 500", ho.Time)
	}
}

func TestGenerateHitObjects_OrderPreserved(t *testing.T) {
	groups := []timing.FinalGroup{
		groupFromEvents(0, mkEvent(domain.SamplesetDrum, domain.SoundNormal, 0, 40, 0)),
		groupFromEvents(500, mkEvent(domain.SamplesetSoft, domain.SoundClap, 0, 50, 500)),
		groupFromEvents(1000,
			mkEvent(domain.SamplesetDrum, domain.SoundNormal, 1, 60, 1000),
			mkEvent(domain.SamplesetSoft, domain.SoundClap, 1, 60, 1000),
		),
	}
	out, vr := GenerateHitObjects(groups, domain.SamplesetSoft)
	if vr != nil {
		t.Fatal(vr.Error())
	}
	if len(out) != 3 {
		t.Fatalf("got %d, want 3", len(out))
	}
	times := []int{out[0].Time, out[1].Time, out[2].Time}
	if !reflect.DeepEqual(times, []int{0, 500, 1000}) {
		t.Errorf("times = %v, want [0 500 1000]", times)
	}
}

func TestGenerateHitObjects_CollectsAllErrors(t *testing.T) {
	groups := []timing.FinalGroup{
		groupFromEvents(0,
			mkEvent(domain.SamplesetDrum, domain.SoundNormal, 0, 40, 0),
			mkEvent(domain.SamplesetNormal, domain.SoundNormal, 0, 40, 0),
		),
		groupFromEvents(100,
			mkEvent(domain.SamplesetDrum, domain.SoundClap, 0, 40, 100),
			mkEvent(domain.SamplesetNormal, domain.SoundWhistle, 0, 40, 100),
		),
	}
	_, vr := GenerateHitObjects(groups, domain.SamplesetSoft)
	if vr == nil || vr.OK() {
		t.Fatal("expected validation errors")
	}
	if len(vr.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d: %s", len(vr.Errors), vr.Error())
	}
}
