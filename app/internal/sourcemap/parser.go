package sourcemap

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"osu-daws-app/internal/domain"
)

const (
	CodeInvalidJSON          = "invalid_json"
	CodeMissingMeta          = "missing_meta"
	CodeInvalidPPQ           = "invalid_ppq"
	CodeInvalidTimeSignature = "invalid_time_signature"
	CodeInvalidSampleset     = "invalid_sampleset"
	CodeInvalidCustomIndex   = "invalid_custom_index"
	CodeInvalidSound         = "invalid_sound"
	CodeInvalidEventFormat   = "invalid_event_format"
	CodeNegativeTick         = "negative_tick"
	CodeInvalidVolume        = "invalid_volume"
)

const metaKey = "_meta"

type rawMeta struct {
	PPQ                    *int `json:"ppq"`
	TimeSignatureNumerator *int `json:"timeSignatureNumerator"`
}

func Parse(data []byte) (*domain.SourceMap, *domain.ValidationResult) {
	res := &domain.ValidationResult{}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		res.Addf(CodeInvalidJSON, "", "invalid JSON: %v", err)
		return nil, res
	}

	meta, ok := parseMeta(top, res)
	if !ok {
		return nil, res
	}

	events := parseSamplesets(top, res)

	if !res.OK() {
		return nil, res
	}

	sort.SliceStable(events, func(i, j int) bool {
		if events[i].Tick != events[j].Tick {
			return events[i].Tick < events[j].Tick
		}
		if events[i].Sampleset != events[j].Sampleset {
			return events[i].Sampleset < events[j].Sampleset
		}
		if events[i].CustomIndex != events[j].CustomIndex {
			return events[i].CustomIndex < events[j].CustomIndex
		}
		return events[i].Sound < events[j].Sound
	})

	return &domain.SourceMap{Meta: meta, Events: events}, res
}

func parseMeta(top map[string]json.RawMessage, res *domain.ValidationResult) (domain.SourceMapMeta, bool) {
	raw, has := top[metaKey]
	if !has {
		res.Addf(CodeMissingMeta, metaKey, "_meta is missing")
		return domain.SourceMapMeta{}, false
	}

	var m rawMeta
	if err := json.Unmarshal(raw, &m); err != nil {
		res.Addf(CodeMissingMeta, metaKey, "_meta is not a valid object: %v", err)
		return domain.SourceMapMeta{}, false
	}

	ok := true
	if m.PPQ == nil {
		res.Addf(CodeInvalidPPQ, "_meta.ppq", "ppq is missing")
		ok = false
	} else if *m.PPQ <= 0 {
		res.Addf(CodeInvalidPPQ, "_meta.ppq", "ppq must be > 0, got %d", *m.PPQ)
		ok = false
	}

	if m.TimeSignatureNumerator == nil {
		res.Addf(CodeInvalidTimeSignature, "_meta.timeSignatureNumerator", "timeSignatureNumerator is missing")
		ok = false
	} else if *m.TimeSignatureNumerator <= 0 {
		res.Addf(CodeInvalidTimeSignature, "_meta.timeSignatureNumerator",
			"timeSignatureNumerator must be > 0, got %d", *m.TimeSignatureNumerator)
		ok = false
	}

	if !ok {
		return domain.SourceMapMeta{}, false
	}
	return domain.SourceMapMeta{PPQ: *m.PPQ, TimeSignatureNumerator: *m.TimeSignatureNumerator}, true
}

func parseSamplesets(top map[string]json.RawMessage, res *domain.ValidationResult) []domain.SourceEvent {
	var events []domain.SourceEvent

	keys := make([]string, 0, len(top))
	for k := range top {
		if k == metaKey {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, samplesetKey := range keys {
		sampleset, err := domain.ParseSampleset(samplesetKey)
		if err != nil {
			res.Addf(CodeInvalidSampleset, samplesetKey, "unknown sampleset %q", samplesetKey)
			continue
		}

		var customIndices map[string]map[string][]string
		if err := json.Unmarshal(top[samplesetKey], &customIndices); err != nil {
			res.Addf(CodeInvalidSampleset, samplesetKey,
				"sampleset %q is not a valid object: %v", samplesetKey, err)
			continue
		}

		events = append(events, parseCustomIndices(sampleset, customIndices, res)...)
	}

	return events
}

func parseCustomIndices(
	sampleset domain.Sampleset,
	indices map[string]map[string][]string,
	res *domain.ValidationResult,
) []domain.SourceEvent {
	var events []domain.SourceEvent

	ciKeys := make([]string, 0, len(indices))
	for k := range indices {
		ciKeys = append(ciKeys, k)
	}
	sort.Strings(ciKeys)

	for _, ciKey := range ciKeys {
		ci, err := strconv.Atoi(ciKey)
		path := sampleset.String() + "." + ciKey
		if err != nil || ci < 0 {
			res.Addf(CodeInvalidCustomIndex, path,
				"custom sampleset index must be a non-negative integer, got %q", ciKey)
			continue
		}

		sounds := indices[ciKey]
		soundKeys := make([]string, 0, len(sounds))
		for k := range sounds {
			soundKeys = append(soundKeys, k)
		}
		sort.Strings(soundKeys)

		for _, soundKey := range soundKeys {
			soundPath := path + "." + soundKey
			sound, err := domain.ParseSound(soundKey)
			if err != nil {
				res.Addf(CodeInvalidSound, soundPath, "unknown sound %q", soundKey)
				continue
			}

			for i, raw := range sounds[soundKey] {
				evPath := soundPath + "[" + strconv.Itoa(i) + "]"
				ev, ok := parseEventString(raw, evPath, res)
				if !ok {
					continue
				}
				events = append(events, domain.SourceEvent{
					Sampleset:   sampleset,
					CustomIndex: ci,
					Sound:       sound,
					Tick:        ev.tick,
					Volume:      ev.volume,
				})
			}
		}
	}

	return events
}

type parsedEvent struct {
	tick   int
	volume int
}

func parseEventString(raw, path string, res *domain.ValidationResult) (parsedEvent, bool) {
	parts := strings.Split(raw, ";")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		res.Addf(CodeInvalidEventFormat, path,
			"event %q must match \"tick;volume\"", raw)
		return parsedEvent{}, false
	}

	tick, err := strconv.Atoi(parts[0])
	if err != nil {
		res.Addf(CodeInvalidEventFormat, path,
			"tick component of %q is not an integer", raw)
		return parsedEvent{}, false
	}
	if tick < 0 {
		res.Addf(CodeNegativeTick, path, "tick must be >= 0, got %d", tick)
		return parsedEvent{}, false
	}

	vol, err := strconv.Atoi(parts[1])
	if err != nil {
		res.Addf(CodeInvalidEventFormat, path,
			"volume component of %q is not an integer", raw)
		return parsedEvent{}, false
	}
	if vol < 0 || vol > 100 {
		res.Addf(CodeInvalidVolume, path, "volume must be in [0,100], got %d", vol)
		return parsedEvent{}, false
	}

	return parsedEvent{tick: tick, volume: vol}, true
}
