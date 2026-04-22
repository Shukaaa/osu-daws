package workspace

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Summary is the compact view of a workspace used by a list screen.
type Summary struct {
	ID               ID
	Name             string
	DAW              DAWType
	ReferenceOsuPath string
	UpdatedAt        time.Time
	Root             string
	Archived         bool
}

// SkippedEntry describes a directory that looked like a workspace but
// could not be loaded. Returning these separately keeps the main list
// alive and lets the UI surface warnings without blocking.
type SkippedEntry struct {
	Path string
	Err  error
}

// ListResult bundles successful summaries and skipped entries.
// Workspaces contains only non-archived entries; Archived is populated
// with the rest so callers can render them in a separate section.
// Both slices share the same sort order: UpdatedAt desc, Name asc, ID asc.
type ListResult struct {
	Workspaces []Summary
	Archived   []Summary
	Skipped    []SkippedEntry
}

// ListWorkspaces scans projectsRoot for workspace directories and loads
// a Summary for each one that has a valid project.odaw. A missing root
// is treated as an empty result (fresh-install case).
func ListWorkspaces(projectsRoot string) (*ListResult, error) {
	entries, err := os.ReadDir(projectsRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &ListResult{}, nil
		}
		return nil, &Error{Code: ErrIO,
			Message: "cannot read projects root: " + projectsRoot, Cause: err}
	}

	result := &ListResult{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		root := filepath.Join(projectsRoot, e.Name())
		paths := PathsFromRoot(root)

		if _, statErr := os.Stat(paths.ProjectFile); statErr != nil {
			if errors.Is(statErr, fs.ErrNotExist) {
				continue
			}
			result.Skipped = append(result.Skipped, SkippedEntry{Path: root, Err: statErr})
			continue
		}

		pf, loadErr := LoadProjectFile(paths)
		if loadErr != nil {
			result.Skipped = append(result.Skipped, SkippedEntry{Path: root, Err: loadErr})
			continue
		}

		s := Summary{
			ID:               pf.ID,
			Name:             pf.Name,
			DAW:              pf.Template.DAW,
			ReferenceOsuPath: pf.ReferenceOsuPath,
			UpdatedAt:        pf.UpdatedAt,
			Root:             root,
			Archived:         pf.Archived,
		}
		if pf.Archived {
			result.Archived = append(result.Archived, s)
		} else {
			result.Workspaces = append(result.Workspaces, s)
		}
	}

	sortSummaries(result.Workspaces)
	sortSummaries(result.Archived)
	sort.SliceStable(result.Skipped, func(i, j int) bool {
		return result.Skipped[i].Path < result.Skipped[j].Path
	})
	return result, nil
}

func sortSummaries(s []Summary) {
	sort.SliceStable(s, func(i, j int) bool {
		a, b := s[i], s[j]
		if !a.UpdatedAt.Equal(b.UpdatedAt) {
			return a.UpdatedAt.After(b.UpdatedAt)
		}
		if a.Name != b.Name {
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		}
		return a.ID < b.ID
	})
}
