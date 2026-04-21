package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAutoProjectName_SuggestWhenEmpty(t *testing.T) {
	a := &AutoProjectName{}
	got, ok := a.Suggest("bladee - shadowface")
	if !ok || got != "bladee - shadowface" {
		t.Fatalf("ok=%v got=%q", ok, got)
	}
	if a.Current() != "bladee - shadowface" {
		t.Errorf("Current = %q", a.Current())
	}
	if a.Edited() {
		t.Error("should not be marked edited after an auto-suggest")
	}
}

func TestAutoProjectName_ReplacesPreviousAutoSuggestion(t *testing.T) {
	a := &AutoProjectName{}
	a.Suggest("First Map")
	// simulate widget OnChanged firing with the value we just set:
	a.UserTyped("First Map")

	got, ok := a.Suggest("Second Map")
	if !ok || got != "Second Map" {
		t.Fatalf("ok=%v got=%q", ok, got)
	}
	if a.Edited() {
		t.Error("still considered edited after auto-only changes")
	}
}

func TestAutoProjectName_DoesNotOverwriteManualEdit(t *testing.T) {
	a := &AutoProjectName{}
	a.Suggest("Suggested A")
	a.UserTyped("Suggested A")       // echo from widget
	a.UserTyped("My Custom Project") // user typed something else

	if !a.Edited() {
		t.Fatal("expected Edited() to be true")
	}
	got, ok := a.Suggest("Suggested B")
	if ok || got != "" {
		t.Errorf("should not suggest over manual edit, got ok=%v %q", ok, got)
	}
	if a.Current() != "My Custom Project" {
		t.Errorf("Current = %q", a.Current())
	}
}

func TestAutoProjectName_ManualEditBeforeAnySuggestion(t *testing.T) {
	a := &AutoProjectName{}
	a.UserTyped("I typed this first")
	if !a.Edited() {
		t.Fatal("expected Edited() true after typing with no prior suggestion")
	}
	if _, ok := a.Suggest("Ignored"); ok {
		t.Error("suggestion should be refused")
	}
}

func TestAutoProjectName_EmptySuggestionIgnored(t *testing.T) {
	a := &AutoProjectName{}
	if _, ok := a.Suggest(""); ok {
		t.Error("empty suggestion must be ignored")
	}
	if _, ok := a.Suggest("   "); ok {
		t.Error("whitespace-only suggestion must be ignored")
	}
}

func TestAutoProjectName_DuplicateSuggestionNoOp(t *testing.T) {
	a := &AutoProjectName{}
	a.Suggest("Same Name")
	a.UserTyped("Same Name")
	if _, ok := a.Suggest("Same Name"); ok {
		t.Error("repeat of same suggestion should be a no-op")
	}
}

func TestSuggestNameFromReference_BlankPath(t *testing.T) {
	if got := suggestNameFromReference(""); got != "" {
		t.Errorf("got %q, want empty", got)
	}
	if got := suggestNameFromReference("   "); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestSuggestNameFromReference_MissingFile(t *testing.T) {
	if got := suggestNameFromReference(filepath.Join(t.TempDir(), "nope.osu")); got != "" {
		t.Errorf("got %q, want empty for missing file", got)
	}
}

func TestSuggestNameFromReference_ParsesMetadata(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ref.osu")
	content := "osu file format v14\r\n\r\n" +
		"[Metadata]\r\n" +
		"Artist:bladee\r\n" +
		"Title:shadowface\r\n" +
		"Version:hard\r\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got := suggestNameFromReference(path)
	want := "bladee - shadowface [hard]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSuggestNameFromReference_MalformedHeaderFallsBack(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.osu")
	if err := os.WriteFile(path, []byte("this is not an osu file"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Parser returns nil map on invalid header; helper must not crash
	// and must return an empty suggestion.
	if got := suggestNameFromReference(path); got != "" {
		t.Errorf("got %q, want empty for malformed file", got)
	}
}
