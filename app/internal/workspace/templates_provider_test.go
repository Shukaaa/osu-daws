package workspace

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- FL Studio provider --------------------------------------------------

func TestFLStudioProvider_Descriptor(t *testing.T) {
	d := FLStudioProvider{}.Descriptor()
	if d.DAW != DAWFLStudio {
		t.Errorf("DAW = %q, want %q", d.DAW, DAWFLStudio)
	}
	if d.ID != FLStudioTemplateID {
		t.Errorf("ID = %q, want %q", d.ID, FLStudioTemplateID)
	}
	if d.Version == "" {
		t.Error("Version must not be empty")
	}
	if d.Label == "" {
		t.Error("Label must not be empty (UI relies on it)")
	}
}

func TestFLStudioProvider_Initialize_WritesMarker(t *testing.T) {
	orig := nowTemplate
	fixed := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	nowTemplate = func() time.Time { return fixed }
	t.Cleanup(func() { nowTemplate = orig })

	root := filepath.Join(t.TempDir(), "ws")
	paths, err := Scaffold(root)
	if err != nil {
		t.Fatal(err)
	}

	if err := (FLStudioProvider{}).Initialize(paths); err != nil {
		t.Fatalf("init: %v", err)
	}

	markerPath := filepath.Join(paths.Template, templateInfoFileName)
	data, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("marker not written: %v", err)
	}

	var info TemplateInfo
	if err := json.Unmarshal(data, &info); err != nil {
		t.Fatalf("marker is not valid JSON: %v", err)
	}
	if info.TemplateID != FLStudioTemplateID {
		t.Errorf("TemplateID = %q", info.TemplateID)
	}
	if info.DAW != DAWFLStudio {
		t.Errorf("DAW = %q", info.DAW)
	}
	if info.Version == "" {
		t.Error("Version missing in marker")
	}
	if !info.InitializedAt.Equal(fixed) {
		t.Errorf("InitializedAt = %v, want %v", info.InitializedAt, fixed)
	}
}

func TestFLStudioProvider_Initialize_Idempotent(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ws")
	paths, _ := Scaffold(root)

	if err := (FLStudioProvider{}).Initialize(paths); err != nil {
		t.Fatal(err)
	}
	// Drop an unrelated file into template/ — it must survive re-init.
	sentinel := filepath.Join(paths.Template, "keep.txt")
	if err := os.WriteFile(sentinel, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (FLStudioProvider{}).Initialize(paths); err != nil {
		t.Fatalf("re-init failed: %v", err)
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Errorf("sentinel destroyed by re-init: %v", err)
	}
}

func TestFLStudioProvider_Initialize_CreatesTemplateDir(t *testing.T) {
	// Even if the template dir doesn't exist yet, Initialize must
	// create it. This matches the contract that CreateWorkspace has
	// already scaffolded paths but we don't assume more than that.
	root := t.TempDir()
	paths := PathsFromRoot(filepath.Join(root, "fresh"))

	if err := (FLStudioProvider{}).Initialize(paths); err != nil {
		t.Fatalf("init: %v", err)
	}
	if info, err := os.Stat(paths.Template); err != nil || !info.IsDir() {
		t.Errorf("template dir missing: %v", err)
	}
}

// --- Catalog lookup -----------------------------------------------------

func TestTemplateCatalog_ProviderByID(t *testing.T) {
	c := NewDefaultCatalog()
	p, ok := c.ProviderByID(FLStudioTemplateID)
	if !ok {
		t.Fatal("expected FL Studio provider to be registered")
	}
	if p.Descriptor().DAW != DAWFLStudio {
		t.Errorf("provider.DAW = %q", p.Descriptor().DAW)
	}
	if _, ok := c.ProviderByID("ghost"); ok {
		t.Error("ProviderByID(ghost) must return false")
	}
}

func TestTemplateCatalog_DefaultProvider(t *testing.T) {
	c := NewDefaultCatalog()
	if c.DefaultProvider().Descriptor().ID != FLStudioTemplateID {
		t.Errorf("default provider ID = %q", c.DefaultProvider().Descriptor().ID)
	}
}

func TestTemplateCatalog_DefaultProvider_PanicsWhenEmpty(t *testing.T) {
	c := &TemplateCatalog{} // empty, no providers
	defer func() {
		if recover() == nil {
			t.Error("expected panic for empty catalog")
		}
	}()
	_ = c.DefaultProvider()
}

func TestNewTemplateCatalog_DuplicateIDPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected panic on duplicate template ID")
		}
	}()
	_ = NewTemplateCatalog(FLStudioProvider{}, FLStudioProvider{})
}

// --- Spy provider for wiring tests --------------------------------------

// spyProvider records whether Initialize was called, on which paths,
// and lets tests force Initialize to return a canned error.
type spyProvider struct {
	desc      TemplateDescriptor
	initErr   error
	initCalls int
	initPaths Paths
}

func (s *spyProvider) Descriptor() TemplateDescriptor { return s.desc }

func (s *spyProvider) Initialize(paths Paths) error {
	s.initCalls++
	s.initPaths = paths
	return s.initErr
}

func TestSpyProvider_SatisfiesInterface(t *testing.T) {
	// Compile-time check: *spyProvider must implement TemplateProvider.
	var _ TemplateProvider = (*spyProvider)(nil)
	// And it's usable through NewTemplateCatalog.
	sp := &spyProvider{desc: TemplateDescriptor{
		DAW: DAWFLStudio, ID: "spy", Label: "Spy", Version: "0",
	}}
	cat := NewTemplateCatalog(sp)
	if _, ok := cat.ProviderByID("spy"); !ok {
		t.Error("spy provider not registered")
	}
	if errors.Is(sp.Initialize(Paths{}), errors.New("unused")) {
		t.Fatal("unreachable")
	}
}
