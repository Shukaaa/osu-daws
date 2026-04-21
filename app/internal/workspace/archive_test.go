package workspace

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"osu-daws-app/internal/domain"
)

// archiveTestCatalog returns a catalog with the FL Studio provider (the
// default shipped one). Local helper for readability.
func archiveTestCatalog(t *testing.T) *TemplateCatalog {
	t.Helper()
	return NewDefaultCatalog()
}

// makeWorkspaceOnDisk scaffolds a real workspace with the FL Studio
// template inside root and returns the loaded Workspace.
func makeWorkspaceOnDisk(t *testing.T, root, name string) *Workspace {
	t.Helper()
	s := NewCreateService(root, archiveTestCatalog(t))
	ws, err := s.Create(CreateRequest{
		Name:             name,
		Template:         s.Catalog.Default(),
		DefaultSampleset: domain.SamplesetSoft,
	})
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	return ws
}

func TestExportImport_RoundTripPreservesTree(t *testing.T) {
	srcRoot := t.TempDir()
	dstRoot := t.TempDir()
	ws := makeWorkspaceOnDisk(t, srcRoot, "My Song")

	// Seed a file into exports/ so we exercise that branch too.
	exported := filepath.Join(ws.Paths.Exports, "mapping.osu")
	if err := os.WriteFile(exported, []byte("exported content"), 0o644); err != nil {
		t.Fatal(err)
	}

	zipPath := filepath.Join(t.TempDir(), "out.zip")
	if err := ExportWorkspace(ws, zipPath); err != nil {
		t.Fatalf("export: %v", err)
	}

	imported, err := ImportWorkspace(dstRoot, zipPath)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if imported.Project.Name != ws.Project.Name {
		t.Errorf("Name = %q, want %q", imported.Project.Name, ws.Project.Name)
	}
	if imported.Project.ID == ws.Project.ID {
		t.Errorf("ID should be freshly minted on import, got same: %s", imported.Project.ID)
	}
	if !strings.HasPrefix(string(imported.Project.ID), "my-song-") {
		t.Errorf("ID %q should start with slug prefix", imported.Project.ID)
	}
	// Check the exports/ file survived.
	dstExport := filepath.Join(imported.Paths.Exports, "mapping.osu")
	data, err := os.ReadFile(dstExport)
	if err != nil {
		t.Fatalf("imported exports file missing: %v", err)
	}
	if string(data) != "exported content" {
		t.Errorf("exports content = %q, want %q", string(data), "exported content")
	}
	// Template tree survived: entry .flp present.
	entry := filepath.Join(imported.Paths.Template, filepath.FromSlash(FLStudioEntryFile))
	if _, err := os.Stat(entry); err != nil {
		t.Errorf("template entry missing after import: %v", err)
	}
}

func TestExport_NilWorkspace(t *testing.T) {
	err := ExportWorkspace(nil, filepath.Join(t.TempDir(), "x.zip"))
	if err == nil {
		t.Fatal("expected error for nil workspace")
	}
}

func TestImport_MissingProjectFile(t *testing.T) {
	// Zip with only an unrelated file and no project.odaw.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("readme.txt")
	_, _ = w.Write([]byte("hi"))
	_ = zw.Close()

	_, err := ImportWorkspaceFrom(t.TempDir(),
		bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err == nil {
		t.Fatal("expected error for archive without project.odaw")
	}
	var we *Error
	if !errors.As(err, &we) || we.Code != ErrProjectFileMissing {
		t.Errorf("got err=%v; want code=%v", err, ErrProjectFileMissing)
	}
}

func TestImport_RejectsZipSlip(t *testing.T) {
	// Craft a zip that contains a valid project.odaw plus a malicious
	// entry trying to escape via "..".
	pf := validProjectFileJSON(t, "Evil")

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w1, _ := zw.Create(ProjectFileName)
	_, _ = w1.Write(pf)
	w2, _ := zw.Create("../escape.txt")
	_, _ = w2.Write([]byte("pwned"))
	_ = zw.Close()

	dst := t.TempDir()
	_, err := ImportWorkspaceFrom(dst, bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err == nil {
		t.Fatal("expected error for zip-slip entry")
	}
	// Nothing should have been written above dst.
	parent := filepath.Dir(dst)
	entries, _ := os.ReadDir(parent)
	for _, e := range entries {
		if e.Name() == "escape.txt" {
			t.Errorf("escape.txt was written to %s", parent)
		}
	}
}

func TestImport_WrappedLayoutIsAccepted(t *testing.T) {
	// Simulate a user who zipped the workspace *folder* rather than
	// its contents: project.odaw is nested one level deep under a
	// prefix directory.
	pf := validProjectFileJSON(t, "Wrapped")

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	_, _ = zw.Create("wrapper/")
	wProj, _ := zw.Create("wrapper/" + ProjectFileName)
	_, _ = wProj.Write(pf)
	wSample, _ := zw.Create("wrapper/template/sample.wav")
	_, _ = wSample.Write([]byte("RIFF"))
	_ = zw.Close()

	dst := t.TempDir()
	ws, err := ImportWorkspaceFrom(dst, bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if ws.Project.Name != "Wrapped" {
		t.Errorf("Name = %q", ws.Project.Name)
	}
	if _, err := os.Stat(filepath.Join(ws.Paths.Template, "sample.wav")); err != nil {
		t.Errorf("nested file not extracted under correct prefix: %v", err)
	}
}

func TestImport_FreshIDPreventsCollision(t *testing.T) {
	root := t.TempDir()
	ws := makeWorkspaceOnDisk(t, root, "Collide")
	zipPath := filepath.Join(t.TempDir(), "out.zip")
	if err := ExportWorkspace(ws, zipPath); err != nil {
		t.Fatal(err)
	}
	// Import the same archive twice into the same projects root.
	a, err := ImportWorkspace(root, zipPath)
	if err != nil {
		t.Fatal(err)
	}
	b, err := ImportWorkspace(root, zipPath)
	if err != nil {
		t.Fatalf("second import should not collide: %v", err)
	}
	if a.Project.ID == b.Project.ID {
		t.Errorf("imports share the same ID: %s", a.Project.ID)
	}
	if a.Paths.Root == b.Paths.Root {
		t.Errorf("imports share the same root path: %s", a.Paths.Root)
	}
}

func TestSuggestExportFileName(t *testing.T) {
	cases := []struct {
		name string
		ws   *Workspace
		want string
	}{
		{"nil", nil, "workspace.zip"},
		{"normal", &Workspace{Project: &ProjectFile{Name: "My Song"}}, "my-song.zip"},
		{"blank falls back to slug default", &Workspace{Project: &ProjectFile{Name: ""}}, "project.zip"},
	}
	for _, c := range cases {
		if got := SuggestExportFileName(c.ws); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// --- small helpers ------------------------------------------------------

func validProjectFileJSON(t *testing.T, name string) []byte {
	t.Helper()
	pf := NewProjectFile(NewID(name), name,
		TemplateRef{DAW: DAWFLStudio, ID: FLStudioTemplateID, Version: FLStudioTemplateVersion},
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	data, err := json.MarshalIndent(pf, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return data
}
