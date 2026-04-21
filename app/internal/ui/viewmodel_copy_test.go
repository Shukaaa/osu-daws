package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"osu-daws-app/internal/domain"
	"osu-daws-app/internal/pipeline"
)

func TestViewModel_CopyToOsuProject(t *testing.T) {
	tempDir := t.TempDir()
	refFile := filepath.Join(tempDir, "song", "normal.osu")

	songDir := filepath.Join(tempDir, "song")
	if err := os.MkdirAll(songDir, 0755); err != nil {
		t.Fatal(err)
	}

	vm := NewViewModel(&stubClipboard{}, nil)
	vm.ReferencePath = refFile

	res := &pipeline.Result{
		OsuContent: "generated hitsounds",
		Reference: &domain.OsuMap{
			Metadata: map[string]string{
				"Artist":  "ArtistName",
				"Title":   "SongTitle",
				"Creator": "MapperName",
			},
		},
	}

	destPath, err := vm.CopyToOsuProject(res)
	if err != nil {
		t.Fatalf("CopyToOsuProject failed: %v", err)
	}

	if !strings.HasPrefix(destPath, songDir) {
		t.Errorf("destPath %q not in songDir %q", destPath, songDir)
	}

	b, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "generated hitsounds" {
		t.Errorf("got %q, want 'generated hitsounds'", string(b))
	}
}

func TestViewModel_CopyToOsuProject_Errors(t *testing.T) {
	vm := NewViewModel(&stubClipboard{}, nil)

	_, err := vm.CopyToOsuProject(nil)
	if err == nil {
		t.Error("expected error for empty DefaultSaveDir")
	}

	vm.ReferencePath = filepath.Join(t.TempDir(), "song", "normal.osu")

	vm.SetStatFunc(func(path string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	})

	_, err = vm.CopyToOsuProject(&pipeline.Result{OsuContent: "data"})
	if err == nil {
		t.Error("expected error when dir does not exist")
	}
}
