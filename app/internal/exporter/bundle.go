package exporter

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var hitsoundFilePrefixes = []string{"soft-", "drum-", "normal-"}

// HitsoundZipResult describes the files written into an exported hitsound zip.
type HitsoundZipResult struct {
	ZipPath       string
	DiffPath      string
	HitsoundFiles []string
}

func FindHitsoundFiles(beatmapDir string) ([]string, error) {
	info, err := os.Stat(beatmapDir)
	if err != nil {
		return nil, fmt.Errorf("cannot access osu! beatmap folder: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("osu! beatmap path is not a folder: %s", beatmapDir)
	}

	entries, err := os.ReadDir(beatmapDir)
	if err != nil {
		return nil, fmt.Errorf("cannot read osu! beatmap folder: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() || !isHitsoundFilename(entry.Name()) {
			continue
		}
		files = append(files, filepath.Join(beatmapDir, entry.Name()))
	}
	sort.Strings(files)
	return files, nil
}

func CreateHitsoundZip(destZip, diffPath, beatmapDir string) (*HitsoundZipResult, error) {
	destZip = strings.TrimSpace(destZip)
	if destZip == "" {
		return nil, fmt.Errorf("zip destination path is required")
	}
	if !strings.EqualFold(filepath.Ext(destZip), ".zip") {
		destZip += ".zip"
	}

	diffInfo, err := os.Stat(diffPath)
	if err != nil {
		return nil, fmt.Errorf("generated hitsound diff not found: %w", err)
	}
	if diffInfo.IsDir() {
		return nil, fmt.Errorf("generated hitsound diff path is a folder: %s", diffPath)
	}

	hitsounds, err := FindHitsoundFiles(beatmapDir)
	if err != nil {
		return nil, err
	}

	if dir := filepath.Dir(destZip); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("cannot create zip destination folder: %w", err)
		}
	}

	out, err := os.Create(destZip)
	if err != nil {
		return nil, fmt.Errorf("cannot create zip file: %w", err)
	}
	defer out.Close()

	zw := zip.NewWriter(out)
	if err := addFileToZip(zw, diffPath); err != nil {
		_ = zw.Close()
		return nil, err
	}
	for _, path := range hitsounds {
		if err := addFileToZip(zw, path); err != nil {
			_ = zw.Close()
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("cannot finish zip file: %w", err)
	}

	return &HitsoundZipResult{ZipPath: destZip, DiffPath: diffPath, HitsoundFiles: hitsounds}, nil
}

func isHitsoundFilename(name string) bool {
	lower := strings.ToLower(name)
	for _, prefix := range hitsoundFilePrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func addFileToZip(zw *zip.Writer, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cannot access file for zip: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("cannot add folder to zip: %s", path)
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return fmt.Errorf("cannot create zip header: %w", err)
	}
	header.Name = filepath.Base(path)
	header.Method = zip.Deflate

	w, err := zw.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("cannot create zip entry: %w", err)
	}

	in, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot open file for zip: %w", err)
	}
	defer in.Close()

	if _, err := io.Copy(w, in); err != nil {
		return fmt.Errorf("cannot write zip entry: %w", err)
	}
	return nil
}
