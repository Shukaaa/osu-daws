package ui

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type stubClipboard struct{ s string }

func (s *stubClipboard) Content() string { return s.s }

func stubOpener(files map[string]string) FileOpener {
	return func(path string) (io.ReadCloser, error) {
		v, ok := files[path]
		if !ok {
			return nil, io.EOF
		}
		return io.NopCloser(strings.NewReader(v)), nil
	}
}

type stubFileInfo struct {
	name  string
	isDir bool
}

func (f stubFileInfo) Name() string       { return f.name }
func (f stubFileInfo) Size() int64        { return 0 }
func (f stubFileInfo) Mode() os.FileMode  { return 0 }
func (f stubFileInfo) ModTime() time.Time { return time.Time{} }
func (f stubFileInfo) IsDir() bool        { return f.isDir }
func (f stubFileInfo) Sys() any           { return nil }

func stubStat(files map[string]bool) StatFunc {
	return func(path string) (os.FileInfo, error) {
		isDir, ok := files[path]
		if !ok {
			return nil, fs.ErrNotExist
		}
		return stubFileInfo{name: path, isDir: isDir}, nil
	}
}

const refOsuUI = `osu file format v14

[General]
AudioFilename: audio.mp3

[Metadata]
Title:Ref
Artist:Tester
Creator:Mapper
Version:Easy

[Difficulty]
HPDrainRate:5
CircleSize:4
OverallDifficulty:5
ApproachRate:5
SliderMultiplier:1.4
SliderTickRate:1

[Events]

[TimingPoints]
0,500,4,1,0,70,1,0

[HitObjects]
`

const validSMUI = `{"_meta":{"ppq":96,"timeSignatureNumerator":4},"drum":{"0":{"normal":["0;50","96;50"]}}}`

func TestViewModel_ReadClipboard_Valid(t *testing.T) {
	vm := NewViewModel(&stubClipboard{s: validSMUI}, nil)
	summary, err := vm.ReadSourceMapFromClipboard()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(summary, "ppq=96") {
		t.Errorf("summary = %q", summary)
	}
	if vm.Segments[0].SourceMapJSON == "" {
		t.Error("expected JSON stored")
	}
}

func TestViewModel_ReadClipboard_Empty(t *testing.T) {
	vm := NewViewModel(&stubClipboard{s: "   "}, nil)
	if _, err := vm.ReadSourceMapFromClipboard(); err == nil {
		t.Fatal("expected error on empty clipboard")
	}
}

func TestViewModel_ReadClipboard_Invalid(t *testing.T) {
	vm := NewViewModel(&stubClipboard{s: `{not json`}, nil)
	if _, err := vm.ReadSourceMapFromClipboard(); err == nil {
		t.Fatal("expected error on invalid JSON")
	}
	if vm.Segments[0].SourceMapJSON != "" {
		t.Error("invalid JSON should not be stored")
	}
}

func TestViewModel_ParseStartTime(t *testing.T) {
	vm := NewViewModel(&stubClipboard{}, nil)

	vm.Segments[0].StartTimeText = ""
	if _, err := vm.ParseSegmentStartTime(0); err == nil {
		t.Error("expected error for empty")
	}
	vm.Segments[0].StartTimeText = "abc"
	if _, err := vm.ParseSegmentStartTime(0); err == nil {
		t.Error("expected error for non-numeric")
	}
	vm.Segments[0].StartTimeText = "1234"
	if v, err := vm.ParseSegmentStartTime(0); err != nil || v != 1234 {
		t.Errorf("got v=%v err=%v", v, err)
	}
	vm.Segments[0].StartTimeText = "1234.5"
	if v, err := vm.ParseSegmentStartTime(0); err != nil || v != 1234.5 {
		t.Errorf("got v=%v err=%v", v, err)
	}
}

func TestViewModel_Generate_HappyPath(t *testing.T) {
	vm := NewViewModel(&stubClipboard{s: validSMUI}, stubOpener(map[string]string{"/ref.osu": refOsuUI}))
	vm.SetStatFunc(stubStat(map[string]bool{"/ref.osu": false}))
	if _, err := vm.ReadSourceMapFromClipboard(); err != nil {
		t.Fatal(err)
	}
	vm.ReferencePath = "/ref.osu"
	vm.Segments[0].StartTimeText = "1000"

	res, err := vm.Generate()
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	if !strings.Contains(res.OsuContent, "[HitObjects]") {
		t.Error("output missing [HitObjects]")
	}
	if !strings.Contains(res.OsuContent, "Version:") {
		t.Error("output missing Version")
	}
}

func TestViewModel_Generate_MissingSourceMap(t *testing.T) {
	vm := NewViewModel(&stubClipboard{}, stubOpener(nil))
	vm.SetStatFunc(stubStat(map[string]bool{"/ref.osu": false}))
	vm.ReferencePath = "/ref.osu"
	vm.Segments[0].StartTimeText = "1000"
	if _, err := vm.Generate(); err == nil {
		t.Fatal("expected error")
	}
}

func TestViewModel_Generate_MissingReference(t *testing.T) {
	vm := NewViewModel(&stubClipboard{s: validSMUI}, stubOpener(nil))
	vm.SetStatFunc(stubStat(nil))
	_, _ = vm.ReadSourceMapFromClipboard()
	vm.Segments[0].StartTimeText = "1000"
	if _, err := vm.Generate(); err == nil {
		t.Fatal("expected error")
	}
}

func TestViewModel_Generate_BadStartTime(t *testing.T) {
	vm := NewViewModel(&stubClipboard{s: validSMUI}, stubOpener(map[string]string{"/ref.osu": refOsuUI}))
	vm.SetStatFunc(stubStat(map[string]bool{"/ref.osu": false}))
	_, _ = vm.ReadSourceMapFromClipboard()
	vm.ReferencePath = "/ref.osu"
	vm.Segments[0].StartTimeText = "abc"
	if _, err := vm.Generate(); err == nil {
		t.Fatal("expected error")
	}
}

func TestViewModel_ValidateReferencePath(t *testing.T) {
	files := map[string]bool{
		"/maps/song/normal.osu": false,
		"/maps/song":            true,
	}
	cases := []struct {
		name    string
		path    string
		wantErr string
	}{
		{"empty", "", "required"},
		{"whitespace only", "   ", "required"},
		{"wrong extension", "/maps/song/normal.txt", ".osu extension"},
		{"missing file", "/maps/song/missing.osu", "does not exist"},
		{"directory", "/maps/song.osu", "does not exist"},
		{"is a directory with .osu ext", "/maps/is_dir.osu", "directory"},
		{"valid file", "/maps/song/normal.osu", ""},
	}
	extra := map[string]bool{"/maps/is_dir.osu": true}
	merged := map[string]bool{}
	for k, v := range files {
		merged[k] = v
	}
	for k, v := range extra {
		merged[k] = v
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			vm := NewViewModel(&stubClipboard{}, nil)
			vm.SetStatFunc(stubStat(merged))
			vm.ReferencePath = c.path
			err := vm.ValidateReferencePath()
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q", c.wantErr)
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("error %q should contain %q", err.Error(), c.wantErr)
			}
		})
	}
}

func TestViewModel_ValidateReferencePath_IsDirectory(t *testing.T) {
	vm := NewViewModel(&stubClipboard{}, nil)
	vm.SetStatFunc(stubStat(map[string]bool{"/maps/dir.osu": true}))
	vm.ReferencePath = "/maps/dir.osu"
	err := vm.ValidateReferencePath()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "directory") {
		t.Errorf("error %q should mention directory", err.Error())
	}
}

func TestViewModel_DefaultSaveDir(t *testing.T) {
	songDir := filepath.Join(string(filepath.Separator), "maps", "song")
	validPath := filepath.Join(songDir, "normal.osu")
	missingParentPath := filepath.Join(string(filepath.Separator), "does", "not", "exist", "normal.osu")

	files := map[string]bool{
		songDir: true,
	}
	cases := []struct {
		name string
		path string
		want string
	}{
		{"empty", "", ""},
		{"whitespace only", "   ", ""},
		{"no separator", "foo.osu", ""},
		{"valid parent exists", validPath, songDir},
		{"parent does not exist", missingParentPath, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			vm := NewViewModel(&stubClipboard{}, nil)
			vm.SetStatFunc(stubStat(files))
			vm.ReferencePath = c.path
			got := vm.DefaultSaveDir()
			if got != c.want {
				t.Errorf("DefaultSaveDir() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestViewModel_DefaultSaveDir_ParentIsFile(t *testing.T) {
	songDir := filepath.Join(string(filepath.Separator), "maps", "song")
	refPath := filepath.Join(songDir, "normal.osu")
	vm := NewViewModel(&stubClipboard{}, nil)
	vm.SetStatFunc(stubStat(map[string]bool{songDir: false}))
	vm.ReferencePath = refPath
	if got := vm.DefaultSaveDir(); got != "" {
		t.Errorf("DefaultSaveDir() = %q, want empty (parent is not a directory)", got)
	}
}

// --- DetectCurrentBeatmap tests ------------------------------------------

type stubDetector struct {
	path string
	err  error
}

func (s *stubDetector) Detect() (string, error) { return s.path, s.err }

func TestViewModel_DetectCurrentBeatmap_Success(t *testing.T) {
	vm := NewViewModel(&stubClipboard{}, nil)
	vm.SetDetector(&stubDetector{path: `C:\osu!\Songs\12345\Artist - Title (Mapper) [Hard].osu`})
	path, err := vm.DetectCurrentBeatmap()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != `C:\osu!\Songs\12345\Artist - Title (Mapper) [Hard].osu` {
		t.Errorf("path = %q", path)
	}
	if vm.ReferencePath != path {
		t.Errorf("ReferencePath = %q, want %q", vm.ReferencePath, path)
	}
}

func TestViewModel_DetectCurrentBeatmap_Error(t *testing.T) {
	vm := NewViewModel(&stubClipboard{}, nil)
	vm.SetDetector(&stubDetector{err: fmt.Errorf("osu! not running")})
	vm.ReferencePath = "/original.osu"
	_, err := vm.DetectCurrentBeatmap()
	if err == nil {
		t.Fatal("expected error")
	}
	// ReferencePath must be untouched on failure.
	if vm.ReferencePath != "/original.osu" {
		t.Errorf("ReferencePath changed to %q on failure", vm.ReferencePath)
	}
}

func TestViewModel_DetectCurrentBeatmap_NoDetector(t *testing.T) {
	vm := NewViewModel(&stubClipboard{}, nil)
	_, err := vm.DetectCurrentBeatmap()
	if err == nil {
		t.Fatal("expected error when detector is nil")
	}
	if !strings.Contains(err.Error(), "not available") {
		t.Errorf("error = %q, want mention of 'not available'", err.Error())
	}
}

func TestViewModel_DetectCurrentBeatmap_EmptyPath(t *testing.T) {
	vm := NewViewModel(&stubClipboard{}, nil)
	vm.SetDetector(&stubDetector{path: "   "})
	_, err := vm.DetectCurrentBeatmap()
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestViewModel_DetectCurrentBeatmap_NotOsuFile(t *testing.T) {
	vm := NewViewModel(&stubClipboard{}, nil)
	vm.SetDetector(&stubDetector{path: `C:\osu!\Songs\file.txt`})
	_, err := vm.DetectCurrentBeatmap()
	if err == nil {
		t.Fatal("expected error for non-.osu file")
	}
	if !strings.Contains(err.Error(), ".osu") {
		t.Errorf("error = %q, want mention of .osu", err.Error())
	}
}
