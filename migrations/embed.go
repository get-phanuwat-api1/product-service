// Package migrations embeds the SQL migration files so the binaries are self-contained.
// File naming follows golang-migrate: {version}_{title}.up.sql / .down.sql
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
