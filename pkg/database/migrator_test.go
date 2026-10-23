package database_test

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/get-phanuwat-api1/backend-challenge/migrations"
	"github.com/get-phanuwat-api1/backend-challenge/pkg/database"
	"github.com/get-phanuwat-api1/backend-challenge/test/testutil"
)

// Migrator tests run against a real MongoDB (testcontainers, or TEST_MONGO_URI).
// The LoadMigrations tests above still run with `go test -short`.
var client *mongo.Client

func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
	flag.Parse()
	if testing.Short() {
		return m.Run()
	}
	db, cleanup, err := testutil.StartMongo(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, "start mongo:", err)
		return 1
	}
	defer cleanup()
	client = db.Client()
	return m.Run()
}

// freshDB returns an empty database that is dropped after the test.
func freshDB(t *testing.T) *mongo.Database {
	t.Helper()
	if testing.Short() {
		t.Skip("needs MongoDB; skipped in -short mode")
	}
	db := client.Database(fmt.Sprintf("migrate_%d", time.Now().UnixNano()))
	t.Cleanup(func() { _ = db.Drop(context.Background()) })
	return db
}

func insertUser(db *mongo.Database, email string) error {
	_, err := db.Collection("users").InsertOne(context.Background(), bson.D{{Key: "email", Value: email}})
	return err
}

func TestMigrator_UpDownWithProjectMigrations(t *testing.T) {
	db := freshDB(t)
	ctx := context.Background()
	mg, err := database.NewMigrator(db, migrations.FS)
	if err != nil {
		t.Fatal(err)
	}

	applied, err := mg.Up(ctx)
	if err != nil || len(applied) == 0 {
		t.Fatalf("Up = %v, %v", applied, err)
	}
	// The unique index from 000001 is live.
	if err := insertUser(db, "a@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := insertUser(db, "a@example.com"); !mongo.IsDuplicateKeyError(err) {
		t.Fatalf("duplicate email should be rejected, got %v", err)
	}

	// A second Up (the next deploy) is a no-op.
	again, err := mg.Up(ctx)
	if err != nil || len(again) != 0 {
		t.Fatalf("second Up = %v, %v; want nothing applied", again, err)
	}
	history, err := mg.History(ctx)
	if err != nil || len(history) != len(applied) || history[0].Version != 1 || history[0].Checksum == "" {
		t.Fatalf("History = %+v, %v", history, err)
	}
	if pending, _ := mg.Pending(ctx); len(pending) != 0 {
		t.Fatalf("Pending = %+v, want none", pending)
	}

	// Roll everything back: the index is gone, duplicates are accepted again.
	reverted, err := mg.Down(ctx, len(applied))
	if err != nil || len(reverted) != len(applied) {
		t.Fatalf("Down = %v, %v", reverted, err)
	}
	if err := insertUser(db, "a@example.com"); err != nil {
		t.Fatalf("after Down, duplicates should be allowed: %v", err)
	}
	if pending, _ := mg.Pending(ctx); len(pending) != len(applied) {
		t.Fatalf("Pending after Down = %d, want %d", len(pending), len(applied))
	}
	if _, err := mg.Down(ctx, 0); err == nil {
		t.Error("Down(0) should fail")
	}
}

func TestMigrator_RefusesModifiedMigration(t *testing.T) {
	db := freshDB(t)
	ctx := context.Background()
	v1 := fstest.MapFS{"000001_create_a.up.json": {Data: []byte(`[{"create": "a"}]`)}}
	if _, err := database.MigrateUp(ctx, db, v1); err != nil {
		t.Fatal(err)
	}

	edited := fstest.MapFS{"000001_create_a.up.json": {Data: []byte(`[{"create": "a_renamed"}]`)}}
	_, err := database.MigrateUp(ctx, db, edited)
	if err == nil || !strings.Contains(err.Error(), "was modified after it was applied") {
		t.Fatalf("got %v, want modified-migration error", err)
	}

	deleted := fstest.MapFS{"000002_other.up.json": {Data: []byte(`[{"create": "b"}]`)}}
	_, err = database.MigrateUp(ctx, db, deleted)
	if err == nil || !strings.Contains(err.Error(), "file is missing") {
		t.Fatalf("got %v, want missing-file error", err)
	}
}

func TestMigrator_FailedMigrationIsNotRecorded(t *testing.T) {
	db := freshDB(t)
	ctx := context.Background()
	bad := fstest.MapFS{
		"000001_ok.up.json":  {Data: []byte(`[{"create": "a"}]`)},
		"000002_bad.up.json": {Data: []byte(`[{"notARealCommand": 1}]`)},
	}
	mg, err := database.NewMigrator(db, bad)
	if err != nil {
		t.Fatal(err)
	}
	applied, err := mg.Up(ctx)
	if err == nil || len(applied) != 1 {
		t.Fatalf("Up = %v, %v; want 1 applied then an error", applied, err)
	}
	history, _ := mg.History(ctx)
	if len(history) != 1 || history[0].Version != 1 {
		t.Fatalf("only the successful migration should be recorded: %+v", history)
	}
}

// Replicas of a new deployment start together: each migration must still be
// applied exactly once.
func TestMigrator_ConcurrentUpAppliesOnce(t *testing.T) {
	db := freshDB(t)
	ctx := context.Background()
	fsys := fstest.MapFS{
		"000001_create_a.up.json": {Data: []byte(`[{"create": "a"}]`)},
		"000002_create_b.up.json": {Data: []byte(`[{"create": "b"}]`)},
	}

	const replicas = 5
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		total int
		errs  []error
	)
	for range replicas {
		wg.Go(func() {
			applied, err := database.MigrateUp(ctx, db, fsys)
			mu.Lock()
			defer mu.Unlock()
			total += len(applied)
			if err != nil {
				errs = append(errs, err)
			}
		})
	}
	wg.Wait()

	if len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	if total != 2 {
		t.Fatalf("migrations applied %d times in total, want exactly 2", total)
	}
	if n, _ := db.Collection("schema_migrations_lock").CountDocuments(ctx, bson.D{}); n != 0 {
		t.Fatalf("lock was not released (%d lock documents)", n)
	}
}

func TestMigrator_TakesOverExpiredLock(t *testing.T) {
	db := freshDB(t)
	ctx := context.Background()
	// A previous instance crashed while holding the lock.
	_, err := db.Collection("schema_migrations_lock").InsertOne(ctx, bson.D{
		{Key: "_id", Value: "migration"}, {Key: "owner", Value: "crashed"}, {Key: "expires_at", Value: time.Now().Add(-time.Minute)},
	})
	if err != nil {
		t.Fatal(err)
	}
	fsys := fstest.MapFS{"000001_create_a.up.json": {Data: []byte(`[{"create": "a"}]`)}}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if applied, err := database.MigrateUp(ctx, db, fsys); err != nil || len(applied) != 1 {
		t.Fatalf("MigrateUp = %v, %v", applied, err)
	}
}
