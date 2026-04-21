//go:build windows

package detect

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

var (
	kernel32                  = syscall.NewLazyDLL("kernel32.dll")
	user32                    = syscall.NewLazyDLL("user32.dll")
	psapi                     = syscall.NewLazyDLL("psapi.dll")
	procCreateToolhelp32      = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW       = kernel32.NewProc("Process32FirstW")
	procProcess32NextW        = kernel32.NewProc("Process32NextW")
	procCloseHandle           = kernel32.NewProc("CloseHandle")
	procOpenProcess           = kernel32.NewProc("OpenProcess")
	procGetModuleFileNameExW  = psapi.NewProc("GetModuleFileNameExW")
	procEnumWindows           = user32.NewProc("EnumWindows")
	procGetWindowThreadProcID = user32.NewProc("GetWindowThreadProcessId")
	procGetWindowTextW        = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW  = user32.NewProc("GetWindowTextLengthW")
	procIsWindowVisible       = user32.NewProc("IsWindowVisible")
)

const (
	thSnapProcess      = 0x00000002
	processQueryInfo   = 0x0400
	processVMRead      = 0x0010
	maxPath            = 260
	invalidHandleValue = ^uintptr(0)
)

type processEntry32W struct {
	Size            uint32
	CntUsage        uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	CntThreads      uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [maxPath]uint16
}

type WindowsProcessFinder struct{}

func (WindowsProcessFinder) FindOsuStable() (*ProcessInfo, *Error) {
	pid, err := findOsuPID()
	if err != nil {
		return nil, &Error{Reason: ReasonProcessNotFound, Message: "osu! stable process not found", Cause: err}
	}

	exePath, err := getProcessExePath(pid)
	if err != nil {
		// Non-fatal: we can still try window title.
		exePath = ""
	}

	title, err := getWindowTitleByPID(pid)
	if err != nil {
		return nil, &Error{Reason: ReasonNoWindowTitle, Message: "could not read osu! window title", Cause: err}
	}

	return &ProcessInfo{ExePath: exePath, WindowTitle: title}, nil
}

func findOsuPID() (uint32, error) {
	snap, _, err := procCreateToolhelp32.Call(thSnapProcess, 0)
	if snap == invalidHandleValue {
		return 0, fmt.Errorf("CreateToolhelp32Snapshot: %w", err)
	}
	defer procCloseHandle.Call(snap)

	var entry processEntry32W
	entry.Size = uint32(unsafe.Sizeof(entry))

	ret, _, err := procProcess32FirstW.Call(snap, uintptr(unsafe.Pointer(&entry)))
	if ret == 0 {
		return 0, fmt.Errorf("Process32First: %w", err)
	}

	for {
		name := syscall.UTF16ToString(entry.ExeFile[:])
		if strings.EqualFold(name, "osu!.exe") {
			return entry.ProcessID, nil
		}
		entry.Size = uint32(unsafe.Sizeof(entry))
		ret, _, _ = procProcess32NextW.Call(snap, uintptr(unsafe.Pointer(&entry)))
		if ret == 0 {
			break
		}
	}
	return 0, fmt.Errorf("no process named osu!.exe found")
}

func getProcessExePath(pid uint32) (string, error) {
	h, _, err := procOpenProcess.Call(processQueryInfo|processVMRead, 0, uintptr(pid))
	if h == 0 {
		return "", fmt.Errorf("OpenProcess: %w", err)
	}
	defer procCloseHandle.Call(h)

	var buf [maxPath * 2]uint16
	n, _, err := procGetModuleFileNameExW.Call(h, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return "", fmt.Errorf("GetModuleFileNameExW: %w", err)
	}
	return syscall.UTF16ToString(buf[:n]), nil
}

func getWindowTitleByPID(pid uint32) (string, error) {
	type result struct {
		title string
		found bool
	}
	var res result

	cb := syscall.NewCallback(func(hwnd uintptr, lparam uintptr) uintptr {
		var windowPID uint32
		procGetWindowThreadProcID.Call(hwnd, uintptr(unsafe.Pointer(&windowPID)))
		if windowPID != pid {
			return 1 // continue
		}
		visible, _, _ := procIsWindowVisible.Call(hwnd)
		if visible == 0 {
			return 1 // skip hidden windows
		}
		length, _, _ := procGetWindowTextLengthW.Call(hwnd)
		if length == 0 {
			return 1
		}
		buf := make([]uint16, length+1)
		procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), length+1)
		title := syscall.UTF16ToString(buf)
		if strings.HasPrefix(title, "osu!") {
			res.title = title
			res.found = true
			return 0 // stop
		}
		return 1
	})

	procEnumWindows.Call(cb, 0)

	if !res.found {
		return "", fmt.Errorf("no visible osu! window found for PID %d", pid)
	}
	return res.title, nil
}
