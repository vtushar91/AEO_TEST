package config

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var mongoClient *mongo.Client

// NewMongoClient initializes and pings the MongoDB client.
func NewMongoClient(ctx context.Context, cfg *Config) (*mongo.Client, error) {
	clientOpts := options.Client().ApplyURI(cfg.MongoURI)

	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Ping the MongoDB server to verify connection
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	mongoClient = client
	fmt.Println("✅ Connected to MongoDB successfully")
	return client, nil
}

// EnsureIndexes creates necessary indexes for your collections.
func EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	userCol := db.Collection("users")

	userIndexes := []mongo.IndexModel{
		{
			Keys:    bson.M{"email": 1},              // ascending index on email
			Options: options.Index().SetUnique(true), // enforce unique email
		},
	}

	if _, err := userCol.Indexes().CreateMany(ctx, userIndexes); err != nil {
		return fmt.Errorf("failed to create user indexes: %w", err)
	}

	fmt.Println("✅ MongoDB indexes ensured successfully")
	return nil
}

// GetMongoClient returns the active MongoDB client
func GetMongoClient() *mongo.Client {
	if mongoClient == nil {
		panic("❌ Mongo client is not initialized. Call NewMongoClient() first.")
	}
	return mongoClient
}

// CloseMongo closes the MongoDB connection safely.
func CloseMongo(ctx context.Context) error {
	if mongoClient != nil {
		if err := mongoClient.Disconnect(ctx); err != nil {
			return fmt.Errorf("failed to disconnect MongoDB: %w", err)
		}
		fmt.Println("🧹 MongoDB connection closed")
	}
	return nil
}
