package database

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Migration is one versioned schema change loaded from a pair of JSON files:
//
//	000001_create_users_email_index.up.json    (required)
//	000001_create_users_email_index.down.json  (optional, needed for rollback)
//
// Each file holds a JSON array of MongoDB database commands (Extended JSON),
// run in order with db.RunCommand, for example:
//
//	[{ "createIndexes": "users", "indexes": [{ "key": { "email": 1 }, "name": "uniq_email", "unique": true }] }]
type Migration struct {
	Version     int64
	Description string // "create users email index", derived from the file name
	Up          []bson.D
	Down        []bson.D
	Checksum    string // sha256 of the .up.json file; detects edits to applied migrations
}

var migrationFileName = regexp.MustCompile(`^(\d+)_([a-z0-9_]+)\.(up|down)\.json$`)

// LoadMigrations reads every *.json file in the root of fsys, sorted by version.
func LoadMigrations(fsys fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}

	byVersion := map[int64]*Migration{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		m := migrationFileName.FindStringSubmatch(e.Name())
		if m == nil {
			return nil, fmt.Errorf("invalid migration file name %q (want 000001_name.up.json / .down.json)", e.Name())
		}
		version, _ := strconv.ParseInt(m[1], 10, 64)
		if version <= 0 {
			return nil, fmt.Errorf("invalid migration version in %q: must be > 0", e.Name())
		}
		description := strings.ReplaceAll(m[2], "_", " ")

		body, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		commands, err := parseCommands(body)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", e.Name(), err)
		}

		mig, ok := byVersion[version]
		if !ok {
			mig = &Migration{Version: version, Description: description}
			byVersion[version] = mig
		} else if mig.Description != description {
			return nil, fmt.Errorf("migration %d has two different names: %q and %q", version, mig.Description, description)
		}

		switch m[3] {
		case "up":
			mig.Up = commands
			sum := sha256.Sum256(body)
			mig.Checksum = hex.EncodeToString(sum[:])
		case "down":
			mig.Down = commands
		}
	}

	out := make([]Migration, 0, len(byVersion))
	for _, m := range byVersion {
		if len(m.Up) == 0 {
			return nil, fmt.Errorf("migration %d (%s) has no .up.json file or it is empty", m.Version, m.Description)
		}
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

// parseCommands decodes a JSON array of Extended JSON command documents.
func parseCommands(body []byte) ([]bson.D, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("want a JSON array of commands: %w", err)
	}
	commands := make([]bson.D, 0, len(raw))
	for i, r := range raw {
		var cmd bson.D
		if err := bson.UnmarshalExtJSON(r, false, &cmd); err != nil {
			return nil, fmt.Errorf("command %d: %w", i, err)
		}
		if len(cmd) == 0 {
			return nil, fmt.Errorf("command %d is empty", i)
		}
		commands = append(commands, cmd)
	}
	return commands, nil
}
