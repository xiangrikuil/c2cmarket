package idempotency

import "time"

type Entry struct {
	UserID       string
	RouteKey     string
	Key          string
	RequestHash  string
	State        string
	Status       int
	ContentType  string
	Body         []byte
	ResourceType string
	ResourceID   string
	CreatedAt    time.Time
	CompletedAt  *time.Time
	ExpiresAt    time.Time
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
