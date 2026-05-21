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

const (
	timingPointAutoSampleSet = 0
	defaultSampleIndex       = 0
	defaultVolume            = 100
)

func ComputeState(g timing.FinalGroup) HitsoundState {
	return HitsoundState{
		SampleIndex: g.CustomIndex,
		Volume:      g.Volume,
	}
}

// GenerateTimingPoints merges reference red timing points with generated green timing points.
// Reference reds keep BPM/meter/effects, but their hitsound sample state is rewritten so stale reference custom samples do not leak into the generated diff.
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
				effective = ComputeState(sortedGroups[j])
				haveEffective = true
				j++
			}
			r = applyRedHitsoundState(r, effective, haveEffective)

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
				SampleSet:   timingPointAutoSampleSet,
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

func applyRedHitsoundState(tp domain.TimingPoint, state HitsoundState, haveState bool) domain.TimingPoint {
	tp.SampleSet = timingPointAutoSampleSet
	if !haveState {
		tp.SampleIndex = defaultSampleIndex
		tp.Volume = defaultVolume
		return tp
	}
	tp.SampleIndex = state.SampleIndex
	tp.Volume = state.Volume
	return tp
}
