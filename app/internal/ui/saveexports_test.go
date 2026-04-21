package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"osu-daws-app/internal/domain"
	"osu-daws-app/internal/pipeline"
)

func newResult(content string, meta map[string]string) *pipeline.Result {
	ref := domain.NewOsuMap()
	for k, v := range meta {
		ref.Metadata[k] = v
	}
	return &pipeline.Result{
		OsuContent: content,
		Reference:  ref,
	}
}

func TestSaveToExports_NoWorkspaceReturnsError(t *testing.T) {
	vm := NewViewModel(nil, nil)
	_, err := vm.SaveToExports(newResult("x", nil))
	if err == nil {
		t.Fatal("expected error when no workspace is set")
	}
	if !strings.Contains(err.Error(), "no active workspace") {
		t.Errorf("error = %v", err)
	}
}

func TestSaveToExports_NilResultReturnsError(t *testing.T) {
	vm := NewViewModel(nil, nil)
	vm.SetWorkspaceExportsDir(t.TempDir())
	_, err := vm.SaveToExports(nil)
	if err == nil {
		t.Fatal("expected error on nil result")
	}
}

func TestSaveToExports_EmptyContentReturnsError(t *testing.T) {
	vm := NewViewModel(nil, nil)
	vm.SetWorkspaceExportsDir(t.TempDir())
	_, err := vm.SaveToExports(newResult("", nil))
	if err == nil {
		t.Fatal("expected error on empty content")
	}
}

func TestSaveToExports_WritesFileWithOsuFilename(t *testing.T) {
	dir := t.TempDir()
	vm := NewViewModel(nil, nil)
	vm.SetWorkspaceExportsDir(dir)

	res := newResult("osu file format v14\n", map[string]string{
		"Artist":  "Camellia",
		"Title":   "GHOST",
		"Creator": "Mapper",
	})

	path, err := vm.SaveToExports(res)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Path is inside the exports dir.
	if filepath.Dir(path) != dir {
		t.Errorf("path %q not inside exports dir %q", path, dir)
	}
	// Uses the osu-style filename.
	if !strings.HasSuffix(path, ".osu") {
		t.Errorf("path %q should end in .osu", path)
	}
	base := filepath.Base(path)
	for _, want := range []string{"Camellia", "GHOST", "Mapper"} {
		if !strings.Contains(base, want) {
			t.Errorf("filename %q missing %q", base, want)
		}
	}

	// File actually exists with the right content.
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "osu file format v14\n" {
		t.Errorf("content = %q", string(got))
	}
}

func TestSaveToExports_CreatesExportsDirIfMissing(t *testing.T) {
	// Point at a not-yet-existing subfolder; SaveToExports must create it.
	dir := filepath.Join(t.TempDir(), "nested", "exports")
	vm := NewViewModel(nil, nil)
	vm.SetWorkspaceExportsDir(dir)

	path, err := vm.SaveToExports(newResult("x", map[string]string{"Artist": "A"}))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not written: %v", err)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Errorf("exports dir not created: %v", err)
	}
}

func TestSaveToExports_IsIdempotent(t *testing.T) {
	// Re-exporting with the same metadata overwrites the same file.
	dir := t.TempDir()
	vm := NewViewModel(nil, nil)
	vm.SetWorkspaceExportsDir(dir)

	path1, err := vm.SaveToExports(newResult("first", map[string]string{"Artist": "A"}))
	if err != nil {
		t.Fatal(err)
	}
	path2, err := vm.SaveToExports(newResult("second", map[string]string{"Artist": "A"}))
	if err != nil {
		t.Fatal(err)
	}
	if path1 != path2 {
		t.Errorf("expected same path, got %q and %q", path1, path2)
	}
	got, _ := os.ReadFile(path2)
	if string(got) != "second" {
		t.Errorf("file not overwritten: %q", string(got))
	}

	// Only one file in the directory.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("expected 1 file, got %d", len(entries))
	}
}

func TestSaveToExports_FallbackPlaceholdersWhenNoMetadata(t *testing.T) {
	dir := t.TempDir()
	vm := NewViewModel(nil, nil)
	vm.SetWorkspaceExportsDir(dir)

	path, err := vm.SaveToExports(newResult("x", nil))
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Base(path)
	for _, want := range []string{"Unknown Artist", "Unknown Title", "Unknown"} {
		if !strings.Contains(base, want) {
			t.Errorf("filename %q missing placeholder %q", base, want)
		}
	}
}

func TestSetWorkspaceExportsDir_EmptyClears(t *testing.T) {
	vm := NewViewModel(nil, nil)
	vm.SetWorkspaceExportsDir("/tmp/x")
	if vm.WorkspaceExportsDir() != "/tmp/x" {
		t.Errorf("WorkspaceExportsDir = %q", vm.WorkspaceExportsDir())
	}
	vm.SetWorkspaceExportsDir("")
	if vm.WorkspaceExportsDir() != "" {
		t.Errorf("expected empty, got %q", vm.WorkspaceExportsDir())
	}
}
