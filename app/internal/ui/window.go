package ui

import (
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"osu-daws-app/internal/detect"
	"osu-daws-app/internal/domain"
	"osu-daws-app/internal/pipeline"
	"osu-daws-app/internal/workspace"

	"fyne.io/fyne/v2"
	fyneApp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type fyneClipboard struct{ w fyne.Window }

func (c *fyneClipboard) Content() string {
	if c.w == nil {
		return ""
	}
	return c.w.Clipboard().Content()
}

var activeWorkspace *workspace.Workspace
var lastUsedReferencePath string

// RunWithOpenPath starts the app. If openProjectPath is non-empty and points
// to a valid project.odaw file, the app opens that workspace directly instead
// of showing the workspace overview.
func RunWithOpenPath(openProjectPath string) {
	a := fyneApp.NewWithID("rip.shuka.osudaws")
	title := "osu!daws"
	if v := a.Metadata().Version; v != "" {
		title = fmt.Sprintf("osu!daws v%s", v)
	}
	w := a.NewWindow(title)
	w.Resize(fyne.NewSize(1000, 950))
	w.CenterOnScreen()

	projectsRoot, rootErr := workspace.EnsureProjectsRoot(workspace.DefaultHomeDir)

	var showStart func()
	showStart = func() {
		svm := NewStartViewModel(projectsRoot)
		svm.LastReferencePath = lastUsedReferencePath
		cb := StartScreenCallbacks{
			OnOpen: func(ws *workspace.Workspace) {
				activeWorkspace = ws
				lastUsedReferencePath = svm.LastReferencePath
				showMain(w, showStart)
			},
			OnCreate: func(ws *workspace.Workspace) {
				activeWorkspace = ws
				lastUsedReferencePath = svm.LastReferencePath
				showMain(w, showStart)
			},
		}
		w.SetContent(BuildStartScreen(w, svm, cb))
		if rootErr != nil {
			dialog.ShowError(rootErr, w)
		}
	}

	if openProjectPath != "" {
		ws, err := workspace.LoadWorkspaceFromProjectFile(openProjectPath)
		if err == nil {
			activeWorkspace = ws
			_ = workspace.SaveLastOpened(projectsRoot, ws.Project.ID)
			showMain(w, showStart)
			w.ShowAndRun()
			return
		}
		showStart()
		dialog.ShowError(err, w)
		w.ShowAndRun()
		return
	}

	showStart()
	w.ShowAndRun()
}

// showMain swaps the window content to the hitsound-generation view for the
// currently active workspace.
func showMain(w fyne.Window, backToStart func()) {
	vm := NewViewModel(&fyneClipboard{w: w}, OSFileOpener)

	if activeWorkspace != nil {
		ApplyWorkspaceState(vm, activeWorkspace)
	} else if lastUsedReferencePath != "" {
		vm.ReferencePath = lastUsedReferencePath
	}

	detector := detect.NewStableDetector(detect.WindowsProcessFinder{})
	vm.SetDetector(&detect.DetectorAdapter{D: detector})

	buildAndShow(w, vm, backToStart)
}

func helpText(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.TextStyle = fyne.TextStyle{Italic: true}
	l.Wrapping = fyne.TextWrapWord
	return l
}

func vSpace(h float32) fyne.CanvasObject {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(fyne.NewSize(0, h))
	return r
}

func sectionTitle(text string) fyne.CanvasObject {
	t := canvas.NewText(text, fyne.CurrentApp().Settings().Theme().Color(
		theme.ColorNameForeground, fyne.CurrentApp().Settings().ThemeVariant()))
	t.TextStyle = fyne.TextStyle{Bold: true}
	t.TextSize = 16
	return t
}

// openPathInOS opens the given file or directory with the OS default handler.
func openPathInOS(p string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", p)
	case "darwin":
		cmd = exec.Command("open", p)
	default:
		cmd = exec.Command("xdg-open", p)
	}
	return cmd.Start()
}

func buildAndShow(w fyne.Window, vm *ViewModel, backToStart func()) {
	refEntry := widget.NewEntry()
	refEntry.SetPlaceHolder(`Path to reference .osu (e.g. C:\maps\song\normal.osu)`)
	refEntry.SetText(vm.ReferencePath)
	refEntry.OnChanged = func(s string) {
		vm.ReferencePath = s
		lastUsedReferencePath = s
	}

	browseBtn := widget.NewButton("Browse…", func() {
		fd := dialog.NewFileOpen(func(rc fyne.URIReadCloser, err error) {
			if err != nil || rc == nil {
				return
			}
			defer rc.Close()
			refEntry.SetText(rc.URI().Path())
			lastUsedReferencePath = rc.URI().Path()
		}, w)
		fd.SetFilter(storage.NewExtensionFileFilter([]string{".osu"}))
		fd.Show()
	})

	detectBtn := widget.NewButton("Select current beatmap", func() {
		path, err := vm.DetectCurrentBeatmap()
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		refEntry.SetText(path)
		lastUsedReferencePath = path
		dialog.ShowInformation("Beatmap detected",
			fmt.Sprintf("Reference set to:\n%s", path), w)
	})

	refRow := container.NewBorder(nil, nil, nil,
		container.NewHBox(detectBtn, browseBtn),
		refEntry,
	)

	output := widget.NewMultiLineEntry()
	output.Wrapping = fyne.TextWrapOff
	output.SetMinRowsVisible(8)
	output.SetPlaceHolder("Generated .osu content appears here after pressing Generate.")

	exportStatus := widget.NewLabel("")
	exportStatus.Wrapping = fyne.TextWrapWord
	exportStatus.TextStyle = fyne.TextStyle{Italic: true}
	exportStatus.Hide()

	var generatedContent string
	var lastResult *pipeline.Result

	copyToOsuBtn := widget.NewButton("Copy diff to osu! project", func() {
		if generatedContent == "" || lastResult == nil {
			dialog.ShowError(fmt.Errorf("no generated content. Please Generate first."), w)
			return
		}
		destPath, err := vm.CopyToOsuProject(lastResult)
		if err != nil {
			dialog.ShowError(fmt.Errorf("could not copy to osu! project:\n%w", err), w)
			return
		}
		dialog.ShowInformation("Success", fmt.Sprintf("Copied successfully to:\n%s", destPath), w)
	})
	copyToOsuBtn.Disable()

	generateBtn := widget.NewButton("Generate", func() {
		res, err := vm.Generate()
		if err != nil {
			output.SetText("")
			generatedContent = ""
			lastResult = nil
			exportStatus.Hide()
			copyToOsuBtn.Disable()
			dialog.ShowError(err, w)
			return
		}
		generatedContent = res.OsuContent
		lastResult = res
		output.SetText(res.OsuContent)
		copyToOsuBtn.Enable()

		if vm.WorkspaceExportsDir() != "" {
			path, saveErr := vm.SaveToExports(res)
			if saveErr != nil {
				exportStatus.SetText("⚠ Could not auto-save to workspace: " + saveErr.Error())
			} else {
				exportStatus.SetText("✓ Saved to: " + path)
			}
			exportStatus.Show()
		} else {
			exportStatus.Hide()
		}

		if activeWorkspace != nil {
			if err := PersistToWorkspace(vm, activeWorkspace); err != nil {
				exportStatus.SetText(exportStatus.Text +
					"\n⚠ Could not update project.odaw: " + err.Error())
				exportStatus.Show()
			}
		}
	})
	generateBtn.Importance = widget.HighImportance

	defaultSelect := widget.NewSelect(
		[]string{string(domain.SamplesetDrum), string(domain.SamplesetSoft), string(domain.SamplesetNormal)},
		func(s string) {
			vm.DefaultSampleset = domain.Sampleset(s)
		},
	)
	defaultSelect.SetSelected(string(vm.DefaultSampleset))

	segmentBox := container.NewVBox()
	var rebuildSegments func()
	rebuildSegments = func() {
		segmentBox.Objects = nil
		for i, seg := range vm.Segments {
			idx := i
			s := seg

			status := widget.NewLabel(s.Status)
			status.Wrapping = fyne.TextWrapWord
			status.TextStyle = fyne.TextStyle{Italic: true}

			readBtn := widget.NewButton("Read from clipboard", func() {
				summary, err := vm.ReadSegmentSourceMapFromClipboard(idx)
				if err != nil {
					s.Status = "Error: see dialog."
					status.SetText(s.Status)
					dialog.ShowError(err, w)
					return
				}
				status.SetText(summary)
			})

			startEntry := widget.NewEntry()
			startEntry.SetPlaceHolder("e.g. 1234")
			startEntry.SetText(s.StartTimeText)
			startEntry.OnChanged = func(v string) { s.StartTimeText = v }

			removeBtn := widget.NewButton("✕", func() {
				if vm.RemoveSegment(idx) {
					rebuildSegments()
				} else {
					dialog.ShowInformation("Cannot remove",
						"At least one segment is required.", w)
				}
			})
			removeBtn.Importance = widget.LowImportance
			if len(vm.Segments) <= 1 {
				removeBtn.Disable()
			}

			startLabel := widget.NewLabel("Start (ms)")
			startLabel.TextStyle = fyne.TextStyle{Bold: true}

			row1 := container.NewBorder(nil, nil, nil,
				removeBtn,
				container.NewBorder(nil, nil, readBtn, nil, status),
			)

			row2 := container.NewBorder(nil, nil,
				startLabel, nil,
				startEntry,
			)

			body := container.NewVBox(row1, vSpace(4), row2)

			card := widget.NewCard(
				fmt.Sprintf("Segment %d", idx+1), "",
				container.NewPadded(body),
			)
			segmentBox.Add(card)
			segmentBox.Add(vSpace(4))
		}
		segmentBox.Refresh()
	}
	rebuildSegments()

	addSegmentBtn := widget.NewButton("＋ Add segment", func() {
		vm.AddSegment()
		rebuildSegments()
	})
	addSegmentBtn.Importance = widget.MediumImportance

	section := func(title, help string, body fyne.CanvasObject) fyne.CanvasObject {
		header := container.NewBorder(nil, nil,
			sectionTitle(title), nil,
			helpText(help),
		)
		inner := container.NewVBox(
			header,
			vSpace(4),
			widget.NewSeparator(),
			vSpace(4),
			body,
		)
		return widget.NewCard("", "", container.NewPadded(inner))
	}

	segmentsSection := section(
		"1. Segments",
		"Each segment is one SourceMap + start time. They are merged into one hitsound difficulty.",
		container.NewVBox(
			segmentBox,
			container.NewHBox(addSegmentBtn),
		),
	)

	referenceSection := section(
		"2. Reference .osu",
		"Metadata and red timing points are reused. Green lines and hit objects are replaced.",
		refRow,
	)

	samplesetLabel := widget.NewLabel("Default sampleset")
	samplesetLabel.TextStyle = fyne.TextStyle{Bold: true}
	samplesetRow := container.NewBorder(nil, nil, samplesetLabel, nil, defaultSelect)

	bottomSection := section(
		"3. Generate",
		"Choose the base sampleset, then generate. The result is saved into the workspace's exports/ folder automatically.",
		container.NewVBox(
			samplesetRow,
			vSpace(6),
			container.NewHBox(generateBtn),
		),
	)

	backBtn := widget.NewButton("Back to Workspaces", func() {
		if activeWorkspace != nil {
			if err := PersistToWorkspace(vm, activeWorkspace); err != nil {
				dialog.ShowError(fmt.Errorf("could not save workspace state: %w", err), w)
				return
			}
		}
		if backToStart != nil {
			backToStart()
		}
	})

	openProjectFolderBtn := widget.NewButton("Open Project Folder", func() {
		if activeWorkspace == nil {
			dialog.ShowError(fmt.Errorf("no workspace loaded"), w)
			return
		}
		p := activeWorkspace.Paths.Root
		if _, err := os.Stat(p); err != nil {
			dialog.ShowError(fmt.Errorf("project folder not found:\n%s", p), w)
			return
		}
		if err := openPathInOS(p); err != nil {
			dialog.ShowError(fmt.Errorf("failed to open project folder:\n%w", err), w)
		}
	})
	openProjectFolderBtn.Importance = widget.MediumImportance

	openOsuFolderBtn := widget.NewButton("Open osu! Folder", func() {
		refPath := strings.TrimSpace(vm.ReferencePath)
		if refPath == "" {
			dialog.ShowError(fmt.Errorf("reference .osu path is empty"), w)
			return
		}
		dir := filepath.Dir(refPath)
		if _, err := os.Stat(dir); err != nil {
			dialog.ShowError(fmt.Errorf("osu! folder not found:\n%s", dir), w)
			return
		}
		if err := openPathInOS(dir); err != nil {
			dialog.ShowError(fmt.Errorf("failed to open osu! folder:\n%w", err), w)
		}
	})
	openOsuFolderBtn.Importance = widget.MediumImportance

	openDawBtn := widget.NewButton("Open DAW", func() {
		if activeWorkspace == nil {
			dialog.ShowError(fmt.Errorf("no workspace loaded"), w)
			return
		}
		ref := activeWorkspace.Project.Template
		if string(ref.DAW) == "" {
			dialog.ShowError(fmt.Errorf("workspace has no DAW template assigned"), w)
			return
		}

		desc, ok := workspace.NewDefaultCatalog().ByID(ref.ID)
		if !ok || desc.EntryFile == "" {
			dialog.ShowError(fmt.Errorf("DAW template not found or has no entry file in catalog: %s", ref.ID), w)
			return
		}

		p := filepath.Join(activeWorkspace.Paths.Template, desc.EntryFile)
		if strings.HasPrefix(desc.EntryFile, "/") || strings.HasPrefix(desc.EntryFile, "\\") || filepath.IsAbs(desc.EntryFile) {
			p = filepath.Join(activeWorkspace.Paths.Template, filepath.Base(desc.EntryFile))
		}
		if _, err := os.Stat(p); err != nil {
			dialog.ShowError(fmt.Errorf("could not find DAW project file:\n%s", p), w)
			return
		}

		if err := openPathInOS(p); err != nil {
			dialog.ShowError(fmt.Errorf("failed to open DAW:\n%w", err), w)
		}
	})
	openDawBtn.Importance = widget.MediumImportance

	form := container.NewVBox(
		container.NewBorder(nil, nil,
			container.NewHBox(backBtn),
			container.NewHBox(openProjectFolderBtn, openOsuFolderBtn, openDawBtn),
		),
		vSpace(4),
		segmentsSection,
		vSpace(8),
		referenceSection,
		vSpace(8),
		bottomSection,
		vSpace(4),
	)

	previewCard := widget.NewCard(
		"Preview", "",
		container.NewBorder(
			container.NewVBox(
				container.NewHBox(copyToOsuBtn),
				exportStatus,
			),
			nil, nil, nil,
			container.NewPadded(output),
		),
	)

	scroll := container.NewVScroll(container.NewPadded(form))

	split := container.NewVSplit(scroll, previewCard)
	split.SetOffset(0.67)

	w.SetContent(container.NewPadded(split))
}
