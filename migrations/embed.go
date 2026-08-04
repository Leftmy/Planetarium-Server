// Package migrations embeds the SQL migration files so the compiled binary
// carries its own schema history and the Docker image stays self-contained.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
