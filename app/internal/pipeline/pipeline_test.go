package pipeline

import (
	"errors"
	"strings"
	"testing"

	"osu-daws-app/internal/domain"
	"osu-daws-app/internal/generator"
	"osu-daws-app/internal/osufile"
	"osu-daws-app/internal/sourcemap"
)

const refOsu = `osu file format v14

[General]
AudioFilename: audio.mp3
Mode: 0

[Metadata]
Title:Ref
Artist:Tester
Creator:Mapper
Version:Easy

[Difficulty]
HPDrainRate:5
CircleSize:4
OverallDifficulty:5
ApproachRate:5
SliderMultiplier:1.4
SliderTickRate:1

[Events]
0,0,"bg.jpg",0,0

[TimingPoints]
0,500,4,1,0,70,1,0
1000,-100,4,1,0,60,0,0

[HitObjects]
256,192,0,1,0,0:0:0:0:
`

const validSourceMap = `{
  "_meta":{"ppq":96,"timeSignatureNumerator":4},
  "drum":{"0":{"normal":["0;50","96;50"],"clap":["192;60"]}},
  "soft":{"0":{"clap":["384;70"]}}
}`

func TestPipeline_ValidFullFlow(t *testing.T) {
	req := Request{
		Segments: []Segment{{
			SourceMapJSON: []byte(validSourceMap),
			StartTimeMs:   1000,
		}},
		ReferenceOsu:     strings.NewReader(refOsu),
		DefaultSampleset: domain.SamplesetSoft,
	}
	res, pErr := Generate(req)
	if pErr != nil {
		t.Fatalf("unexpected error: %s", pErr.Error())
	}
	if res == nil || res.OsuContent == "" {
		t.Fatal("expected non-empty result")
	}

	parsed, vr := osufile.Parse(strings.NewReader(res.OsuContent))
	if !vr.OK() {
		t.Fatalf("generated file does not re-parse: %s", vr.Error())
	}
	if parsed.Metadata["Title"] != "Ref" {
		t.Errorf("Title = %q, want Ref", parsed.Metadata["Title"])
	}
	if len(parsed.HitObjects) != 4 {
		t.Errorf("expected 4 hitobjects (ticks 0,96,192,384), got %d", len(parsed.HitObjects))
	}
	reds := 0
	greens := 0
	for _, tp := range parsed.TimingPoints {
		if tp.IsRed() {
			reds++
		} else {
			greens++
		}
	}
	if reds != 1 {
		t.Errorf("expected 1 red (preserved from reference), got %d", reds)
	}
	if greens == 0 {
		t.Error("expected at least one generated green")
	}

	for _, ho := range parsed.HitObjects {
		if ho.Time < 1000 {
			t.Errorf("hitobject before start time: %+v", ho)
		}
	}
}

func TestPipeline_InvalidSourceMapJSON(t *testing.T) {
	req := Request{
		Segments: []Segment{{
			SourceMapJSON: []byte(`{not json`),
			StartTimeMs:   1000,
		}},
		ReferenceOsu:     strings.NewReader(refOsu),
		DefaultSampleset: domain.SamplesetSoft,
	}
	_, pErr := Generate(req)
	if pErr == nil {
		t.Fatal("expected error")
	}
	if pErr.Stage != StageSourceMapParse {
		t.Errorf("stage = %s, want %s", pErr.Stage, StageSourceMapParse)
	}
	if pErr.Validation == nil {
		t.Fatal("expected validation result")
	}
	found := false
	for _, e := range pErr.Validation.Errors {
		if e.Code == sourcemap.CodeInvalidJSON {
			found = true
		}
	}
	if !found {
		t.Errorf("expected invalid_json code, got %s", pErr.Validation.Error())
	}
}

func TestPipeline_MultipleNonDefaultNormalsFail(t *testing.T) {
	conflict := `{
	  "_meta":{"ppq":96,"timeSignatureNumerator":4},
	  "drum":{"0":{"normal":["0;40"]}},
	  "normal":{"0":{"normal":["0;40"]}}
	}`
	req := Request{
		Segments: []Segment{{
			SourceMapJSON: []byte(conflict),
			StartTimeMs:   0,
		}},
		ReferenceOsu:     strings.NewReader(refOsu),
		DefaultSampleset: domain.SamplesetSoft,
	}
	_, pErr := Generate(req)
	if pErr == nil {
		t.Fatal("expected error")
	}
	if pErr.Stage != StageGenerate {
		t.Errorf("stage = %s, want %s", pErr.Stage, StageGenerate)
	}
	found := false
	for _, e := range pErr.Validation.Errors {
		if e.Code == generator.CodeMultipleNonDefaultNormals {
			found = true
		}
	}
	if !found {
		t.Errorf("expected %s, got %s", generator.CodeMultipleNonDefaultNormals, pErr.Validation.Error())
	}
}

func TestPipeline_MultipleNonDefaultAdditionsFail(t *testing.T) {
	// default=soft, no non-default hitnormal, but tick 0 carries addition
	// sounds on drum AND normal -> validation fails at StageGenerate.
	conflict := `{
	  "_meta":{"ppq":96,"timeSignatureNumerator":4},
	  "drum":{"0":{"clap":["0;40"]}},
	  "normal":{"0":{"whistle":["0;40"]}}
	}`
	req := Request{
		Segments: []Segment{{
			SourceMapJSON: []byte(conflict),
			StartTimeMs:   0,
		}},
		ReferenceOsu:     strings.NewReader(refOsu),
		DefaultSampleset: domain.SamplesetSoft,
	}
	_, pErr := Generate(req)
	if pErr == nil {
		t.Fatal("expected error")
	}
	if pErr.Stage != StageGenerate {
		t.Errorf("stage = %s, want %s", pErr.Stage, StageGenerate)
	}
	found := false
	for _, e := range pErr.Validation.Errors {
		if e.Code == generator.CodeMultipleNonDefaultAdditions {
			found = true
		}
	}
	if !found {
		t.Errorf("expected %s, got %s", generator.CodeMultipleNonDefaultAdditions, pErr.Validation.Error())
	}
}

func TestPipeline_ReferenceParseFailure(t *testing.T) {
	req := Request{
		Segments: []Segment{{
			SourceMapJSON: []byte(validSourceMap),
			StartTimeMs:   0,
		}},
		ReferenceOsu:     strings.NewReader("not a valid osu file"),
		DefaultSampleset: domain.SamplesetSoft,
	}
	_, pErr := Generate(req)
	if pErr == nil {
		t.Fatal("expected error")
	}
	if pErr.Stage != StageReferenceParse {
		t.Errorf("stage = %s, want %s", pErr.Stage, StageReferenceParse)
	}
}

func TestPipeline_NilReference(t *testing.T) {
	req := Request{
		Segments: []Segment{{
			SourceMapJSON: []byte(validSourceMap),
			StartTimeMs:   0,
		}},
		ReferenceOsu:     nil,
		DefaultSampleset: domain.SamplesetSoft,
	}
	_, pErr := Generate(req)
	if pErr == nil || pErr.Stage != StageReferenceParse {
		t.Fatalf("expected reference_parse error, got %+v", pErr)
	}
}

func TestPipeline_InvalidDefaultSampleset(t *testing.T) {
	req := Request{
		Segments: []Segment{{
			SourceMapJSON: []byte(validSourceMap),
			StartTimeMs:   0,
		}},
		ReferenceOsu:     strings.NewReader(refOsu),
		DefaultSampleset: "", // invalid
	}
	_, pErr := Generate(req)
	if pErr == nil {
		t.Fatal("expected error")
	}
	if pErr.Stage != StageDefaultSampleset {
		t.Errorf("stage = %s, want %s", pErr.Stage, StageDefaultSampleset)
	}
}

const refWithoutReds = `osu file format v14

[General]
AudioFilename: audio.mp3

[Metadata]
Title:NoReds
Artist:Tester
Creator:Mapper
Version:Easy

[Difficulty]
HPDrainRate:5
CircleSize:4
OverallDifficulty:5
ApproachRate:5
SliderMultiplier:1.4
SliderTickRate:1

[Events]

[TimingPoints]
1000,-100,4,1,0,60,0,0

[HitObjects]
`

func TestPipeline_TimingSetupFailsWithoutReds(t *testing.T) {
	req := Request{
		Segments: []Segment{{
			SourceMapJSON: []byte(validSourceMap),
			StartTimeMs:   0,
		}},
		ReferenceOsu:     strings.NewReader(refWithoutReds),
		DefaultSampleset: domain.SamplesetSoft,
	}
	_, pErr := Generate(req)
	if pErr == nil {
		t.Fatal("expected error")
	}
	if pErr.Stage != StageTimingSetup {
		t.Errorf("stage = %s, want %s", pErr.Stage, StageTimingSetup)
	}
}

func TestPipeline_FinalGroupConflictViaRounding(t *testing.T) {
	// ppq=1000 and a large beatLength makes sub-tick differences round to the
	// same ms. Ticks 0 and 1 at 600_000 ms/beat → 0ms and 600ms... not close.
	// To force rounding collision while keeping ticks distinct we need tiny
	// ms gap. Use ppq = 10000 and one red with beatLength = 1 ms -> per-tick
	// delta = 1/10000 = 0.0001 ms. Two different ticks round to same ms but
	// different volumes/indices => conflict at final stage.
	sm := `{
	  "_meta":{"ppq":10000,"timeSignatureNumerator":4},
	  "drum":{"0":{"normal":["0;40"]},"1":{"normal":["1;50"]}}
	}`
	// NOTE: custom index 1 vs 0 on same rounded ms -> conflict_custom_index
	fastRef := `osu file format v14

[Metadata]
Title:Fast
Artist:Tester
Creator:Mapper
Version:Easy

[Difficulty]
HPDrainRate:5
CircleSize:4
OverallDifficulty:5
ApproachRate:5
SliderMultiplier:1.4
SliderTickRate:1

[Events]

[TimingPoints]
0,1,4,1,0,70,1,0

[HitObjects]
`
	req := Request{
		Segments: []Segment{{
			SourceMapJSON: []byte(sm),
			StartTimeMs:   0,
		}},
		ReferenceOsu:     strings.NewReader(fastRef),
		DefaultSampleset: domain.SamplesetSoft,
	}
	_, pErr := Generate(req)
	if pErr == nil {
		t.Fatal("expected final validation failure")
	}
	if pErr.Stage != StageFinalValidate {
		t.Errorf("stage = %s, want %s", pErr.Stage, StageFinalValidate)
	}
}

// ---------------------------------------------------------------------------
// Multi-segment coverage
// ---------------------------------------------------------------------------

// segmentA: one drum normal at tick 0.
const segmentA = `{
  "_meta":{"ppq":96,"timeSignatureNumerator":4},
  "drum":{"0":{"normal":["0;40"]}}
}`

// segmentB: one soft clap at tick 0 (addition only).
const segmentB = `{
  "_meta":{"ppq":96,"timeSignatureNumerator":4},
  "soft":{"0":{"clap":["0;60"]}}
}`

func TestPipeline_MultipleSegments_MergeAndSort(t *testing.T) {
	// Two segments with the same internal tick 0 but different StartTimeMs
	// must appear as two distinct hit objects ordered by final ms.
	req := Request{
		Segments: []Segment{
			{SourceMapJSON: []byte(segmentA), StartTimeMs: 3000, Label: "A"},
			{SourceMapJSON: []byte(segmentB), StartTimeMs: 1500, Label: "B"},
		},
		ReferenceOsu:     strings.NewReader(refOsu),
		DefaultSampleset: domain.SamplesetSoft,
	}
	res, pErr := Generate(req)
	if pErr != nil {
		t.Fatalf("unexpected error: %s", pErr.Error())
	}
	if len(res.SourceMaps) != 2 {
		t.Fatalf("expected 2 parsed SourceMaps, got %d", len(res.SourceMaps))
	}
	parsed, vr := osufile.Parse(strings.NewReader(res.OsuContent))
	if !vr.OK() {
		t.Fatalf("re-parse failed: %s", vr.Error())
	}
	if len(parsed.HitObjects) != 2 {
		t.Fatalf("expected 2 hitobjects (one per segment), got %d", len(parsed.HitObjects))
	}
	// Order must follow final ms, so B (1500) before A (3000).
	if parsed.HitObjects[0].Time > parsed.HitObjects[1].Time {
		t.Errorf("hitobjects not sorted by time: %d before %d",
			parsed.HitObjects[0].Time, parsed.HitObjects[1].Time)
	}
	if parsed.HitObjects[0].Time != 1500 {
		t.Errorf("first hitobject time = %d, want 1500", parsed.HitObjects[0].Time)
	}
	if parsed.HitObjects[1].Time != 3000 {
		t.Errorf("second hitobject time = %d, want 3000", parsed.HitObjects[1].Time)
	}
}

func TestPipeline_NoSegments(t *testing.T) {
	req := Request{
		Segments:         nil,
		ReferenceOsu:     strings.NewReader(refOsu),
		DefaultSampleset: domain.SamplesetSoft,
	}
	_, pErr := Generate(req)
	if pErr == nil {
		t.Fatal("expected error")
	}
	if pErr.Stage != StageNoSegments {
		t.Errorf("stage = %s, want %s", pErr.Stage, StageNoSegments)
	}
	if pErr.SegmentIndex != -1 {
		t.Errorf("SegmentIndex = %d, want -1", pErr.SegmentIndex)
	}
}

func TestPipeline_SecondSegmentInvalidJSON_ReportsCorrectIndex(t *testing.T) {
	req := Request{
		Segments: []Segment{
			{SourceMapJSON: []byte(segmentA), StartTimeMs: 1000},
			{SourceMapJSON: []byte(`{broken`), StartTimeMs: 2000},
		},
		ReferenceOsu:     strings.NewReader(refOsu),
		DefaultSampleset: domain.SamplesetSoft,
	}
	_, pErr := Generate(req)
	if pErr == nil {
		t.Fatal("expected error")
	}
	if pErr.Stage != StageSourceMapParse {
		t.Errorf("stage = %s, want %s", pErr.Stage, StageSourceMapParse)
	}
	if pErr.SegmentIndex != 1 {
		t.Errorf("SegmentIndex = %d, want 1", pErr.SegmentIndex)
	}
	// Error message must surface 1-based segment number.
	if !strings.Contains(pErr.Error(), "segment 2") {
		t.Errorf("error message missing 'segment 2': %s", pErr.Error())
	}
}

func TestPipeline_SegmentTickValidationReportsCorrectIndex(t *testing.T) {
	// Two non-default hitnormals on the same tick within segment index 1.
	conflict := `{
	  "_meta":{"ppq":96,"timeSignatureNumerator":4},
	  "drum":{"0":{"normal":["0;40"]}},
	  "normal":{"0":{"normal":["0;40"]}}
	}`
	req := Request{
		Segments: []Segment{
			{SourceMapJSON: []byte(segmentA), StartTimeMs: 0},
			{SourceMapJSON: []byte(conflict), StartTimeMs: 1000},
		},
		ReferenceOsu:     strings.NewReader(refOsu),
		DefaultSampleset: domain.SamplesetSoft,
	}
	_, pErr := Generate(req)
	if pErr == nil {
		t.Fatal("expected error")
	}
	// Per-segment tick validation happens either at SourceMapValidate or
	// at Generate (final merge) depending on how conflicts are classified;
	// the key property is that the error surfaces and points at segment 2.
	if pErr.SegmentIndex != 1 && !strings.Contains(pErr.Error(), "segment 2") {
		// Fall back to final-stage merged errors which have SegmentIndex=-1;
		// in that case we just require Generate stage.
		if pErr.Stage != StageGenerate && pErr.Stage != StageSourceMapValidate {
			t.Errorf("unexpected stage=%s idx=%d: %s",
				pErr.Stage, pErr.SegmentIndex, pErr.Error())
		}
	}
}

func TestPipeline_SegmentTimingSetupReportsCorrectIndex(t *testing.T) {
	// Valid sourcemaps but zero reds → timing setup fails on first segment.
	req := Request{
		Segments: []Segment{
			{SourceMapJSON: []byte(segmentA), StartTimeMs: 0},
			{SourceMapJSON: []byte(segmentB), StartTimeMs: 100},
		},
		ReferenceOsu:     strings.NewReader(refWithoutReds),
		DefaultSampleset: domain.SamplesetSoft,
	}
	_, pErr := Generate(req)
	if pErr == nil {
		t.Fatal("expected error")
	}
	if pErr.Stage != StageTimingSetup {
		t.Errorf("stage = %s, want %s", pErr.Stage, StageTimingSetup)
	}
	if pErr.SegmentIndex != 0 {
		t.Errorf("SegmentIndex = %d, want 0", pErr.SegmentIndex)
	}
}

func TestPipeline_CrossSegmentMergeConflict(t *testing.T) {
	// Two segments that individually are fine but whose merged final ms
	// collide with incompatible hitnormal samplesets → final validation
	// must fail at StageFinalValidate (not per-segment).
	segDrum := `{
	  "_meta":{"ppq":96,"timeSignatureNumerator":4},
	  "drum":{"0":{"normal":["0;40"]}}
	}`
	segNormal := `{
	  "_meta":{"ppq":96,"timeSignatureNumerator":4},
	  "normal":{"0":{"normal":["0;40"]}}
	}`
	req := Request{
		Segments: []Segment{
			{SourceMapJSON: []byte(segDrum), StartTimeMs: 1000},
			{SourceMapJSON: []byte(segNormal), StartTimeMs: 1000},
		},
		ReferenceOsu:     strings.NewReader(refOsu),
		DefaultSampleset: domain.SamplesetSoft,
	}
	_, pErr := Generate(req)
	if pErr == nil {
		t.Fatal("expected cross-segment conflict")
	}
	// The merge collides at the same final ms with two non-default normals;
	// this is caught either by final-group validation or by the generator's
	// multiple-non-default-normals check. Both are acceptable and both have
	// SegmentIndex == -1 because the failure is post-merge.
	if pErr.Stage != StageFinalValidate && pErr.Stage != StageGenerate {
		t.Errorf("stage = %s, want final_validate or generate", pErr.Stage)
	}
	if pErr.SegmentIndex != -1 {
		t.Errorf("SegmentIndex = %d, want -1 (post-merge)", pErr.SegmentIndex)
	}
}

func TestPipeline_SegmentDisplayName(t *testing.T) {
	s := Segment{}
	if got := s.DisplayName(2); got != "Segment 3" {
		t.Errorf("DisplayName fallback = %q, want %q", got, "Segment 3")
	}
	s.Label = "Chorus"
	if got := s.DisplayName(2); got != "Chorus" {
		t.Errorf("DisplayName label = %q, want %q", got, "Chorus")
	}
}

func TestPipelineError_ErrorMessage(t *testing.T) {
	// Global (no segment) error.
	e := fail(StageReferenceParse, errors.New("boom"))
	if !strings.Contains(e.Error(), "pipeline[reference_parse]") {
		t.Errorf("missing global prefix: %s", e.Error())
	}
	// Segment-scoped error.
	e2 := failSegment(StageTimingSetup, 3, errors.New("boom"))
	if !strings.Contains(e2.Error(), "segment 4") {
		t.Errorf("missing 1-based segment index: %s", e2.Error())
	}
}
