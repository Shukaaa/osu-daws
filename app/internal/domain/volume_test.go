package domain

import "testing"

func TestVolumeFormatter_Apply(t *testing.T) {
	cases := []struct {
		name string
		step int
		in   int
		want int
	}{
		{"step 0 keeps value", 0, 73, 73},
		{"step 1 keeps value", 1, 73, 73},
		{"step 5 rounds up at midpoint", 5, 73, 75},
		{"step 5 rounds 94 up to 95", 5, 94, 95},
		{"step 5 rounds 82 down to 80", 5, 82, 80},
		{"step 5 keeps 55", 5, 55, 55},
		{"step 5 rounds 96 up to 95", 5, 96, 95},
		{"step 10 rounds 73 to 70", 10, 73, 70},
		{"step 10 rounds 75 to 80", 10, 75, 80},
		{"step 25 rounds 60 to 50", 25, 60, 50},
		{"step 25 rounds 63 to 75", 25, 63, 75},
		{"clamps negative input", 5, -10, 0},
		{"clamps over-100 input", 5, 150, 100},
		{"never exceeds 100 after rounding", 7, 99, 98},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := VolumeFormatter{Step: tc.step}
			if got := f.Apply(tc.in); got != tc.want {
				t.Errorf("Apply(%d) with step=%d = %d, want %d", tc.in, tc.step, got, tc.want)
			}
		})
	}
}

func TestVolumeFormatter_Enabled(t *testing.T) {
	cases := []struct {
		step    int
		enabled bool
	}{
		{0, false},
		{1, false},
		{2, true},
		{5, true},
		{100, true},
	}
	for _, tc := range cases {
		if got := (VolumeFormatter{Step: tc.step}).Enabled(); got != tc.enabled {
			t.Errorf("Enabled with step=%d = %v, want %v", tc.step, got, tc.enabled)
		}
	}
}

func TestVolumeFormatter_IsValid(t *testing.T) {
	if !(VolumeFormatter{Step: 0}).IsValid() {
		t.Error("step 0 should be valid")
	}
	if !(VolumeFormatter{Step: 5}).IsValid() {
		t.Error("step 5 should be valid")
	}
	if (VolumeFormatter{Step: -1}).IsValid() {
		t.Error("negative step should be invalid")
	}
	if (VolumeFormatter{Step: 101}).IsValid() {
		t.Error("step > 100 should be invalid")
	}
}
