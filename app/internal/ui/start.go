package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"osu-daws-app/internal/detect"
	"osu-daws-app/internal/domain"
	"osu-daws-app/internal/workspace"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

// StartScreenCallbacks wires the start view to the rest of the app.
// OnOpen and OnCreate are both invoked with a ready-to-use workspace.
type StartScreenCallbacks struct {
	OnOpen   func(*workspace.Workspace)
	OnCreate func(*workspace.Workspace)
}

// BuildStartScreen constructs the workspace overview. The caller owns the
// window and must attach the returned object via SetContent.
func BuildStartScreen(w fyne.Window, svm *StartViewModel, cb StartScreenCallbacks) fyne.CanvasObject {
	origOpen, origCreate := cb.OnOpen, cb.OnCreate
	cb.OnOpen = func(ws *workspace.Workspace) {
		svm.MarkOpened(ws)
		if origOpen != nil {
			origOpen(ws)
		}
	}
	cb.OnCreate = func(ws *workspace.Workspace) {
		svm.MarkOpened(ws)
		if origCreate != nil {
			origCreate(ws)
		}
	}

	content := container.NewStack()

	var rerender func()

	createBtn := widget.NewButton("＋  Create New Workspace", func() {
		showCreateWorkspaceDialog(w, svm, cb, rerender)
	})
	createBtn.Importance = widget.HighImportance

	refreshBtn := widget.NewButton("Refresh", func() {
		if err := svm.Refresh(); err != nil {
			dialog.ShowError(err, w)
			return
		}
		rerender()
	})

	importBtn := widget.NewButton("Import…", func() {
		showImportWorkspaceDialog(w, svm, cb, rerender)
	})

	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Search workspaces by name…")
	searchEntry.SetText(svm.SearchQuery)
	searchEntry.OnChanged = func(s string) {
		svm.SearchQuery = s
		rerender()
	}

	header := container.NewBorder(
		nil, nil,
		sectionTitle("Workspaces"),
		container.NewHBox(refreshBtn, importBtn, createBtn),
		searchEntry,
	)

	rerender = func() {
		body := buildWorkspaceList(w, svm, cb, rerender)
		content.Objects = []fyne.CanvasObject{
			container.NewBorder(
				container.NewVBox(
					container.NewPadded(header),
					vSpace(4),
					widget.NewSeparator(),
				),
				nil, nil, nil,
				container.NewPadded(body),
			),
		}
		content.Refresh()
	}

	// Initial load.
	if err := svm.Refresh(); err != nil {
		dialog.ShowError(err, w)
	}
	rerender()

	return content
}

// buildWorkspaceList renders the active workspace list (or an empty
// state) plus — when present — a "Last opened" shortcut above and an
// "Archived" accordion below. rerender is threaded through so per-row
// archive toggles can refresh the screen.
func buildWorkspaceList(
	w fyne.Window,
	svm *StartViewModel,
	cb StartScreenCallbacks,
	rerender func(),
) fyne.CanvasObject {
	activeItems := svm.FilteredWorkspaces()
	totalActive := len(svm.Workspaces())
	totalArchived := len(svm.Archived())
	filtering := strings.TrimSpace(svm.SearchQuery) != ""

	// --- center: active list or empty state -----------------------------
	var center fyne.CanvasObject
	if len(activeItems) == 0 {
		center = container.NewCenter(container.NewVBox(
			emptyStateLabel(svm.SearchQuery, filtering, totalActive, totalArchived),
		))
	} else {
		rows := container.NewVBox()
		for i := range activeItems {
			item := activeItems[i]
			rows.Add(workspaceRow(item,
				func() { openSummary(w, cb, item) },
				func() { showExportWorkspaceDialog(w, svm, item) },
				func() { archiveSummary(w, svm, item, true, rerender) },
			))
			rows.Add(vSpace(4))
		}
		center = container.NewVScroll(rows)
	}

	// --- top: last-opened shortcut + skipped banner --------------------
	var top fyne.CanvasObject
	if !filtering {
		if last, ok := svm.LastOpenedSummary(); ok {
			top = buildLastOpenedSection(w, cb, last)
		}
	}
	if banner := maybeSkippedBanner(svm.Skipped()); banner != nil {
		top = stackIfBoth(top, banner)
	}

	// --- bottom: archived accordion ------------------------------------
	var bottom fyne.CanvasObject
	if totalArchived > 0 {
		bottom = buildArchivedSection(
			w, svm, cb, svm.FilteredArchived(), totalArchived, filtering, rerender)
	}

	return container.NewBorder(top, bottom, nil, nil, center)
}

// emptyStateLabel picks the right "nothing to show" message based on
// whether the user is filtering and whether archived workspaces exist
// that could explain the empty active list.
func emptyStateLabel(query string, filtering bool, totalActive, totalArchived int) *widget.Label {
	var msg string
	switch {
	case filtering && totalActive+totalArchived > 0:
		msg = fmt.Sprintf(
			"No workspaces match %q.\n\nTry a different search term or clear the search.",
			query,
		)
	case totalActive == 0 && totalArchived > 0:
		msg = "No active workspaces.\n\nAll your workspaces are currently archived. " +
			"Expand “Archived” below to restore one, or create a new workspace."
	default:
		msg = "No workspaces yet.\n\nClick “Create New Workspace” to start a new project."
	}
	lbl := widget.NewLabel(msg)
	lbl.Wrapping = fyne.TextWrapWord
	lbl.Alignment = fyne.TextAlignCenter
	return lbl
}

// buildArchivedSection renders a collapsed accordion titled
// "Archived (N)" (or "Archived (m of N)" while filtering). Expanding it
// reveals archived workspace cards with Unarchive actions.
func buildArchivedSection(
	w fyne.Window,
	svm *StartViewModel,
	cb StartScreenCallbacks,
	items []workspace.Summary,
	totalArchived int,
	filtering bool,
	rerender func(),
) fyne.CanvasObject {
	var content fyne.CanvasObject
	if len(items) == 0 {
		lbl := widget.NewLabel(fmt.Sprintf(
			"No archived workspaces match %q.", svm.SearchQuery))
		lbl.Wrapping = fyne.TextWrapWord
		lbl.Alignment = fyne.TextAlignCenter
		content = container.NewPadded(lbl)
	} else {
		rows := container.NewVBox()
		for i := range items {
			item := items[i]
			rows.Add(workspaceRow(item,
				func() { openSummary(w, cb, item) },
				func() { showExportWorkspaceDialog(w, svm, item) },
				func() { archiveSummary(w, svm, item, false, rerender) },
			))
			rows.Add(vSpace(4))
		}
		content = rows
	}

	title := fmt.Sprintf("Archived (%d)", totalArchived)
	if filtering {
		title = fmt.Sprintf("Archived (%d of %d)", len(items), totalArchived)
	}
	acc := widget.NewAccordion(widget.NewAccordionItem(title, content))
	return container.NewPadded(acc)
}

// stackIfBoth vertically concatenates two optional objects. Returns
// nil when both are nil, the surviving object when exactly one is nil,
// and a VBox otherwise.
func stackIfBoth(a, b fyne.CanvasObject) fyne.CanvasObject {
	switch {
	case a == nil && b == nil:
		return nil
	case a == nil:
		return b
	case b == nil:
		return a
	default:
		return container.NewVBox(a, b)
	}
}

// openSummary resolves a Summary to a Workspace and hands it to OnOpen.
func openSummary(w fyne.Window, cb StartScreenCallbacks, s workspace.Summary) {
	ws, err := workspace.LoadWorkspace(s.Root)
	if err != nil {
		dialog.ShowError(err, w)
		return
	}
	if cb.OnOpen != nil {
		cb.OnOpen(ws)
	}
}

// archiveSummary flips the archived flag and rerenders so the workspace
// moves between the active list and the archived accordion without the
// user needing to refresh manually.
func archiveSummary(
	w fyne.Window,
	svm *StartViewModel,
	s workspace.Summary,
	archived bool,
	rerender func(),
) {
	if err := svm.SetArchived(s, archived); err != nil {
		dialog.ShowError(err, w)
		return
	}
	rerender()
}

func buildLastOpenedSection(
	w fyne.Window,
	cb StartScreenCallbacks,
	s workspace.Summary,
) fyne.CanvasObject {
	openBtn := widget.NewButton("Open", func() {
		ws, err := workspace.LoadWorkspace(s.Root)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		if cb.OnOpen != nil {
			cb.OnOpen(ws)
		}
	})
	openBtn.Importance = widget.HighImportance

	meta := widget.NewLabel(formatMeta(s))
	meta.TextStyle = fyne.TextStyle{Italic: true}
	meta.Wrapping = fyne.TextWrapWord

	body := container.NewBorder(nil, nil, nil, openBtn, meta)
	card := widget.NewCard("Last opened", displayName(s.Name), body)
	return container.NewPadded(card)
}

func workspaceRow(s workspace.Summary, onOpen, onExport, onArchiveToggle func()) fyne.CanvasObject {
	name := widget.NewLabel(displayName(s.Name))
	name.TextStyle = fyne.TextStyle{Bold: true}

	openBtn := widget.NewButton("Open", onOpen)
	openBtn.Importance = widget.HighImportance

	exportBtn := widget.NewButton("Export…", onExport)

	archiveLabel := "Archive"
	if s.Archived {
		archiveLabel = "Unarchive"
	}
	archiveBtn := widget.NewButton(archiveLabel, onArchiveToggle)

	meta := widget.NewLabel(formatMeta(s))
	meta.TextStyle = fyne.TextStyle{Italic: true}
	meta.Wrapping = fyne.TextWrapWord

	path := widget.NewLabel(s.Root)
	path.TextStyle = fyne.TextStyle{Italic: true}
	path.Wrapping = fyne.TextWrapWord

	actions := container.NewHBox(archiveBtn, exportBtn, openBtn)

	body := container.NewVBox(
		container.NewBorder(nil, nil, nil, actions, name),
		path,
		meta,
	)
	return widget.NewCard("", "", container.NewPadded(body))
}

func displayName(s string) string {
	if s == "" {
		return "(unnamed workspace)"
	}
	return s
}

func formatMeta(s workspace.Summary) string {
	parts := []string{"Updated " + formatTimeRelative(s.UpdatedAt)}
	if s.ReferenceOsuPath != "" {
		parts = append(parts, "Ref: "+filepath.Base(s.ReferenceOsuPath))
	}
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "  ·  "
		}
		out += p
	}
	return out
}

func formatTimeRelative(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	diff := time.Since(t)
	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	case diff < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(diff.Hours()/24))
	default:
		return t.Local().Format("2006-01-02")
	}
}

func maybeSkippedBanner(skipped []workspace.SkippedEntry) fyne.CanvasObject {
	if len(skipped) == 0 {
		return nil
	}
	msg := fmt.Sprintf(
		"⚠ %d workspace directory could not be loaded (corrupt or unsupported project file)",
		len(skipped),
	)
	if len(skipped) != 1 {
		msg = fmt.Sprintf(
			"⚠ %d workspace directories could not be loaded (corrupt or unsupported project files)",
			len(skipped),
		)
	}
	lbl := widget.NewLabel(msg)
	lbl.Wrapping = fyne.TextWrapWord
	return container.NewPadded(widget.NewCard("", "", container.NewPadded(lbl)))
}

func showCreateWorkspaceDialog(
	w fyne.Window,
	svm *StartViewModel,
	cb StartScreenCallbacks,
	rerender func(),
) {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("e.g. My Song - hard")

	auto := &AutoProjectName{}
	nameEntry.OnChanged = func(s string) { auto.UserTyped(s) }

	refEntry := widget.NewEntry()
	refEntry.SetPlaceHolder(`Optional: path to reference .osu`)
	if svm.LastReferencePath != "" {
		refEntry.SetText(svm.LastReferencePath)
	}

	applySuggestion := func() {
		if s, ok := auto.Suggest(suggestNameFromReference(refEntry.Text)); ok {
			nameEntry.SetText(s)
		}
	}
	refEntry.OnChanged = func(string) { applySuggestion() }

	browseBtn := widget.NewButton("Browse…", func() {
		fd := dialog.NewFileOpen(func(rc fyne.URIReadCloser, err error) {
			if err != nil || rc == nil {
				return
			}
			defer rc.Close()
			refEntry.SetText(rc.URI().Path())
			svm.LastReferencePath = rc.URI().Path()
			applySuggestion()
		}, w)
		fd.SetFilter(storage.NewExtensionFileFilter([]string{".osu"}))
		fd.Show()
	})

	detectBtn := widget.NewButton("Select current beatmap", func() {
		detector := &detect.DetectorAdapter{
			D: detect.NewStableDetector(detect.WindowsProcessFinder{}),
		}
		path, err := detector.Detect()
		if err != nil {
			dialog.ShowError(
				fmt.Errorf("could not detect current beatmap: %w", err), w)
			return
		}
		refEntry.SetText(path)
		svm.LastReferencePath = path
		applySuggestion()
	})

	refRow := container.NewBorder(nil, nil, nil,
		container.NewHBox(detectBtn, browseBtn),
		refEntry,
	)

	tmplByLabel := map[string]workspace.TemplateDescriptor{}
	tmplOptions := []string{}
	for _, t := range svm.Catalog.List() {
		tmplByLabel[t.Label] = t
		tmplOptions = append(tmplOptions, t.Label)
	}
	tmplSelect := widget.NewSelect(tmplOptions, nil)
	tmplSelect.SetSelected(svm.Catalog.Default().Label)

	samplesetSelect := widget.NewSelect(
		[]string{
			string(domain.SamplesetSoft),
			string(domain.SamplesetNormal),
			string(domain.SamplesetDrum),
		},
		nil,
	)
	samplesetSelect.SetSelected(string(domain.SamplesetSoft))

	errLabel := widget.NewLabel("")
	errLabel.Wrapping = fyne.TextWrapWord
	errLabel.Hide()

	form := container.NewVBox(
		widget.NewForm(
			&widget.FormItem{Text: "Name", Widget: nameEntry,
				HintText: "Required"},
			&widget.FormItem{Text: "Reference .osu", Widget: refRow,
				HintText: "Optional: selecting a file auto-fills the project name"},
			&widget.FormItem{Text: "Template", Widget: tmplSelect,
				HintText: "DAW template used for the project"},
			&widget.FormItem{Text: "Default sampleset", Widget: samplesetSelect},
		),
		errLabel,
	)

	var d dialog.Dialog
	submit := func() {
		req := workspace.CreateRequest{
			Name:             nameEntry.Text,
			ReferenceOsuPath: refEntry.Text,
			Template:         tmplByLabel[tmplSelect.Selected],
			DefaultSampleset: domain.Sampleset(samplesetSelect.Selected),
		}
		svm.LastReferencePath = refEntry.Text
		ws, err := svm.CreateWorkspace(req)
		if err != nil {
			errLabel.SetText(formatCreateError(err))
			errLabel.Show()
			return
		}
		d.Hide()
		if cb.OnCreate != nil {
			cb.OnCreate(ws)
			return
		}
		rerender()
	}

	d = dialog.NewCustomConfirm(
		"Create new workspace",
		"Create",
		"Cancel",
		form,
		func(ok bool) {
			if ok {
				submit()
			}
		},
		w,
	)
	d.Resize(fyne.NewSize(750, 360))
	d.Show()
}

func formatCreateError(err error) string {
	if fe, ok := err.(workspace.FieldErrors); ok {
		var b []byte
		b = append(b, "Please fix the following:\n"...)
		for _, field := range []string{
			workspace.FieldName,
			workspace.FieldReferenceOsuPath,
			workspace.FieldTemplate,
			workspace.FieldDefaultSampleset,
		} {
			if msg, has := fe[field]; has {
				b = append(b, "  • "...)
				b = append(b, msg...)
				b = append(b, '\n')
			}
		}
		return string(b)
	}
	return err.Error()
}

// showExportWorkspaceDialog prompts the user for a destination .zip
// and exports the selected workspace there. Uses Fyne's file save
// dialog; the view model owns the actual zip write.
func showExportWorkspaceDialog(
	w fyne.Window,
	svm *StartViewModel,
	item workspace.Summary,
) {
	fd := dialog.NewFileSave(func(wc fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		if wc == nil {
			return // user cancelled
		}
		// We need the target path; Fyne's URIWriteCloser exposes it
		// via URI().Path(). Close the handle first so we own the file.
		target := wc.URI().Path()
		_ = wc.Close()

		if !strings.EqualFold(filepath.Ext(target), workspace.ArchiveFileExtension) {
			target += workspace.ArchiveFileExtension
		}
		if err := svm.ExportWorkspaceToZip(item, target); err != nil {
			dialog.ShowError(fmt.Errorf("export failed: %w", err), w)
			return
		}
		dialog.ShowInformation(
			"Workspace exported",
			"Saved to:\n"+target,
			w,
		)
	}, w)

	// Suggest a friendly filename derived from the workspace name.
	fd.SetFileName(workspace.SuggestExportFileName(&workspace.Workspace{
		Project: &workspace.ProjectFile{Name: item.Name},
	}))
	fd.SetFilter(storage.NewExtensionFileFilter([]string{workspace.ArchiveFileExtension}))
	fd.Show()
}

// showImportWorkspaceDialog prompts the user for a .zip archive and
// imports it under a fresh workspace ID. On success the list refreshes;
// the callback opens the newly imported workspace if OnCreate is wired.
func showImportWorkspaceDialog(
	w fyne.Window,
	svm *StartViewModel,
	cb StartScreenCallbacks,
	rerender func(),
) {
	fd := dialog.NewFileOpen(func(rc fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		if rc == nil {
			return // user cancelled
		}
		src := rc.URI().Path()
		_ = rc.Close()

		ws, err := svm.ImportWorkspaceFromZip(src)
		if err != nil {
			dialog.ShowError(fmt.Errorf("import failed: %w", err), w)
			return
		}
		rerender()
		dialog.ShowInformation(
			"Workspace imported",
			fmt.Sprintf("%q was imported successfully.", ws.Project.Name),
			w,
		)
		if cb.OnCreate != nil {
			cb.OnCreate(ws)
		}
	}, w)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{workspace.ArchiveFileExtension}))
	fd.Show()
}
