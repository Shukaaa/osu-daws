package ui

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestLatestExportedDiffPathFallsBackToNewestOsuInExports(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.osu")
	newPath := filepath.Join(dir, "new.osu")
	if err := os.WriteFile(oldPath, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-time.Hour)
	newTime := time.Now()
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newPath, newTime, newTime); err != nil {
		t.Fatal(err)
	}

	vm := NewViewModel(nil, nil)
	vm.SetWorkspaceExportsDir(dir)

	got, err := vm.LatestExportedDiffPath()
	if err != nil {
		t.Fatalf("LatestExportedDiffPath failed: %v", err)
	}
	if got != newPath {
		t.Fatalf("LatestExportedDiffPath = %q, want %q", got, newPath)
	}
}

func TestExportHSDiffZip(t *testing.T) {
	root := t.TempDir()
	beatmapDir := filepath.Join(root, "osu")
	exportsDir := filepath.Join(root, "exports")
	if err := os.MkdirAll(beatmapDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(exportsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	refPath := filepath.Join(beatmapDir, "ref.osu")
	if err := os.WriteFile(refPath, []byte("ref"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name := range map[string]bool{"soft-hitnormal.wav": true, "drum-clap.ogg": true, "audio.mp3": true} {
		if err := os.WriteFile(filepath.Join(beatmapDir, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	vm := NewViewModel(nil, nil)
	vm.ReferencePath = refPath
	vm.SetWorkspaceExportsDir(exportsDir)
	res := newResult("generated", map[string]string{"Artist": "A", "Title": "T", "Creator": "C"})
	if _, err := vm.SaveToExports(res); err != nil {
		t.Fatal(err)
	}

	zipPath := filepath.Join(root, "hs-bundle.zip")
	zipRes, err := vm.ExportHSDiffZip(zipPath)
	if err != nil {
		t.Fatalf("ExportHSDiffZip failed: %v", err)
	}
	if zipRes.ZipPath != zipPath {
		t.Fatalf("ZipPath = %q, want %q", zipRes.ZipPath, zipPath)
	}

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	entries := map[string]bool{}
	for _, f := range zr.File {
		entries[f.Name] = true
	}
	for _, want := range []string{filepath.Base(vm.LastSavedExportPath()), "soft-hitnormal.wav", "drum-clap.ogg"} {
		if !entries[want] {
			t.Fatalf("zip missing %q; entries=%v", want, entries)
		}
	}
	if entries["audio.mp3"] {
		t.Fatal("zip should not include non-hitsound audio.mp3")
	}
}
