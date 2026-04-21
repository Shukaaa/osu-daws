package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeProject is a tiny helper that creates a workspace directory at
// projectsRoot/<id> and writes an arbitrary JSON payload to its
// project.odaw file.
func writeProject(t *testing.T, projectsRoot string, id string, raw map[string]any) string {
	t.Helper()
	root := filepath.Join(projectsRoot, id)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ProjectFileName), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// validRaw returns a minimally valid project.odaw payload.
func validRaw(id, name string, updated time.Time) map[string]any {
	return map[string]any{
		"version":            CurrentProjectFileVersion,
		"id":                 id,
		"name":               name,
		"template":           map[string]string{"daw": string(DAWFLStudio), "id": "osu!daw hitsound template"},
		"default_sampleset":  "Soft",
		"segments":           []any{},
		"reference_osu_path": `C:\maps\ref.osu`,
		"created_at":         updated.Add(-time.Hour),
		"updated_at":         updated,
	}
}

func TestListWorkspaces_MissingRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "does", "not", "exist")
	res, err := ListWorkspaces(root)
	if err != nil {
		t.Fatalf("missing root should not error, got %v", err)
	}
	if len(res.Workspaces) != 0 || len(res.Skipped) != 0 {
		t.Errorf("expected empty result, got %+v", res)
	}
}

func TestListWorkspaces_EmptyRoot(t *testing.T) {
	root := t.TempDir()
	res, err := ListWorkspaces(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Workspaces) != 0 || len(res.Skipped) != 0 {
		t.Errorf("expected empty result, got %+v", res)
	}
}

func TestListWorkspaces_MultipleValid_StableOrder(t *testing.T) {
	root := t.TempDir()
	t0 := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 4, 19, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)

	writeProject(t, root, "alpha-aaa111", validRaw("alpha-aaa111", "Alpha", t0))
	writeProject(t, root, "bravo-bbb222", validRaw("bravo-bbb222", "Bravo", t2)) // newest
	writeProject(t, root, "charl-ccc333", validRaw("charl-ccc333", "Charlie", t1))

	res, err := ListWorkspaces(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Workspaces) != 3 {
		t.Fatalf("expected 3, got %d", len(res.Workspaces))
	}

	// Sorted by UpdatedAt desc: Bravo (t2), Charlie (t1), Alpha (t0).
	want := []string{"Bravo", "Charlie", "Alpha"}
	for i, w := range want {
		if res.Workspaces[i].Name != w {
			t.Errorf("pos %d: got %q, want %q", i, res.Workspaces[i].Name, w)
		}
	}

	// Sanity: Summary fields populated.
	top := res.Workspaces[0]
	if top.ID != ID("bravo-bbb222") {
		t.Errorf("top.ID = %q", top.ID)
	}
	if top.DAW != DAWFLStudio {
		t.Errorf("top.DAW = %q", top.DAW)
	}
	if top.ReferenceOsuPath == "" {
		t.Errorf("top.ReferenceOsuPath should be populated")
	}
	if top.Root != filepath.Join(root, "bravo-bbb222") {
		t.Errorf("top.Root = %q", top.Root)
	}
	if !top.UpdatedAt.Equal(t2) {
		t.Errorf("top.UpdatedAt = %v, want %v", top.UpdatedAt, t2)
	}
}

func TestListWorkspaces_SameUpdatedAt_TieBreaker(t *testing.T) {
	root := t.TempDir()
	ts := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
	writeProject(t, root, "z-1", validRaw("z-1", "Zebra", ts))
	writeProject(t, root, "a-1", validRaw("a-1", "Alpha", ts))
	writeProject(t, root, "m-1", validRaw("m-1", "Mango", ts))

	res, _ := ListWorkspaces(root)
	want := []string{"Alpha", "Mango", "Zebra"}
	for i, w := range want {
		if res.Workspaces[i].Name != w {
			t.Errorf("pos %d: got %q, want %q", i, res.Workspaces[i].Name, w)
		}
	}
}

func TestListWorkspaces_IgnoresNonWorkspaceDirs(t *testing.T) {
	root := t.TempDir()
	// A regular file at top level.
	_ = os.WriteFile(filepath.Join(root, "README.txt"), []byte("hi"), 0o644)
	// A subdirectory without project.odaw.
	_ = os.MkdirAll(filepath.Join(root, "not-a-workspace"), 0o755)
	// A valid workspace.
	writeProject(t, root, "ok-1", validRaw("ok-1", "OK", time.Now().UTC()))

	res, err := ListWorkspaces(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Workspaces) != 1 || res.Workspaces[0].Name != "OK" {
		t.Errorf("expected only OK, got %+v", res.Workspaces)
	}
	if len(res.Skipped) != 0 {
		t.Errorf("unexpected skipped entries: %+v", res.Skipped)
	}
}

func TestListWorkspaces_CorruptProjectFile_Skipped(t *testing.T) {
	root := t.TempDir()
	// Valid one.
	writeProject(t, root, "good-1", validRaw("good-1", "Good", time.Now().UTC()))

	// Corrupt JSON.
	corrupt := filepath.Join(root, "bad-1")
	_ = os.MkdirAll(corrupt, 0o755)
	_ = os.WriteFile(filepath.Join(corrupt, ProjectFileName), []byte("{not json"), 0o644)

	// Incomplete (missing name).
	writeProject(t, root, "incomplete-1", map[string]any{
		"version":           CurrentProjectFileVersion,
		"id":                "incomplete-1",
		"default_sampleset": "Soft",
	})

	res, err := ListWorkspaces(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Workspaces) != 1 || res.Workspaces[0].Name != "Good" {
		t.Errorf("workspaces = %+v", res.Workspaces)
	}
	if len(res.Skipped) != 2 {
		t.Fatalf("expected 2 skipped, got %d: %+v", len(res.Skipped), res.Skipped)
	}
	// Each skipped entry must carry a non-nil error and the workspace path.
	for _, s := range res.Skipped {
		if s.Err == nil {
			t.Errorf("skipped entry %q missing error", s.Path)
		}
		if s.Path == "" {
			t.Errorf("skipped entry missing path")
		}
	}
	// Skipped is sorted by path for determinism.
	if res.Skipped[0].Path >= res.Skipped[1].Path {
		t.Errorf("skipped not sorted: %+v", res.Skipped)
	}
}

func TestListWorkspaces_UnsupportedVersion_Skipped(t *testing.T) {
	root := t.TempDir()
	writeProject(t, root, "future-1", map[string]any{
		"version":           999,
		"id":                "future-1",
		"name":              "Future",
		"template":          map[string]string{"daw": "flstudio", "id": "t"},
		"default_sampleset": "Soft",
		"segments":          []any{},
		"created_at":        time.Now().UTC(),
		"updated_at":        time.Now().UTC(),
	})
	res, err := ListWorkspaces(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Workspaces) != 0 {
		t.Errorf("future-version workspace must be skipped, got %+v", res.Workspaces)
	}
	if len(res.Skipped) != 1 {
		t.Fatalf("expected 1 skipped, got %d", len(res.Skipped))
	}
}

func TestListWorkspaces_RoundtripAfterCreate(t *testing.T) {
	// Integration-style: create via CreateWorkspace, list via
	// ListWorkspaces, confirm the summary matches.
	projectsRoot := filepath.Join(t.TempDir(), "projects")
	pf := NewProjectFile(ID("integ-1"), "Integration Test",
		TemplateRef{DAW: DAWFLStudio, ID: "osu!daw hitsound template"},
		time.Now().UTC())
	pf.ReferenceOsuPath = `C:\ref.osu`

	ws, err := CreateWorkspace(projectsRoot, pf)
	if err != nil {
		t.Fatal(err)
	}

	res, err := ListWorkspaces(projectsRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Workspaces) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(res.Workspaces))
	}
	got := res.Workspaces[0]
	if got.ID != pf.ID {
		t.Errorf("ID = %q", got.ID)
	}
	if got.Name != pf.Name {
		t.Errorf("Name = %q", got.Name)
	}
	if got.DAW != DAWFLStudio {
		t.Errorf("DAW = %q", got.DAW)
	}
	if got.ReferenceOsuPath != pf.ReferenceOsuPath {
		t.Errorf("ReferenceOsuPath = %q", got.ReferenceOsuPath)
	}
	if got.Root != ws.Paths.Root {
		t.Errorf("Root = %q, want %q", got.Root, ws.Paths.Root)
	}
}
