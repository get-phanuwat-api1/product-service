package database

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Migration is one versioned schema change loaded from a pair of SQL files:
//
//	000001_create_products_table.up.sql    (required)
//	000001_create_products_table.down.sql  (optional, needed for rollback)
type Migration struct {
	Version     int64
	Description string // "create products table", derived from the file name
	UpSQL       string
	DownSQL     string
	Checksum    string // sha256 of UpSQL; detects edits to already-applied migrations
}

var migrationFileName = regexp.MustCompile(`^(\d+)_([a-z0-9_]+)\.(up|down)\.sql$`)

// LoadMigrations reads every *.sql file in the root of fsys, sorted by version.
func LoadMigrations(fsys fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}

	byVersion := map[int64]*Migration{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		m := migrationFileName.FindStringSubmatch(e.Name())
		if m == nil {
			return nil, fmt.Errorf("invalid migration file name %q (want 000001_name.up.sql / .down.sql)", e.Name())
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

		mig, ok := byVersion[version]
		if !ok {
			mig = &Migration{Version: version, Description: description}
			byVersion[version] = mig
		} else if mig.Description != description {
			return nil, fmt.Errorf("migration %d has two different names: %q and %q", version, mig.Description, description)
		}

		switch m[3] {
		case "up":
			mig.UpSQL = string(body)
			sum := sha256.Sum256(body)
			mig.Checksum = hex.EncodeToString(sum[:])
		case "down":
			mig.DownSQL = string(body)
		}
	}

	out := make([]Migration, 0, len(byVersion))
	for _, m := range byVersion {
		if strings.TrimSpace(m.UpSQL) == "" {
			return nil, fmt.Errorf("migration %d (%s) has no .up.sql file", m.Version, m.Description)
		}
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}
