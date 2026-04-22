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

type TemplateProvider interface {
	Descriptor() TemplateDescriptor
	Initialize(paths Paths) error
}

type TemplateMigrator interface {
	Migrate(paths Paths, fromVersion string) error
}

var registeredProviders []TemplateProvider

func RegisterProvider(p TemplateProvider) {
	registeredProviders = append(registeredProviders, p)
}

// TemplateCatalog is a read-only registry of available templates.
type TemplateCatalog struct {
	providers []TemplateProvider
}

// NewTemplateCatalog builds a catalog from the given providers. It
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

// NewDefaultCatalog returns the catalog shipped with the current
func NewDefaultCatalog() *TemplateCatalog {
	return NewTemplateCatalog(registeredProviders...)
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
func (c *TemplateCatalog) Default() TemplateDescriptor {
	return c.DefaultProvider().Descriptor()
}

// ProviderByID looks up the live provider with the given catalog ID.
func (c *TemplateCatalog) ProviderByID(id string) (TemplateProvider, bool) {
	for _, p := range c.providers {
		if p.Descriptor().ID == id {
			return p, true
		}
	}
	return nil, false
}

// DefaultProvider returns the first registered provider. Panics when
func (c *TemplateCatalog) DefaultProvider() TemplateProvider {
	if len(c.providers) == 0 {
		panic("workspace: TemplateCatalog is empty")
	}
	return c.providers[0]
}
