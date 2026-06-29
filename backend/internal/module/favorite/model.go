package favorite

import "time"

const (
	TargetCarpool    = "carpool"
	TargetAPIService = "api_service"
)

type Favorite struct {
	ID         string
	UserID     string
	TargetType string
	TargetID   string
	CreatedAt  time.Time
}

type ListItem struct {
	Favorite
	Title    string
	Subtitle string
	Status   string
	To       string
}

type TargetSummary struct {
	Title    string
	Subtitle string
	Status   string
	To       string
}

type MutationResult struct {
	Favorited bool
	Favorite  *ListItem
}
