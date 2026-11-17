// Package migrations embeds the numbered SQL migrations applied at startup.
package migrations

import (
	"embed"
	"io/fs"
	"sort"
	"strings"
)

//go:embed *.sql
var files embed.FS

// Names returns the migration filenames in lexical (numbered) order.
func Names() ([]string, error) {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names, nil
}

// Read returns the SQL body for a migration.
func Read(name string) (string, error) {
	body, err := files.ReadFile(name)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
