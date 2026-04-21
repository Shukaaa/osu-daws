package workspace

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"osu-daws-app/internal/domain"
)

func TestSlug(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"simple lowercase", "mysong", "mysong"},
		{"mixed case lowered", "MySong", "mysong"},
		{"spaces to dash", "My Song", "my-song"},
		{"punctuation collapsed", "My!!Song??", "my-song"},
		{"leading and trailing dashes trimmed", "  -My Song-  ", "my-song"},
		{"multiple spaces collapsed", "a     b", "a-b"},
		{"digits kept", "Song 2024", "song-2024"},
		{"invalid filename chars", `ab:c/d\e`, "ab-c-d-e"},
		{"only punctuation falls back", "!!!---???", "project"},
		{"empty falls back", "", "project"},
		{"whitespace falls back", "   ", "project"},
		{"non-ascii letters dropped", "ヨアソビ Song", "song"},
		{"non-ascii only falls back", "ヨアソビ", "project"},
		{"mix with numbers", "Track 01 - Intro", "track-01-intro"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Slug(c.in)
			if got != c.want {
				t.Errorf("Slug(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestSlug_ResultIsFilesystemSafe(t *testing.T) {
	// Every produced slug must only contain [a-z0-9-] and must not start
	// or end with '-'.
	re := regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	inputs := []string{
		"My Song!",
		"Track 01",
		"ab:c/d\\e",
		"ヨアソビ Song",
		"   ",
		"a     b",
	}
	for _, in := range inputs {
		s := Slug(in)
		if !re.MatchString(s) {
			t.Errorf("Slug(%q) = %q is not filesystem-safe", in, s)
		}
	}
}

func TestNewID_Format(t *testing.T) {
	id := NewID("My Song")
	s := id.String()
	if !strings.HasPrefix(s, "my-song-") {
		t.Errorf("ID %q missing slug prefix", s)
	}
	suffix := strings.TrimPrefix(s, "my-song-")
	if len(suffix) != randomSuffixLen {
		t.Errorf("suffix %q has length %d, want %d", suffix, len(suffix), randomSuffixLen)
	}
	// Suffix must be lowercase hex.
	re := regexp.MustCompile(`^[0-9a-f]+$`)
	if !re.MatchString(suffix) {
		t.Errorf("suffix %q is not lowercase hex", suffix)
	}
}

func TestNewID_Unique(t *testing.T) {
	// Generating many IDs for the same name must not collide in practice.
	seen := map[ID]struct{}{}
	for i := 0; i < 200; i++ {
		id := NewID("Same Name")
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate ID generated: %s", id)
		}
		seen[id] = struct{}{}
	}
}

func TestNewID_EmptyName(t *testing.T) {
	id := NewID("")
	s := id.String()
	if !strings.HasPrefix(s, "project-") {
		t.Errorf("empty-name ID %q should fall back to 'project-' prefix", s)
	}
}

func TestPathsFromRoot(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "tmp", "ws", "my-song-abc123")
	p := PathsFromRoot(root)

	if p.Root != root {
		t.Errorf("Root = %q, want %q", p.Root, root)
	}
	if got, want := p.ProjectFile, filepath.Join(root, "project.odaw"); got != want {
		t.Errorf("ProjectFile = %q, want %q", got, want)
	}
	if got, want := p.Template, filepath.Join(root, "template"); got != want {
		t.Errorf("Template = %q, want %q", got, want)
	}
	if got, want := p.Exports, filepath.Join(root, "exports"); got != want {
		t.Errorf("Exports = %q, want %q", got, want)
	}
}

func TestDAWType_IsValid(t *testing.T) {
	cases := []struct {
		in   DAWType
		want bool
	}{
		{DAWFLStudio, true},
		{"ableton", false},
		{"", false},
	}
	for _, c := range cases {
		if got := c.in.IsValid(); got != c.want {
			t.Errorf("DAWType(%q).IsValid() = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestNewProjectFile_Defaults(t *testing.T) {
	now := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	id := ID("my-song-abc123")
	tmpl := TemplateRef{DAW: DAWFLStudio, ID: "osu!daw hitsound template"}

	p := NewProjectFile(id, "My Song", tmpl, now)

	if p.Version != CurrentProjectFileVersion {
		t.Errorf("Version = %d, want %d", p.Version, CurrentProjectFileVersion)
	}
	if p.ID != id {
		t.Errorf("ID = %q, want %q", p.ID, id)
	}
	if p.Name != "My Song" {
		t.Errorf("Name = %q, want %q", p.Name, "My Song")
	}
	if p.Template != tmpl {
		t.Errorf("Template = %+v, want %+v", p.Template, tmpl)
	}
	if p.DefaultSampleset != domain.SamplesetSoft {
		t.Errorf("DefaultSampleset = %q, want %q", p.DefaultSampleset, domain.SamplesetSoft)
	}
	if p.Segments == nil {
		t.Error("Segments must be non-nil (use empty slice, not nil)")
	}
	if len(p.Segments) != 0 {
		t.Errorf("Segments should start empty, got %d", len(p.Segments))
	}
	if !p.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt = %v, want %v", p.CreatedAt, now)
	}
	if !p.UpdatedAt.Equal(now) {
		t.Errorf("UpdatedAt = %v, want %v", p.UpdatedAt, now)
	}
}

func TestNewWorkspace(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "ws", "my-song-abc123")
	now := time.Now().UTC()
	pf := NewProjectFile("my-song-abc123", "My Song",
		TemplateRef{DAW: DAWFLStudio, ID: "t"}, now)

	ws := NewWorkspace(root, pf)
	if ws == nil {
		t.Fatal("NewWorkspace returned nil")
	}
	if ws.Project != pf {
		t.Error("Workspace.Project must reference the provided file")
	}
	if ws.Paths.Root != root {
		t.Errorf("Paths.Root = %q, want %q", ws.Paths.Root, root)
	}
	if ws.Paths.ProjectFile == "" || ws.Paths.Template == "" || ws.Paths.Exports == "" {
		t.Errorf("Paths not fully populated: %+v", ws.Paths)
	}
}
