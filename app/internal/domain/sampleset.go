package domain

import "fmt"

type Sampleset string

const (
	SamplesetDrum   Sampleset = "drum"
	SamplesetSoft   Sampleset = "soft"
	SamplesetNormal Sampleset = "normal"
)

func AllSamplesets() []Sampleset {
	return []Sampleset{SamplesetDrum, SamplesetSoft, SamplesetNormal}
}

func (s Sampleset) String() string {
	return string(s)
}

func (s Sampleset) IsValid() bool {
	switch s {
	case SamplesetDrum, SamplesetSoft, SamplesetNormal:
		return true
	}
	return false
}

func ParseSampleset(raw string) (Sampleset, error) {
	s := Sampleset(raw)
	if !s.IsValid() {
		return "", fmt.Errorf("invalid sampleset %q", raw)
	}
	return s, nil
}
