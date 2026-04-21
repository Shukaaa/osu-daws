package timing

import (
	"errors"
	"fmt"
	"sort"

	"osu-daws-app/internal/domain"
)

// Converter turns SourceMap ticks into absolute osu! milliseconds.
//
// Timing model:
//
//	The earliest tick found in the SourceMap is the "tick origin" and is
//	pinned to the user-supplied StartTime (ms). From there the converter
//	walks forward through the reference map's red timing points, switching
//	beat length whenever the running millisecond clock crosses a red TP's
//	Time. Green timing points are intentionally ignored.
//
// Assumptions:
//
//   - At least one red timing point exists.
//   - If StartTime is located before the first red TP, that first TP's
//     BeatLength is used as the active beat length. This keeps conversion
//     defined instead of failing when the user pins the first note slightly
//     before the map's first red line.
//   - PPQ comes from the SourceMap meta and is assumed > 0.
type Converter struct {
	startTime  float64
	tickOrigin int
	ppq        int
	reds       []domain.TimingPoint
}

type ConvertedEvent struct {
	Source domain.SourceEvent
	TimeMs float64
}

func NewConverter(sm *domain.SourceMap, reds []domain.TimingPoint, startTimeMs float64) (*Converter, error) {
	if sm == nil {
		return nil, errors.New("timing: SourceMap is nil")
	}
	if sm.Meta.PPQ <= 0 {
		return nil, fmt.Errorf("timing: invalid ppq %d", sm.Meta.PPQ)
	}
	if len(sm.Events) == 0 {
		return nil, errors.New("timing: SourceMap has no events")
	}

	filtered := make([]domain.TimingPoint, 0, len(reds))
	for _, tp := range reds {
		if tp.IsRed() {
			filtered = append(filtered, tp)
		}
	}
	if len(filtered) == 0 {
		return nil, errors.New("timing: no red timing points provided")
	}
	sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].Time < filtered[j].Time })
	for _, tp := range filtered {
		if tp.BeatLength <= 0 {
			return nil, fmt.Errorf("timing: red TP at %d has non-positive beatLength %g", tp.Time, tp.BeatLength)
		}
	}

	origin := sm.Events[0].Tick
	for _, e := range sm.Events {
		if e.Tick < origin {
			origin = e.Tick
		}
	}

	return &Converter{
		startTime:  startTimeMs,
		tickOrigin: origin,
		ppq:        sm.Meta.PPQ,
		reds:       filtered,
	}, nil
}

func (c *Converter) TickOrigin() int { return c.tickOrigin }

func (c *Converter) StartTime() float64 { return c.startTime }

func (c *Converter) TickToMs(tick int) float64 {
	beats := float64(tick-c.tickOrigin) / float64(c.ppq)
	return c.advanceBeats(c.startTime, beats)
}

func (c *Converter) TickToMsInt(tick int) int {
	return roundHalfAwayFromZero(c.TickToMs(tick))
}

func (c *Converter) ConvertEvents(sm *domain.SourceMap) []ConvertedEvent {
	out := make([]ConvertedEvent, len(sm.Events))
	for i, e := range sm.Events {
		out[i] = ConvertedEvent{Source: e, TimeMs: c.TickToMs(e.Tick)}
	}
	return out
}

func (c *Converter) advanceBeats(fromMs, beats float64) float64 {
	if beats == 0 {
		return fromMs
	}
	if beats > 0 {
		return c.advanceForward(fromMs, beats)
	}
	return c.advanceBackward(fromMs, -beats)
}

func (c *Converter) advanceForward(fromMs, beats float64) float64 {
	currentMs := fromMs
	idx := c.activeIndex(currentMs)
	for beats > 0 {
		bl := c.reds[idx].BeatLength
		var segmentMs float64
		if idx+1 < len(c.reds) {
			segmentMs = float64(c.reds[idx+1].Time) - currentMs
		} else {
			// Last red TP — consume all remaining beats here.
			return currentMs + beats*bl
		}
		segmentBeats := segmentMs / bl
		if beats <= segmentBeats {
			return currentMs + beats*bl
		}
		beats -= segmentBeats
		currentMs += segmentMs
		idx++
		if idx >= len(c.reds) {
			return currentMs + beats*c.reds[len(c.reds)-1].BeatLength
		}
	}
	return currentMs
}

func (c *Converter) advanceBackward(fromMs, beats float64) float64 {
	currentMs := fromMs
	idx := c.activeIndex(currentMs)
	for beats > 0 {
		bl := c.reds[idx].BeatLength
		var segmentMs float64
		if idx > 0 {
			segmentMs = currentMs - float64(c.reds[idx].Time)
		} else {
			return currentMs - beats*bl
		}
		segmentBeats := segmentMs / bl
		if beats <= segmentBeats {
			return currentMs - beats*bl
		}
		beats -= segmentBeats
		currentMs -= segmentMs
		idx--
		if idx < 0 {
			return currentMs - beats*c.reds[0].BeatLength
		}
	}
	return currentMs
}

func (c *Converter) activeIndex(ms float64) int {
	idx := 0
	for i, tp := range c.reds {
		if float64(tp.Time) <= ms {
			idx = i
		} else {
			break
		}
	}
	return idx
}

func roundHalfAwayFromZero(v float64) int {
	if v >= 0 {
		return int(v + 0.5)
	}
	return int(v - 0.5)
}
