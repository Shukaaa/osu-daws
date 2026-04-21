package domain

type SourceMapMeta struct {
	PPQ                    int `json:"ppq"`
	TimeSignatureNumerator int `json:"timeSignatureNumerator"`
}

type RawSourceMap struct {
	Meta       SourceMapMeta           `json:"_meta"`
	Samplesets map[string]RawSampleset `json:"-"`
}

type RawSampleset map[string]RawCustomIndex

type RawCustomIndex map[string][]string

type SourceEvent struct {
	Sampleset   Sampleset
	CustomIndex int
	Sound       Sound
	Tick        int
	Volume      int
}

type SourceMap struct {
	Meta   SourceMapMeta
	Events []SourceEvent
}
