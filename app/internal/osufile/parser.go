package osufile

import (
	"bufio"
	"io"
	"os"
	"strconv"
	"strings"

	"osu-daws-app/internal/domain"
)

const (
	CodeInvalidOsuHeader    = "invalid_osu_header"
	CodeInvalidTimingPoint  = "invalid_timing_point"
	CodeInvalidHitObject    = "invalid_hit_object"
	CodeInvalidKeyValueLine = "invalid_key_value_line"
	CodeFileRead            = "file_read_error"
)

func ParseFile(path string) (*domain.OsuMap, *domain.ValidationResult) {
	res := &domain.ValidationResult{}
	f, err := os.Open(path)
	if err != nil {
		res.Addf(CodeFileRead, path, "cannot open file: %v", err)
		return nil, res
	}
	defer f.Close()
	return Parse(f)
}

func Parse(r io.Reader) (*domain.OsuMap, *domain.ValidationResult) {
	res := &domain.ValidationResult{}
	m := domain.NewOsuMap()

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	if !scanner.Scan() {
		res.Addf(CodeInvalidOsuHeader, "header", "file is empty")
		return nil, res
	}
	header := strings.TrimSpace(stripBOM(scanner.Text()))
	if !strings.HasPrefix(header, "osu file format") {
		res.Addf(CodeInvalidOsuHeader, "header", "missing \"osu file format\" header, got %q", header)
		return nil, res
	}

	var section string
	lineNo := 1
	for scanner.Scan() {
		lineNo++
		raw := scanner.Text()
		line := strings.TrimRight(raw, "\r")
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section = trimmed[1 : len(trimmed)-1]
			continue
		}

		switch section {
		case "General", "Editor", "Metadata", "Difficulty":
			parseKeyValue(trimmed, section, lineNo, targetMap(m, section), res)
		case "Events":
			m.Events = append(m.Events, line)
		case "TimingPoints":
			if tp, ok := parseTimingPoint(trimmed, lineNo, res); ok {
				m.TimingPoints = append(m.TimingPoints, tp)
			}
		case "HitObjects":
			if ho, ok := parseHitObject(trimmed, lineNo, res); ok {
				m.HitObjects = append(m.HitObjects, ho)
			}
		default:
		}
	}
	if err := scanner.Err(); err != nil {
		res.Addf(CodeFileRead, "", "scan error: %v", err)
		return nil, res
	}

	return m, res
}

func targetMap(m *domain.OsuMap, section string) map[string]string {
	switch section {
	case "General":
		return m.General
	case "Editor":
		return m.Editor
	case "Metadata":
		return m.Metadata
	case "Difficulty":
		return m.Difficulty
	}
	return nil
}

func parseKeyValue(line, section string, lineNo int, target map[string]string, res *domain.ValidationResult) {
	idx := strings.Index(line, ":")
	if idx <= 0 {
		res.Addf(CodeInvalidKeyValueLine, section+":"+strconv.Itoa(lineNo),
			"expected \"key:value\", got %q", line)
		return
	}
	key := strings.TrimSpace(line[:idx])
	val := strings.TrimSpace(line[idx+1:])
	target[key] = val
}

func parseTimingPoint(line string, lineNo int, res *domain.ValidationResult) (domain.TimingPoint, bool) {
	parts := strings.Split(line, ",")
	if len(parts) < 2 {
		res.Addf(CodeInvalidTimingPoint, "TimingPoints:"+strconv.Itoa(lineNo),
			"need at least time,beatLength, got %q", line)
		return domain.TimingPoint{}, false
	}

	timeVal, err := parseFloatInt(parts[0])
	if err != nil {
		res.Addf(CodeInvalidTimingPoint, "TimingPoints:"+strconv.Itoa(lineNo),
			"invalid time %q: %v", parts[0], err)
		return domain.TimingPoint{}, false
	}

	beatLen, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		res.Addf(CodeInvalidTimingPoint, "TimingPoints:"+strconv.Itoa(lineNo),
			"invalid beatLength %q: %v", parts[1], err)
		return domain.TimingPoint{}, false
	}

	tp := domain.TimingPoint{
		Time:        timeVal,
		BeatLength:  beatLen,
		Meter:       4,
		SampleSet:   0,
		SampleIndex: 0,
		Volume:      100,
		Uninherited: beatLen > 0,
		Effects:     0,
	}

	if len(parts) >= 3 {
		if v, err := strconv.Atoi(strings.TrimSpace(parts[2])); err == nil {
			tp.Meter = v
		}
	}
	if len(parts) >= 4 {
		if v, err := strconv.Atoi(strings.TrimSpace(parts[3])); err == nil {
			tp.SampleSet = v
		}
	}
	if len(parts) >= 5 {
		if v, err := strconv.Atoi(strings.TrimSpace(parts[4])); err == nil {
			tp.SampleIndex = v
		}
	}
	if len(parts) >= 6 {
		if v, err := strconv.Atoi(strings.TrimSpace(parts[5])); err == nil {
			tp.Volume = v
		}
	}
	if len(parts) >= 7 {
		if v, err := strconv.Atoi(strings.TrimSpace(parts[6])); err == nil {
			tp.Uninherited = v != 0
		}
	}
	if len(parts) >= 8 {
		if v, err := strconv.Atoi(strings.TrimSpace(parts[7])); err == nil {
			tp.Effects = v
		}
	}

	return tp, true
}

func parseHitObject(line string, lineNo int, res *domain.ValidationResult) (domain.HitObject, bool) {
	parts := strings.Split(line, ",")
	if len(parts) < 5 {
		res.Addf(CodeInvalidHitObject, "HitObjects:"+strconv.Itoa(lineNo),
			"need at least x,y,time,type,hitSound, got %q", line)
		return domain.HitObject{}, false
	}

	ints := make([]int, 5)
	for i := 0; i < 5; i++ {
		v, err := strconv.Atoi(strings.TrimSpace(parts[i]))
		if err != nil {
			res.Addf(CodeInvalidHitObject, "HitObjects:"+strconv.Itoa(lineNo),
				"field %d (%q) is not an integer", i, parts[i])
			return domain.HitObject{}, false
		}
		ints[i] = v
	}

	ho := domain.HitObject{
		X: ints[0], Y: ints[1], Time: ints[2], Type: ints[3], HitSound: ints[4],
	}

	if len(parts) > 5 {
		rest := parts[5:]
		last := rest[len(rest)-1]
		if looksLikeHitSample(last) {
			ho.ObjectParams = strings.Join(rest[:len(rest)-1], ",")
			ho.HitSample = last
		} else {
			ho.ObjectParams = strings.Join(rest, ",")
		}
	}
	return ho, true
}

func looksLikeHitSample(s string) bool {
	return strings.Count(s, ":") >= 3
}

func parseFloatInt(s string) (int, error) {
	s = strings.TrimSpace(s)
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	if f >= 0 {
		return int(f + 0.5), nil
	}
	return int(f - 0.5), nil
}

func stripBOM(s string) string {
	return strings.TrimPrefix(s, "\ufeff")
}

func RedTimingPoints(m *domain.OsuMap) []domain.TimingPoint {
	out := make([]domain.TimingPoint, 0, len(m.TimingPoints))
	for _, tp := range m.TimingPoints {
		if tp.IsRed() {
			out = append(out, tp)
		}
	}
	return out
}

func GreenTimingPoints(m *domain.OsuMap) []domain.TimingPoint {
	out := make([]domain.TimingPoint, 0, len(m.TimingPoints))
	for _, tp := range m.TimingPoints {
		if tp.IsGreen() {
			out = append(out, tp)
		}
	}
	return out
}
