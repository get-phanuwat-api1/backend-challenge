// Package testutil contains helpers shared by repository and component tests.
package testutil

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	tcmongo "github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/get-phanuwat-api1/backend-challenge/internal/infrastructure/mongodb"
	"github.com/get-phanuwat-api1/backend-challenge/migrations"
	"github.com/get-phanuwat-api1/backend-challenge/pkg/database"
)

// StartMongo returns a MongoDB database with all migrations applied, for tests.
//
// If TEST_MONGO_URI is set it is used directly; otherwise a disposable
// MongoDB container is started with testcontainers (Docker required).
// Each call uses a fresh, uniquely named database.
// The returned cleanup func must be called when done.
func StartMongo(ctx context.Context) (*mongo.Database, func(), error) {
	uri := os.Getenv("TEST_MONGO_URI")
	var container *tcmongo.MongoDBContainer

	if uri == "" {
		var err error
		container, err = tcmongo.Run(ctx, "mongo:8")
		if err != nil {
			return nil, nil, err
		}
		uri, err = container.ConnectionString(ctx)
		if err != nil {
			_ = container.Terminate(ctx)
			return nil, nil, err
		}
	}

	terminate := func() {
		if container != nil {
			_ = container.Terminate(context.Background())
		}
	}

	client, err := database.NewMongoClient(ctx, uri)
	if err != nil {
		terminate()
		return nil, nil, err
	}
	db := client.Database(fmt.Sprintf("test_%d", time.Now().UnixNano()))
	if _, err := database.MigrateUp(ctx, db, migrations.FS); err != nil {
		_ = client.Disconnect(ctx)
		terminate()
		return nil, nil, err
	}

	cleanup := func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
		terminate()
	}
	return db, cleanup, nil
}

// ClearCollections removes all documents (but keeps indexes) between tests.
func ClearCollections(t *testing.T, db *mongo.Database) {
	t.Helper()
	if _, err := db.Collection(mongodb.UsersCollection).DeleteMany(context.Background(), bson.D{}); err != nil {
		t.Fatalf("clear users: %v", err)
	}
}
