package generator

import (
	"fmt"

	"osu-daws-app/internal/domain"
	"osu-daws-app/internal/timing"
)

const (
	CodeMultipleNonDefaultNormals   = "multiple_non_default_normals"
	CodeMultipleNonDefaultAdditions = "multiple_non_default_additions"
)

type Resolution struct {
	SampleSet   int
	AdditionSet int
}

// ResolveHitsound applies the default-sampleset rules to the events of a
// single FinalGroup and returns the sampleSet / additionSet the hit object
// should carry. It reports a structured validation error when:
//
//   - more than one non-default sampleset carries hitnormal on the same
//     timestamp (CodeMultipleNonDefaultNormals);
//   - no non-default sampleset carries hitnormal but more than one distinct
//     non-default addition-only sampleset exists on the same timestamp
//     (CodeMultipleNonDefaultAdditions).
//
// Rules applied on success:
//
//   - default-only timestamp            -> sampleSet = auto, additionSet = auto
//   - non-default hitnormal on set X    -> sampleSet = X
//   - no non-default hitnormal, only a
//     non-default addition on set Y     -> additionSet = Y
//   - non-default hitnormal on X plus a
//     default addition sound            -> additionSet = default (explicit,
//     to avoid osu!'s fallback of additionSet to sampleSet)
//   - additionSet equal to sampleSet    -> collapsed to auto (osu! falls back)
func ResolveHitsound(
	events []timing.ConvertedEvent,
	defaultSet domain.Sampleset,
) (Resolution, *domain.ValidationError) {
	nonDefaultNormal := make(map[domain.Sampleset]struct{})
	nonDefaultAddition := make(map[domain.Sampleset]struct{})
	hasDefaultAddition := false

	for _, e := range events {
		s := e.Source.Sampleset
		snd := e.Source.Sound
		if s == defaultSet {
			if snd != domain.SoundNormal {
				hasDefaultAddition = true
			}
			continue
		}
		if snd == domain.SoundNormal {
			nonDefaultNormal[s] = struct{}{}
		} else {
			nonDefaultAddition[s] = struct{}{}
		}
	}

	if len(nonDefaultNormal) > 1 {
		return Resolution{}, domain.NewValidationError(
			CodeMultipleNonDefaultNormals, "",
			fmt.Sprintf("%d non-default samplesets carry hitnormal on the same timestamp: %s",
				len(nonDefaultNormal), formatSet(nonDefaultNormal)),
		)
	}
	if len(nonDefaultNormal) == 0 && len(nonDefaultAddition) > 1 {
		return Resolution{}, domain.NewValidationError(
			CodeMultipleNonDefaultAdditions, "",
			fmt.Sprintf("%d distinct non-default addition-only samplesets on the same timestamp: %s",
				len(nonDefaultAddition), formatSet(nonDefaultAddition)),
		)
	}

	ss := SampleSetAuto
	for s := range nonDefaultNormal {
		ss = SamplesetToInt(s)
	}

	as := SampleSetAuto
	for s := range nonDefaultAddition {
		as = SamplesetToInt(s)
	}

	if ss != SampleSetAuto && as == SampleSetAuto && hasDefaultAddition {
		as = SamplesetToInt(defaultSet)
	}

	if as != SampleSetAuto && as == ss {
		as = SampleSetAuto
	}

	return Resolution{SampleSet: ss, AdditionSet: as}, nil
}

func formatSet(m map[domain.Sampleset]struct{}) string {
	out := ""
	for s := range m {
		if out != "" {
			out += ","
		}
		out += string(s)
	}
	return out
}
