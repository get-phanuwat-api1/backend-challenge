// Command migrate manages MongoDB schema migrations.
//
//	go run ./cmd/migrate up               apply all pending migrations
//	go run ./cmd/migrate down [N]         roll back the latest N migrations (default 1)
//	go run ./cmd/migrate status           show applied and pending migrations
//	go run ./cmd/migrate create NAME      create new up/down files in ./migrations
//
// The API also applies pending migrations on every start when
// database.auto_migrate is true (the default), i.e. on every deployment.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/get-phanuwat-api1/backend-challenge/config"
	"github.com/get-phanuwat-api1/backend-challenge/migrations"
	"github.com/get-phanuwat-api1/backend-challenge/pkg/database"
)

const usage = "usage: migrate <up | down [N] | status | create NAME>"

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New(usage)
	}
	cmd, rest := args[0], args[1:]

	if cmd == "create" {
		if len(rest) != 1 {
			return errors.New("usage: migrate create NAME")
		}
		return create("migrations", rest[0])
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	client, err := database.NewMongoClient(ctx, cfg.Database.URI)
	if err != nil {
		return err
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	mg, err := database.NewMigrator(client.Database(cfg.Database.Name), migrations.FS)
	if err != nil {
		return err
	}

	switch cmd {
	case "up":
		applied, err := mg.Up(ctx)
		if err != nil {
			return err
		}
		if len(applied) == 0 {
			fmt.Println("no pending migrations")
		}
		for _, m := range applied {
			fmt.Printf("applied  %06d  %s\n", m.Version, m.Description)
		}
	case "down":
		steps := 1
		if len(rest) == 1 {
			if steps, err = strconv.Atoi(rest[0]); err != nil || steps <= 0 {
				return errors.New("N must be a positive integer")
			}
		}
		reverted, err := mg.Down(ctx, steps)
		if err != nil {
			return err
		}
		if len(reverted) == 0 {
			fmt.Println("nothing to roll back")
		}
		for _, m := range reverted {
			fmt.Printf("reverted %06d  %s\n", m.Version, m.Description)
		}
	case "status":
	default:
		return fmt.Errorf("unknown command %q\n%s", cmd, usage)
	}
	return printStatus(ctx, mg)
}

func printStatus(ctx context.Context, mg *database.Migrator) error {
	history, err := mg.History(ctx)
	if err != nil {
		return err
	}
	pending, err := mg.Pending(ctx)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "\nVERSION\tDESCRIPTION\tSTATUS\tAPPLIED AT\tTIME")
	for _, h := range history {
		fmt.Fprintf(w, "%06d\t%s\tapplied\t%s\t%dms\n", h.Version, h.Description, h.AppliedAt.Local().Format("2006-01-02 15:04:05"), h.ExecutionMs)
	}
	for _, p := range pending {
		fmt.Fprintf(w, "%06d\t%s\tpending\t-\t-\n", p.Version, p.Description)
	}
	return w.Flush()
}

var (
	versionPrefix = regexp.MustCompile(`^(\d+)_`)
	nonAlnum      = regexp.MustCompile(`[^a-zA-Z0-9]+`)
)

// create writes the next sequential {version}_{name}.up.json / .down.json pair.
func create(dir, name string) error {
	name = strings.Trim(strings.ToLower(nonAlnum.ReplaceAllString(name, "_")), "_")
	if name == "" {
		return errors.New("NAME must contain letters or digits")
	}
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
	for _, direction := range []string{"up", "down"} {
		path := filepath.Join(dir, fmt.Sprintf("%06d_%s.%s.json", next, name, direction))
		if err := os.WriteFile(path, []byte("[\n]\n"), 0o644); err != nil {
			return err
		}
		fmt.Println("created", path)
	}
	return nil
}
