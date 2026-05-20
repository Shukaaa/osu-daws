package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveToExports_VersioningDisabled_NoSuffix(t *testing.T) {
	vm := NewViewModel(nil, nil)
	vm.SetWorkspaceExportsDir(t.TempDir())
	res := newResult("abc", map[string]string{"Artist": "A", "Title": "T", "Creator": "C"})

	path, err := vm.SaveToExports(res)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(filepath.Base(path), " v") {
		t.Errorf("did not expect version suffix in %q", path)
	}
	if vm.LastExportVersion() != 0 {
		t.Errorf("LastExportVersion: got %d, want 0", vm.LastExportVersion())
	}
}

func TestSaveToExports_Versioning_BumpsEachCall(t *testing.T) {
	vm := NewViewModel(nil, nil)
	vm.SetWorkspaceExportsDir(t.TempDir())
	vm.VersioningEnabled = true
	res := newResult("abc", map[string]string{"Artist": "A", "Title": "T", "Creator": "C"})

	want := []string{" v1", " v2", " v3"}
	for i, suffix := range want {
		vm.pendingVersion = vm.lastExportVersion + 1
		path, err := vm.SaveToExports(res)
		if err != nil {
			t.Fatalf("save %d: %v", i+1, err)
		}
		if !strings.Contains(filepath.Base(path), suffix) {
			t.Errorf("save %d: filename %q missing %q", i+1, path, suffix)
		}
		if vm.LastExportVersion() != i+1 {
			t.Errorf("save %d: LastExportVersion=%d, want %d",
				i+1, vm.LastExportVersion(), i+1)
		}
		if _, err := os.Stat(path); err != nil {
			t.Errorf("save %d: file missing: %v", i+1, err)
		}
	}
}

func TestSaveToExports_Versioning_PendingWithoutSaveStaysStable(t *testing.T) {
	vm := NewViewModel(nil, nil)
	vm.SetWorkspaceExportsDir(t.TempDir())
	vm.VersioningEnabled = true
	vm.SetLastExportVersion(2)

	if got := vm.effectiveVersion(); got != 3 {
		t.Errorf("first effectiveVersion=%d, want 3", got)
	}
	vm.pendingVersion = 3
	if got := vm.effectiveVersion(); got != 3 {
		t.Errorf("second effectiveVersion=%d, want 3 (unchanged until save)", got)
	}
}

func TestSetLastExportVersion_ClampsNegative(t *testing.T) {
	vm := NewViewModel(nil, nil)
	vm.SetLastExportVersion(-5)
	if vm.LastExportVersion() != 0 {
		t.Errorf("got %d, want 0", vm.LastExportVersion())
	}
}

func TestCopyToOsuProject_VersionedFilenameMatchesLastExport(t *testing.T) {
	tmp := t.TempDir()
	refDir := filepath.Join(tmp, "osu")
	if err := os.MkdirAll(refDir, 0o755); err != nil {
		t.Fatal(err)
	}
	refPath := filepath.Join(refDir, "ref.osu")
	if err := os.WriteFile(refPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	vm := NewViewModel(nil, nil)
	vm.ReferencePath = refPath
	vm.VersioningEnabled = true
	vm.SetLastExportVersion(4)
	res := newResult("content", map[string]string{
		"Artist": "A", "Title": "T", "Creator": "C",
	})

	path, err := vm.CopyToOsuProject(res)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(filepath.Base(path), " v4]") {
		t.Errorf("copy path %q does not match v4", path)
	}
}
