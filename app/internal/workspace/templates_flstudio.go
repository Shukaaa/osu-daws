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

// FLStudioTemplateID is the catalog ID of the FL Studio hitsound template.
const FLStudioTemplateID = "osu!daw hitsound template"

// FLStudioTemplateVersion tracks the on-disk layout of the FL Studio
// template. Bump alongside changes to the shipped .flp / sample pack.
const FLStudioTemplateVersion = "1"

const (
	flStudioAssetsRoot = "flstudio_assets"
	// FLStudioRootDir is the subdirectory inside paths.Template that
	// contains the .flp and Samples/ tree. Relative paths inside the
	// .flp resolve against this directory.
	FLStudioRootDir = "osu!daw hitsound template"
	// FLStudioEntryFile is the relative path (from paths.Template) of
	// the main FL Studio project file users should open.
	FLStudioEntryFile = FLStudioRootDir + "/osu!daw hitsound template.flp"
)

// flStudioAssets bundles the FL Studio template tree into the binary.
//
//go:embed flstudio_assets
var flStudioAssets embed.FS

// FLStudioProvider copies the embedded FL Studio template into the
// workspace's template/ directory and writes a template_info.json marker.
type FLStudioProvider struct{}

func (FLStudioProvider) Descriptor() TemplateDescriptor {
	return TemplateDescriptor{
		DAW:       DAWFLStudio,
		ID:        FLStudioTemplateID,
		Label:     "FL Studio - osu!daw hitsound template",
		Version:   FLStudioTemplateVersion,
		EntryFile: FLStudioEntryFile,
	}
}

const templateInfoFileName = "template_info.json"

// TemplateInfo is the JSON marker written inside a workspace's template/
// folder after a provider has initialized it. Future migrations inspect
// this file to decide whether to re-run or upgrade assets.
type TemplateInfo struct {
	TemplateID string  `json:"template_id"`
	DAW        DAWType `json:"daw"`
	Version    string  `json:"version"`

	RootDir   string `json:"root_dir,omitempty"`
	EntryFile string `json:"entry_file,omitempty"`

	InitializedAt time.Time `json:"initialized_at"`
}

// nowTemplate is the clock used by Initialize. Tests may override it.
var nowTemplate = func() time.Time { return time.Now().UTC() }

// Initialize materializes the embedded FL Studio template inside
// paths.Template and writes the marker file. Idempotent: re-running
// refreshes asset contents and the marker and leaves unrelated files
// (user work under different names) untouched.
func (p FLStudioProvider) Initialize(paths Paths) error {
	if err := os.MkdirAll(paths.Template, 0o755); err != nil {
		return &Error{Code: ErrIO,
			Message: "cannot create template directory: " + paths.Template, Cause: err}
	}

	if err := copyEmbedTree(flStudioAssets, flStudioAssetsRoot, paths.Template); err != nil {
		return &Error{Code: ErrIO,
			Message: "cannot copy FL Studio template assets", Cause: err}
	}

	desc := p.Descriptor()
	info := TemplateInfo{
		TemplateID:    desc.ID,
		DAW:           desc.DAW,
		Version:       desc.Version,
		RootDir:       FLStudioRootDir,
		EntryFile:     FLStudioEntryFile,
		InitializedAt: nowTemplate(),
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

// copyEmbedTree walks src inside efs and mirrors its contents into dst.
// Existing files are overwritten. embed.FS uses forward slashes and they
// are translated to the host separator.
func copyEmbedTree(efs embed.FS, src, dst string) error {
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
