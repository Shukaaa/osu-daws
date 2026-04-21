package detect

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type ProcessInfo struct {
	ExePath     string
	WindowTitle string
}

type ProcessFinder interface {
	FindOsuStable() (*ProcessInfo, *Error)
}

type WalkDirFunc func(root string, fn fs.WalkDirFunc) error

type StableDetector struct {
	finder  ProcessFinder
	walkDir WalkDirFunc
}

func NewStableDetector(finder ProcessFinder) *StableDetector {
	return &StableDetector{
		finder:  finder,
		walkDir: filepath.WalkDir,
	}
}

func (d *StableDetector) SetWalkDir(fn WalkDirFunc) { d.walkDir = fn }

func (d *StableDetector) Detect() (string, *Error) {
	info, derr := d.finder.FindOsuStable()
	if derr != nil {
		return "", derr
	}

	if strings.TrimSpace(info.WindowTitle) == "" {
		return "", errorf(ReasonNoWindowTitle, "osu! window title is empty")
	}

	bm := ParseWindowTitle(info.WindowTitle)
	if bm == nil {
		return "", errorf(ReasonNoBeatmapSelected,
			"no beatmap selected in osu! (title: %q)", info.WindowTitle)
	}

	songsDir := filepath.Join(filepath.Dir(info.ExePath), "Songs")
	if fi, err := os.Stat(songsDir); err != nil || !fi.IsDir() {
		return "", errorf(ReasonSongsNotFound,
			"Songs directory not found at %q", songsDir)
	}

	osuPath, derr := d.findOsuFile(songsDir, bm)
	if derr != nil {
		return "", derr
	}

	return osuPath, nil
}

func (d *StableDetector) findOsuFile(songsDir string, bm *BeatmapInfo) (string, *Error) {
	wantVersion := strings.ToLower("[" + bm.Version + "]")
	wantTitle := strings.ToLower(bm.Title)
	wantArtist := strings.ToLower(bm.Artist)

	var bestMatch string

	_ = d.walkDir(songsDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if entry.IsDir() {
			rel, _ := filepath.Rel(songsDir, path)
			if rel != "." && strings.Contains(rel, string(filepath.Separator)) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".osu") {
			return nil
		}

		name := strings.ToLower(entry.Name())
		if strings.Contains(name, wantVersion) &&
			strings.Contains(name, wantTitle) &&
			strings.Contains(name, wantArtist) {
			bestMatch = path
			return filepath.SkipAll
		}
		return nil
	})

	if bestMatch == "" {
		return "", errorf(ReasonFileNotResolved,
			"could not find a .osu file matching %q - %q [%s] in %q",
			bm.Artist, bm.Title, bm.Version, songsDir)
	}

	return bestMatch, nil
}
