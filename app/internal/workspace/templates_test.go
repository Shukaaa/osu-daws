package workspace

import "testing"

func TestDefaultCatalog_HasFLStudio(t *testing.T) {
	c := NewDefaultCatalog()
	list := c.List()
	if len(list) == 0 {
		t.Fatal("expected at least one template")
	}
	var found bool
	for _, e := range list {
		if e.DAW == DAWFLStudio && e.ID == "osu!daw hitsound template" {
			found = true
			if e.Label == "" {
				t.Error("FL Studio template must have a user-facing Label")
			}
		}
	}
	if !found {
		t.Errorf("default catalog missing FL Studio entry, got %+v", list)
	}
}

func TestTemplateCatalog_List_Stable(t *testing.T) {
	c := NewDefaultCatalog()
	a := c.List()
	b := c.List()
	if len(a) != len(b) {
		t.Fatalf("List size varies: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("List[%d] differs: %+v vs %+v", i, a[i], b[i])
		}
	}
	// Mutating the returned slice must not affect the catalog.
	a[0].Label = "mutated"
	if c.List()[0].Label == "mutated" {
		t.Error("List() returned a live reference to the internal slice")
	}
}

func TestTemplateCatalog_ByID(t *testing.T) {
	c := NewDefaultCatalog()
	if _, ok := c.ByID("osu!daw hitsound template"); !ok {
		t.Error("expected FL Studio template to be found by ID")
	}
	if _, ok := c.ByID("nonexistent"); ok {
		t.Error("ByID(nonexistent) should return false")
	}
}

func TestTemplateCatalog_Default(t *testing.T) {
	c := NewDefaultCatalog()
	def := c.Default()
	if def.DAW != DAWFLStudio {
		t.Errorf("default DAW = %q, want %q", def.DAW, DAWFLStudio)
	}
}

func TestTemplateDescriptor_AsRef(t *testing.T) {
	d := TemplateDescriptor{DAW: DAWFLStudio, ID: "x", Label: "X", Version: "2"}
	ref := d.AsRef()
	if ref.DAW != DAWFLStudio || ref.ID != "x" || ref.Version != "2" {
		t.Errorf("AsRef = %+v", ref)
	}
	// Label must not leak — it's a UI-only concern, not part of the
	// persisted project file.
	// (TemplateRef has no Label field, so this is structural.)
}
