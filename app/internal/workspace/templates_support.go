package workspace

import (
	"embed"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const templateInfoFileName = "template_info.json"

type TemplateInfo struct {
	TemplateID    string    `json:"template_id"`
	DAW           DAWType   `json:"daw"`
	Version       string    `json:"version"`
	InitializedAt time.Time `json:"initialized_at"`

	// Extra is the provider-defined payload. Unmarshal into the
	// provider's companion struct (e.g. FLStudioExtra) to read it.
	Extra json.RawMessage `json:"extra,omitempty"`
}

func (t TemplateInfo) DecodeExtra(v any) error {
	if len(t.Extra) == 0 {
		return nil
	}
	return json.Unmarshal(t.Extra, v)
}

var nowTemplate = func() time.Time { return time.Now().UTC() }

func WriteTemplateMarker(paths Paths, desc TemplateDescriptor, extra any) error {
	var raw json.RawMessage
	if extra != nil {
		b, err := json.Marshal(extra)
		if err != nil {
			return &Error{Code: ErrIO,
				Message: "cannot marshal template extra payload", Cause: err}
		}
		raw = b
	}
	info := TemplateInfo{
		TemplateID:    desc.ID,
		DAW:           desc.DAW,
		Version:       desc.Version,
		InitializedAt: nowTemplate(),
		Extra:         raw,
	}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return &Error{Code: ErrIO,
			Message: "cannot marshal template info", Cause: err}
	}
	target := filepath.Join(paths.Template, templateInfoFileName)
	if err := os.WriteFile(target, data, 0o644); err != nil {
		return &Error{Code: ErrIO,
			Message: "cannot write template marker: " + target, Cause: err}
	}
	return nil
}

func ReadTemplateMarker(paths Paths) (TemplateInfo, bool, error) {
	target := filepath.Join(paths.Template, templateInfoFileName)
	data, err := os.ReadFile(target)
	if err != nil {
		if os.IsNotExist(err) {
			return TemplateInfo{}, false, nil
		}
		return TemplateInfo{}, false, err
	}
	var info TemplateInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return TemplateInfo{}, true, err
	}
	return info, true, nil
}

func CopyEmbedTree(efs embed.FS, src, dst string) error {
	return fs.WalkDir(efs, src, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel := strings.TrimPrefix(p, src)
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" {
			return nil
		}
		target := filepath.Join(dst, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		data, err := efs.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
