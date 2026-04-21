package domain

type HitObject struct {
	X            int    `json:"x"`
	Y            int    `json:"y"`
	Time         int    `json:"time"`
	Type         int    `json:"type"`
	HitSound     int    `json:"hitSound"`
	ObjectParams string `json:"objectParams,omitempty"`
	HitSample    string `json:"hitSample,omitempty"`
}
