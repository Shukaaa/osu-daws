package detect

type DetectorAdapter struct {
	D *StableDetector
}

func (a *DetectorAdapter) Detect() (string, error) {
	path, derr := a.D.Detect()
	if derr != nil {
		return "", derr
	}
	return path, nil
}
