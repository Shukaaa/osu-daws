package sourcemap

import (
	"sort"
	"strconv"
	"strings"

	"osu-daws-app/internal/domain"
)

const (
	CodeConflictVolume      = "conflict_volume"
	CodeConflictCustomIndex = "conflict_custom_index"
)

type TickGroup struct {
	Tick   int
	Events []domain.SourceEvent
}

func (g TickGroup) Volumes() []int {
	return distinctInts(g.Events, func(e domain.SourceEvent) int { return e.Volume })
}

func (g TickGroup) CustomIndices() []int {
	return distinctInts(g.Events, func(e domain.SourceEvent) int { return e.CustomIndex })
}

func (g TickGroup) Samplesets() []domain.Sampleset {
	seen := make(map[domain.Sampleset]struct{}, len(g.Events))
	out := make([]domain.Sampleset, 0, len(g.Events))
	for _, e := range g.Events {
		if _, ok := seen[e.Sampleset]; ok {
			continue
		}
		seen[e.Sampleset] = struct{}{}
		out = append(out, e.Sampleset)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func GroupByTick(events []domain.SourceEvent) []TickGroup {
	if len(events) == 0 {
		return nil
	}

	buckets := make(map[int][]domain.SourceEvent)
	for _, e := range events {
		buckets[e.Tick] = append(buckets[e.Tick], e)
	}

	ticks := make([]int, 0, len(buckets))
	for t := range buckets {
		ticks = append(ticks, t)
	}
	sort.Ints(ticks)

	out := make([]TickGroup, 0, len(ticks))
	for _, t := range ticks {
		out = append(out, TickGroup{Tick: t, Events: buckets[t]})
	}
	return out
}

func ValidateTickGroups(groups []TickGroup) *domain.ValidationResult {
	res := &domain.ValidationResult{}
	for _, g := range groups {
		validateGroup(g, res)
	}
	return res
}

func validateGroup(g TickGroup, res *domain.ValidationResult) {
	path := "tick[" + strconv.Itoa(g.Tick) + "]"

	if vols := g.Volumes(); len(vols) > 1 {
		res.Addf(CodeConflictVolume, path,
			"tick %d has conflicting volumes: %s", g.Tick, joinInts(vols))
	}

	if cis := g.CustomIndices(); len(cis) > 1 {
		res.Addf(CodeConflictCustomIndex, path,
			"tick %d has conflicting custom sampleset indices: %s", g.Tick, joinInts(cis))
	}
}

func distinctInts(events []domain.SourceEvent, pick func(domain.SourceEvent) int) []int {
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

func joinInts(xs []int) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = strconv.Itoa(x)
	}
	return strings.Join(parts, ",")
}
