package workspace

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"osu-daws-app/internal/domain"
)

// fakeStat returns a StatFunc that knows a fixed set of files/dirs.
// Keys are absolute paths; values are true for directories.
func fakeStat(files map[string]bool) StatFunc {
	return func(path string) (os.FileInfo, error) {
		isDir, ok := files[path]
		if !ok {
			return nil, fs.ErrNotExist
		}
		return fakeFileInfo{name: filepath.Base(path), isDir: isDir}, nil
	}
}

type fakeFileInfo struct {
	name  string
	isDir bool
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return 0 }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return f.isDir }
func (f fakeFileInfo) Sys() any           { return nil }

func newServiceForTest(t *testing.T) *CreateService {
	t.Helper()
	root := filepath.Join(t.TempDir(), "projects")
	s := NewCreateService(root, NewDefaultCatalog())
	s.SetClock(func() time.Time {
		return time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	})
	s.SetStatFunc(fakeStat(nil))
	return s
}

func validRequest(cat *TemplateCatalog) CreateRequest {
	return CreateRequest{
		Name:             "My Song",
		Template:         cat.Default(),
		DefaultSampleset: domain.SamplesetSoft,
	}
}

func TestValidate_HappyPath(t *testing.T) {
	s := newServiceForTest(t)
	errs := s.Validate(validRequest(s.Catalog))
	if !errs.OK() {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidate_FieldErrors_TableDriven(t *testing.T) {
	cat := NewDefaultCatalog()
	base := validRequest(cat)

	cases := []struct {
		name      string
		mutate    func(*CreateRequest)
		wantField string
		wantSub   string
	}{
		{"empty name", func(r *CreateRequest) { r.Name = "" }, FieldName, "required"},
		{"whitespace name", func(r *CreateRequest) { r.Name = "   " }, FieldName, "required"},
		{"missing template", func(r *CreateRequest) { r.Template = TemplateDescriptor{} }, FieldTemplate, "required"},
		{"unknown template", func(r *CreateRequest) {
			r.Template = TemplateDescriptor{DAW: DAWFLStudio, ID: "ghost"}
		}, FieldTemplate, "Unknown template"},
		{"invalid daw", func(r *CreateRequest) {
			r.Template = TemplateDescriptor{DAW: "ableton", ID: "osu!daw hitsound template"}
		}, FieldTemplate, "Unsupported DAW"},
		{"empty sampleset", func(r *CreateRequest) { r.DefaultSampleset = "" }, FieldDefaultSampleset, "required"},
		{"unknown sampleset", func(r *CreateRequest) { r.DefaultSampleset = "funky" }, FieldDefaultSampleset, "Unsupported"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newServiceForTest(t)
			req := base
			c.mutate(&req)
			errs := s.Validate(req)
			if errs.OK() {
				t.Fatalf("expected error on %s", c.wantField)
			}
			got, ok := errs[c.wantField]
			if !ok {
				t.Fatalf("field %q not reported; errs = %v", c.wantField, errs)
			}
			if !strings.Contains(got, c.wantSub) {
				t.Errorf("message %q should contain %q", got, c.wantSub)
			}
		})
	}
}

func TestValidate_ReferencePath_Optional(t *testing.T) {
	s := newServiceForTest(t)
	req := validRequest(s.Catalog)
	req.ReferenceOsuPath = "" // not supplied
	if errs := s.Validate(req); !errs.OK() {
		t.Errorf("empty reference path should be OK, got %v", errs)
	}
}

func TestValidate_ReferencePath_WrongExt(t *testing.T) {
	s := newServiceForTest(t)
	req := validRequest(s.Catalog)
	req.ReferenceOsuPath = `C:\maps\song.txt`
	errs := s.Validate(req)
	if errs.OK() {
		t.Fatal("expected error")
	}
	if !strings.Contains(errs[FieldReferenceOsuPath], ".osu extension") {
		t.Errorf("msg = %q", errs[FieldReferenceOsuPath])
	}
}

func TestValidate_ReferencePath_Missing(t *testing.T) {
	s := newServiceForTest(t)
	s.SetStatFunc(fakeStat(nil))
	req := validRequest(s.Catalog)
	req.ReferenceOsuPath = `C:\maps\missing.osu`
	errs := s.Validate(req)
	if !strings.Contains(errs[FieldReferenceOsuPath], "does not exist") {
		t.Errorf("msg = %q", errs[FieldReferenceOsuPath])
	}
}

func TestValidate_ReferencePath_IsDirectory(t *testing.T) {
	s := newServiceForTest(t)
	s.SetStatFunc(fakeStat(map[string]bool{`/tmp/thing.osu`: true}))
	req := validRequest(s.Catalog)
	req.ReferenceOsuPath = `/tmp/thing.osu`
	errs := s.Validate(req)
	if !strings.Contains(errs[FieldReferenceOsuPath], "directory") {
		t.Errorf("msg = %q", errs[FieldReferenceOsuPath])
	}
}

func TestValidate_ReferencePath_Valid(t *testing.T) {
	s := newServiceForTest(t)
	s.SetStatFunc(fakeStat(map[string]bool{`/tmp/ref.osu`: false}))
	req := validRequest(s.Catalog)
	req.ReferenceOsuPath = `/tmp/ref.osu`
	if errs := s.Validate(req); !errs.OK() {
		t.Errorf("expected OK, got %v", errs)
	}
}

func TestCreate_WritesProjectFile(t *testing.T) {
	s := newServiceForTest(t)
	s.SetStatFunc(fakeStat(map[string]bool{`/tmp/ref.osu`: false}))

	req := validRequest(s.Catalog)
	req.ReferenceOsuPath = `/tmp/ref.osu`

	ws, err := s.Create(req)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	pf := ws.Project

	// Identity
	if !strings.HasPrefix(string(pf.ID), "my-song-") {
		t.Errorf("ID = %q, want prefix 'my-song-'", pf.ID)
	}
	if pf.Name != "My Song" {
		t.Errorf("Name = %q", pf.Name)
	}

	// Template persisted without the UI Label field.
	if pf.Template.DAW != DAWFLStudio {
		t.Errorf("Template.DAW = %q", pf.Template.DAW)
	}
	if pf.Template.ID != "osu!daw hitsound template" {
		t.Errorf("Template.ID = %q", pf.Template.ID)
	}

	// Default sampleset and reference path round-tripped.
	if pf.DefaultSampleset != domain.SamplesetSoft {
		t.Errorf("DefaultSampleset = %q", pf.DefaultSampleset)
	}
	if pf.ReferenceOsuPath != `/tmp/ref.osu` {
		t.Errorf("ReferenceOsuPath = %q", pf.ReferenceOsuPath)
	}

	// All directories exist.
	for _, dir := range []string{ws.Paths.Root, ws.Paths.Template, ws.Paths.Exports} {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			t.Errorf("missing dir %q: %v", dir, err)
		}
	}

	// project.odaw exists.
	if _, err := os.Stat(ws.Paths.ProjectFile); err != nil {
		t.Errorf("project.odaw missing: %v", err)
	}

	// Round-trip through Load.
	loaded, err := LoadWorkspace(ws.Paths.Root)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if loaded.Project.ID != pf.ID || loaded.Project.Name != pf.Name {
		t.Errorf("reload mismatch: %+v", loaded.Project)
	}
}

func TestCreate_ValidationErrorNotWrittenToDisk(t *testing.T) {
	s := newServiceForTest(t)
	req := validRequest(s.Catalog)
	req.Name = "" // invalid

	_, err := s.Create(req)
	if err == nil {
		t.Fatal("expected validation error")
	}
	// Error must be FieldErrors so UI can surface per-field messages.
	fe, ok := err.(FieldErrors)
	if !ok {
		t.Fatalf("error type = %T, want FieldErrors", err)
	}
	if _, has := fe[FieldName]; !has {
		t.Errorf("expected FieldName error, got %v", fe)
	}

	// Projects root must still be empty.
	entries, _ := os.ReadDir(s.ProjectsRoot)
	if len(entries) != 0 {
		t.Errorf("expected no workspace dirs, got %d", len(entries))
	}
}

func TestCreate_TrimsReferencePath(t *testing.T) {
	s := newServiceForTest(t)
	s.SetStatFunc(fakeStat(map[string]bool{`/tmp/ref.osu`: false}))
	req := validRequest(s.Catalog)
	req.ReferenceOsuPath = "  /tmp/ref.osu  "
	ws, err := s.Create(req)
	if err != nil {
		t.Fatal(err)
	}
	if ws.Project.ReferenceOsuPath != `/tmp/ref.osu` {
		t.Errorf("ReferenceOsuPath = %q, want trimmed", ws.Project.ReferenceOsuPath)
	}
}

func TestFieldErrors_ErrorString(t *testing.T) {
	fe := FieldErrors{FieldName: "Name is required.", FieldTemplate: "Template is required."}
	msg := fe.Error()
	// Deterministic ordering in the message.
	if !strings.Contains(msg, FieldName) || !strings.Contains(msg, FieldTemplate) {
		t.Errorf("error string missing field names: %q", msg)
	}
}

// --- Template provider wiring -------------------------------------------

func TestCreate_InvokesProviderInitialize(t *testing.T) {
	sp := &spyProvider{desc: TemplateDescriptor{
		DAW: DAWFLStudio, ID: "spy-tpl", Label: "Spy", Version: "1",
	}}
	s := newServiceForTest(t)
	s.Catalog = NewTemplateCatalog(sp)

	req := validRequest(s.Catalog)
	ws, err := s.Create(req)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if sp.initCalls != 1 {
		t.Errorf("Initialize called %d times, want 1", sp.initCalls)
	}
	if sp.initPaths.Root != ws.Paths.Root {
		t.Errorf("Initialize paths.Root = %q, want %q", sp.initPaths.Root, ws.Paths.Root)
	}
	if sp.initPaths.Template != ws.Paths.Template {
		t.Errorf("Initialize paths.Template = %q, want %q",
			sp.initPaths.Template, ws.Paths.Template)
	}
}

func TestCreate_ProviderInitializeErrorSurfaces(t *testing.T) {
	sp := &spyProvider{
		desc: TemplateDescriptor{
			DAW: DAWFLStudio, ID: "bad-tpl", Label: "Bad", Version: "1",
		},
		initErr: os.ErrPermission,
	}
	s := newServiceForTest(t)
	s.Catalog = NewTemplateCatalog(sp)

	req := validRequest(s.Catalog)
	ws, err := s.Create(req)
	if err == nil {
		t.Fatal("expected error when Initialize fails")
	}
	if ws == nil {
		t.Fatal("workspace should still be returned so UI can surface the path")
	}
	// The error must be structured so callers can distinguish it.
	var werr *Error
	if !errorsAs(err, &werr) || werr.Code != ErrIO {
		t.Errorf("error = %v, want ErrIO", err)
	}
}

func TestCreate_UsesFLStudioProviderByDefault(t *testing.T) {
	// End-to-end: default catalog + default template → marker file written.
	s := newServiceForTest(t)
	req := validRequest(s.Catalog) // uses catalog's Default()

	ws, err := s.Create(req)
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(ws.Paths.Template, templateInfoFileName)
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("FL Studio marker not written: %v", err)
	}

	// And the project file records the selected template.
	if ws.Project.Template.ID != FLStudioTemplateID {
		t.Errorf("project.Template.ID = %q, want %q",
			ws.Project.Template.ID, FLStudioTemplateID)
	}
	if ws.Project.Template.DAW != DAWFLStudio {
		t.Errorf("project.Template.DAW = %q, want %q",
			ws.Project.Template.DAW, DAWFLStudio)
	}
}
