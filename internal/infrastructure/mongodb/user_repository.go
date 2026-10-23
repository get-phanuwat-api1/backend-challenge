// Package mongodb is the adapter that implements application.UserRepository on
// MongoDB with the official Go driver.
package mongodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/get-phanuwat-api1/backend-challenge/internal/application"
	"github.com/get-phanuwat-api1/backend-challenge/internal/domain"
)

// UsersCollection is the MongoDB collection that stores users.
const UsersCollection = "users"

// userDocument is the persistence model; it keeps BSON concerns out of the domain.
type userDocument struct {
	ID        bson.ObjectID `bson:"_id"`
	Name      string        `bson:"name"`
	Email     string        `bson:"email"`
	Password  string        `bson:"password"`
	CreatedAt time.Time     `bson:"created_at"`
}

func (d userDocument) toDomain() domain.User {
	return domain.User{
		ID:           d.ID.Hex(),
		Name:         d.Name,
		Email:        d.Email,
		PasswordHash: d.Password,
		CreatedAt:    d.CreatedAt.UTC(),
	}
}

type userRepository struct {
	coll *mongo.Collection
}

// NewUserRepository returns an application.UserRepository backed by db.
func NewUserRepository(db *mongo.Database) application.UserRepository {
	return &userRepository{coll: db.Collection(UsersCollection)}
}

func parseID(id string) (bson.ObjectID, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return bson.ObjectID{}, domain.ErrInvalidUserID
	}
	return oid, nil
}

func (r *userRepository) CreateUser(ctx context.Context, u *domain.User) error {
	doc := userDocument{
		ID:        bson.NewObjectID(),
		Name:      u.Name,
		Email:     u.Email,
		Password:  u.PasswordHash,
		CreatedAt: u.CreatedAt,
	}
	if _, err := r.coll.InsertOne(ctx, doc); err != nil {
		// The unique index on email (migrations/000001) makes MongoDB reject
		// duplicates atomically, even when two sign-ups race.
		if mongo.IsDuplicateKeyError(err) {
			return domain.ErrEmailAlreadyExists
		}
		return fmt.Errorf("insert user: %w", err)
	}
	u.ID = doc.ID.Hex()
	return nil
}

func (r *userRepository) findOne(ctx context.Context, filter bson.D) (*domain.User, error) {
	var doc userDocument
	if err := r.coll.FindOne(ctx, filter).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("find user: %w", err)
	}
	u := doc.toDomain()
	return &u, nil
}

func (r *userRepository) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	oid, err := parseID(id)
	if err != nil {
		return nil, err
	}
	return r.findOne(ctx, bson.D{{Key: "_id", Value: oid}})
}

func (r *userRepository) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	return r.findOne(ctx, bson.D{{Key: "email", Value: email}})
}

func (r *userRepository) ListUsers(ctx context.Context) ([]domain.User, error) {
	cur, err := r.coll.Find(ctx, bson.D{}, options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("find users: %w", err)
	}
	var docs []userDocument
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode users: %w", err)
	}
	users := make([]domain.User, len(docs))
	for i, d := range docs {
		users[i] = d.toDomain()
	}
	return users, nil
}

func (r *userRepository) UpdateUser(ctx context.Context, u *domain.User) error {
	oid, err := parseID(u.ID)
	if err != nil {
		return err
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: oid}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "name", Value: u.Name},
			{Key: "email", Value: u.Email},
		}}},
	)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return domain.ErrEmailAlreadyExists
		}
		return fmt.Errorf("update user: %w", err)
	}
	if res.MatchedCount == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *userRepository) DeleteUserByID(ctx context.Context, id string) error {
	oid, err := parseID(id)
	if err != nil {
		return err
	}
	res, err := r.coll.DeleteOne(ctx, bson.D{{Key: "_id", Value: oid}})
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if res.DeletedCount == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *userRepository) CountUsers(ctx context.Context) (int64, error) {
	n, err := r.coll.CountDocuments(ctx, bson.D{})
	if err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return n, nil
}
