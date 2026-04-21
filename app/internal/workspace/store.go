package workspace

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// now is the clock used by Save. Tests may override it.
var now = func() time.Time { return time.Now().UTC() }

// Scaffold creates the workspace directory layout under root. Idempotent.
// Does not write the project file; call SaveProjectFile for that.
func Scaffold(root string) (Paths, error) {
	paths := PathsFromRoot(root)
	for _, dir := range []string{paths.Root, paths.Template, paths.Exports} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return paths, &Error{Code: ErrIO,
				Message: "cannot create directory: " + dir, Cause: err}
		}
	}
	return paths, nil
}

// EnsureExports makes sure the exports directory exists.
func EnsureExports(paths Paths) error {
	if err := os.MkdirAll(paths.Exports, 0o755); err != nil {
		return &Error{Code: ErrIO,
			Message: "cannot create exports directory: " + paths.Exports, Cause: err}
	}
	return nil
}

// SaveProjectFile writes pf to <paths.ProjectFile> as JSON. The write is
// atomic (temp file + rename). UpdatedAt is refreshed; CreatedAt is
// filled in if missing.
func SaveProjectFile(paths Paths, pf *ProjectFile) error {
	if pf == nil {
		return &Error{Code: ErrProjectFileIncomplete,
			Message: "project file is nil"}
	}
	t := now()
	if pf.CreatedAt.IsZero() {
		pf.CreatedAt = t
	}
	pf.UpdatedAt = t
	if pf.Version == 0 {
		pf.Version = CurrentProjectFileVersion
	}
	if pf.Segments == nil {
		pf.Segments = []SegmentInput{}
	}

	data, err := json.MarshalIndent(pf, "", "  ")
	if err != nil {
		return &Error{Code: ErrProjectFileInvalid,
			Message: "cannot marshal project file", Cause: err}
	}

	if err := os.MkdirAll(filepath.Dir(paths.ProjectFile), 0o755); err != nil {
		return &Error{Code: ErrIO,
			Message: "cannot create workspace directory", Cause: err}
	}

	tmp := paths.ProjectFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return &Error{Code: ErrIO,
			Message: "cannot write temp project file: " + tmp, Cause: err}
	}
	if err := os.Rename(tmp, paths.ProjectFile); err != nil {
		_ = os.Remove(tmp)
		return &Error{Code: ErrIO,
			Message: "cannot rename temp project file into place", Cause: err}
	}
	return nil
}

// LoadProjectFile reads and parses project.odaw. Structured errors
// distinguish missing / unreadable / malformed / unsupported-version /
// incomplete files.
func LoadProjectFile(paths Paths) (*ProjectFile, error) {
	data, err := os.ReadFile(paths.ProjectFile)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, &Error{Code: ErrProjectFileMissing,
				Message: "project file not found: " + paths.ProjectFile, Cause: err}
		}
		return nil, &Error{Code: ErrIO,
			Message: "cannot read project file: " + paths.ProjectFile, Cause: err}
	}

	var pf ProjectFile
	if err := json.Unmarshal(data, &pf); err != nil {
		return nil, &Error{Code: ErrProjectFileInvalid,
			Message: "project file is not valid JSON: " + paths.ProjectFile, Cause: err}
	}

	if pf.Version <= 0 || pf.Version > CurrentProjectFileVersion {
		return nil, &Error{Code: ErrProjectFileVersion,
			Message: "unsupported project file version"}
	}
	if strings.TrimSpace(string(pf.ID)) == "" {
		return nil, &Error{Code: ErrProjectFileIncomplete,
			Message: "project file is missing required field: id"}
	}
	if strings.TrimSpace(pf.Name) == "" {
		return nil, &Error{Code: ErrProjectFileIncomplete,
			Message: "project file is missing required field: name"}
	}
	if pf.Segments == nil {
		pf.Segments = []SegmentInput{}
	}
	return &pf, nil
}

// CreateWorkspace scaffolds a new workspace directory and writes its
// initial project.odaw. It refuses to overwrite a non-empty existing
// directory.
func CreateWorkspace(projectsRoot string, pf *ProjectFile) (*Workspace, error) {
	if pf == nil || strings.TrimSpace(string(pf.ID)) == "" {
		return nil, &Error{Code: ErrProjectFileIncomplete,
			Message: "project file must have an ID"}
	}
	root := WorkspaceRoot(projectsRoot, pf.ID)

	if entries, err := os.ReadDir(root); err == nil && len(entries) > 0 {
		return nil, &Error{Code: ErrWorkspaceAlreadyExists,
			Message: "workspace directory already exists and is not empty: " + root}
	}

	paths, err := Scaffold(root)
	if err != nil {
		return nil, err
	}
	if err := SaveProjectFile(paths, pf); err != nil {
		return nil, err
	}
	return &Workspace{Paths: paths, Project: pf}, nil
}

// LoadWorkspace resolves and reads a workspace by its root directory.
func LoadWorkspace(root string) (*Workspace, error) {
	paths := PathsFromRoot(root)
	pf, err := LoadProjectFile(paths)
	if err != nil {
		return nil, err
	}
	return &Workspace{Paths: paths, Project: pf}, nil
}

// LoadWorkspaceFromProjectFile loads a workspace given a direct path to
// its project.odaw manifest. The workspace's Paths is derived from the
// parent directory of the given file.
func LoadWorkspaceFromProjectFile(path string) (*Workspace, error) {
	if path == "" {
		return nil, &Error{Code: ErrProjectFileIncomplete,
			Message: "project file path is empty"}
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, &Error{Code: ErrProjectFileIncomplete,
			Message: "cannot resolve project file path: " + err.Error()}
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, &Error{Code: ErrProjectFileIncomplete,
			Message: "cannot open project file: " + err.Error()}
	}
	if info.IsDir() {
		return nil, &Error{Code: ErrProjectFileIncomplete,
			Message: "project file path is a directory: " + abs}
	}
	if filepath.Base(abs) != ProjectFileName {
		return nil, &Error{Code: ErrProjectFileIncomplete,
			Message: "file is not a " + ProjectFileName + " manifest: " + abs}
	}
	return LoadWorkspace(filepath.Dir(abs))
}

// Save persists the workspace's project file to disk.
func (w *Workspace) Save() error {
	if w == nil || w.Project == nil {
		return &Error{Code: ErrProjectFileIncomplete,
			Message: "workspace or project file is nil"}
	}
	return SaveProjectFile(w.Paths, w.Project)
}
