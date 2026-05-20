package ui

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"osu-daws-app/internal/domain"
	"osu-daws-app/internal/exporter"
	"osu-daws-app/internal/pipeline"
	"osu-daws-app/internal/sourcemap"
)

type ClipboardReader interface {
	Content() string
}

// BeatmapDetector abstracts the detection of the currently selected beatmap
// from a running osu! client.
type BeatmapDetector interface {
	Detect() (string, error)
}

type FileOpener func(path string) (io.ReadCloser, error)

type StatFunc func(path string) (os.FileInfo, error)

func OSFileOpener(path string) (io.ReadCloser, error) {
	return os.Open(path)
}

type SegmentInput struct {
	SourceMapJSON string
	StartTimeText string
	Status        string
}

type ViewModel struct {
	Segments         []*SegmentInput
	ReferencePath    string
	DefaultSampleset domain.Sampleset

	// VolumeStep is the rounding step (in volume %) applied to all source
	// events before generation. 0 disables rounding.
	VolumeStep int

	// VersioningEnabled toggles automatic v1, v2, ... export versioning.
	// When true:
	//   - Generate() injects " v<N>" into the diff name / Metadata.Version.
	//   - SaveToExports() writes to a versioned filename and bumps the counter.
	//   - CopyToOsuProject() copies using the same versioned filename.
	VersioningEnabled bool

	workspaceExportsDir string

	// lastExportVersion mirrors ws.Project.LastExportVersion (0 when unset).
	lastExportVersion int
	// pendingVersion is the version that the currently generated content
	// uses. It equals lastExportVersion+1 while a generation is pending a
	// save, and is promoted to lastExportVersion on a successful save.
	pendingVersion int
	// lastSavedExportPath is the absolute path written by the most recent
	// successful SaveToExports call (may be "" when nothing has been saved).
	lastSavedExportPath string

	clipboard ClipboardReader
	opener    FileOpener
	stat      StatFunc
	detector  BeatmapDetector
}

func NewViewModel(cb ClipboardReader, opener FileOpener) *ViewModel {
	if opener == nil {
		opener = OSFileOpener
	}
	return &ViewModel{
		Segments:         []*SegmentInput{{Status: "No SourceMap loaded yet."}},
		DefaultSampleset: domain.SamplesetSoft,
		clipboard:        cb,
		opener:           opener,
		stat:             os.Stat,
	}
}

// SetDetector configures the beatmap detector used by DetectCurrentBeatmap.
func (vm *ViewModel) SetDetector(d BeatmapDetector) { vm.detector = d }

func (vm *ViewModel) SetStatFunc(f StatFunc) { vm.stat = f }

// SetWorkspaceExportsDir points the VM at the active workspace's exports/
// directory. Pass "" to disable workspace auto-save.
func (vm *ViewModel) SetWorkspaceExportsDir(dir string) {
	vm.workspaceExportsDir = dir
}

func (vm *ViewModel) WorkspaceExportsDir() string {
	return vm.workspaceExportsDir
}

// SetLastExportVersion seeds the counter used for automatic versioning
// (typically loaded from the active workspace's project.odaw).
func (vm *ViewModel) SetLastExportVersion(v int) {
	if v < 0 {
		v = 0
	}
	vm.lastExportVersion = v
	vm.pendingVersion = 0
}

// LastExportVersion returns the highest version written to exports/.
func (vm *ViewModel) LastExportVersion() int { return vm.lastExportVersion }

// LastSavedExportPath returns the most recent workspace export path, or "".
func (vm *ViewModel) LastSavedExportPath() string { return vm.lastSavedExportPath }

func (vm *ViewModel) LatestExportedDiffPath() (string, error) {
	if vm.lastSavedExportPath != "" {
		if info, err := os.Stat(vm.lastSavedExportPath); err == nil && !info.IsDir() {
			return vm.lastSavedExportPath, nil
		}
	}
	if vm.workspaceExportsDir == "" {
		return "", fmt.Errorf("no active workspace exports folder")
	}
	entries, err := os.ReadDir(vm.workspaceExportsDir)
	if err != nil {
		return "", fmt.Errorf("cannot read workspace exports folder: %w", err)
	}

	type candidate struct {
		path string
		mod  int64
	}
	var candidates []candidate
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".osu") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		candidates = append(candidates, candidate{
			path: filepath.Join(vm.workspaceExportsDir, entry.Name()),
			mod:  info.ModTime().UnixNano(),
		})
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("no generated hitsound diff found in workspace exports. Please Generate first")
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].mod == candidates[j].mod {
			return candidates[i].path > candidates[j].path
		}
		return candidates[i].mod > candidates[j].mod
	})
	return candidates[0].path, nil
}

func (vm *ViewModel) DefaultHSDiffZipPath() (string, error) {
	diffPath, err := vm.LatestExportedDiffPath()
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(diffPath, filepath.Ext(diffPath)) + ".zip", nil
}

func (vm *ViewModel) ExportHSDiffZip(destZip string) (*exporter.HitsoundZipResult, error) {
	diffPath, err := vm.LatestExportedDiffPath()
	if err != nil {
		return nil, err
	}
	beatmapDir := vm.DefaultSaveDir()
	if beatmapDir == "" {
		return nil, fmt.Errorf("cannot determine osu! beatmap folder from reference path")
	}
	return exporter.CreateHitsoundZip(destZip, diffPath, beatmapDir)
}

// effectiveVersion is the version number used for filenames and diff
// name injection. 0 means "no version suffix".
func (vm *ViewModel) effectiveVersion() int {
	if !vm.VersioningEnabled {
		return 0
	}
	if vm.pendingVersion > 0 {
		return vm.pendingVersion
	}
	return vm.lastExportVersion + 1
}

// SaveToExports writes res.OsuContent to the active workspace's exports/
// folder using the canonical osu-style filename derived from the reference
// map metadata. Returns the absolute path written to.
func (vm *ViewModel) SaveToExports(res *pipeline.Result) (string, error) {
	if vm.workspaceExportsDir == "" {
		return "", fmt.Errorf("no active workspace")
	}
	if res == nil || res.OsuContent == "" {
		return "", fmt.Errorf("no generated content to save")
	}
	if err := os.MkdirAll(vm.workspaceExportsDir, 0o755); err != nil {
		return "", fmt.Errorf("cannot create exports directory: %w", err)
	}
	version := vm.effectiveVersion()
	path := exporter.DefaultExportPathVersioned(vm.workspaceExportsDir, res.Reference, version)
	if err := os.WriteFile(path, []byte(res.OsuContent), 0o644); err != nil {
		return "", fmt.Errorf("cannot write export: %w", err)
	}
	if version > 0 {
		vm.lastExportVersion = version
		vm.pendingVersion = 0
	}
	vm.lastSavedExportPath = path
	return path, nil
}

func (vm *ViewModel) CopyToOsuProject(res *pipeline.Result) (string, error) {
	dir := vm.DefaultSaveDir()
	if dir == "" {
		return "", fmt.Errorf("cannot determine osu! project directory from reference path")
	}
	if res == nil || res.OsuContent == "" {
		return "", fmt.Errorf("no generated content to export")
	}
	version := 0
	if vm.VersioningEnabled {
		version = vm.lastExportVersion
		if version == 0 {
			version = vm.effectiveVersion()
		}
	}
	path := exporter.DefaultExportPathVersioned(dir, res.Reference, version)
	if err := os.WriteFile(path, []byte(res.OsuContent), 0o644); err != nil {
		return "", fmt.Errorf("cannot write to osu! project: %w", err)
	}
	return path, nil
}

func (vm *ViewModel) ValidateReferencePath() error {
	p := strings.TrimSpace(vm.ReferencePath)
	if p == "" {
		return fmt.Errorf("reference path is required")
	}
	if !strings.EqualFold(filepath.Ext(p), ".osu") {
		return fmt.Errorf("reference file must have a .osu extension")
	}
	if vm.stat == nil {
		return nil
	}
	info, err := vm.stat(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("reference file does not exist: %s", p)
		}
		return fmt.Errorf("cannot access reference file: %v", err)
	}
	if info.IsDir() {
		return fmt.Errorf("reference path is a directory, not a file: %s", p)
	}
	return nil
}

func (vm *ViewModel) DefaultSaveDir() string {
	p := strings.TrimSpace(vm.ReferencePath)
	if p == "" {
		return ""
	}
	dir := filepath.Dir(p)
	if dir == "" || dir == "." {
		return ""
	}
	if vm.stat == nil {
		return dir
	}
	info, err := vm.stat(dir)
	if err != nil || !info.IsDir() {
		return ""
	}
	return dir
}

func (vm *ViewModel) ReadSourceMapFromClipboard() (string, error) {
	return vm.ReadSegmentSourceMapFromClipboard(0)
}

func (vm *ViewModel) AddSegment() int {
	vm.Segments = append(vm.Segments, &SegmentInput{Status: "No SourceMap loaded yet."})
	return len(vm.Segments) - 1
}

func (vm *ViewModel) RemoveSegment(index int) bool {
	if index < 0 || index >= len(vm.Segments) {
		return false
	}
	if len(vm.Segments) <= 1 {
		return false
	}
	vm.Segments = append(vm.Segments[:index], vm.Segments[index+1:]...)
	return true
}

func (vm *ViewModel) ReadSegmentSourceMapFromClipboard(index int) (string, error) {
	if index < 0 || index >= len(vm.Segments) {
		return "", fmt.Errorf("segment index %d out of range", index)
	}
	if vm.clipboard == nil {
		return "", fmt.Errorf("clipboard is not available")
	}
	raw := strings.TrimSpace(vm.clipboard.Content())
	if raw == "" {
		return "", fmt.Errorf("clipboard is empty")
	}
	sm, res := sourcemap.Parse([]byte(raw))
	if !res.OK() {
		return "", fmt.Errorf("invalid SourceMap: %s", res.Error())
	}
	vm.Segments[index].SourceMapJSON = raw
	summary := fmt.Sprintf("SourceMap OK · ppq=%d · %d events", sm.Meta.PPQ, len(sm.Events))
	vm.Segments[index].Status = summary
	return summary, nil
}

func (vm *ViewModel) ParseSegmentStartTime(index int) (float64, error) {
	if index < 0 || index >= len(vm.Segments) {
		return 0, fmt.Errorf("segment index %d out of range", index)
	}
	s := strings.TrimSpace(vm.Segments[index].StartTimeText)
	if s == "" {
		return 0, fmt.Errorf("segment %d: start time is required", index+1)
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("segment %d: start time %q is not a number", index+1, s)
	}
	return v, nil
}

func (vm *ViewModel) Generate() (*pipeline.Result, error) {
	if len(vm.Segments) == 0 {
		return nil, fmt.Errorf("at least one segment is required")
	}
	segments := make([]pipeline.Segment, 0, len(vm.Segments))
	for i, s := range vm.Segments {
		if strings.TrimSpace(s.SourceMapJSON) == "" {
			return nil, fmt.Errorf("segment %d: no SourceMap loaded: read from clipboard first", i+1)
		}
		startMs, err := vm.ParseSegmentStartTime(i)
		if err != nil {
			return nil, err
		}
		segments = append(segments, pipeline.Segment{
			SourceMapJSON: []byte(s.SourceMapJSON),
			StartTimeMs:   startMs,
			Label:         fmt.Sprintf("Segment %d", i+1),
		})
	}

	if err := vm.ValidateReferencePath(); err != nil {
		return nil, err
	}

	rc, err := vm.opener(vm.ReferencePath)
	if err != nil {
		return nil, fmt.Errorf("cannot open reference: %w", err)
	}
	defer rc.Close()

	req := pipeline.Request{
		Segments:         segments,
		ReferenceOsu:     rc,
		DefaultSampleset: vm.DefaultSampleset,
		ExportOptions:    exporter.Options{},
		VolumeFormatter:  domain.VolumeFormatter{Step: vm.VolumeStep},
	}
	if vm.VersioningEnabled {
		vm.pendingVersion = vm.lastExportVersion + 1
		req.ExportOptions.Version = vm.pendingVersion
	} else {
		vm.pendingVersion = 0
	}
	res, pErr := pipeline.Generate(req)
	if pErr != nil {
		return nil, pErr
	}
	return res, nil
}

// DetectCurrentBeatmap attempts to detect the currently selected beatmap
// from a running osu! client and stores its .osu file path as ReferencePath.
func (vm *ViewModel) DetectCurrentBeatmap() (string, error) {
	if vm.detector == nil {
		return "", fmt.Errorf("beatmap detection is not available")
	}
	path, err := vm.detector.Detect()
	if err != nil {
		return "", fmt.Errorf("could not detect current beatmap: %w", err)
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("detection returned an empty path")
	}
	if !strings.EqualFold(filepath.Ext(path), ".osu") {
		return "", fmt.Errorf("detected file is not a .osu file: %s", path)
	}
	vm.ReferencePath = path
	return path, nil
}
