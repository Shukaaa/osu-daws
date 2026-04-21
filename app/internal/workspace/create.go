package workspace

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"osu-daws-app/internal/domain"
)

// CreateRequest collects the inputs for a new workspace.
type CreateRequest struct {
	Name             string
	ReferenceOsuPath string
	Template         TemplateDescriptor
	DefaultSampleset domain.Sampleset
}

// FieldErrors maps a form field name to a user-readable message. Field
// names are stable so UI code can bind them to specific input widgets.
type FieldErrors map[string]string

const (
	FieldName             = "name"
	FieldReferenceOsuPath = "reference_osu_path"
	FieldTemplate         = "template"
	FieldDefaultSampleset = "default_sampleset"
)

func (f FieldErrors) OK() bool { return len(f) == 0 }

func (f FieldErrors) sortedFields() []string {
	fields := make([]string, 0, len(f))
	for k := range f {
		fields = append(fields, k)
	}
	sort.Strings(fields)
	return fields
}

func (f FieldErrors) Error() string {
	if f.OK() {
		return "no errors"
	}
	var b strings.Builder
	b.WriteString("invalid workspace request:")
	for _, name := range f.sortedFields() {
		b.WriteString(" ")
		b.WriteString(name)
		b.WriteString("=")
		b.WriteString(f[name])
		b.WriteString(";")
	}
	return b.String()
}

// StatFunc reads filesystem metadata. Injected so tests can validate
// reference paths without touching the real disk.
type StatFunc func(path string) (os.FileInfo, error)

// CreateService builds new workspaces from validated CreateRequests.
type CreateService struct {
	ProjectsRoot string
	Catalog      *TemplateCatalog

	now  func() time.Time
	stat StatFunc
}

// NewCreateService wires a service with production defaults.
func NewCreateService(projectsRoot string, cat *TemplateCatalog) *CreateService {
	return &CreateService{
		ProjectsRoot: projectsRoot,
		Catalog:      cat,
		now:          func() time.Time { return time.Now().UTC() },
		stat:         os.Stat,
	}
}

func (s *CreateService) SetClock(f func() time.Time) { s.now = f }

func (s *CreateService) SetStatFunc(f StatFunc) { s.stat = f }

// Validate returns FieldErrors describing every invalid field; an empty
// map means the request is good to execute.
func (s *CreateService) Validate(req CreateRequest) FieldErrors {
	errs := FieldErrors{}

	if strings.TrimSpace(req.Name) == "" {
		errs[FieldName] = "Name is required."
	}

	switch {
	case s.Catalog == nil:
		errs[FieldTemplate] = "No template catalog is available."
	case req.Template.ID == "":
		errs[FieldTemplate] = "Template is required."
	default:
		if _, ok := s.Catalog.ByID(req.Template.ID); !ok {
			errs[FieldTemplate] = "Unknown template: " + req.Template.ID
		} else if !req.Template.DAW.IsValid() {
			errs[FieldTemplate] = "Unsupported DAW: " + string(req.Template.DAW)
		}
	}

	switch {
	case req.DefaultSampleset == "":
		errs[FieldDefaultSampleset] = "Default sampleset is required."
	case !req.DefaultSampleset.IsValid():
		errs[FieldDefaultSampleset] = "Unsupported default sampleset: " + string(req.DefaultSampleset)
	}

	if ref := strings.TrimSpace(req.ReferenceOsuPath); ref != "" {
		if !strings.EqualFold(filepath.Ext(ref), ".osu") {
			errs[FieldReferenceOsuPath] = "Reference file must have a .osu extension."
		} else if s.stat != nil {
			info, err := s.stat(ref)
			switch {
			case err != nil && errors.Is(err, fs.ErrNotExist):
				errs[FieldReferenceOsuPath] = "Reference file does not exist: " + ref
			case err != nil:
				errs[FieldReferenceOsuPath] = "Cannot access reference file: " + err.Error()
			case info != nil && info.IsDir():
				errs[FieldReferenceOsuPath] = "Reference path is a directory, not a file."
			}
		}
	}

	return errs
}

// Create validates the request, builds the project file and scaffolds
// the workspace directory. On validation failure the returned error is
// the FieldErrors map, so callers can type-assert for per-field messages.
//
// After scaffolding, the selected TemplateProvider's Initialize hook
// runs. Initialization failures are surfaced without deleting the
// partially-created workspace so the user can retry.
func (s *CreateService) Create(req CreateRequest) (*Workspace, error) {
	if errs := s.Validate(req); !errs.OK() {
		return nil, errs
	}

	name := strings.TrimSpace(req.Name)
	pf := NewProjectFile(
		NewID(name),
		name,
		req.Template.AsRef(),
		s.now(),
	)
	pf.DefaultSampleset = req.DefaultSampleset
	pf.ReferenceOsuPath = strings.TrimSpace(req.ReferenceOsuPath)

	ws, err := CreateWorkspace(s.ProjectsRoot, pf)
	if err != nil {
		return nil, err
	}

	if s.Catalog != nil {
		if provider, ok := s.Catalog.ProviderByID(req.Template.ID); ok {
			if err := provider.Initialize(ws.Paths); err != nil {
				return ws, &Error{Code: ErrIO,
					Message: "template initialization failed", Cause: err}
			}
		}
	}
	return ws, nil
}
