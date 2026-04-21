package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"osu-daws-app/internal/domain"
)

// stubHome returns a HomeDirFunc pointing at a temp dir.
func stubHome(dir string) HomeDirFunc {
	return func() (string, error) { return dir, nil }
}

func TestProjectsRoot(t *testing.T) {
	home := t.TempDir()
	got, err := ProjectsRoot(stubHome(home))
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "Documents", "osu!daws", "projects")
	if got != want {
		t.Errorf("ProjectsRoot = %q, want %q", got, want)
	}
	// ProjectsRoot itself must not touch the filesystem.
	if _, err := os.Stat(got); !os.IsNotExist(err) {
		t.Errorf("ProjectsRoot must not create the directory, stat err = %v", err)
	}
}

func TestProjectsRoot_HomeUnavailable(t *testing.T) {
	bad := func() (string, error) { return "", os.ErrPermission }
	_, err := ProjectsRoot(bad)
	if err == nil {
		t.Fatal("expected error")
	}
	var werr *Error
	if !errorsAs(err, &werr) || werr.Code != ErrHomeDirUnavailable {
		t.Errorf("got %v, want ErrHomeDirUnavailable", err)
	}
}

func TestEnsureProjectsRoot_Creates(t *testing.T) {
	home := t.TempDir()
	root, err := EnsureProjectsRoot(stubHome(home))
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Errorf("expected directory at %q", root)
	}
}

func TestWorkspaceRoot(t *testing.T) {
	root := filepath.Join("foo", "projects")
	got := WorkspaceRoot(root, ID("my-song-abc123"))
	want := filepath.Join(root, "my-song-abc123")
	if got != want {
		t.Errorf("WorkspaceRoot = %q, want %q", got, want)
	}
}

func TestScaffold_CreatesLayout(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ws")
	paths, err := Scaffold(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{paths.Root, paths.Template, paths.Exports} {
		info, err := os.Stat(d)
		if err != nil {
			t.Errorf("missing directory %q: %v", d, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%q is not a directory", d)
		}
	}
}

func TestScaffold_Idempotent(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ws")
	if _, err := Scaffold(root); err != nil {
		t.Fatal(err)
	}
	// Put a sentinel inside exports/ and re-scaffold; it must survive.
	sentinel := filepath.Join(root, ExportsDirName, "keep.txt")
	if err := os.WriteFile(sentinel, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Scaffold(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Errorf("sentinel disappeared: %v", err)
	}
}

func TestEnsureExports(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ws")
	paths := PathsFromRoot(root)
	if err := EnsureExports(paths); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(paths.Exports)
	if err != nil || !info.IsDir() {
		t.Errorf("exports dir not created: info=%v err=%v", info, err)
	}
}

func TestSaveAndLoadProjectFile_Roundtrip(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ws")
	paths, err := Scaffold(root)
	if err != nil {
		t.Fatal(err)
	}
	pf := NewProjectFile(
		ID("my-song-abc123"),
		"My Song",
		TemplateRef{DAW: DAWFLStudio, ID: "osu!daw hitsound template", Version: "1"},
		time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC),
	)
	pf.ReferenceOsuPath = filepath.Join("C:", "maps", "song.osu")
	pf.Segments = []SegmentInput{
		{SourceMapJSON: `{"_meta":{"ppq":96}}`, StartTimeText: "1234", Label: "Chorus"},
	}

	if err := SaveProjectFile(paths, pf); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadProjectFile(paths)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if loaded.ID != pf.ID {
		t.Errorf("ID = %q, want %q", loaded.ID, pf.ID)
	}
	if loaded.Name != pf.Name {
		t.Errorf("Name = %q, want %q", loaded.Name, pf.Name)
	}
	if loaded.Template != pf.Template {
		t.Errorf("Template = %+v, want %+v", loaded.Template, pf.Template)
	}
	if loaded.ReferenceOsuPath != pf.ReferenceOsuPath {
		t.Errorf("ReferenceOsuPath = %q, want %q", loaded.ReferenceOsuPath, pf.ReferenceOsuPath)
	}
	if loaded.DefaultSampleset != domain.SamplesetSoft {
		t.Errorf("DefaultSampleset = %q", loaded.DefaultSampleset)
	}
	if len(loaded.Segments) != 1 || loaded.Segments[0].Label != "Chorus" {
		t.Errorf("Segments = %+v", loaded.Segments)
	}
	if loaded.Version != CurrentProjectFileVersion {
		t.Errorf("Version = %d", loaded.Version)
	}
	if loaded.CreatedAt.IsZero() || loaded.UpdatedAt.IsZero() {
		t.Errorf("timestamps not persisted: created=%v updated=%v", loaded.CreatedAt, loaded.UpdatedAt)
	}
}

func TestSaveProjectFile_UpdatesTimestampsAndAtomic(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ws")
	paths, _ := Scaffold(root)
	pf := NewProjectFile(ID("a-1"), "A", TemplateRef{DAW: DAWFLStudio, ID: "t"}, time.Time{})

	if err := SaveProjectFile(paths, pf); err != nil {
		t.Fatal(err)
	}
	if pf.CreatedAt.IsZero() {
		t.Error("CreatedAt should be filled by Save when zero")
	}
	firstUpdate := pf.UpdatedAt

	// Second save: CreatedAt preserved, UpdatedAt refreshed.
	time.Sleep(2 * time.Millisecond)
	if err := SaveProjectFile(paths, pf); err != nil {
		t.Fatal(err)
	}
	if !pf.UpdatedAt.After(firstUpdate) {
		t.Errorf("UpdatedAt not refreshed: first=%v second=%v", firstUpdate, pf.UpdatedAt)
	}

	// No stray .tmp files left behind.
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("leftover tmp file: %s", e.Name())
		}
	}
}

func TestLoadProjectFile_Missing(t *testing.T) {
	paths := PathsFromRoot(filepath.Join(t.TempDir(), "nope"))
	_, err := LoadProjectFile(paths)
	if err == nil {
		t.Fatal("expected error")
	}
	var werr *Error
	if !errorsAs(err, &werr) || werr.Code != ErrProjectFileMissing {
		t.Errorf("got %v, want ErrProjectFileMissing", err)
	}
}

func TestLoadProjectFile_InvalidJSON(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ws")
	paths, _ := Scaffold(root)
	if err := os.WriteFile(paths.ProjectFile, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadProjectFile(paths)
	var werr *Error
	if !errorsAs(err, &werr) || werr.Code != ErrProjectFileInvalid {
		t.Errorf("got %v, want ErrProjectFileInvalid", err)
	}
}

func TestLoadProjectFile_BadVersion(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ws")
	paths, _ := Scaffold(root)
	// Version 999 = from the future.
	raw := map[string]any{
		"version":           999,
		"id":                "x-1",
		"name":              "X",
		"template":          map[string]string{"daw": "flstudio", "id": "t"},
		"default_sampleset": "Soft",
		"segments":          []any{},
		"created_at":        time.Now().UTC(),
		"updated_at":        time.Now().UTC(),
	}
	data, _ := json.Marshal(raw)
	_ = os.WriteFile(paths.ProjectFile, data, 0o644)

	_, err := LoadProjectFile(paths)
	var werr *Error
	if !errorsAs(err, &werr) || werr.Code != ErrProjectFileVersion {
		t.Errorf("got %v, want ErrProjectFileVersion", err)
	}
}

func TestLoadProjectFile_Incomplete(t *testing.T) {
	cases := []struct {
		name string
		raw  map[string]any
	}{
		{"missing id", map[string]any{"version": 1, "name": "X", "default_sampleset": "Soft"}},
		{"blank id", map[string]any{"version": 1, "id": "   ", "name": "X", "default_sampleset": "Soft"}},
		{"missing name", map[string]any{"version": 1, "id": "a-1", "default_sampleset": "Soft"}},
		{"blank name", map[string]any{"version": 1, "id": "a-1", "name": "   ", "default_sampleset": "Soft"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "ws")
			paths, _ := Scaffold(root)
			data, _ := json.Marshal(c.raw)
			_ = os.WriteFile(paths.ProjectFile, data, 0o644)
			_, err := LoadProjectFile(paths)
			var werr *Error
			if !errorsAs(err, &werr) || werr.Code != ErrProjectFileIncomplete {
				t.Errorf("got %v, want ErrProjectFileIncomplete", err)
			}
		})
	}
}

func TestCreateWorkspace(t *testing.T) {
	projectsRoot := filepath.Join(t.TempDir(), "projects")
	pf := NewProjectFile(ID("my-song-abc123"), "My Song",
		TemplateRef{DAW: DAWFLStudio, ID: "t"}, time.Now().UTC())

	ws, err := CreateWorkspace(projectsRoot, pf)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Paths populated.
	if ws.Paths.Root == "" || ws.Paths.ProjectFile == "" {
		t.Error("paths not populated")
	}
	// project.odaw exists.
	if _, err := os.Stat(ws.Paths.ProjectFile); err != nil {
		t.Errorf("project file not written: %v", err)
	}
	// template/ and exports/ exist.
	for _, d := range []string{ws.Paths.Template, ws.Paths.Exports} {
		if info, err := os.Stat(d); err != nil || !info.IsDir() {
			t.Errorf("dir missing: %s err=%v", d, err)
		}
	}
}

func TestCreateWorkspace_RejectsExisting(t *testing.T) {
	projectsRoot := filepath.Join(t.TempDir(), "projects")
	id := ID("busy-1")
	root := WorkspaceRoot(projectsRoot, id)
	// Pre-populate with a file to make the dir non-empty.
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "occupant.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	pf := NewProjectFile(id, "X", TemplateRef{DAW: DAWFLStudio, ID: "t"}, time.Now().UTC())
	_, err := CreateWorkspace(projectsRoot, pf)
	var werr *Error
	if !errorsAs(err, &werr) || werr.Code != ErrWorkspaceAlreadyExists {
		t.Errorf("got %v, want ErrWorkspaceAlreadyExists", err)
	}
}

func TestCreateWorkspace_AcceptsEmptyExistingDir(t *testing.T) {
	projectsRoot := filepath.Join(t.TempDir(), "projects")
	id := ID("empty-1")
	root := WorkspaceRoot(projectsRoot, id)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	pf := NewProjectFile(id, "X", TemplateRef{DAW: DAWFLStudio, ID: "t"}, time.Now().UTC())
	if _, err := CreateWorkspace(projectsRoot, pf); err != nil {
		t.Errorf("empty existing dir should be accepted: %v", err)
	}
}

func TestLoadWorkspace_Roundtrip(t *testing.T) {
	projectsRoot := filepath.Join(t.TempDir(), "projects")
	pf := NewProjectFile(ID("rt-1"), "RT", TemplateRef{DAW: DAWFLStudio, ID: "t"}, time.Now().UTC())
	ws, err := CreateWorkspace(projectsRoot, pf)
	if err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadWorkspace(ws.Paths.Root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Project.ID != pf.ID {
		t.Errorf("ID = %q, want %q", loaded.Project.ID, pf.ID)
	}
	if loaded.Paths.Root != ws.Paths.Root {
		t.Errorf("Paths.Root = %q, want %q", loaded.Paths.Root, ws.Paths.Root)
	}
}

func TestWorkspace_Save(t *testing.T) {
	projectsRoot := filepath.Join(t.TempDir(), "projects")
	pf := NewProjectFile(ID("s-1"), "Save", TemplateRef{DAW: DAWFLStudio, ID: "t"}, time.Now().UTC())
	ws, err := CreateWorkspace(projectsRoot, pf)
	if err != nil {
		t.Fatal(err)
	}
	ws.Project.Name = "Renamed"
	if err := ws.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	reloaded, err := LoadWorkspace(ws.Paths.Root)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Project.Name != "Renamed" {
		t.Errorf("Name = %q, want Renamed", reloaded.Project.Name)
	}
}

// errorsAs is a local shim so we don't need to import "errors" just for As.
func errorsAs(err error, target **Error) bool {
	for err != nil {
		if e, ok := err.(*Error); ok {
			*target = e
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
