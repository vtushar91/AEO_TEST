package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type TokenRecord struct {
	Token     string    `bson:"token"`
	Email     string    `bson:"email"`
	Purpose   string    `bson:"purpose"` // e.g. "verify_email"
	ExpiresAt time.Time `bson:"expires_at"`
	CreatedAt time.Time `bson:"created_at"`
}

type TokenRepo struct {
	col *mongo.Collection
}

func NewTokenRepo(db *mongo.Database, colName string) *TokenRepo {
	return &TokenRepo{col: db.Collection(colName)}
}

func (r *TokenRepo) Create(ctx context.Context, t *TokenRecord) error {
	t.CreatedAt = time.Now().UTC()
	_, err := r.col.InsertOne(ctx, t)
	return err
}

func (r *TokenRepo) FindValid(ctx context.Context, token, purpose string) (*TokenRecord, error) {
	var rec TokenRecord
	err := r.col.FindOne(ctx, bson.M{"token": token, "purpose": purpose, "expires_at": bson.M{"$gt": time.Now().UTC()}}).Decode(&rec)
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

func (r *TokenRepo) Delete(ctx context.Context, token string) error {
	_, err := r.col.DeleteOne(ctx, bson.M{"token": token})
	return err
}
