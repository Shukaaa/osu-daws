package domain

import "testing"

func TestParseSound(t *testing.T) {
	cases := []struct {
		in      string
		want    Sound
		wantErr bool
	}{
		{"normal", SoundNormal, false},
		{"clap", SoundClap, false},
		{"whistle", SoundWhistle, false},
		{"finish", SoundFinish, false},
		{"Normal", "", true},
		{"", "", true},
		{"slide", "", true},
	}
	for _, c := range cases {
		got, err := ParseSound(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("ParseSound(%q) err=%v wantErr=%v", c.in, err, c.wantErr)
			continue
		}
		if !c.wantErr && got != c.want {
			t.Errorf("ParseSound(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
