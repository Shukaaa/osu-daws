package sourcemap

import (
	"testing"

	"osu-daws-app/internal/domain"
)

const validJSON = `{
  "_meta":{"ppq":96,"timeSignatureNumerator":4},
  "drum":{"0":{"normal":["0;40","96;40"]}},
  "soft":{"5":{"clap":["192;60"]}}
}`

func TestParseValid(t *testing.T) {
	sm, res := Parse([]byte(validJSON))
	if !res.OK() {
		t.Fatalf("expected OK, got errors: %s", res.Error())
	}
	if sm == nil {
		t.Fatal("expected non-nil SourceMap")
	}
	if sm.Meta.PPQ != 96 || sm.Meta.TimeSignatureNumerator != 4 {
		t.Errorf("unexpected meta: %+v", sm.Meta)
	}
	if len(sm.Events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(sm.Events))
	}

	want := []domain.SourceEvent{
		{Sampleset: domain.SamplesetDrum, CustomIndex: 0, Sound: domain.SoundNormal, Tick: 0, Volume: 40},
		{Sampleset: domain.SamplesetDrum, CustomIndex: 0, Sound: domain.SoundNormal, Tick: 96, Volume: 40},
		{Sampleset: domain.SamplesetSoft, CustomIndex: 5, Sound: domain.SoundClap, Tick: 192, Volume: 60},
	}
	for i, e := range want {
		if sm.Events[i] != e {
			t.Errorf("event[%d] = %+v, want %+v", i, sm.Events[i], e)
		}
	}
}

func TestParseAllSoundsAndSamplesets(t *testing.T) {
	j := `{
	  "_meta":{"ppq":48,"timeSignatureNumerator":3},
	  "drum":{"0":{"normal":["0;100"],"clap":["10;50"],"whistle":["20;25"],"finish":["30;0"]}},
	  "soft":{"1":{"normal":["40;1"]}},
	  "normal":{"2":{"finish":["50;99"]}}
	}`
	sm, res := Parse([]byte(j))
	if !res.OK() {
		t.Fatalf("unexpected errors: %s", res.Error())
	}
	if len(sm.Events) != 6 {
		t.Fatalf("expected 6 events, got %d", len(sm.Events))
	}
}

func hasCode(res *domain.ValidationResult, code string) bool {
	for _, e := range res.Errors {
		if e.Code == code {
			return true
		}
	}
	return false
}

func TestParseInvalid(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		wantCode string
	}{
		{
			name:     "invalid json",
			input:    `{not json`,
			wantCode: CodeInvalidJSON,
		},
		{
			name:     "missing meta",
			input:    `{"drum":{"0":{"normal":["0;40"]}}}`,
			wantCode: CodeMissingMeta,
		},
		{
			name:     "missing ppq",
			input:    `{"_meta":{"timeSignatureNumerator":4}}`,
			wantCode: CodeInvalidPPQ,
		},
		{
			name:     "zero ppq",
			input:    `{"_meta":{"ppq":0,"timeSignatureNumerator":4}}`,
			wantCode: CodeInvalidPPQ,
		},
		{
			name:     "missing time signature",
			input:    `{"_meta":{"ppq":96}}`,
			wantCode: CodeInvalidTimeSignature,
		},
		{
			name:     "negative time signature",
			input:    `{"_meta":{"ppq":96,"timeSignatureNumerator":-1}}`,
			wantCode: CodeInvalidTimeSignature,
		},
		{
			name:     "invalid sampleset",
			input:    `{"_meta":{"ppq":96,"timeSignatureNumerator":4},"bass":{"0":{"normal":["0;40"]}}}`,
			wantCode: CodeInvalidSampleset,
		},
		{
			name:     "invalid sound",
			input:    `{"_meta":{"ppq":96,"timeSignatureNumerator":4},"drum":{"0":{"slide":["0;40"]}}}`,
			wantCode: CodeInvalidSound,
		},
		{
			name:     "invalid custom index non-numeric",
			input:    `{"_meta":{"ppq":96,"timeSignatureNumerator":4},"drum":{"abc":{"normal":["0;40"]}}}`,
			wantCode: CodeInvalidCustomIndex,
		},
		{
			name:     "invalid custom index negative",
			input:    `{"_meta":{"ppq":96,"timeSignatureNumerator":4},"drum":{"-1":{"normal":["0;40"]}}}`,
			wantCode: CodeInvalidCustomIndex,
		},
		{
			name:     "invalid event format no semicolon",
			input:    `{"_meta":{"ppq":96,"timeSignatureNumerator":4},"drum":{"0":{"normal":["040"]}}}`,
			wantCode: CodeInvalidEventFormat,
		},
		{
			name:     "invalid event format too many parts",
			input:    `{"_meta":{"ppq":96,"timeSignatureNumerator":4},"drum":{"0":{"normal":["0;40;1"]}}}`,
			wantCode: CodeInvalidEventFormat,
		},
		{
			name:     "invalid event format empty tick",
			input:    `{"_meta":{"ppq":96,"timeSignatureNumerator":4},"drum":{"0":{"normal":[";40"]}}}`,
			wantCode: CodeInvalidEventFormat,
		},
		{
			name:     "invalid event format non-integer tick",
			input:    `{"_meta":{"ppq":96,"timeSignatureNumerator":4},"drum":{"0":{"normal":["a;40"]}}}`,
			wantCode: CodeInvalidEventFormat,
		},
		{
			name:     "invalid event format non-integer volume",
			input:    `{"_meta":{"ppq":96,"timeSignatureNumerator":4},"drum":{"0":{"normal":["0;x"]}}}`,
			wantCode: CodeInvalidEventFormat,
		},
		{
			name:     "negative tick",
			input:    `{"_meta":{"ppq":96,"timeSignatureNumerator":4},"drum":{"0":{"normal":["-5;40"]}}}`,
			wantCode: CodeNegativeTick,
		},
		{
			name:     "volume above 100",
			input:    `{"_meta":{"ppq":96,"timeSignatureNumerator":4},"drum":{"0":{"normal":["0;101"]}}}`,
			wantCode: CodeInvalidVolume,
		},
		{
			name:     "volume below 0",
			input:    `{"_meta":{"ppq":96,"timeSignatureNumerator":4},"drum":{"0":{"normal":["0;-1"]}}}`,
			wantCode: CodeInvalidVolume,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sm, res := Parse([]byte(c.input))
			if res.OK() {
				t.Fatalf("expected failure, got OK with sm=%+v", sm)
			}
			if sm != nil {
				t.Errorf("expected nil SourceMap on failure, got %+v", sm)
			}
			if !hasCode(res, c.wantCode) {
				t.Errorf("expected error code %q, got: %s", c.wantCode, res.Error())
			}
		})
	}
}

func TestParseCollectsMultipleErrors(t *testing.T) {
	input := `{
	  "_meta":{"ppq":96,"timeSignatureNumerator":4},
	  "drum":{"0":{"normal":["-1;40","0;200"]}}
	}`
	_, res := Parse([]byte(input))
	if res.OK() {
		t.Fatal("expected failure")
	}
	if !hasCode(res, CodeNegativeTick) {
		t.Errorf("expected negative_tick in errors: %s", res.Error())
	}
	if !hasCode(res, CodeInvalidVolume) {
		t.Errorf("expected invalid_volume in errors: %s", res.Error())
	}
}
