// Package db provides embedded database migrations for use with
// runtimedatabase.Migrate.
package db

import "embed"

//go:embed migrations/*.sql
var Migrations embed.FS
