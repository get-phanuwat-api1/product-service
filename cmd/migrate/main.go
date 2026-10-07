// Command migrate manages database schema migrations.
//
//	go run ./cmd/migrate up               apply all pending migrations
//	go run ./cmd/migrate down [N]         roll back N migrations (default 1)
//	go run ./cmd/migrate version          print current version
//	go run ./cmd/migrate force V          set version V (fix dirty state)
//	go run ./cmd/migrate create NAME      create new up/down files in ./migrations
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/phanuwat/product-service/config"
	"github.com/phanuwat/product-service/migrations"
	"github.com/phanuwat/product-service/pkg/database"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: migrate <up|down [N]|version|force V|create NAME>")
	}
	cmd, rest := args[0], args[1:]

	if cmd == "create" {
		if len(rest) != 1 {
			return fmt.Errorf("usage: migrate create NAME")
		}
		return create("migrations", rest[0])
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	mg, err := database.NewMigrator(migrations.FS, cfg.Database.DSN())
	if err != nil {
		return err
	}
	defer mg.Close()

	switch cmd {
	case "up":
		if err := mg.Up(); err != nil {
			return err
		}
	case "down":
		steps := 1
		if len(rest) == 1 {
			if steps, err = strconv.Atoi(rest[0]); err != nil || steps <= 0 {
				return fmt.Errorf("N must be a positive integer")
			}
		}
		if err := mg.Down(steps); err != nil {
			return err
		}
	case "force":
		if len(rest) != 1 {
			return fmt.Errorf("usage: migrate force V")
		}
		v, err := strconv.Atoi(rest[0])
		if err != nil {
			return fmt.Errorf("V must be an integer")
		}
		if err := mg.Force(v); err != nil {
			return err
		}
	case "version":
	default:
		return fmt.Errorf("unknown command %q", cmd)
	}

	v, dirty, err := mg.Version()
	if err != nil {
		return err
	}
	fmt.Printf("%s done. current version: %d (dirty: %t)\n", cmd, v, dirty)
	return nil
}

var versionPrefix = regexp.MustCompile(`^(\d+)_`)

// create writes the next sequential {version}_{name}.up.sql / .down.sql pair.
func create(dir, name string) error {
	name = strings.ToLower(regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(name, "_"))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	next := 1
	for _, e := range entries {
		if m := versionPrefix.FindStringSubmatch(e.Name()); m != nil {
			if v, _ := strconv.Atoi(m[1]); v >= next {
				next = v + 1
			}
		}
	}
	for _, dirn := range []string{"up", "down"} {
		path := filepath.Join(dir, fmt.Sprintf("%06d_%s.%s.sql", next, name, dirn))
		if err := os.WriteFile(path, []byte("-- "+dirn+" migration\n"), 0o644); err != nil {
			return err
		}
		fmt.Println("created", path)
	}
	return nil
}
