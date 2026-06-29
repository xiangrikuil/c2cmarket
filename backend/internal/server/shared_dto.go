package server

type reviewActionRequest struct {
	Reason string `json:"reason"`
}

type membershipEndRequest struct {
	Reason string `json:"reason"`
}

type emptyRequest struct{}
