// Package workspace defines the domain model and persistence for
// osu!daws workspaces. A workspace represents one beatmap project and
// lives outside the osu! Songs folder, typically under:
//
//	Documents/osu!daws/projects/<project-id>/
//	  project.odaw
//	  template/
//	  exports/
package workspace

import (
	"path/filepath"
	"time"

	"osu-daws-app/internal/domain"
)

// CurrentProjectFileVersion is the schema version written into new
// project.odaw files. Bump on backwards-incompatible changes.
const CurrentProjectFileVersion = 1

const ProjectFileName = "project.odaw"
const TemplateDirName = "template"
const ExportsDirName = "exports"

// ID uniquely identifies a workspace/project on disk. It is used as the
// workspace directory name and must be filesystem-safe. The canonical
// form is "<slug>-<shortRandom>".
type ID string

func (i ID) String() string { return string(i) }

// DAWType identifies which DAW template family a workspace uses.
type DAWType string

const (
	DAWFLStudio DAWType = "flstudio"
)

// IsValid reports whether t is a known DAW type.
func (t DAWType) IsValid() bool {
	switch t {
	case DAWFLStudio:
		return true
	}
	return false
}

// TemplateRef records which DAW template a workspace was created from.
type TemplateRef struct {
	DAW     DAWType `json:"daw"`
	ID      string  `json:"id"`
	Version string  `json:"version,omitempty"`
}

// Paths holds the absolute filesystem paths that make up a workspace's
// on-disk layout.
type Paths struct {
	Root        string
	ProjectFile string
	Template    string
	Exports     string
}

// PathsFromRoot derives the full Paths layout from a root directory.
func PathsFromRoot(root string) Paths {
	return Paths{
		Root:        root,
		ProjectFile: filepath.Join(root, ProjectFileName),
		Template:    filepath.Join(root, TemplateDirName),
		Exports:     filepath.Join(root, ExportsDirName),
	}
}

// SegmentInput is the persisted form of one SourceMap segment inside a project.
type SegmentInput struct {
	SourceMapJSON string `json:"source_map_json"`
	StartTimeText string `json:"start_time_text"`
	Label         string `json:"label,omitempty"`
}

// ProjectFile is the on-disk model of project.odaw. It is serialised as
// JSON with stable field tags so new optional fields can be added with
// `omitempty` without breaking older projects.
type ProjectFile struct {
	Version int `json:"version"`

	ID   ID     `json:"id"`
	Name string `json:"name"`

	Template TemplateRef `json:"template"`

	ReferenceOsuPath string           `json:"reference_osu_path,omitempty"`
	DefaultSampleset domain.Sampleset `json:"default_sampleset"`

	Segments []SegmentInput `json:"segments"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewProjectFile builds a fresh ProjectFile with sensible defaults and
// the current schema version.
func NewProjectFile(id ID, name string, tmpl TemplateRef, now time.Time) *ProjectFile {
	return &ProjectFile{
		Version:          CurrentProjectFileVersion,
		ID:               id,
		Name:             name,
		Template:         tmpl,
		DefaultSampleset: domain.SamplesetSoft,
		Segments:         []SegmentInput{},
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

// Workspace is the in-memory representation of a loaded project. It
// bundles the project manifest with the resolved filesystem paths.
type Workspace struct {
	Paths   Paths
	Project *ProjectFile
}

// NewWorkspace builds a Workspace value from a root directory and an
// in-memory project file. It does not touch the filesystem.
func NewWorkspace(root string, project *ProjectFile) *Workspace {
	return &Workspace{
		Paths:   PathsFromRoot(root),
		Project: project,
	}
}
