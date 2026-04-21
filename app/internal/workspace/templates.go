package workspace

// TemplateDescriptor describes a DAW template the user can pick when
// creating a workspace. A TemplateDescriptor is the catalog entry; the
// actual .flp (or equivalent) files are not copied yet — template copy
// is a later step. For now, selecting a template only records which one
// was chosen into project.odaw.
type TemplateDescriptor struct {
	// DAW is the template family (e.g. FL Studio).
	DAW DAWType
	// ID is the stable catalog key, also written into project.odaw.
	ID string
	// Label is the user-facing name shown in the UI.
	Label string
	// Version tags the template revision for future migrations.
	Version string
	// EntryFile is the relative path (from paths.Template) to the main project file to open.
	EntryFile string
}

// AsRef returns the compact TemplateRef form used inside project.odaw.
func (t TemplateDescriptor) AsRef() TemplateRef {
	return TemplateRef{DAW: t.DAW, ID: t.ID, Version: t.Version}
}

// TemplateProvider is the extension point for a DAW/template. A
// provider knows its own metadata (via Descriptor) and performs the
// DAW-specific initialization of a freshly scaffolded workspace (via
// Initialize). Initialize is called after CreateWorkspace has made the
// directory layout, so <paths.Template>/ already exists and is empty
// on first run.
//
// Implementations should be pure w.r.t. Descriptor() — same values on
// every call — and idempotent w.r.t. Initialize where possible (safe
// to re-run on an existing workspace).
type TemplateProvider interface {
	// Descriptor returns the catalog entry describing this template.
	Descriptor() TemplateDescriptor
	// Initialize performs DAW-specific setup inside the workspace. The
	// supplied Paths are already scaffolded.
	Initialize(paths Paths) error
}

// TemplateCatalog is a read-only registry of available templates.
// Internally it stores TemplateProviders; callers can retrieve just
// the descriptor metadata via List/ByID/Default, and look up the live
// provider via ProviderByID / DefaultProvider.
//
// The catalog is intentionally tiny today: only the FL Studio hitsound
// template is shipped. The shape is ready for more DAWs to slot in
// later without changing the UI layer.
type TemplateCatalog struct {
	providers []TemplateProvider
}

// NewTemplateCatalog builds a catalog from the given providers. It
// panics on duplicate template IDs, which is a programmer error. Tests
// use this constructor to inject spy providers.
func NewTemplateCatalog(providers ...TemplateProvider) *TemplateCatalog {
	seen := map[string]struct{}{}
	for _, p := range providers {
		id := p.Descriptor().ID
		if _, dup := seen[id]; dup {
			panic("workspace: duplicate template ID in catalog: " + id)
		}
		seen[id] = struct{}{}
	}
	return &TemplateCatalog{providers: providers}
}

// NewDefaultCatalog returns the catalog shipped with the current build.
func NewDefaultCatalog() *TemplateCatalog {
	return NewTemplateCatalog(
		FLStudioProvider{},
	)
}

// List returns all catalog entries in a stable order.
func (c *TemplateCatalog) List() []TemplateDescriptor {
	out := make([]TemplateDescriptor, len(c.providers))
	for i, p := range c.providers {
		out[i] = p.Descriptor()
	}
	return out
}

// ByID returns the descriptor with the given catalog ID, if any.
func (c *TemplateCatalog) ByID(id string) (TemplateDescriptor, bool) {
	if p, ok := c.ProviderByID(id); ok {
		return p.Descriptor(), true
	}
	return TemplateDescriptor{}, false
}

// Default returns the first catalog entry. Used as the default UI
// selection. Panics only if the catalog is empty, which is a
// programmer error for a shipped build.
func (c *TemplateCatalog) Default() TemplateDescriptor {
	return c.DefaultProvider().Descriptor()
}

// ProviderByID looks up the live provider with the given catalog ID.
// Returns (nil, false) when the ID is unknown.
func (c *TemplateCatalog) ProviderByID(id string) (TemplateProvider, bool) {
	for _, p := range c.providers {
		if p.Descriptor().ID == id {
			return p, true
		}
	}
	return nil, false
}

// DefaultProvider returns the first registered provider. Panics when
// the catalog is empty, which is a programmer error for a shipped
// build.
func (c *TemplateCatalog) DefaultProvider() TemplateProvider {
	if len(c.providers) == 0 {
		panic("workspace: TemplateCatalog is empty")
	}
	return c.providers[0]
}
