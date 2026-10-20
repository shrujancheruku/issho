// Package db embeds the SQL migrations so binaries can migrate without files on disk.
package db

import "embed"

//go:embed migrations/*.sql
var Migrations embed.FS
