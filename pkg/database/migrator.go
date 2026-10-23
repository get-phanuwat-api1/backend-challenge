package database

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	// HistoryCollection records one document per applied migration.
	HistoryCollection = "schema_migrations"
	// lockCollection holds the single lock document taken while migrating.
	lockCollection = "schema_migrations_lock"
	lockID         = "migration"
	// lockTTL bounds how long a crashed instance can block others.
	lockTTL       = 5 * time.Minute
	lockRetryWait = 500 * time.Millisecond
)

// AppliedMigration is one document of the schema_migrations history collection.
type AppliedMigration struct {
	Version     int64     `bson:"_id"`
	Description string    `bson:"description"`
	Checksum    string    `bson:"checksum"`
	AppliedAt   time.Time `bson:"applied_at"`
	ExecutionMs int64     `bson:"execution_ms"`
}

// Migrator applies versioned migrations to a MongoDB database and records one
// history document per migration in schema_migrations.
//
// A lock document ensures that several instances starting at the same time
// (e.g. replicas of a new deployment) never migrate concurrently.
//
// MongoDB cannot wrap index or collection commands in a transaction, so a
// migration that fails half way is not recorded and is retried on the next
// run: write commands that are safe to repeat (createIndexes is).
type Migrator struct {
	db         *mongo.Database
	migrations []Migration
}

// NewMigrator loads the migrations from fsys.
func NewMigrator(db *mongo.Database, fsys fs.FS) (*Migrator, error) {
	migs, err := LoadMigrations(fsys)
	if err != nil {
		return nil, err
	}
	return &Migrator{db: db, migrations: migs}, nil
}

// MigrateUp is a convenience helper: apply every pending migration and return the ones applied.
func MigrateUp(ctx context.Context, db *mongo.Database, fsys fs.FS) ([]Migration, error) {
	m, err := NewMigrator(db, fsys)
	if err != nil {
		return nil, err
	}
	return m.Up(ctx)
}

// Up applies all pending migrations in version order.
func (m *Migrator) Up(ctx context.Context) ([]Migration, error) {
	var applied []Migration
	err := m.withLock(ctx, func() error {
		history, err := m.loadHistory(ctx)
		if err != nil {
			return err
		}
		if err := m.verify(history); err != nil {
			return err
		}
		for _, mig := range m.migrations {
			if _, done := history[mig.Version]; done {
				continue
			}
			if err := m.apply(ctx, mig); err != nil {
				return err
			}
			applied = append(applied, mig)
		}
		return nil
	})
	return applied, err
}

// Down rolls back the latest `steps` applied migrations, newest first.
func (m *Migrator) Down(ctx context.Context, steps int) ([]Migration, error) {
	if steps <= 0 {
		return nil, errors.New("steps must be > 0")
	}
	var reverted []Migration
	err := m.withLock(ctx, func() error {
		history, err := m.loadHistory(ctx)
		if err != nil {
			return err
		}
		for i := len(m.migrations) - 1; i >= 0 && len(reverted) < steps; i-- {
			mig := m.migrations[i]
			if _, done := history[mig.Version]; !done {
				continue
			}
			if err := m.revert(ctx, mig); err != nil {
				return err
			}
			reverted = append(reverted, mig)
		}
		return nil
	})
	return reverted, err
}

// History returns the applied migrations, oldest first.
func (m *Migrator) History(ctx context.Context) ([]AppliedMigration, error) {
	history, err := m.loadHistory(ctx)
	if err != nil {
		return nil, err
	}
	return sortedHistory(history), nil
}

// Pending returns the migrations that are not applied yet.
func (m *Migrator) Pending(ctx context.Context) ([]Migration, error) {
	history, err := m.loadHistory(ctx)
	if err != nil {
		return nil, err
	}
	var pending []Migration
	for _, mig := range m.migrations {
		if _, done := history[mig.Version]; !done {
			pending = append(pending, mig)
		}
	}
	return pending, nil
}

// --- internals -------------------------------------------------------------------

// withLock runs fn while holding the migration lock document.
//
// Acquiring is one atomic upsert: it inserts the lock when there is none, or
// takes it over when the previous holder's lease expired. When another
// instance holds a live lock the upsert hits the unique _id and we retry.
func (m *Migrator) withLock(ctx context.Context, fn func() error) error {
	owner := randomOwner()
	locks := m.db.Collection(lockCollection)
	for {
		now := time.Now()
		_, err := locks.UpdateOne(ctx,
			bson.D{{Key: "_id", Value: lockID}, {Key: "expires_at", Value: bson.D{{Key: "$lt", Value: now}}}},
			bson.D{{Key: "$set", Value: bson.D{
				{Key: "owner", Value: owner},
				{Key: "expires_at", Value: now.Add(lockTTL)},
			}}},
			options.UpdateOne().SetUpsert(true),
		)
		if err == nil {
			break
		}
		if !mongo.IsDuplicateKeyError(err) {
			return fmt.Errorf("acquire migration lock: %w", err)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("acquire migration lock: %w", ctx.Err())
		case <-time.After(lockRetryWait):
		}
	}
	defer func() {
		_, _ = locks.DeleteOne(context.WithoutCancel(ctx), bson.D{{Key: "_id", Value: lockID}, {Key: "owner", Value: owner}})
	}()
	return fn()
}

func (m *Migrator) loadHistory(ctx context.Context) (map[int64]AppliedMigration, error) {
	cur, err := m.db.Collection(HistoryCollection).Find(ctx, bson.D{})
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", HistoryCollection, err)
	}
	var docs []AppliedMigration
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode %s: %w", HistoryCollection, err)
	}
	history := make(map[int64]AppliedMigration, len(docs))
	for _, h := range docs {
		history[h.Version] = h
	}
	return history, nil
}

// verify refuses to continue when an applied migration was edited or deleted.
func (m *Migrator) verify(history map[int64]AppliedMigration) error {
	files := map[int64]Migration{}
	for _, mig := range m.migrations {
		files[mig.Version] = mig
	}
	for _, h := range sortedHistory(history) {
		mig, ok := files[h.Version]
		if !ok {
			return fmt.Errorf("migration %d (%s) is applied but its file is missing", h.Version, h.Description)
		}
		if mig.Checksum != h.Checksum {
			return fmt.Errorf("migration %d (%s) was modified after it was applied: create a new migration instead", h.Version, h.Description)
		}
	}
	return nil
}

func (m *Migrator) apply(ctx context.Context, mig Migration) error {
	start := time.Now()
	if err := m.run(ctx, mig.Up); err != nil {
		return fmt.Errorf("apply migration %d (%s): %w", mig.Version, mig.Description, err)
	}
	_, err := m.db.Collection(HistoryCollection).InsertOne(ctx, AppliedMigration{
		Version:     mig.Version,
		Description: mig.Description,
		Checksum:    mig.Checksum,
		AppliedAt:   time.Now().UTC(),
		ExecutionMs: time.Since(start).Milliseconds(),
	})
	if err != nil {
		return fmt.Errorf("record migration %d (%s): %w", mig.Version, mig.Description, err)
	}
	return nil
}

func (m *Migrator) revert(ctx context.Context, mig Migration) error {
	if len(mig.Down) == 0 {
		return fmt.Errorf("migration %d (%s) has no .down.json file", mig.Version, mig.Description)
	}
	if err := m.run(ctx, mig.Down); err != nil {
		return fmt.Errorf("revert migration %d (%s): %w", mig.Version, mig.Description, err)
	}
	if _, err := m.db.Collection(HistoryCollection).DeleteOne(ctx, bson.D{{Key: "_id", Value: mig.Version}}); err != nil {
		return fmt.Errorf("unrecord migration %d (%s): %w", mig.Version, mig.Description, err)
	}
	return nil
}

func (m *Migrator) run(ctx context.Context, commands []bson.D) error {
	for i, cmd := range commands {
		if err := m.db.RunCommand(ctx, cmd).Err(); err != nil {
			return fmt.Errorf("command %d (%s): %w", i, cmd[0].Key, err)
		}
	}
	return nil
}

func sortedHistory(history map[int64]AppliedMigration) []AppliedMigration {
	out := make([]AppliedMigration, 0, len(history))
	for _, h := range history {
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out
}

func randomOwner() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
