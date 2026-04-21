package workspace

import (
	"os"
	"path/filepath"
)

const (
	AppDirName      = "osu!daws"
	ProjectsDirName = "projects"
)

// HomeDirFunc returns the user's home directory. Injectable so tests can
// redirect "Documents" into a temp dir without touching the real HOME.
type HomeDirFunc func() (string, error)

var DefaultHomeDir HomeDirFunc = os.UserHomeDir

// ProjectsRoot returns the absolute path of <home>/Documents/osu!daws/projects.
// It performs no I/O beyond locating the home directory.
func ProjectsRoot(homeDir HomeDirFunc) (string, error) {
	if homeDir == nil {
		homeDir = DefaultHomeDir
	}
	home, err := homeDir()
	if err != nil {
		return "", &Error{Code: ErrHomeDirUnavailable,
			Message: "cannot locate user home directory", Cause: err}
	}
	return filepath.Join(home, "Documents", AppDirName, ProjectsDirName), nil
}

// WorkspaceRoot returns <projectsRoot>/<id>.
func WorkspaceRoot(projectsRoot string, id ID) string {
	return filepath.Join(projectsRoot, string(id))
}

// EnsureProjectsRoot makes sure the global projects directory exists.
func EnsureProjectsRoot(homeDir HomeDirFunc) (string, error) {
	root, err := ProjectsRoot(homeDir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", &Error{Code: ErrIO,
			Message: "cannot create projects root: " + root, Cause: err}
	}
	return root, nil
}
