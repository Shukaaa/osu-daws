//go:build !windows

package detect

type NonWindowsProcessFinder struct{}

func (NonWindowsProcessFinder) FindOsuStable() (*ProcessInfo, *Error) {
	return nil, errorf(ReasonProcessNotFound,
		"osu! stable detection is only supported on Windows")
}
