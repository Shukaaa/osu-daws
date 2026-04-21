package domain

type TimingPoint struct {
	Time        int     `json:"time"`
	BeatLength  float64 `json:"beatLength"`
	Meter       int     `json:"meter"`
	SampleSet   int     `json:"sampleSet"`
	SampleIndex int     `json:"sampleIndex"`
	Volume      int     `json:"volume"`
	Uninherited bool    `json:"uninherited"`
	Effects     int     `json:"effects"`
}

func (t TimingPoint) IsRed() bool {
	return t.Uninherited
}

func (t TimingPoint) IsGreen() bool {
	return !t.Uninherited
}
