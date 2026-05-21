package pipeline

import (
	"strings"
	"testing"

	"osu-daws-app/internal/domain"
)

const sourceMapWithVolumeNoise = `{
  "_meta":{"ppq":96,"timeSignatureNumerator":4},
  "drum":{"0":{"normal":["0;94","96;82"]}},
  "soft":{"0":{"clap":["192;55"]}}
}`

func TestPipeline_VolumeFormatter_RoundsToStep(t *testing.T) {
	req := Request{
		Segments: []Segment{{
			SourceMapJSON: []byte(sourceMapWithVolumeNoise),
			StartTimeMs:   1000,
		}},
		ReferenceOsu:     strings.NewReader(refOsu),
		DefaultSampleset: domain.SamplesetSoft,
		VolumeFormatter:  domain.VolumeFormatter{Step: 5},
	}
	res, pErr := Generate(req)
	if pErr != nil {
		t.Fatalf("Generate failed: %v", pErr)
	}
	wants := map[int]int{0: 95, 96: 80, 192: 55}
	got := map[int]int{}
	for _, sm := range res.SourceMaps {
		for _, e := range sm.Events {
			got[e.Tick] = e.Volume
		}
	}
	for tick, want := range wants {
		if got[tick] != want {
			t.Errorf("tick %d volume = %d, want %d", tick, got[tick], want)
		}
	}
}

// With rounding step=5, 94 and 96 both snap to 95, so what would otherwise
// be a same-tick volume conflict resolves cleanly.
func TestPipeline_VolumeFormatter_NormalizesNearDuplicates(t *testing.T) {
	src := `{
  "_meta":{"ppq":96,"timeSignatureNumerator":4},
  "drum":{"0":{"normal":["0;94"]}},
  "soft":{"0":{"clap":["0;96"]}}
}`
	reqStrict := Request{
		Segments:         []Segment{{SourceMapJSON: []byte(src), StartTimeMs: 1000}},
		ReferenceOsu:     strings.NewReader(refOsu),
		DefaultSampleset: domain.SamplesetSoft,
	}
	if _, pErr := Generate(reqStrict); pErr == nil {
		t.Fatalf("expected volume-conflict validation error without formatter")
	}

	reqRounded := Request{
		Segments:         []Segment{{SourceMapJSON: []byte(src), StartTimeMs: 1000}},
		ReferenceOsu:     strings.NewReader(refOsu),
		DefaultSampleset: domain.SamplesetSoft,
		VolumeFormatter:  domain.VolumeFormatter{Step: 5},
	}
	if _, pErr := Generate(reqRounded); pErr != nil {
		t.Fatalf("expected success with rounding, got: %v", pErr)
	}
}
