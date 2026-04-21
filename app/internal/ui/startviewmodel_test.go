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
