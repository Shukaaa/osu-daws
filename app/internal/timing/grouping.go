package timing

import (
	"sort"
	"strconv"

	"osu-daws-app/internal/domain"
	"osu-daws-app/internal/sourcemap"
)

type FinalGroup struct {
	TimeMs      int
	Volume      int
	CustomIndex int
	Samplesets  []domain.Sampleset
	Sounds      []domain.Sound
	Events      []ConvertedEvent
}

func GroupByFinalTime(events []ConvertedEvent) []FinalGroup {
	if len(events) == 0 {
		return nil
	}

	type bucket struct {
		ms     int
		events []ConvertedEvent
	}
	order := []int{}
	buckets := map[int]*bucket{}
	for _, e := range events {
		ms := roundHalfAwayFromZero(e.TimeMs)
		b, ok := buckets[ms]
		if !ok {
			b = &bucket{ms: ms}
			buckets[ms] = b
			order = append(order, ms)
		}
		b.events = append(b.events, e)
	}

	sort.Ints(order)

	out := make([]FinalGroup, 0, len(order))
	for _, ms := range order {
		b := buckets[ms]
		g := FinalGroup{
			TimeMs:      ms,
			Volume:      b.events[0].Source.Volume,
			CustomIndex: b.events[0].Source.CustomIndex,
			Events:      b.events,
		}
		seenSS := map[domain.Sampleset]struct{}{}
		seenSnd := map[domain.Sound]struct{}{}
		for _, e := range b.events {
			if _, dup := seenSS[e.Source.Sampleset]; !dup {
				seenSS[e.Source.Sampleset] = struct{}{}
				g.Samplesets = append(g.Samplesets, e.Source.Sampleset)
			}
			if _, dup := seenSnd[e.Source.Sound]; !dup {
				seenSnd[e.Source.Sound] = struct{}{}
				g.Sounds = append(g.Sounds, e.Source.Sound)
			}
		}
		out = append(out, g)
	}
	return out
}

func ValidateFinalGroups(groups []FinalGroup) *domain.ValidationResult {
	res := &domain.ValidationResult{}
	for _, g := range groups {
		validateFinalGroup(g, res)
	}
	return res
}

func validateFinalGroup(g FinalGroup, res *domain.ValidationResult) {
	path := "ms[" + strconv.Itoa(g.TimeMs) + "]"

	vols := distinctEventInts(g.Events, func(e ConvertedEvent) int { return e.Source.Volume })
	if len(vols) > 1 {
		res.Addf(sourcemap.CodeConflictVolume, path,
			"ms %d has conflicting volumes: %v", g.TimeMs, vols)
	}

	cis := distinctEventInts(g.Events, func(e ConvertedEvent) int { return e.Source.CustomIndex })
	if len(cis) > 1 {
		res.Addf(sourcemap.CodeConflictCustomIndex, path,
			"ms %d has conflicting custom sampleset indices: %v", g.TimeMs, cis)
	}
}

func distinctEventInts(events []ConvertedEvent, pick func(ConvertedEvent) int) []int {
	seen := make(map[int]struct{}, len(events))
	out := make([]int, 0, len(events))
	for _, e := range events {
		v := pick(e)
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Ints(out)
	return out
}
