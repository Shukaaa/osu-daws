package detect

import (
	"os"
	"path/filepath"
	"testing"
)

// --- stub ProcessFinder --------------------------------------------------

type stubFinder struct {
	info *ProcessInfo
	err  *Error
}

func (s *stubFinder) FindOsuStable() (*ProcessInfo, *Error) {
	return s.info, s.err
}

// --- helpers for fake Songs dir ------------------------------------------

// fakeSongsDir creates a temporary Songs directory with .osu files whose
// names follow the standard osu! naming convention:
//
//	<Artist> - <Title> (<Creator>) [<Version>].osu
func fakeSongsDir(t *testing.T, files []string) string {
	t.Helper()
	base := t.TempDir()
	songsDir := filepath.Join(base, "Songs")
	songFolder := filepath.Join(songsDir, "12345 SongFolder")
	if err := os.MkdirAll(songFolder, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(songFolder, f), []byte("osu file"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return base
}

// --- tests ---------------------------------------------------------------

func TestStableDetector_HappyPath(t *testing.T) {
	base := fakeSongsDir(t, []string{
		"Camellia - GHOST (Mapper) [Extra].osu",
		"Camellia - GHOST (Mapper) [Normal].osu",
	})
	d := NewStableDetector(&stubFinder{
		info: &ProcessInfo{
			ExePath:     filepath.Join(base, "osu!.exe"),
			WindowTitle: "osu!  - Camellia - GHOST [Extra]",
		},
	})
	path, derr := d.Detect()
	if derr != nil {
		t.Fatalf("unexpected error: %s", derr.Error())
	}
	if filepath.Ext(path) != ".osu" {
		t.Errorf("path = %q, want .osu extension", path)
	}
	if !containsCI(filepath.Base(path), "[Extra]") {
		t.Errorf("expected [Extra] in filename, got %q", filepath.Base(path))
	}
}

func TestStableDetector_ProcessNotFound(t *testing.T) {
	d := NewStableDetector(&stubFinder{
		err: errorf(ReasonProcessNotFound, "not found"),
	})
	_, derr := d.Detect()
	if derr == nil {
		t.Fatal("expected error")
	}
	if derr.Reason != ReasonProcessNotFound {
		t.Errorf("reason = %d, want ProcessNotFound", derr.Reason)
	}
}

func TestStableDetector_NoBeatmapSelected(t *testing.T) {
	base := fakeSongsDir(t, nil)
	d := NewStableDetector(&stubFinder{
		info: &ProcessInfo{
			ExePath:     filepath.Join(base, "osu!.exe"),
			WindowTitle: "osu!",
		},
	})
	_, derr := d.Detect()
	if derr == nil {
		t.Fatal("expected error")
	}
	if derr.Reason != ReasonNoBeatmapSelected {
		t.Errorf("reason = %d, want NoBeatmapSelected", derr.Reason)
	}
}

func TestStableDetector_EmptyTitle(t *testing.T) {
	d := NewStableDetector(&stubFinder{
		info: &ProcessInfo{
			ExePath:     `C:\osu!\osu!.exe`,
			WindowTitle: "",
		},
	})
	_, derr := d.Detect()
	if derr == nil {
		t.Fatal("expected error")
	}
	if derr.Reason != ReasonNoWindowTitle {
		t.Errorf("reason = %d, want NoWindowTitle", derr.Reason)
	}
}

func TestStableDetector_SongsNotFound(t *testing.T) {
	// Use a temp dir without a Songs subdirectory.
	base := t.TempDir()
	d := NewStableDetector(&stubFinder{
		info: &ProcessInfo{
			ExePath:     filepath.Join(base, "osu!.exe"),
			WindowTitle: "osu!  - Camellia - GHOST [Extra]",
		},
	})
	_, derr := d.Detect()
	if derr == nil {
		t.Fatal("expected error")
	}
	if derr.Reason != ReasonSongsNotFound {
		t.Errorf("reason = %d, want SongsNotFound", derr.Reason)
	}
}

func TestStableDetector_FileNotResolved(t *testing.T) {
	base := fakeSongsDir(t, []string{
		"Other - Song (Mapper) [Normal].osu",
	})
	d := NewStableDetector(&stubFinder{
		info: &ProcessInfo{
			ExePath:     filepath.Join(base, "osu!.exe"),
			WindowTitle: "osu!  - Camellia - GHOST [Extra]",
		},
	})
	_, derr := d.Detect()
	if derr == nil {
		t.Fatal("expected error")
	}
	if derr.Reason != ReasonFileNotResolved {
		t.Errorf("reason = %d, want FileNotResolved", derr.Reason)
	}
}

func TestStableDetector_CaseInsensitiveMatch(t *testing.T) {
	base := fakeSongsDir(t, []string{
		"camellia - ghost (mapper) [extra].osu",
	})
	d := NewStableDetector(&stubFinder{
		info: &ProcessInfo{
			ExePath:     filepath.Join(base, "osu!.exe"),
			WindowTitle: "osu!  - Camellia - GHOST [Extra]",
		},
	})
	path, derr := d.Detect()
	if derr != nil {
		t.Fatalf("unexpected error: %s", derr.Error())
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}
}

func TestStableDetector_DashInTitle(t *testing.T) {
	base := fakeSongsDir(t, []string{
		"YOASOBI - Idol - Oshi no Ko (Mapper) [Insane].osu",
	})
	d := NewStableDetector(&stubFinder{
		info: &ProcessInfo{
			ExePath:     filepath.Join(base, "osu!.exe"),
			WindowTitle: "osu!  - YOASOBI - Idol - Oshi no Ko [Insane]",
		},
	})
	path, derr := d.Detect()
	if derr != nil {
		t.Fatalf("unexpected error: %s", derr.Error())
	}
	if !containsCI(filepath.Base(path), "Oshi no Ko") {
		t.Errorf("filename = %q", filepath.Base(path))
	}
}

func containsCI(s, sub string) bool {
	return len(s) >= len(sub) &&
		filepath.Base(s) != "" &&
		len(sub) > 0 &&
		// simple case-insensitive contains
		func() bool {
			sl, subl := []rune(s), []rune(sub)
			for i := 0; i <= len(sl)-len(subl); i++ {
				match := true
				for j := range subl {
					a, b := sl[i+j], subl[j]
					if a != b && a != b+32 && a != b-32 {
						match = false
						break
					}
				}
				if match {
					return true
				}
			}
			return false
		}()
}
