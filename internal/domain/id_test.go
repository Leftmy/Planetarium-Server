package domain_test

import (
	"testing"

	"github.com/leftmy/planetarium-server/internal/domain"
)

// The whole point of NewID is that it is not uuid.New(), which returns v4.
// Random primary keys scatter inserts across the index, and nothing else in the
// codebase would notice the difference.
func TestNewIDIsVersion7(t *testing.T) {
	id, err := domain.NewID()
	if err != nil {
		t.Fatalf("NewID() error = %v", err)
	}

	if got := id.Version(); got != 7 {
		t.Errorf("NewID() version = %d, want 7", got)
	}
}

// v7 encodes a timestamp in its leading bits, so identifiers generated in order
// sort in order. That ordering is the reason inserts stay at the end of the
// B-tree instead of fragmenting it.
func TestNewIDIsTimeOrdered(t *testing.T) {
	const count = 50

	prev, err := domain.NewID()
	if err != nil {
		t.Fatalf("NewID() error = %v", err)
	}

	for i := 1; i < count; i++ {
		next, err := domain.NewID()
		if err != nil {
			t.Fatalf("NewID() error = %v", err)
		}

		if next.String() <= prev.String() {
			t.Fatalf("id %d (%s) does not sort after %s", i, next, prev)
		}

		prev = next
	}
}
