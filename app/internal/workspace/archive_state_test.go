package workspace

import (
	"path/filepath"
	"testing"
	"time"

	"osu-daws-app/internal/domain"
)

func makeArchivableWorkspace(t *testing.T, root, name string) *Workspace {
	t.Helper()
	s := NewCreateService(root, NewDefaultCatalog())
	ws, err := s.Create(CreateRequest{
		Name:             name,
		Template:         s.Catalog.Default(),
		DefaultSampleset: domain.SamplesetSoft,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return ws
}

func TestArchiveState_DefaultsToFalse(t *testing.T) {
	ws := makeArchivableWorkspace(t, t.TempDir(), "Fresh")
	if ws.Project.Archived {
		t.Error("fresh workspace must not be archived by default")
	}
	// Round-trip: loading from disk keeps Archived=false.
	loaded, err := LoadWorkspace(ws.Paths.Root)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Project.Archived {
		t.Error("reloaded workspace should keep Archived=false")
	}
}

func TestSetArchived_Roundtrip(t *testing.T) {
	ws := makeArchivableWorkspace(t, t.TempDir(), "Subject")

	if err := ArchiveWorkspace(ws.Paths); err != nil {
		t.Fatalf("archive: %v", err)
	}
	loaded, err := LoadWorkspace(ws.Paths.Root)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Project.Archived {
		t.Error("Archived=true did not persist")
	}

	if err := UnarchiveWorkspace(ws.Paths); err != nil {
		t.Fatalf("unarchive: %v", err)
	}
	loaded, err = LoadWorkspace(ws.Paths.Root)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Project.Archived {
		t.Error("Archived=false did not persist")
	}
}

func TestSetArchived_NoOpWhenUnchanged(t *testing.T) {
	ws := makeArchivableWorkspace(t, t.TempDir(), "Stable")
	before := ws.Project.UpdatedAt

	// Sleep + write nothing, UpdatedAt must not advance.
	time.Sleep(5 * time.Millisecond)
	if err := SetArchived(ws.Paths, false); err != nil {
		t.Fatal(err)
	}
	loaded, _ := LoadWorkspace(ws.Paths.Root)
	if !loaded.Project.UpdatedAt.Equal(before) {
		t.Errorf("UpdatedAt = %v, want unchanged %v",
			loaded.Project.UpdatedAt, before)
	}
}

func TestListWorkspaces_SplitsArchived(t *testing.T) {
	root := t.TempDir()
	active := makeArchivableWorkspace(t, root, "Active")
	archived := makeArchivableWorkspace(t, root, "Stashed")
	if err := ArchiveWorkspace(archived.Paths); err != nil {
		t.Fatal(err)
	}

	res, err := ListWorkspaces(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Workspaces) != 1 || res.Workspaces[0].Name != "Active" {
		t.Errorf("active list = %+v", res.Workspaces)
	}
	if len(res.Archived) != 1 || res.Archived[0].Name != "Stashed" {
		t.Errorf("archived list = %+v", res.Archived)
	}
	// Archived flag propagates onto the summary.
	if !res.Archived[0].Archived || res.Workspaces[0].Archived {
		t.Errorf("Archived flag inconsistent: active=%v archived=%v",
			res.Workspaces[0].Archived, res.Archived[0].Archived)
	}
	_ = active
}

func TestSetArchived_RejectsMissingProjectFile(t *testing.T) {
	// No workspace scaffolded under this path; SetArchived must surface
	// a structured error rather than create a stub project.odaw.
	bogus := PathsFromRoot(filepath.Join(t.TempDir(), "does", "not", "exist"))
	if err := SetArchived(bogus, true); err == nil {
		t.Error("expected error for missing project file")
	}
}
