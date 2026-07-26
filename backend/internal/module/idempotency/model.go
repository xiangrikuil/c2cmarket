package idempotency

import "time"

const (
	ProcessingLifetime        = 15 * time.Minute
	FailedRetention           = time.Hour
	CompletedRetention        = 7 * 24 * time.Hour
	MaxCachedResponseBodySize = 64 * 1024
)

type Entry struct {
	UserID           string
	RouteKey         string
	Key              string
	RequestHash      string
	State            string
	Status           int
	ContentType      string
	Body             []byte
	BodyCacheAllowed bool
	ResourceType     string
	ResourceID       string
	CreatedAt        time.Time
	CompletedAt      *time.Time
	ExpiresAt        time.Time
}

type Completion struct {
	Status        int
	ContentType   string
	Body          []byte
	SkipBodyCache bool
	ResourceType  string
	ResourceID    string
	Headers       map[string]string
}
