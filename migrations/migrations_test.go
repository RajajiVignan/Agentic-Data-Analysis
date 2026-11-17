package migrations

import (
	"sort"
	"strings"
	"testing"
)

func TestNamesAreNumberedAndSorted(t *testing.T) {
	names, err := Names()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) == 0 {
		t.Fatal("no migrations found")
	}
	if !sort.StringsAreSorted(names) {
		t.Fatalf("migrations are not sorted: %v", names)
	}
	for _, name := range names {
		if !strings.HasSuffix(name, ".sql") {
			t.Fatalf("unexpected migration name %q", name)
		}
		if len(name) < 6 || name[4] != '_' {
			t.Fatalf("migration %q must start with a 4-digit number and underscore", name)
		}
		for _, r := range name[:4] {
			if r < '0' || r > '9' {
				t.Fatalf("migration %q must start with a 4-digit number", name)
			}
		}
	}
}

func TestEveryMigrationIsNonEmpty(t *testing.T) {
	names, err := Names()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		body, err := Read(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if strings.TrimSpace(body) == "" {
			t.Fatalf("migration %s is empty", name)
		}
	}
}
