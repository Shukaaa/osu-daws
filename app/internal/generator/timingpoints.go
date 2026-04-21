package generator

import (
	"sort"

	"osu-daws-app/internal/domain"
	"osu-daws-app/internal/timing"
)

type HitsoundState struct {
	SampleIndex int
	Volume      int
}

const DefaultGreenBeatLength = -100.0

const greenSampleSet = 0

func ComputeState(g timing.FinalGroup) HitsoundState {
	return HitsoundState{
		SampleIndex: g.CustomIndex,
		Volume:      g.Volume,
	}
}

// GenerateTimingPoints merges preserved red timing points with generated
// green timing points derived from the final groups.
//
// Generation rules:
//
//  1. Reds from the reference map are preserved in place (Time, BeatLength,
//     Meter, SampleSet, Effects unchanged). Their SampleIndex and Volume
//     are rewritten to match the hitsound state that is active when we
//     reach them, so they do not visually "reset" playback state.
//  2. A green line is emitted only when volume or custom sample index
//     changes between consecutive groups. Note-level sampleset switches
//     alone never emit a timing point; that information lives on the hit
//     object itself.
//  3. If a state change lands exactly on a red's timestamp, the red is
//     updated in place instead of inserting a redundant green at the same ms.
//  4. Reds preceding the first group keep their original state.
//  5. Generated greens use SampleSet = 0 (inherit) so they do not interfere
//     with the explicit per-hitobject sampleset decisions.
func GenerateTimingPoints(
	groups []timing.FinalGroup,
	reds []domain.TimingPoint,
) []domain.TimingPoint {
	sortedReds := make([]domain.TimingPoint, len(reds))
	copy(sortedReds, reds)
	sort.SliceStable(sortedReds, func(i, j int) bool { return sortedReds[i].Time < sortedReds[j].Time })

	sortedGroups := make([]timing.FinalGroup, len(groups))
	copy(sortedGroups, groups)
	sort.SliceStable(sortedGroups, func(i, j int) bool { return sortedGroups[i].TimeMs < sortedGroups[j].TimeMs })

	out := make([]domain.TimingPoint, 0, len(sortedReds)+len(sortedGroups))

	var effective HitsoundState
	haveEffective := false
	lastMeter := 4

	i, j := 0, 0
	for i < len(sortedReds) || j < len(sortedGroups) {
		redFirst := i < len(sortedReds) &&
			(j >= len(sortedGroups) || sortedReds[i].Time <= sortedGroups[j].TimeMs)

		if redFirst {
			r := sortedReds[i]
			if r.Meter > 0 {
				lastMeter = r.Meter
			}

			if j < len(sortedGroups) && sortedGroups[j].TimeMs == r.Time {
				desired := ComputeState(sortedGroups[j])
				if !haveEffective || effective != desired {
					r.SampleIndex = desired.SampleIndex
					r.Volume = desired.Volume
					effective = desired
					haveEffective = true
				} else {
					r.SampleIndex = effective.SampleIndex
					r.Volume = effective.Volume
				}
				j++
			} else if haveEffective {
				r.SampleIndex = effective.SampleIndex
				r.Volume = effective.Volume
			}

			out = append(out, r)
			i++
			continue
		}

		g := sortedGroups[j]
		desired := ComputeState(g)
		if !haveEffective || effective != desired {
			out = append(out, domain.TimingPoint{
				Time:        g.TimeMs,
				BeatLength:  DefaultGreenBeatLength,
				Meter:       lastMeter,
				SampleSet:   greenSampleSet,
				SampleIndex: desired.SampleIndex,
				Volume:      desired.Volume,
				Uninherited: false,
				Effects:     0,
			})
			effective = desired
			haveEffective = true
		}
		j++
	}

	return out
}
