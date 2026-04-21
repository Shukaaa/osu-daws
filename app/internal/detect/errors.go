package detect

import "fmt"

type Reason int

const (
	ReasonProcessNotFound Reason = iota
	ReasonNoWindowTitle
	ReasonNoBeatmapSelected
	ReasonSongsNotFound
	ReasonFileNotResolved
	ReasonFileNotOsu
)

type Error struct {
	Reason  Reason
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.Cause }

func errorf(reason Reason, format string, args ...any) *Error {
	return &Error{Reason: reason, Message: fmt.Sprintf(format, args...)}
}
