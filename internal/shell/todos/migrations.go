package todos

import (
	"embed"
	"io/fs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrations is the todos feature's schema, owned here and handed to
// kernel.Migrate at startup. Each feature exposes its own like this, so the
// framework never embeds feature SQL itself. The returned FS is rooted at the
// migrations directory, so filenames are bare (e.g. "0001_init.sql").
func Migrations() fs.FS {
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		panic("todos.Migrations: " + err.Error())
	}
	return sub
}
