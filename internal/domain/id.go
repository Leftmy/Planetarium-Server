package domain

import "github.com/google/uuid"

// NewID returns a primary key for any entity in the schema.
//
// It is UUIDv7 rather than the more familiar uuid.New(), which is v4: v7 values
// are time-ordered, so inserts land at the end of the index instead of scattered
// across it. That matters most for the tables that grow fastest — quiz_attempts,
// node_progress, user_activity_days.
//
// Generate identifiers through this function rather than calling uuid directly;
// a stray uuid.New() would silently reintroduce random keys.
func NewID() (uuid.UUID, error) {
	return uuid.NewV7()
}
