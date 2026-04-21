package ui

import (
	"fmt"
	"strings"
	"time"

	"osu-daws-app/internal/workspace"
)

// StartViewModel is the UI-free logic layer for the workspace overview
// screen. It lists existing workspaces and creates new ones.
type StartViewModel struct {
	ProjectsRoot      string
	LastReferencePath string
	Catalog           *workspace.TemplateCatalog

	SearchQuery string

	create *workspace.CreateService
	now    func() time.Time

	lastList *workspace.ListResult
}

// NewStartViewModel returns a StartViewModel bound to projectsRoot.
func NewStartViewModel(projectsRoot string) *StartViewModel {
	cat := workspace.NewDefaultCatalog()
	return &StartViewModel{
		ProjectsRoot:      projectsRoot,
		LastReferencePath: "",
		Catalog:           cat,
		create:            workspace.NewCreateService(projectsRoot, cat),
		now:               func() time.Time { return time.Now().UTC() },
	}
}

// SetClock overrides the time source; used by tests.
func (svm *StartViewModel) SetClock(f func() time.Time) {
	svm.now = f
	if svm.create != nil {
		svm.create.SetClock(f)
	}
}

// SetStatFunc overrides the filesystem stat function used during create validation.
func (svm *StartViewModel) SetStatFunc(f workspace.StatFunc) {
	if svm.create != nil {
		svm.create.SetStatFunc(f)
	}
}

// Refresh rescans the projects root. Missing roots produce an empty list.
func (svm *StartViewModel) Refresh() error {
	res, err := workspace.ListWorkspaces(svm.ProjectsRoot)
	if err != nil {
		return fmt.Errorf("cannot list workspaces: %w", err)
	}
	svm.lastList = res
	return nil
}

// Workspaces returns the last refreshed workspace summaries in stable order.
func (svm *StartViewModel) Workspaces() []workspace.Summary {
	if svm.lastList == nil {
		return nil
	}
	return svm.lastList.Workspaces
}

// FilteredWorkspaces returns Workspaces() filtered by SearchQuery as a
// case-insensitive substring match against Summary.Name. An empty query
// returns the full list unchanged.
func (svm *StartViewModel) FilteredWorkspaces() []workspace.Summary {
	all := svm.Workspaces()
	q := strings.TrimSpace(strings.ToLower(svm.SearchQuery))
	if q == "" {
		return all
	}
	out := make([]workspace.Summary, 0, len(all))
	for _, s := range all {
		if strings.Contains(strings.ToLower(s.Name), q) {
			out = append(out, s)
		}
	}
	return out
}

// Skipped returns directories that looked like workspaces but could not be loaded.
func (svm *StartViewModel) Skipped() []workspace.SkippedEntry {
	if svm.lastList == nil {
		return nil
	}
	return svm.lastList.Skipped
}

// CreateWorkspace validates the request, scaffolds the workspace on disk, and refreshes the list.
func (svm *StartViewModel) CreateWorkspace(req workspace.CreateRequest) (*workspace.Workspace, error) {
	ws, err := svm.create.Create(req)
	if err != nil {
		return nil, err
	}
	_ = svm.Refresh()
	return ws, nil
}

func (svm *StartViewModel) ImportWorkspaceFromZip(zipPath string) (*workspace.Workspace, error) {
	ws, err := workspace.ImportWorkspace(svm.ProjectsRoot, zipPath)
	if err != nil {
		return nil, err
	}
	_ = svm.Refresh()
	return ws, nil
}

func (svm *StartViewModel) ExportWorkspaceToZip(summary workspace.Summary, destZip string) error {
	ws, err := workspace.LoadWorkspace(summary.Root)
	if err != nil {
		return err
	}
	return workspace.ExportWorkspace(ws, destZip)
}

func (svm *StartViewModel) MarkOpened(ws *workspace.Workspace) {
	if ws == nil || ws.Project == nil {
		return
	}
	_ = workspace.SaveLastOpened(svm.ProjectsRoot, ws.Project.ID)
}

func (svm *StartViewModel) LastOpenedSummary() (workspace.Summary, bool) {
	id, ok, _ := workspace.LoadLastOpened(svm.ProjectsRoot)
	if !ok {
		return workspace.Summary{}, false
	}
	for _, s := range svm.Workspaces() {
		if s.ID == id {
			return s, true
		}
	}
	return workspace.Summary{}, false
}

func (svm *StartViewModel) Archived() []workspace.Summary {
	if svm.lastList == nil {
		return nil
	}
	return svm.lastList.Archived
}

func (svm *StartViewModel) FilteredArchived() []workspace.Summary {
	all := svm.Archived()
	q := strings.TrimSpace(strings.ToLower(svm.SearchQuery))
	if q == "" {
		return all
	}
	out := make([]workspace.Summary, 0, len(all))
	for _, s := range all {
		if strings.Contains(strings.ToLower(s.Name), q) {
			out = append(out, s)
		}
	}
	return out
}

func (svm *StartViewModel) SetArchived(summary workspace.Summary, archived bool) error {
	paths := workspace.PathsFromRoot(summary.Root)
	if err := workspace.SetArchived(paths, archived); err != nil {
		return err
	}
	return svm.Refresh()
}
