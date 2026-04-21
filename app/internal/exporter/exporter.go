package exporter

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"osu-daws-app/internal/domain"
)

const (
	osuFileFormat         = "osu file format v14"
	DefaultDifficultyName = "osu!daw's HS"
)

// Options controls export-time behaviour.
type Options struct {
	// DifficultyName, if non-empty, overrides Metadata.Version verbatim.
	// When empty, DefaultDifficultyName is used.
	DifficultyName string
}

func (o Options) diffName() string {
	if o.DifficultyName != "" {
		return o.DifficultyName
	}
	return DefaultDifficultyName
}

var sectionKeyOrder = map[string][]string{
	"General": {
		"AudioFilename", "AudioLeadIn", "PreviewTime", "Countdown", "SampleSet",
		"StackLeniency", "Mode", "LetterboxInBreaks", "StoryFireInFront",
		"UseSkinSprites", "AlwaysShowPlayfield", "OverlayPosition", "SkinPreference",
		"EpilepsyWarning", "CountdownOffset", "SpecialStyle", "WidescreenStoryboard",
		"SamplesMatchPlaybackRate",
	},
	"Editor": {
		"Bookmarks", "DistanceSpacing", "BeatDivisor", "GridSize", "TimelineZoom",
	},
	"Metadata": {
		"Title", "TitleUnicode", "Artist", "ArtistUnicode", "Creator", "Version",
		"Source", "Tags", "BeatmapID", "BeatmapSetID",
	},
	"Difficulty": {
		"HPDrainRate", "CircleSize", "OverallDifficulty", "ApproachRate",
		"SliderMultiplier", "SliderTickRate",
	},
}

func Export(
	ref *domain.OsuMap,
	timingPoints []domain.TimingPoint,
	hitobjects []domain.HitObject,
	opts Options,
) string {
	var b bytes.Buffer
	_ = ExportTo(&b, ref, timingPoints, hitobjects, opts)
	return b.String()
}

func ExportTo(
	w io.Writer,
	ref *domain.OsuMap,
	timingPoints []domain.TimingPoint,
	hitobjects []domain.HitObject,
	opts Options,
) error {
	if ref == nil {
		return fmt.Errorf("export: reference OsuMap is nil")
	}

	general := cloneMap(ref.General)
	general["StackLeniency"] = "0"
	general["Mode"] = "2"

	metadata := cloneMap(ref.Metadata)
	metadata["Version"] = opts.diffName()
	metadata["BeatmapID"] = "0"

	writeLine := func(s string) {
		_, _ = io.WriteString(w, s)
		_, _ = io.WriteString(w, "\n")
	}

	writeLine(osuFileFormat)
	writeLine("")

	writeKeyValueSection("General", general, writeLine)
	writeKeyValueSection("Editor", ref.Editor, writeLine)
	writeKeyValueSection("Metadata", metadata, writeLine)
	writeKeyValueSection("Difficulty", ref.Difficulty, writeLine)

	writeLine("[Events]")
	for _, e := range ref.Events {
		writeLine(e)
	}
	writeLine("")

	writeLine("[TimingPoints]")
	for _, tp := range timingPoints {
		writeLine(formatTimingPoint(tp))
	}
	writeLine("")

	writeLine("[HitObjects]")
	for _, ho := range hitobjects {
		writeLine(formatHitObject(ho))
	}

	return nil
}

func writeKeyValueSection(name string, section map[string]string, writeLine func(string)) {
	writeLine("[" + name + "]")
	known := sectionKeyOrder[name]
	written := make(map[string]bool, len(section))

	for _, k := range known {
		if v, ok := section[k]; ok {
			writeLine(k + ":" + v)
			written[k] = true
		}
	}
	extra := make([]string, 0, len(section))
	for k := range section {
		if !written[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	for _, k := range extra {
		writeLine(k + ":" + section[k])
	}
	writeLine("")
}

func formatTimingPoint(tp domain.TimingPoint) string {
	uninherited := 0
	if tp.Uninherited {
		uninherited = 1
	}
	meter := tp.Meter
	if meter == 0 {
		meter = 4
	}
	return strings.Join([]string{
		strconv.Itoa(tp.Time),
		formatFloat(tp.BeatLength),
		strconv.Itoa(meter),
		strconv.Itoa(tp.SampleSet),
		strconv.Itoa(tp.SampleIndex),
		strconv.Itoa(tp.Volume),
		strconv.Itoa(uninherited),
		strconv.Itoa(tp.Effects),
	}, ",")
}

func formatHitObject(ho domain.HitObject) string {
	parts := []string{
		strconv.Itoa(ho.X),
		strconv.Itoa(ho.Y),
		strconv.Itoa(ho.Time),
		strconv.Itoa(ho.Type),
		strconv.Itoa(ho.HitSound),
	}
	if ho.ObjectParams != "" {
		parts = append(parts, ho.ObjectParams)
	}
	if ho.HitSample != "" {
		parts = append(parts, ho.HitSample)
	}
	return strings.Join(parts, ",")
}

func formatFloat(v float64) string {
	if v == float64(int64(v)) {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}

func cloneMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
