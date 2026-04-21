package domain

import "testing"

func TestParseSampleset(t *testing.T) {
	cases := []struct {
		in      string
		want    Sampleset
		wantErr bool
	}{
		{"drum", SamplesetDrum, false},
		{"soft", SamplesetSoft, false},
		{"normal", SamplesetNormal, false},
		{"Drum", "", true},
		{"", "", true},
		{"bass", "", true},
	}
	for _, c := range cases {
		got, err := ParseSampleset(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("ParseSampleset(%q) err=%v wantErr=%v", c.in, err, c.wantErr)
			continue
		}
		if !c.wantErr && got != c.want {
			t.Errorf("ParseSampleset(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSamplesetIsValid(t *testing.T) {
	if !SamplesetDrum.IsValid() || !SamplesetSoft.IsValid() || !SamplesetNormal.IsValid() {
		t.Fatal("expected all canonical samplesets to be valid")
	}
	if Sampleset("").IsValid() || Sampleset("bass").IsValid() {
		t.Fatal("expected invalid samplesets to report invalid")
	}
}
