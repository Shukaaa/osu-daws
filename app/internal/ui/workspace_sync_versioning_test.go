package ui

import (
	"testing"

	"osu-daws-app/internal/workspace"
)

func TestApplyWorkspaceState_LoadsVersioning(t *testing.T) {
	ws := newTestWorkspaceOnDisk(t, func(pf *workspace.ProjectFile) {
		pf.VersioningEnabled = true
		pf.LastExportVersion = 7
	})

	vm := NewViewModel(&stubClipboard{}, nil)
	ApplyWorkspaceState(vm, ws)

	if !vm.VersioningEnabled {
		t.Errorf("VersioningEnabled = false, want true")
	}
	if vm.LastExportVersion() != 7 {
		t.Errorf("LastExportVersion = %d, want 7", vm.LastExportVersion())
	}
}

func TestPersistToWorkspace_WritesVersioning(t *testing.T) {
	ws := newTestWorkspaceOnDisk(t, nil)
	vm := NewViewModel(&stubClipboard{}, nil)
	vm.VersioningEnabled = true
	vm.SetLastExportVersion(5)

	if err := PersistToWorkspace(vm, ws); err != nil {
		t.Fatalf("PersistToWorkspace: %v", err)
	}

	reloaded, err := workspace.LoadWorkspace(ws.Paths.Root)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	if !reloaded.Project.VersioningEnabled {
		t.Errorf("VersioningEnabled not persisted")
	}
	if reloaded.Project.LastExportVersion != 5 {
		t.Errorf("LastExportVersion = %d, want 5", reloaded.Project.LastExportVersion)
	}
}
