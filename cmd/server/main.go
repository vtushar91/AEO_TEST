package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"auth-microservice/internal/config"
	"auth-microservice/internal/handler"
	"auth-microservice/internal/middleware"
	"auth-microservice/internal/pkg"
	"auth-microservice/internal/repository"
	"auth-microservice/internal/service"

	"github.com/cdipaolo/sentiment"
)

func main() {
	// load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// connect mongo
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := config.NewMongoClient(ctx, cfg)
	if err != nil {
		log.Fatalf("mongo connect error: %v", err)
	}
	db := client.Database(cfg.DBName)
	//Create Index for Email
	if err := config.EnsureIndexes(ctx, db); err != nil {
		log.Fatal(err)
	}
	//PgSql Initialized
	config.ConnectToPostgres(cfg)
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := config.GetDB().Ping(ctx); err != nil {
		log.Fatalf("Postgres not ready: %v", err)
	}
	log.Println("✅ Postgres ready")
	//Analysis Model init
	model, err := sentiment.Restore()
	if err != nil {
		log.Fatalf("failed to load sentiment model: %v", err)
	}
	pkg.SetSentimentModel(&model)

	// repositories
	userRepo := repository.NewUserRepo(db, cfg.UserCol)
	tokenRepo := repository.NewTokenRepo(db, cfg.TokenCol)
	promptRepo := repository.NewPromptRepo(config.GetDB())

	// services
	authSvc := service.NewAuthService(userRepo, tokenRepo, cfg)
	userSvc := service.NewUserService(userRepo, cfg.OpenApiKey)
	promptSvc := service.NewPromptService(promptRepo, cfg.OpenApiKey)

	// handlers
	h := handler.NewHandler(authSvc, userSvc, cfg, promptSvc)

	// routes
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// ✅ Wrap mux with CORS middleware
	corsMux := middleware.CORS(mux)

	addr := "0.0.0.0:" + cfg.Port
	srv := &http.Server{
		Addr:    addr,
		Handler: corsMux, // 👈 use corsMux here
	}

	log.Printf("listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
