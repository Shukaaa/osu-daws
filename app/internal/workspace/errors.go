package workspace

import "fmt"

// ErrorCode classifies persistence failures so callers can react
// programmatically.
type ErrorCode string

const (
	ErrHomeDirUnavailable     ErrorCode = "home_dir_unavailable"
	ErrIO                     ErrorCode = "io"
	ErrProjectFileMissing     ErrorCode = "project_file_missing"
	ErrProjectFileInvalid     ErrorCode = "project_file_invalid"
	ErrProjectFileVersion     ErrorCode = "project_file_version"
	ErrProjectFileIncomplete  ErrorCode = "project_file_incomplete"
	ErrWorkspaceAlreadyExists ErrorCode = "workspace_already_exists"
)

// Error is the structured error type returned by persistence functions.
type Error struct {
	Code    ErrorCode
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("workspace[%s]: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("workspace[%s]: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Cause }
