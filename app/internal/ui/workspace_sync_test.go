package ui

import (
	"path/filepath"
	"testing"
	"time"

	"osu-daws-app/internal/domain"
	"osu-daws-app/internal/workspace"
)

// newTestWorkspaceOnDisk creates a real scaffolded workspace under a
// temp dir so both Apply and Persist helpers can exercise the full
// workspace.Save/Load round-trip without mocking the filesystem.
func newTestWorkspaceOnDisk(t *testing.T, seed func(pf *workspace.ProjectFile)) *workspace.Workspace {
	t.Helper()
	projectsRoot := filepath.Join(t.TempDir(), "projects")
	pf := workspace.NewProjectFile(
		workspace.ID("audit-1"),
		"Audit Test",
		workspace.TemplateRef{DAW: workspace.DAWFLStudio, ID: "t"},
		time.Now().UTC(),
	)
	if seed != nil {
		seed(pf)
	}
	ws, err := workspace.CreateWorkspace(projectsRoot, pf)
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	return ws
}

func TestApplyWorkspaceState_FullState(t *testing.T) {
	ws := newTestWorkspaceOnDisk(t, func(pf *workspace.ProjectFile) {
		pf.ReferenceOsuPath = "/maps/song/normal.osu"
		pf.DefaultSampleset = domain.SamplesetDrum
		pf.Segments = []workspace.SegmentInput{
			{SourceMapJSON: validSMUI, StartTimeText: "1000"},
			{SourceMapJSON: "", StartTimeText: ""},
		}
	})

	vm := NewViewModel(&stubClipboard{}, nil)
	ApplyWorkspaceState(vm, ws)

	if vm.ReferencePath != "/maps/song/normal.osu" {
		t.Errorf("ReferencePath = %q", vm.ReferencePath)
	}
	if vm.DefaultSampleset != domain.SamplesetDrum {
		t.Errorf("DefaultSampleset = %q", vm.DefaultSampleset)
	}
	if got := vm.WorkspaceExportsDir(); got != ws.Paths.Exports {
		t.Errorf("WorkspaceExportsDir = %q, want %q", got, ws.Paths.Exports)
	}
	if len(vm.Segments) != 2 {
		t.Fatalf("Segments len = %d, want 2", len(vm.Segments))
	}
	if vm.Segments[0].SourceMapJSON != validSMUI {
		t.Errorf("Segment[0] JSON not restored")
	}
	if vm.Segments[0].StartTimeText != "1000" {
		t.Errorf("Segment[0] start time = %q", vm.Segments[0].StartTimeText)
	}
	// Status must be re-derived from the loaded JSON, not left at the
	// "No SourceMap loaded yet." default.
	if vm.Segments[0].Status == "" ||
		vm.Segments[0].Status == "No SourceMap loaded yet." {
		t.Errorf("Segment[0] status not re-derived: %q", vm.Segments[0].Status)
	}
	if vm.Segments[1].Status != "No SourceMap loaded yet." {
		t.Errorf("Segment[1] empty status = %q", vm.Segments[1].Status)
	}
}

func TestApplyWorkspaceState_EmptySegmentsKeepsPlaceholder(t *testing.T) {
	ws := newTestWorkspaceOnDisk(t, nil)

	vm := NewViewModel(&stubClipboard{}, nil)
	ApplyWorkspaceState(vm, ws)

	if len(vm.Segments) != 1 {
		t.Fatalf("expected 1 placeholder segment, got %d", len(vm.Segments))
	}
	if vm.Segments[0].SourceMapJSON != "" {
		t.Error("placeholder segment should have empty JSON")
	}
}

func TestApplyWorkspaceState_NilInputs(t *testing.T) {
	// Must not panic and must not mutate the VM.
	vm := NewViewModel(&stubClipboard{}, nil)
	before := vm.ReferencePath
	ApplyWorkspaceState(nil, nil)
	ApplyWorkspaceState(vm, nil)
	if vm.ReferencePath != before {
		t.Error("VM mutated unexpectedly")
	}
}

func TestPersistToWorkspace_Roundtrip(t *testing.T) {
	ws := newTestWorkspaceOnDisk(t, nil)

	vm := NewViewModel(&stubClipboard{}, nil)
	ApplyWorkspaceState(vm, ws)

	// Mutate the VM the way the UI would.
	vm.ReferencePath = "/maps/new/hard.osu"
	vm.DefaultSampleset = domain.SamplesetNormal
	vm.Segments = []*SegmentInput{
		{SourceMapJSON: validSMUI, StartTimeText: "1234", Status: "ignored"},
		{SourceMapJSON: "", StartTimeText: "", Status: "placeholder"},
	}

	if err := PersistToWorkspace(vm, ws); err != nil {
		t.Fatalf("PersistToWorkspace: %v", err)
	}

	// Reload from disk to make sure the changes were actually written
	// and survived JSON round-trip.
	reloaded, err := workspace.LoadWorkspace(ws.Paths.Root)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	if reloaded.Project.ReferenceOsuPath != "/maps/new/hard.osu" {
		t.Errorf("ReferenceOsuPath = %q", reloaded.Project.ReferenceOsuPath)
	}
	if reloaded.Project.DefaultSampleset != domain.SamplesetNormal {
		t.Errorf("DefaultSampleset = %q", reloaded.Project.DefaultSampleset)
	}
	if len(reloaded.Project.Segments) != 2 {
		t.Fatalf("Segments len = %d", len(reloaded.Project.Segments))
	}
	if reloaded.Project.Segments[0].SourceMapJSON != validSMUI ||
		reloaded.Project.Segments[0].StartTimeText != "1234" {
		t.Errorf("Segment[0] = %+v", reloaded.Project.Segments[0])
	}
}

func TestPersistToWorkspace_NilSafe(t *testing.T) {
	vm := NewViewModel(&stubClipboard{}, nil)
	if err := PersistToWorkspace(nil, nil); err != nil {
		t.Errorf("nil inputs returned %v, want nil", err)
	}
	if err := PersistToWorkspace(vm, nil); err != nil {
		t.Errorf("nil workspace returned %v, want nil", err)
	}
}

func TestApplyThenPersist_FullRoundtrip(t *testing.T) {
	ws := newTestWorkspaceOnDisk(t, func(pf *workspace.ProjectFile) {
		pf.ReferenceOsuPath = "/ref.osu"
		pf.DefaultSampleset = domain.SamplesetSoft
		pf.Segments = []workspace.SegmentInput{
			{SourceMapJSON: validSMUI, StartTimeText: "500"},
		}
	})

	vm := NewViewModel(&stubClipboard{}, nil)
	ApplyWorkspaceState(vm, ws)

	// Persist without touching anything — project file must remain
	// semantically identical.
	if err := PersistToWorkspace(vm, ws); err != nil {
		t.Fatalf("PersistToWorkspace: %v", err)
	}
	reloaded, err := workspace.LoadWorkspace(ws.Paths.Root)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Project.ReferenceOsuPath != "/ref.osu" {
		t.Errorf("ReferenceOsuPath drifted: %q", reloaded.Project.ReferenceOsuPath)
	}
	if reloaded.Project.DefaultSampleset != domain.SamplesetSoft {
		t.Errorf("DefaultSampleset drifted: %q", reloaded.Project.DefaultSampleset)
	}
	if len(reloaded.Project.Segments) != 1 ||
		reloaded.Project.Segments[0].SourceMapJSON != validSMUI ||
		reloaded.Project.Segments[0].StartTimeText != "500" {
		t.Errorf("Segments drifted: %+v", reloaded.Project.Segments)
	}
}
