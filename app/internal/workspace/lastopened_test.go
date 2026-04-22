package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLastOpened_RoundTrip(t *testing.T) {
	root := t.TempDir()

	if _, ok, err := LoadLastOpened(root); err != nil || ok {
		t.Fatalf("fresh root should return (_, false, nil); got (_, %v, %v)", ok, err)
	}

	id := ID("my-song-abcdef")
	if err := SaveLastOpened(root, id); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, ok, err := LoadLastOpened(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !ok || got != id {
		t.Errorf("got (%q, %v), want (%q, true)", got, ok, id)
	}
}

func TestLastOpened_Overwrites(t *testing.T) {
	root := t.TempDir()
	_ = SaveLastOpened(root, "first-id")
	_ = SaveLastOpened(root, "second-id")
	got, _, _ := LoadLastOpened(root)
	if got != "second-id" {
		t.Errorf("got %q, want second-id", got)
	}
}

func TestLastOpened_EmptyInputsAreNoOps(t *testing.T) {
	if err := SaveLastOpened("", "x"); err != nil {
		t.Errorf("empty root should be no-op: %v", err)
	}
	if err := SaveLastOpened(t.TempDir(), ""); err != nil {
		t.Errorf("empty id should be no-op: %v", err)
	}
}

func TestLastOpened_CorruptFileTreatedAsNone(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, lastOpenedFileName),
		[]byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	id, ok, err := LoadLastOpened(root)
	if err != nil {
		t.Errorf("corrupt file should not error, got %v", err)
	}
	if ok || id != "" {
		t.Errorf("corrupt file should yield (\"\", false), got (%q, %v)", id, ok)
	}
}

func TestLastOpened_Clear(t *testing.T) {
	root := t.TempDir()
	_ = SaveLastOpened(root, "to-be-cleared")

	if err := ClearLastOpened(root); err != nil {
		t.Fatalf("clear: %v", err)
	}
	_, ok, _ := LoadLastOpened(root)
	if ok {
		t.Error("after clear, LoadLastOpened should return false")
	}
	// Second clear is a no-op.
	if err := ClearLastOpened(root); err != nil {
		t.Errorf("second clear should be no-op: %v", err)
	}
}

func TestLastOpened_DoesNotAppearAsWorkspace(t *testing.T) {
	root := t.TempDir()
	_ = SaveLastOpened(root, "x-y-z")

	res, err := ListWorkspaces(root)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(res.Workspaces) != 0 || len(res.Skipped) != 0 {
		t.Errorf("state file must not surface in workspace listing, got %+v", res)
	}
}
