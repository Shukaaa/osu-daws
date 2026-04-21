package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"osu-daws-app/internal/domain"
	"osu-daws-app/internal/workspace"
)

func createTestWorkspace(svm *StartViewModel, name string) (*workspace.Workspace, error) {
	return svm.CreateWorkspace(workspace.CreateRequest{
		Name:             name,
		Template:         svm.Catalog.Default(),
		DefaultSampleset: domain.SamplesetSoft,
	})
}

func TestStartVM_Refresh_MissingRoot(t *testing.T) {
	svm := NewStartViewModel(filepath.Join(t.TempDir(), "not", "there"))
	if err := svm.Refresh(); err != nil {
		t.Fatalf("missing root should not error, got %v", err)
	}
	if len(svm.Workspaces()) != 0 {
		t.Errorf("expected empty list, got %d", len(svm.Workspaces()))
	}
	if len(svm.Skipped()) != 0 {
		t.Errorf("expected empty skipped, got %d", len(svm.Skipped()))
	}
}

func TestStartVM_BeforeRefresh_EmptyList(t *testing.T) {
	svm := NewStartViewModel(t.TempDir())
	if ws := svm.Workspaces(); ws != nil {
		t.Errorf("Workspaces() before Refresh should return nil, got %v", ws)
	}
	if sk := svm.Skipped(); sk != nil {
		t.Errorf("Skipped() before Refresh should return nil, got %v", sk)
	}
}

func TestStartVM_Refresh_ListsExistingWorkspaces(t *testing.T) {
	root := t.TempDir()

	svm := NewStartViewModel(root)

	if _, err := createTestWorkspace(svm, "Alpha"); err != nil {
		t.Fatal(err)
	}
	// Small gap so UpdatedAt is strictly greater on the second save.
	time.Sleep(5 * time.Millisecond)
	if _, err := createTestWorkspace(svm, "Bravo"); err != nil {
		t.Fatal(err)
	}

	if err := svm.Refresh(); err != nil {
		t.Fatal(err)
	}
	got := svm.Workspaces()
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
	// Sorted by UpdatedAt desc → newest first.
	if got[0].Name != "Bravo" || got[1].Name != "Alpha" {
		t.Errorf("order = %q,%q; want Bravo,Alpha (newest first)", got[0].Name, got[1].Name)
	}
}

func TestStartVM_CreateNamed_EmptyRejected(t *testing.T) {
	cases := []string{"", "   ", "\t"}
	for _, in := range cases {
		t.Run("input="+in, func(t *testing.T) {
			svm := NewStartViewModel(t.TempDir())
			_, err := createTestWorkspace(svm, in)
			if err == nil {
				t.Fatal("expected error for blank name")
			}
			if !strings.Contains(err.Error(), "Name is required") {
				t.Errorf("error = %q, want mention of 'Name is required'", err.Error())
			}
		})
	}
}

func TestStartVM_CreateNamed_PersistsFilesOnDisk(t *testing.T) {
	root := t.TempDir()
	svm := NewStartViewModel(root)
	svm.SetClock(func() time.Time { return time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC) })

	ws, err := createTestWorkspace(svm, "My Song")
	if err != nil {
		t.Fatal(err)
	}
	// ID uses the slugged name as prefix.
	if !strings.HasPrefix(string(ws.Project.ID), "my-song-") {
		t.Errorf("ID %q should start with my-song-", ws.Project.ID)
	}
	if ws.Project.Name != "My Song" {
		t.Errorf("Name = %q", ws.Project.Name)
	}
	if ws.Project.Template.DAW != workspace.DAWFLStudio {
		t.Errorf("DAW = %q", ws.Project.Template.DAW)
	}
	// project.odaw, template/, exports/ all exist.
	for _, p := range []string{ws.Paths.ProjectFile, ws.Paths.Template, ws.Paths.Exports} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing %q: %v", p, err)
		}
	}
}

func TestStartVM_CreateNamed_RefreshedAfterCreate(t *testing.T) {
	root := t.TempDir()
	svm := NewStartViewModel(root)

	if _, err := createTestWorkspace(svm, "Zeta"); err != nil {
		t.Fatal(err)
	}
	got := svm.Workspaces()
	if len(got) != 1 || got[0].Name != "Zeta" {
		t.Errorf("expected Zeta in list, got %+v", got)
	}
}

func TestStartVM_CreateNamed_SameNameUniqueIDs(t *testing.T) {
	root := t.TempDir()
	svm := NewStartViewModel(root)

	a, err := createTestWorkspace(svm, "Same")
	if err != nil {
		t.Fatal(err)
	}
	b, err := createTestWorkspace(svm, "Same")
	if err != nil {
		t.Fatalf("second create should succeed via random suffix, got %v", err)
	}
	if a.Project.ID == b.Project.ID {
		t.Errorf("IDs collided: %s", a.Project.ID)
	}
	if err := svm.Refresh(); err != nil {
		t.Fatal(err)
	}
	if len(svm.Workspaces()) != 2 {
		t.Errorf("expected 2 workspaces, got %d", len(svm.Workspaces()))
	}
}

func TestStartVM_Refresh_SurfacesSkipped(t *testing.T) {
	root := t.TempDir()

	// Plant a corrupt "workspace".
	bad := filepath.Join(root, "broken-1")
	if err := os.MkdirAll(bad, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bad, workspace.ProjectFileName), []byte("{nope"), 0o644); err != nil {
		t.Fatal(err)
	}

	svm := NewStartViewModel(root)
	if _, err := createTestWorkspace(svm, "Good"); err != nil {
		t.Fatal(err)
	}
	if err := svm.Refresh(); err != nil {
		t.Fatal(err)
	}
	if len(svm.Workspaces()) != 1 {
		t.Errorf("expected 1 good workspace, got %d", len(svm.Workspaces()))
	}
	if len(svm.Skipped()) != 1 {
		t.Errorf("expected 1 skipped, got %d", len(svm.Skipped()))
	}
}

func TestStartVM_CreateWorkspace_FullRequest(t *testing.T) {
	root := t.TempDir()
	svm := NewStartViewModel(root)
	svm.SetStatFunc(func(p string) (os.FileInfo, error) {
		// pretend the reference file exists
		if p == "/tmp/ref.osu" {
			return nil, nil
		}
		return nil, os.ErrNotExist
	})

	ws, err := svm.CreateWorkspace(workspace.CreateRequest{
		Name:             "Full Request",
		ReferenceOsuPath: "/tmp/ref.osu",
		Template:         svm.Catalog.Default(),
		DefaultSampleset: "drum",
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if ws.Project.Name != "Full Request" {
		t.Errorf("Name = %q", ws.Project.Name)
	}
	if ws.Project.ReferenceOsuPath != "/tmp/ref.osu" {
		t.Errorf("ReferenceOsuPath = %q", ws.Project.ReferenceOsuPath)
	}
	if string(ws.Project.DefaultSampleset) != "drum" {
		t.Errorf("DefaultSampleset = %q", ws.Project.DefaultSampleset)
	}
}

func TestStartVM_CreateWorkspace_FieldErrorsReturned(t *testing.T) {
	svm := NewStartViewModel(t.TempDir())
	_, err := svm.CreateWorkspace(workspace.CreateRequest{
		Name:             "", // invalid
		Template:         svm.Catalog.Default(),
		DefaultSampleset: "soft",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	fe, ok := err.(workspace.FieldErrors)
	if !ok {
		t.Fatalf("error type = %T, want workspace.FieldErrors", err)
	}
	if _, has := fe[workspace.FieldName]; !has {
		t.Errorf("expected FieldName error, got %v", fe)
	}
}

func TestStartVM_FilteredWorkspaces(t *testing.T) {
	root := t.TempDir()
	svm := NewStartViewModel(root)

	for _, n := range []string{"Alpha Map", "Beta Song", "Gamma"} {
		if _, err := createTestWorkspace(svm, n); err != nil {
			t.Fatal(err)
		}
	}
	if err := svm.Refresh(); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name  string
		query string
		want  []string // subset of names expected (order-insensitive)
	}{
		{"empty returns all", "", []string{"Alpha Map", "Beta Song", "Gamma"}},
		{"whitespace returns all", "   ", []string{"Alpha Map", "Beta Song", "Gamma"}},
		{"substring match", "song", []string{"Beta Song"}},
		{"case insensitive", "GAMMA", []string{"Gamma"}},
		{"no match", "zzz", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svm.SearchQuery = c.query
			got := svm.FilteredWorkspaces()
			if len(got) != len(c.want) {
				t.Fatalf("len=%d want=%d (%v)", len(got), len(c.want), namesOf(got))
			}
			for _, wantName := range c.want {
				found := false
				for _, g := range got {
					if g.Name == wantName {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("missing %q in %v", wantName, namesOf(got))
				}
			}
		})
	}
}

func TestStartVM_FilteredWorkspaces_BeforeRefresh(t *testing.T) {
	svm := NewStartViewModel(t.TempDir())
	svm.SearchQuery = "anything"
	if got := svm.FilteredWorkspaces(); len(got) != 0 {
		t.Errorf("expected empty result before Refresh, got %d", len(got))
	}
}

func namesOf(ws []workspace.Summary) []string {
	out := make([]string, len(ws))
	for i, w := range ws {
		out[i] = w.Name
	}
	return out
}

func TestStartVM_ExportImportRoundTrip(t *testing.T) {
	srcRoot := t.TempDir()
	dstRoot := t.TempDir()
	zipPath := filepath.Join(t.TempDir(), "export.zip")

	srcSVM := NewStartViewModel(srcRoot)
	if _, err := createTestWorkspace(srcSVM, "Round Trip"); err != nil {
		t.Fatal(err)
	}
	if err := srcSVM.Refresh(); err != nil {
		t.Fatal(err)
	}
	items := srcSVM.Workspaces()
	if len(items) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(items))
	}

	if err := srcSVM.ExportWorkspaceToZip(items[0], zipPath); err != nil {
		t.Fatalf("export: %v", err)
	}
	info, err := os.Stat(zipPath)
	if err != nil || info.Size() == 0 {
		t.Fatalf("zip file missing or empty: %v", err)
	}

	// Import into a separate projects root.
	dstSVM := NewStartViewModel(dstRoot)
	imported, err := dstSVM.ImportWorkspaceFromZip(zipPath)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if imported.Project.Name != "Round Trip" {
		t.Errorf("imported name = %q", imported.Project.Name)
	}
	// View model should see the new workspace without an extra Refresh.
	if len(dstSVM.Workspaces()) != 1 {
		t.Errorf("expected 1 imported workspace after refresh, got %d",
			len(dstSVM.Workspaces()))
	}
}

func TestStartVM_LastOpened_MarksAndResolves(t *testing.T) {
	root := t.TempDir()
	svm := NewStartViewModel(root)

	// No history yet.
	if _, ok := svm.LastOpenedSummary(); ok {
		t.Error("fresh VM should not report a last opened workspace")
	}

	first, err := createTestWorkspace(svm, "First")
	if err != nil {
		t.Fatal(err)
	}
	second, err := createTestWorkspace(svm, "Second")
	if err != nil {
		t.Fatal(err)
	}
	// Mark only the second one.
	svm.MarkOpened(second)
	if err := svm.Refresh(); err != nil {
		t.Fatal(err)
	}
	got, ok := svm.LastOpenedSummary()
	if !ok {
		t.Fatal("expected a last-opened entry")
	}
	if got.ID != second.Project.ID {
		t.Errorf("last opened = %q, want %q", got.ID, second.Project.ID)
	}
	_ = first
}

func TestStartVM_LastOpened_IgnoresDeletedWorkspace(t *testing.T) {
	root := t.TempDir()
	svm := NewStartViewModel(root)

	ws, err := createTestWorkspace(svm, "Doomed")
	if err != nil {
		t.Fatal(err)
	}
	svm.MarkOpened(ws)

	// Nuke the workspace directory, refresh, then query.
	if err := os.RemoveAll(ws.Paths.Root); err != nil {
		t.Fatal(err)
	}
	if err := svm.Refresh(); err != nil {
		t.Fatal(err)
	}
	if _, ok := svm.LastOpenedSummary(); ok {
		t.Error("last opened must be hidden when target workspace no longer exists")
	}
}

func TestStartVM_LastOpened_OnlyOneAtATime(t *testing.T) {
	root := t.TempDir()
	svm := NewStartViewModel(root)

	a, _ := createTestWorkspace(svm, "A")
	b, _ := createTestWorkspace(svm, "B")
	c, _ := createTestWorkspace(svm, "C")

	svm.MarkOpened(a)
	svm.MarkOpened(b)
	svm.MarkOpened(c)
	_ = svm.Refresh()

	got, ok := svm.LastOpenedSummary()
	if !ok || got.ID != c.Project.ID {
		t.Errorf("expected last opened = C (%q), got ok=%v id=%q",
			c.Project.ID, ok, got.ID)
	}
}

func TestStartVM_LastOpened_NilSafe(t *testing.T) {
	svm := NewStartViewModel(t.TempDir())
	// Must not panic.
	svm.MarkOpened(nil)
	svm.MarkOpened(&workspace.Workspace{})
}
