package generator

import (
	"fmt"
	"strconv"
	"strings"

	"osu-daws-app/internal/domain"
	"osu-daws-app/internal/timing"
)

const (
	DefaultX      = 256
	DefaultY      = 192
	HitObjectType = 1
)

const (
	HitSoundNormal  = 1 << 0
	HitSoundWhistle = 1 << 1
	HitSoundFinish  = 1 << 2
	HitSoundClap    = 1 << 3
)

const (
	SampleSetAuto   = 0
	SampleSetNormal = 1
	SampleSetSoft   = 2
	SampleSetDrum   = 3
)

func SamplesetToInt(s domain.Sampleset) int {
	switch s {
	case domain.SamplesetNormal:
		return SampleSetNormal
	case domain.SamplesetSoft:
		return SampleSetSoft
	case domain.SamplesetDrum:
		return SampleSetDrum
	}
	return SampleSetAuto
}

func SoundsToBitmask(sounds []domain.Sound) int {
	mask := 0
	for _, s := range sounds {
		switch s {
		case domain.SoundNormal:
			mask |= HitSoundNormal
		case domain.SoundWhistle:
			mask |= HitSoundWhistle
		case domain.SoundFinish:
			mask |= HitSoundFinish
		case domain.SoundClap:
			mask |= HitSoundClap
		}
	}
	return mask
}

func GenerateHitObject(
	g timing.FinalGroup,
	defaultSet domain.Sampleset,
) (domain.HitObject, *domain.ValidationError) {
	res, verr := ResolveHitsound(g.Events, defaultSet)
	if verr != nil {
		verr.Field = fmt.Sprintf("ms[%d]", g.TimeMs)
		return domain.HitObject{}, verr
	}

	return domain.HitObject{
		X:            DefaultX,
		Y:            DefaultY,
		Time:         g.TimeMs,
		Type:         HitObjectType,
		HitSound:     SoundsToBitmask(g.Sounds),
		ObjectParams: "",
		HitSample:    formatHitSample(res.SampleSet, res.AdditionSet, g.CustomIndex, 0),
	}, nil
}

func GenerateHitObjects(
	groups []timing.FinalGroup,
	defaultSet domain.Sampleset,
) ([]domain.HitObject, *domain.ValidationResult) {
	result := &domain.ValidationResult{}
	out := make([]domain.HitObject, 0, len(groups))
	for _, g := range groups {
		ho, verr := GenerateHitObject(g, defaultSet)
		if verr != nil {
			result.Add(verr)
			continue
		}
		out = append(out, ho)
	}
	if !result.OK() {
		return nil, result
	}
	return out, nil
}

func formatHitSample(sampleSet, additionSet, index, volume int) string {
	var b strings.Builder
	b.WriteString(strconv.Itoa(sampleSet))
	b.WriteByte(':')
	b.WriteString(strconv.Itoa(additionSet))
	b.WriteByte(':')
	b.WriteString(strconv.Itoa(index))
	b.WriteByte(':')
	b.WriteString(strconv.Itoa(volume))
	b.WriteByte(':')
	return b.String()
}
