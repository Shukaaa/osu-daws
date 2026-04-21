package main

import (
	"os"

	"osu-daws-app/internal/ui"
)

func main() {
	// If the first positional argument is a path to a .odaw project file,
	// the app opens that workspace directly instead of showing the
	// overview. This is the app-level hook for OS-level file association.
	var openPath string
	for _, arg := range os.Args[1:] {
		if len(arg) > 0 && arg[0] != '-' {
			openPath = arg
			break
		}
	}
	ui.RunWithOpenPath(openPath)
}
