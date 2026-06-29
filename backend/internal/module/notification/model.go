package notification

import "time"

type Notification struct {
	ID              string
	UserID          string
	Type            string
	Title           string
	Body            string
	TargetType      string
	TargetID        string
	TargetURL       string
	SourceEventType string
	SourceEventID   string
	ReadAt          *time.Time
	CreatedAt       time.Time
}

type ReadAllResult struct {
	Count int
	Items []Notification
}
