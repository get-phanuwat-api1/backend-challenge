// Package migrations embeds the MongoDB migration files so the binaries are
// self-contained. File naming: {version}_{title}.up.json / .down.json, each a
// JSON array of MongoDB database commands (see pkg/database.Migration).
package migrations

import "embed"

//go:embed *.json
var FS embed.FS
