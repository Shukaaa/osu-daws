package domain

import "testing"

func TestTimingPointRedGreen(t *testing.T) {
	red := TimingPoint{Time: 0, BeatLength: 500, Uninherited: true}
	green := TimingPoint{Time: 1000, BeatLength: -100, Uninherited: false}

	if !red.IsRed() || red.IsGreen() {
		t.Errorf("expected red timing point, got IsRed=%v IsGreen=%v", red.IsRed(), red.IsGreen())
	}
	if !green.IsGreen() || green.IsRed() {
		t.Errorf("expected green timing point, got IsRed=%v IsGreen=%v", green.IsRed(), green.IsGreen())
	}
}
