package ui

import (
	"fmt"

	"osu-daws-app/internal/sourcemap"
	"osu-daws-app/internal/workspace"
)

// ApplyWorkspaceState populates the ViewModel from an opened workspace.
// The workspace becomes the source of truth for the main view: reference
// path, default sampleset, segments and the exports directory.
func ApplyWorkspaceState(vm *ViewModel, ws *workspace.Workspace) {
	if vm == nil || ws == nil || ws.Project == nil {
		return
	}

	if p := ws.Project.ReferenceOsuPath; p != "" {
		vm.ReferencePath = p
	}
	if s := ws.Project.DefaultSampleset; s != "" {
		vm.DefaultSampleset = s
	}
	vm.SetWorkspaceExportsDir(ws.Paths.Exports)

	if len(ws.Project.Segments) == 0 {
		vm.Segments = []*SegmentInput{{Status: "No SourceMap loaded yet."}}
		return
	}
	out := make([]*SegmentInput, 0, len(ws.Project.Segments))
	for _, s := range ws.Project.Segments {
		out = append(out, &SegmentInput{
			SourceMapJSON: s.SourceMapJSON,
			StartTimeText: s.StartTimeText,
			Status:        deriveSegmentStatus(s.SourceMapJSON),
		})
	}
	vm.Segments = out
}

// PersistToWorkspace writes the current ViewModel state back into the
// workspace's project.odaw. Nil workspaces are a no-op.
func PersistToWorkspace(vm *ViewModel, ws *workspace.Workspace) error {
	if vm == nil || ws == nil || ws.Project == nil {
		return nil
	}

	ws.Project.ReferenceOsuPath = vm.ReferencePath
	if vm.DefaultSampleset != "" {
		ws.Project.DefaultSampleset = vm.DefaultSampleset
	}

	out := make([]workspace.SegmentInput, 0, len(vm.Segments))
	for _, s := range vm.Segments {
		if s == nil {
			continue
		}
		out = append(out, workspace.SegmentInput{
			SourceMapJSON: s.SourceMapJSON,
			StartTimeText: s.StartTimeText,
		})
	}
	ws.Project.Segments = out

	return ws.Save()
}

// deriveSegmentStatus produces a user-facing status line mirroring the
// one generated when reading a SourceMap from the clipboard.
func deriveSegmentStatus(jsonText string) string {
	if jsonText == "" {
		return "No SourceMap loaded yet."
	}
	sm, res := sourcemap.Parse([]byte(jsonText))
	if !res.OK() {
		return "⚠ Stored SourceMap is invalid: " + res.Error()
	}
	return fmt.Sprintf("SourceMap OK · ppq=%d · %d events",
		sm.Meta.PPQ, len(sm.Events))
}
