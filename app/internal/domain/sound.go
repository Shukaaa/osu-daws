package domain

import "fmt"

type Sound string

const (
	SoundNormal  Sound = "normal"
	SoundClap    Sound = "clap"
	SoundWhistle Sound = "whistle"
	SoundFinish  Sound = "finish"
)

func (s Sound) String() string {
	return string(s)
}

func (s Sound) IsValid() bool {
	switch s {
	case SoundNormal, SoundClap, SoundWhistle, SoundFinish:
		return true
	}
	return false
}

func ParseSound(raw string) (Sound, error) {
	s := Sound(raw)
	if !s.IsValid() {
		return "", fmt.Errorf("invalid sound %q", raw)
	}
	return s, nil
}
