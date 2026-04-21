package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadWorkspaceFromProjectFile_HappyPath(t *testing.T) {
	projectsRoot := filepath.Join(t.TempDir(), "projects")
	pf := NewProjectFile(ID("file-open-1"), "FileOpen",
		TemplateRef{DAW: DAWFLStudio, ID: "t"}, time.Now().UTC())
	ws, err := CreateWorkspace(projectsRoot, pf)
	if err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadWorkspaceFromProjectFile(ws.Paths.ProjectFile)
	if err != nil {
		t.Fatalf("LoadWorkspaceFromProjectFile: %v", err)
	}
	if loaded.Project.ID != pf.ID {
		t.Errorf("ID = %q, want %q", loaded.Project.ID, pf.ID)
	}
	if loaded.Paths.Root != ws.Paths.Root {
		t.Errorf("Paths.Root = %q, want %q", loaded.Paths.Root, ws.Paths.Root)
	}
}

func TestLoadWorkspaceFromProjectFile_EmptyPath(t *testing.T) {
	_, err := LoadWorkspaceFromProjectFile("")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestLoadWorkspaceFromProjectFile_Missing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist", ProjectFileName)
	_, err := LoadWorkspaceFromProjectFile(missing)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadWorkspaceFromProjectFile_WrongName(t *testing.T) {
	dir := t.TempDir()
	other := filepath.Join(dir, "something.txt")
	if err := os.WriteFile(other, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadWorkspaceFromProjectFile(other)
	if err == nil {
		t.Fatal("expected error for wrong filename")
	}
	if !strings.Contains(err.Error(), ProjectFileName) {
		t.Errorf("error should mention %q: %v", ProjectFileName, err)
	}
}

func TestLoadWorkspaceFromProjectFile_IsDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ProjectFileName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := LoadWorkspaceFromProjectFile(dir)
	if err == nil {
		t.Fatal("expected error when path is a directory")
	}
}

func TestLoadWorkspaceFromProjectFile_InvalidJSON(t *testing.T) {
	root := t.TempDir()
	if _, err := Scaffold(root); err != nil {
		t.Fatal(err)
	}
	pfPath := filepath.Join(root, ProjectFileName)
	if err := os.WriteFile(pfPath, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadWorkspaceFromProjectFile(pfPath)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
