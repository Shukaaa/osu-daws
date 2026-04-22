package workspace

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const lastOpenedFileName = ".last_opened.json"

type LastOpenedRecord struct {
	ID ID        `json:"id"`
	At time.Time `json:"at,omitempty"`
}

func SaveLastOpened(projectsRoot string, id ID) error {
	if strings.TrimSpace(projectsRoot) == "" {
		return nil
	}
	if strings.TrimSpace(string(id)) == "" {
		return nil
	}
	if err := os.MkdirAll(projectsRoot, 0o755); err != nil {
		return &Error{Code: ErrIO,
			Message: "cannot create projects root for last-opened state", Cause: err}
	}
	rec := LastOpenedRecord{ID: id, At: time.Now().UTC()}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return &Error{Code: ErrIO,
			Message: "cannot marshal last-opened record", Cause: err}
	}
	target := filepath.Join(projectsRoot, lastOpenedFileName)
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return &Error{Code: ErrIO,
			Message: "cannot write last-opened record: " + tmp, Cause: err}
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return &Error{Code: ErrIO,
			Message: "cannot rename last-opened record into place", Cause: err}
	}
	return nil
}

func LoadLastOpened(projectsRoot string) (ID, bool, error) {
	target := filepath.Join(projectsRoot, lastOpenedFileName)
	data, err := os.ReadFile(target)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", false, nil
		}
		return "", false, &Error{Code: ErrIO,
			Message: "cannot read last-opened record: " + target, Cause: err}
	}
	var rec LastOpenedRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return "", false, nil
	}
	if strings.TrimSpace(string(rec.ID)) == "" {
		return "", false, nil
	}
	return rec.ID, true, nil
}

func ClearLastOpened(projectsRoot string) error {
	target := filepath.Join(projectsRoot, lastOpenedFileName)
	if err := os.Remove(target); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return &Error{Code: ErrIO,
			Message: "cannot remove last-opened record: " + target, Cause: err}
	}
	return nil
}
