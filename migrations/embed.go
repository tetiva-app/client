// Package migrations embeds the SQL schema migrations so every entry point
// (app, cmd/mcp, tests) runs the same files through the same runner.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
