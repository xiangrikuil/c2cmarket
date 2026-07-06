package health

import (
	"context"
	"time"
)

type Checker interface {
	Readiness(context.Context) Status
}

type Status struct {
	Configured            bool
	OK                    bool
	SchemaVersion         *int64
	SchemaDirty           *bool
	ExpectedSchemaVersion int64
	CheckedAt             time.Time
	FailureSummary        string
}
