package workspace

import (
	"embed"
	"os"
)

// FLStudioTemplateID is the catalog ID of the FL Studio hitsound template.
const FLStudioTemplateID = "osu!daw hitsound template"

// FLStudioTemplateVersion tracks the on-disk layout of the FL Studio
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

// FLStudioExtra is the FL-specific payload stored inside
type FLStudioExtra struct {
	RootDir   string `json:"root_dir"`
	EntryFile string `json:"entry_file"`
}

// FLStudioProvider materializes the embedded FL Studio template into a
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

// Initialize copies the embedded asset tree into paths.Template and
// writes the generic template marker with an FLStudioExtra payload.
func (p FLStudioProvider) Initialize(paths Paths) error {
	if err := os.MkdirAll(paths.Template, 0o755); err != nil {
		return &Error{Code: ErrIO,
			Message: "cannot create template directory: " + paths.Template, Cause: err}
	}
	if err := CopyEmbedTree(flStudioAssets, flStudioAssetsRoot, paths.Template); err != nil {
		return &Error{Code: ErrIO,
			Message: "cannot copy FL Studio template assets", Cause: err}
	}
	return WriteTemplateMarker(paths, p.Descriptor(), FLStudioExtra{
		RootDir:   FLStudioRootDir,
		EntryFile: FLStudioEntryFile,
	})
}

// Self-register the provider so NewDefaultCatalog picks it up without
// edits to templates.go. Adding a new DAW is therefore a single-file
// change: drop a templates_<daw>.go alongside this one, implement
// TemplateProvider, call RegisterProvider from init().
func init() { RegisterProvider(FLStudioProvider{}) }
