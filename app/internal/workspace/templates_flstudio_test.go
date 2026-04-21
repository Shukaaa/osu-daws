package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// expectedFLStudioFiles are the relative paths (from paths.Template)
// the provider must create on a fresh init. Kept in sync manually with
// the embedded asset tree.
var expectedFLStudioFiles = []string{
	"osu!daw hitsound template/osu!daw hitsound template.flp",
	"osu!daw hitsound template/Samples/drum-hitclap.wav",
	"osu!daw hitsound template/Samples/drum-hitfinish.wav",
	"osu!daw hitsound template/Samples/drum-hitnormal.wav",
	"osu!daw hitsound template/Samples/drum-hitwhistle.wav",
	"osu!daw hitsound template/Samples/normal-hitclap.wav",
	"osu!daw hitsound template/Samples/normal-hitfinish.wav",
	"osu!daw hitsound template/Samples/normal-hitnormal.wav",
	"osu!daw hitsound template/Samples/normal-hitwhistle.wav",
	"osu!daw hitsound template/Samples/soft-hitclap.wav",
	"osu!daw hitsound template/Samples/soft-hitfinish.wav",
	"osu!daw hitsound template/Samples/soft-hitnormal.wav",
	"osu!daw hitsound template/Samples/soft-hitwhistle.wav",
}

func TestFLStudioInit_CopiesAllAssets(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ws")
	paths, err := Scaffold(root)
	if err != nil {
		t.Fatal(err)
	}

	if err := (FLStudioProvider{}).Initialize(paths); err != nil {
		t.Fatalf("init: %v", err)
	}

	for _, rel := range expectedFLStudioFiles {
		p := filepath.Join(paths.Template, filepath.FromSlash(rel))
		info, err := os.Stat(p)
		if err != nil {
			t.Errorf("missing %q: %v", rel, err)
			continue
		}
		if info.IsDir() {
			t.Errorf("%q is a directory, want file", rel)
		}
		if info.Size() == 0 {
			t.Errorf("%q is empty... embed did not deliver content", rel)
		}
	}
}

func TestFLStudioInit_EntryFileResolves(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ws")
	paths, _ := Scaffold(root)
	if err := (FLStudioProvider{}).Initialize(paths); err != nil {
		t.Fatal(err)
	}
	// The entry file path stored in the marker must exist on disk.
	entry := filepath.Join(paths.Template, filepath.FromSlash(FLStudioEntryFile))
	if _, err := os.Stat(entry); err != nil {
		t.Errorf("FLStudioEntryFile does not resolve: %v", err)
	}
}

func TestFLStudioInit_MarkerIncludesLayoutInfo(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ws")
	paths, _ := Scaffold(root)
	if err := (FLStudioProvider{}).Initialize(paths); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(paths.Template, templateInfoFileName))
	if err != nil {
		t.Fatal(err)
	}
	var info TemplateInfo
	if err := json.Unmarshal(data, &info); err != nil {
		t.Fatal(err)
	}
	var extra FLStudioExtra
	if err := info.DecodeExtra(&extra); err != nil {
		t.Fatalf("decode extra: %v", err)
	}
	if extra.RootDir != FLStudioRootDir {
		t.Errorf("RootDir = %q, want %q", extra.RootDir, FLStudioRootDir)
	}
	if extra.EntryFile != FLStudioEntryFile {
		t.Errorf("EntryFile = %q, want %q", extra.EntryFile, FLStudioEntryFile)
	}
	if !strings.HasSuffix(extra.EntryFile, ".flp") {
		t.Errorf("EntryFile should end in .flp, got %q", extra.EntryFile)
	}
}

func TestFLStudioInit_Idempotent_RefreshesAssets(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ws")
	paths, _ := Scaffold(root)

	if err := (FLStudioProvider{}).Initialize(paths); err != nil {
		t.Fatal(err)
	}
	flp := filepath.Join(paths.Template, filepath.FromSlash(FLStudioEntryFile))
	orig, err := os.ReadFile(flp)
	if err != nil {
		t.Fatal(err)
	}
	// Mangle the .flp; re-init must restore the embedded bytes.
	if err := os.WriteFile(flp, []byte("corrupted"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (FLStudioProvider{}).Initialize(paths); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(flp)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(orig) {
		t.Errorf("after re-init: len=%d, want %d (embed content should win)",
			len(after), len(orig))
	}
}

func TestFLStudioInit_Idempotent_LeavesUnrelatedFilesAlone(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ws")
	paths, _ := Scaffold(root)

	if err := (FLStudioProvider{}).Initialize(paths); err != nil {
		t.Fatal(err)
	}
	// A user-owned file sitting next to the template must survive re-init.
	userFile := filepath.Join(paths.Template, "user-notes.txt")
	if err := os.WriteFile(userFile, []byte("my notes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (FLStudioProvider{}).Initialize(paths); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(userFile)
	if err != nil {
		t.Errorf("user file destroyed by re-init: %v", err)
	}
	if string(got) != "my notes" {
		t.Errorf("user file mutated: got %q", string(got))
	}
}

func TestCreate_FLStudioDefault_ProducesFullTemplate(t *testing.T) {
	// End-to-end via the service + default catalog + default template.
	s := newServiceForTest(t)
	ws, err := s.Create(validRequest(s.Catalog))
	if err != nil {
		t.Fatal(err)
	}
	// Every expected asset is present.
	for _, rel := range expectedFLStudioFiles {
		if _, err := os.Stat(filepath.Join(ws.Paths.Template, filepath.FromSlash(rel))); err != nil {
			t.Errorf("missing after Create: %q (%v)", rel, err)
		}
	}
}
