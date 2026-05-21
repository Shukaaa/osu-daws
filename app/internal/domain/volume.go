package domain

// VolumeFormatter normalizes per-event volume percentages to a fixed step
// (e.g. multiples of 5%). This compensates for tiny imprecisions when
// authoring volumes in a DAW where pixel-perfect adjustments are hard, so
// values like 94 become 95 and 82 becomes 80 with a step of 5.
//
// A Step of 0 or 1 disables formatting and leaves the volume untouched.
type VolumeFormatter struct {
	Step int `json:"step,omitempty"`
}

// IsValid reports whether the formatter step is in the allowed range.
// Step 0 means "no rounding".
func (f VolumeFormatter) IsValid() bool {
	return f.Step >= 0 && f.Step <= 100
}

// Enabled reports whether Apply will actually round.
func (f VolumeFormatter) Enabled() bool { return f.Step > 1 }

// Apply rounds volume to the nearest multiple of Step using
// half-up rounding, clamped to [0, 100].
func (f VolumeFormatter) Apply(volume int) int {
	if volume < 0 {
		volume = 0
	}
	if volume > 100 {
		volume = 100
	}
	if !f.Enabled() {
		return volume
	}
	step := f.Step
	rounded := ((volume + step/2) / step) * step
	if rounded > 100 {
		rounded = 100
	}
	if rounded < 0 {
		rounded = 0
	}
	return rounded
}
