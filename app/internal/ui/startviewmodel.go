package ui

import (
	"fmt"
	"time"

	"osu-daws-app/internal/workspace"
)

// StartViewModel is the UI-free logic layer for the workspace overview
// screen. It lists existing workspaces and creates new ones.
type StartViewModel struct {
	ProjectsRoot      string
	LastReferencePath string
	Catalog           *workspace.TemplateCatalog

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
