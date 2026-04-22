package workspace

import (
	"archive/zip"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const ArchiveFileExtension = ".zip"

func ExportWorkspace(ws *Workspace, destZip string) error {
	if ws == nil || ws.Paths.Root == "" {
		return &Error{Code: ErrProjectFileIncomplete,
			Message: "workspace has no root path"}
	}
	if strings.TrimSpace(destZip) == "" {
		return &Error{Code: ErrIO, Message: "destination zip path is empty"}
	}
	if err := os.MkdirAll(filepath.Dir(destZip), 0o755); err != nil {
		return &Error{Code: ErrIO,
			Message: "cannot create destination directory", Cause: err}
	}

	tmp := destZip + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return &Error{Code: ErrIO, Message: "cannot create zip: " + tmp, Cause: err}
	}

	if writeErr := ExportWorkspaceTo(ws, f); writeErr != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return writeErr
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return &Error{Code: ErrIO, Message: "cannot close zip: " + tmp, Cause: err}
	}
	if err := os.Rename(tmp, destZip); err != nil {
		_ = os.Remove(tmp)
		return &Error{Code: ErrIO,
			Message: "cannot rename zip into place", Cause: err}
	}
	return nil
}

func ExportWorkspaceTo(ws *Workspace, w io.Writer) error {
	if ws == nil || ws.Paths.Root == "" {
		return &Error{Code: ErrProjectFileIncomplete,
			Message: "workspace has no root path"}
	}
	if info, err := os.Stat(ws.Paths.Root); err != nil || !info.IsDir() {
		return &Error{Code: ErrIO,
			Message: "workspace root is not a directory: " + ws.Paths.Root,
			Cause:   err}
	}
	zw := zip.NewWriter(w)

	walkErr := filepath.WalkDir(ws.Paths.Root, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if p == ws.Paths.Root {
			return nil
		}
		rel, err := filepath.Rel(ws.Paths.Root, p)
		if err != nil {
			return err
		}
		// Skip the transient project.odaw.tmp we use during atomic writes.
		base := filepath.Base(p)
		if base == ProjectFileName+".tmp" {
			return nil
		}
		name := filepath.ToSlash(rel)

		if d.IsDir() {
			_, err := zw.Create(name + "/")
			return err
		}
		srcInfo, err := d.Info()
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(srcInfo)
		if err != nil {
			return err
		}
		header.Name = name
		header.Method = zip.Deflate
		writer, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		src, err := os.Open(p)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(writer, src)
		closeErr := src.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if walkErr != nil {
		_ = zw.Close()
		return &Error{Code: ErrIO,
			Message: "cannot pack workspace into zip", Cause: walkErr}
	}
	if err := zw.Close(); err != nil {
		return &Error{Code: ErrIO, Message: "cannot finalize zip", Cause: err}
	}
	return nil
}

func ImportWorkspace(projectsRoot, srcZip string) (*Workspace, error) {
	if strings.TrimSpace(srcZip) == "" {
		return nil, &Error{Code: ErrIO, Message: "source zip path is empty"}
	}
	f, err := os.Open(srcZip)
	if err != nil {
		return nil, &Error{Code: ErrIO,
			Message: "cannot open zip: " + srcZip, Cause: err}
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, &Error{Code: ErrIO,
			Message: "cannot stat zip: " + srcZip, Cause: err}
	}
	return ImportWorkspaceFrom(projectsRoot, f, info.Size())
}

func ImportWorkspaceFrom(projectsRoot string, r io.ReaderAt, size int64) (*Workspace, error) {
	if strings.TrimSpace(projectsRoot) == "" {
		return nil, &Error{Code: ErrIO, Message: "projects root is empty"}
	}
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, &Error{Code: ErrIO,
			Message: "cannot read zip archive", Cause: err}
	}

	projectEntry, prefix, err := locateProjectFileInZip(zr)
	if err != nil {
		return nil, err
	}
	pfBytes, err := readZipFile(projectEntry)
	if err != nil {
		return nil, &Error{Code: ErrIO,
			Message: "cannot read " + ProjectFileName + " from zip", Cause: err}
	}
	pf, err := unmarshalImportedProjectFile(pfBytes)
	if err != nil {
		return nil, err
	}

	// Allocate a brand-new ID so imports never clobber existing
	// workspaces, even if two people traded the same archive.
	pf.ID = NewID(pf.Name)
	pf.UpdatedAt = time.Now().UTC()
	if pf.CreatedAt.IsZero() {
		pf.CreatedAt = pf.UpdatedAt
	}

	dstRoot := WorkspaceRoot(projectsRoot, pf.ID)
	if err := os.MkdirAll(dstRoot, 0o755); err != nil {
		return nil, &Error{Code: ErrIO,
			Message: "cannot create workspace directory: " + dstRoot, Cause: err}
	}
	absDstRoot, err := filepath.Abs(dstRoot)
	if err != nil {
		return nil, &Error{Code: ErrIO,
			Message: "cannot resolve workspace directory", Cause: err}
	}

	for _, f := range zr.File {
		rel, skip := stripPrefix(f.Name, prefix)
		if skip {
			continue
		}
		// project.odaw is written separately below with the new ID.
		if rel == ProjectFileName {
			continue
		}
		if err := extractZipEntry(f, absDstRoot, rel); err != nil {
			// Best-effort cleanup so a failed import doesn't leave a
			// half-extracted workspace lying around.
			_ = os.RemoveAll(dstRoot)
			return nil, err
		}
	}

	paths := PathsFromRoot(dstRoot)
	if err := SaveProjectFile(paths, pf); err != nil {
		_ = os.RemoveAll(dstRoot)
		return nil, err
	}
	return &Workspace{Paths: paths, Project: pf}, nil
}

func SuggestExportFileName(ws *Workspace) string {
	name := "workspace"
	if ws != nil && ws.Project != nil {
		if s := Slug(ws.Project.Name); s != "" {
			name = s
		}
	}
	return name + ArchiveFileExtension
}
func locateProjectFileInZip(zr *zip.Reader) (*zip.File, string, error) {
	var best *zip.File
	var bestPrefix string
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "/") {
			continue
		}
		if path.Base(f.Name) != ProjectFileName {
			continue
		}
		prefix := strings.TrimSuffix(f.Name, ProjectFileName)
		if best == nil || len(prefix) < len(bestPrefix) {
			best = f
			bestPrefix = prefix
		}
	}
	if best == nil {
		return nil, "", &Error{Code: ErrProjectFileMissing,
			Message: "zip does not contain " + ProjectFileName}
	}
	return best, bestPrefix, nil
}

func stripPrefix(name, prefix string) (rel string, skip bool) {
	if prefix == "" {
		return name, name == ""
	}
	if !strings.HasPrefix(name, prefix) {
		return "", true
	}
	rel = strings.TrimPrefix(name, prefix)
	if rel == "" {
		return "", true
	}
	return rel, false
}

func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func unmarshalImportedProjectFile(data []byte) (*ProjectFile, error) {
	var pf ProjectFile
	if err := json.Unmarshal(data, &pf); err != nil {
		return nil, &Error{Code: ErrProjectFileInvalid,
			Message: "project file in zip is not valid JSON", Cause: err}
	}
	if pf.Version <= 0 || pf.Version > CurrentProjectFileVersion {
		return nil, &Error{Code: ErrProjectFileVersion,
			Message: "unsupported project file version in zip"}
	}
	if strings.TrimSpace(pf.Name) == "" {
		return nil, &Error{Code: ErrProjectFileIncomplete,
			Message: "project file in zip is missing required field: name"}
	}
	if pf.Segments == nil {
		pf.Segments = []SegmentInput{}
	}
	return &pf, nil
}

func extractZipEntry(f *zip.File, absDstRoot, rel string) error {
	cleaned := path.Clean(rel)
	if cleaned == "." || cleaned == ".." ||
		strings.HasPrefix(cleaned, "../") ||
		strings.HasPrefix(cleaned, "/") ||
		strings.Contains(cleaned, "/../") ||
		strings.HasSuffix(cleaned, "/..") {
		return &Error{Code: ErrIO,
			Message: "refusing unsafe zip entry: " + f.Name}
	}
	target := filepath.Join(absDstRoot, filepath.FromSlash(cleaned))
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return &Error{Code: ErrIO,
			Message: "cannot resolve zip entry path", Cause: err}
	}
	if absTarget != absDstRoot &&
		!strings.HasPrefix(absTarget, absDstRoot+string(filepath.Separator)) {
		return &Error{Code: ErrIO,
			Message: "zip entry escapes destination: " + f.Name}
	}

	if f.FileInfo().IsDir() || strings.HasSuffix(f.Name, "/") {
		return os.MkdirAll(absTarget, 0o755)
	}
	if err := os.MkdirAll(filepath.Dir(absTarget), 0o755); err != nil {
		return &Error{Code: ErrIO,
			Message: "cannot create directory for zip entry", Cause: err}
	}
	rc, err := f.Open()
	if err != nil {
		return &Error{Code: ErrIO,
			Message: "cannot open zip entry: " + f.Name, Cause: err}
	}
	defer rc.Close()
	out, err := os.Create(absTarget)
	if err != nil {
		return &Error{Code: ErrIO,
			Message: "cannot create file: " + absTarget, Cause: err}
	}
	_, copyErr := io.Copy(out, rc)
	closeErr := out.Close()
	switch {
	case copyErr != nil:
		return &Error{Code: ErrIO,
			Message: "cannot write file: " + absTarget, Cause: copyErr}
	case closeErr != nil:
		return &Error{Code: ErrIO,
			Message: "cannot close file: " + absTarget, Cause: closeErr}
	}
	return nil
}

var _ = SuggestExportFileName(nil)
