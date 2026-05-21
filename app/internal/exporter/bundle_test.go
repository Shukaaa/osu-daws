package exporter

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestFindHitsoundFiles(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"soft-hitnormal.wav":   "soft",
		"drum-clap.ogg":        "drum",
		"Normal-finish.wav":    "normal",
		"audio.mp3":            "ignore",
		"custom-soft-test.wav": "ignore",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "soft-folder"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := FindHitsoundFiles(dir)
	if err != nil {
		t.Fatalf("FindHitsoundFiles failed: %v", err)
	}

	want := []string{
		filepath.Join(dir, "Normal-finish.wav"),
		filepath.Join(dir, "drum-clap.ogg"),
		filepath.Join(dir, "soft-hitnormal.wav"),
	}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("files = %#v, want %#v", got, want)
	}
}

func TestCreateHitsoundZip(t *testing.T) {
	root := t.TempDir()
	beatmapDir := filepath.Join(root, "osu")
	exportsDir := filepath.Join(root, "exports")
	if err := os.MkdirAll(beatmapDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(exportsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	diffPath := filepath.Join(exportsDir, "Artist - Title (Mapper) [osu!daw's HS].osu")
	if err := os.WriteFile(diffPath, []byte("osu diff"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"soft-hitnormal.wav": "soft sample",
		"drum-clap.ogg":      "drum sample",
		"normal-whistle.wav": "normal sample",
		"song.mp3":           "audio",
	} {
		if err := os.WriteFile(filepath.Join(beatmapDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	zipPath := filepath.Join(root, "bundle")
	res, err := CreateHitsoundZip(zipPath, diffPath, beatmapDir)
	if err != nil {
		t.Fatalf("CreateHitsoundZip failed: %v", err)
	}
	if res.ZipPath != zipPath+".zip" {
		t.Fatalf("ZipPath = %q, want %q", res.ZipPath, zipPath+".zip")
	}
	if len(res.HitsoundFiles) != 3 {
		t.Fatalf("HitsoundFiles len = %d, want 3", len(res.HitsoundFiles))
	}

	zr, err := zip.OpenReader(res.ZipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	got := make(map[string]string)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		got[f.Name] = string(b)
	}

	want := map[string]string{
		filepath.Base(diffPath): "osu diff",
		"soft-hitnormal.wav":    "soft sample",
		"drum-clap.ogg":         "drum sample",
		"normal-whistle.wav":    "normal sample",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("zip entries = %#v, want %#v", got, want)
	}
}

func TestCreateHitsoundZipMissingDiff(t *testing.T) {
	_, err := CreateHitsoundZip(filepath.Join(t.TempDir(), "x.zip"), filepath.Join(t.TempDir(), "missing.osu"), t.TempDir())
	if err == nil {
		t.Fatal("expected missing diff error")
	}
}
