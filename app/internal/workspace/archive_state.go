package workspace

func SetArchived(paths Paths, archived bool) error {
	pf, err := LoadProjectFile(paths)
	if err != nil {
		return err
	}
	if pf.Archived == archived {
		return nil
	}
	pf.Archived = archived
	return SaveProjectFile(paths, pf)
}

func ArchiveWorkspace(paths Paths) error { return SetArchived(paths, true) }

func UnarchiveWorkspace(paths Paths) error { return SetArchived(paths, false) }
