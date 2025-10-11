package config

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var dbPool *pgxpool.Pool

// ConnectToPostgres initializes the PostgreSQL connection pool
func ConnectToPostgres(cfg *Config) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var err error
	dbPool, err = pgxpool.New(ctx, cfg.PostgresURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}

	if err = dbPool.Ping(ctx); err != nil {
		log.Fatalf("Unable to ping database: %v", err)
	}

	log.Println("✅ Connected to PostgreSQL database successfully")
}

// GetDB returns the global PostgreSQL pool
func GetDB() *pgxpool.Pool {
	if dbPool == nil {
		log.Fatal("❌ Database connection pool is not initialized")
	}
	return dbPool
}

// ClosePostgres safely closes the PostgreSQL pool
func ClosePostgres() {
	if dbPool != nil {
		dbPool.Close()
		log.Println("🧹 PostgreSQL connection closed")
	}
}
