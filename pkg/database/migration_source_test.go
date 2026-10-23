package database_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/get-phanuwat-api1/backend-challenge/migrations"
	"github.com/get-phanuwat-api1/backend-challenge/pkg/database"
)

const createIndex = `[{"createIndexes": "users", "indexes": [{"key": {"email": 1}, "name": "uniq_email", "unique": true}]}]`

func TestLoadMigrations_Sorted(t *testing.T) {
	fsys := fstest.MapFS{
		"000002_second.up.json":   {Data: []byte(`[{"create": "b"}]`)},
		"000001_first.up.json":    {Data: []byte(createIndex)},
		"000001_first.down.json":  {Data: []byte(`[{"dropIndexes": "users", "index": "uniq_email"}]`)},
		"README.md":               {Data: []byte("ignored")},
		"000002_second.down.json": {Data: []byte(`[{"drop": "b"}]`)},
	}
	migs, err := database.LoadMigrations(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if len(migs) != 2 || migs[0].Version != 1 || migs[1].Version != 2 {
		t.Fatalf("unexpected migrations: %+v", migs)
	}
	first := migs[0]
	if first.Description != "first" || len(first.Up) != 1 || len(first.Down) != 1 || len(first.Checksum) != 64 {
		t.Fatalf("unexpected first migration: %+v", first)
	}
	if first.Up[0][0].Key != "createIndexes" {
		t.Fatalf("command not parsed: %v", first.Up[0])
	}
}

func TestLoadMigrations_Errors(t *testing.T) {
	tests := map[string]struct {
		fsys fstest.MapFS
		want string
	}{
		"bad name":         {fstest.MapFS{"create_users.up.json": {Data: []byte(createIndex)}}, "invalid migration file name"},
		"zero version":     {fstest.MapFS{"000000_x.up.json": {Data: []byte(createIndex)}}, "must be > 0"},
		"not an array":     {fstest.MapFS{"000001_x.up.json": {Data: []byte(`{"create": "a"}`)}}, "JSON array"},
		"empty command":    {fstest.MapFS{"000001_x.up.json": {Data: []byte(`[{}]`)}}, "empty"},
		"missing up":       {fstest.MapFS{"000001_x.down.json": {Data: []byte(`[{"drop": "a"}]`)}}, "no .up.json"},
		"empty up":         {fstest.MapFS{"000001_x.up.json": {Data: []byte(`[]`)}}, "no .up.json"},
		"name mismatch":    {fstest.MapFS{"000001_a.up.json": {Data: []byte(createIndex)}, "000001_b.down.json": {Data: []byte(`[{"drop": "a"}]`)}}, "two different names"},
		"invalid ext json": {fstest.MapFS{"000001_x.up.json": {Data: []byte(`[{"insert": "a", "at": {"$date": "not-a-date"}}]`)}}, "command 0"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := database.LoadMigrations(tt.fsys)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

// The project's own migration files must always load.
func TestProjectMigrationsAreValid(t *testing.T) {
	migs, err := database.LoadMigrations(migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range migs {
		if len(m.Down) == 0 {
			t.Errorf("migration %d (%s) has no down file", m.Version, m.Description)
		}
	}
}
