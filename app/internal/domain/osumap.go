package domain

type OsuMap struct {
	General      map[string]string
	Editor       map[string]string
	Metadata     map[string]string
	Difficulty   map[string]string
	Events       []string
	TimingPoints []TimingPoint
	HitObjects   []HitObject
}

func NewOsuMap() *OsuMap {
	return &OsuMap{
		General:    map[string]string{},
		Editor:     map[string]string{},
		Metadata:   map[string]string{},
		Difficulty: map[string]string{},
	}
}
