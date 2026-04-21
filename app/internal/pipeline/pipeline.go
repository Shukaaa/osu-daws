package pipeline

import (
	"fmt"
	"io"
	"sort"

	"osu-daws-app/internal/domain"
	"osu-daws-app/internal/exporter"
	"osu-daws-app/internal/generator"
	"osu-daws-app/internal/osufile"
	"osu-daws-app/internal/sourcemap"
	"osu-daws-app/internal/timing"
)

type Stage string

const (
	StageNoSegments        Stage = "no_segments"
	StageSourceMapParse    Stage = "sourcemap_parse"
	StageSourceMapValidate Stage = "sourcemap_validate"
	StageReferenceParse    Stage = "reference_parse"
	StageDefaultSampleset  Stage = "default_sampleset"
	StageTimingSetup       Stage = "timing_setup"
	StageFinalValidate     Stage = "final_validate"
	StageGenerate          Stage = "generate"
)

type Segment struct {
	SourceMapJSON []byte
	StartTimeMs   float64
	Label         string
}

func (s Segment) DisplayName(index int) string {
	if s.Label != "" {
		return s.Label
	}
	return fmt.Sprintf("Segment %d", index+1)
}

type Request struct {
	Segments         []Segment
	ReferenceOsu     io.Reader
	DefaultSampleset domain.Sampleset
	ExportOptions    exporter.Options
}

type Result struct {
	OsuContent string
	SourceMaps []*domain.SourceMap
	Reference  *domain.OsuMap
	HitObjects []domain.HitObject
	TimingPts  []domain.TimingPoint
}

type Error struct {
	Stage        Stage
	SegmentIndex int
	Cause        error
	Validation   *domain.ValidationResult
}

func (e *Error) Error() string {
	prefix := fmt.Sprintf("pipeline[%s]", e.Stage)
	if e.SegmentIndex >= 0 {
		prefix = fmt.Sprintf("pipeline[%s, segment %d]", e.Stage, e.SegmentIndex+1)
	}
	if e.Validation != nil {
		return fmt.Sprintf("%s: %s", prefix, e.Validation.Error())
	}
	return fmt.Sprintf("%s: %v", prefix, e.Cause)
}

func (e *Error) Unwrap() error { return e.Cause }

func fail(stage Stage, cause error) *Error {
	return &Error{Stage: stage, SegmentIndex: -1, Cause: cause}
}

func failValidation(stage Stage, res *domain.ValidationResult) *Error {
	return &Error{Stage: stage, SegmentIndex: -1, Validation: res}
}

func failSegment(stage Stage, idx int, cause error) *Error {
	return &Error{Stage: stage, SegmentIndex: idx, Cause: cause}
}

func failSegmentValidation(stage Stage, idx int, res *domain.ValidationResult) *Error {
	return &Error{Stage: stage, SegmentIndex: idx, Validation: res}
}

// Generate runs the full hitsound-difficulty generation pipeline for one or
// more SourceMap segments.
//
// Flow:
//  1. For each segment: parse + validate the SourceMap JSON, then group its
//     events by tick and validate tick-level conflicts. Each segment is
//     validated independently.
//  2. Parse the reference .osu and extract red timing points.
//  3. Validate the default sampleset.
//  4. For each segment: build a tick->ms converter anchored at the segment's
//     StartTimeMs and convert its events independently.
//  5. Merge all converted events, sort by final timestamp, group by ms, and
//     run the merged final-level validation.
//  6. Generate hit objects and merged timing points, then export.
func Generate(req Request) (*Result, *Error) {
	if len(req.Segments) == 0 {
		return nil, fail(StageNoSegments, fmt.Errorf("at least one segment is required"))
	}

	segmentSMs := make([]*domain.SourceMap, len(req.Segments))
	for i, seg := range req.Segments {
		sm, vr := sourcemap.Parse(seg.SourceMapJSON)
		if !vr.OK() {
			return nil, failSegmentValidation(StageSourceMapParse, i, vr)
		}
		tickGroups := sourcemap.GroupByTick(sm.Events)
		if vr := sourcemap.ValidateTickGroups(tickGroups); !vr.OK() {
			return nil, failSegmentValidation(StageSourceMapValidate, i, vr)
		}
		segmentSMs[i] = sm
	}

	if req.ReferenceOsu == nil {
		return nil, fail(StageReferenceParse, fmt.Errorf("reference osu reader is nil"))
	}
	ref, vr := osufile.Parse(req.ReferenceOsu)
	if !vr.OK() {
		return nil, failValidation(StageReferenceParse, vr)
	}
	reds := osufile.RedTimingPoints(ref)

	if !req.DefaultSampleset.IsValid() {
		return nil, fail(StageDefaultSampleset, fmt.Errorf("invalid default sampleset %q", req.DefaultSampleset))
	}

	var merged []timing.ConvertedEvent
	for i, seg := range req.Segments {
		sm := segmentSMs[i]
		conv, err := timing.NewConverter(sm, reds, seg.StartTimeMs)
		if err != nil {
			return nil, failSegment(StageTimingSetup, i, err)
		}
		merged = append(merged, conv.ConvertEvents(sm)...)
	}

	sort.SliceStable(merged, func(i, j int) bool { return merged[i].TimeMs < merged[j].TimeMs })

	finalGroups := timing.GroupByFinalTime(merged)
	if vr := timing.ValidateFinalGroups(finalGroups); !vr.OK() {
		return nil, failValidation(StageFinalValidate, vr)
	}

	hitobjects, gvr := generator.GenerateHitObjects(finalGroups, req.DefaultSampleset)
	if gvr != nil {
		return nil, failValidation(StageGenerate, gvr)
	}
	timingPoints := generator.GenerateTimingPoints(finalGroups, reds)

	out := exporter.Export(ref, timingPoints, hitobjects, req.ExportOptions)

	return &Result{
		OsuContent: out,
		SourceMaps: segmentSMs,
		Reference:  ref,
		HitObjects: hitobjects,
		TimingPts:  timingPoints,
	}, nil
}
