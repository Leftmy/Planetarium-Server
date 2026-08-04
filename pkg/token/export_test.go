package token

import "time"

// SetNow replaces the manager's clock so a test can look at an expired token
// without waiting for it to expire.
//
// It lives in a _test.go file on purpose: the file is compiled only into the
// test binary, so production code has no way to move the clock.
func SetNow(m *Manager, now func() time.Time) {
	m.now = now
}
