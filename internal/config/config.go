package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	// MongoDB
	MongoURI  string
	DBName    string
	UserCol   string
	TokenCol  string
	PromptCol string
	//PostgreSQL
	PostgresURL string
	// Server
	Port string

	// Email
	Email       string
	EmailKey    string
	EmailSecret string

	// JWT / Auth
	AccessSecret string

	// OAuth (optional)
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	FrontendURL        string

	// Other optional keys
	OpenApiKey string
}

// Load reads environment variables and validates required ones.
func Load() (*Config, error) {
	// Load .env file if it exists (optional)
	_ = godotenv.Load() // ignore error if no file; env vars can come from the system

	var missing []string

	getRequired := func(key string) string {
		val := os.Getenv(key)
		if val == "" {
			missing = append(missing, key)
		}
		return val
	}

	getOptional := func(key string) string {
		return os.Getenv(key)
	}

	cfg := &Config{
		// Required
		MongoURI:     getRequired("MONGO_URI"),
		DBName:       getRequired("DB_NAME"),
		UserCol:      getRequired("USER_COL"),
		TokenCol:     getRequired("TOKEN_COL"),
		PromptCol:    getRequired("PROMPT_COL"),
		PostgresURL:  getRequired("POSTGRES_URL"),
		Port:         getRequired("PORT"),
		Email:        getRequired("EMAIL"),
		EmailKey:     getRequired("EMAIL_KEY"),
		AccessSecret: getRequired("ACCESS_SECRET"),
		EmailSecret:  getRequired("EMAIL_SECRET"),
		OpenApiKey:   getRequired("OPENAI_API_KEY"),

		// Optional
		GoogleClientID:     getOptional("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: getOptional("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  getOptional("GOOGLE_REDIRECT_URL"),
		FrontendURL:        getOptional("FrontendURL"),
	}

	if len(missing) > 0 {
		return nil, errors.New("missing required environment variables: " + fmt.Sprint(missing))
	}

	// Set a default for GoogleRedirectURL if Google OAuth is partially configured
	if cfg.GoogleRedirectURL == "" && cfg.GoogleClientID != "" && cfg.GoogleClientSecret != "" {
		cfg.GoogleRedirectURL = "http://localhost:" + cfg.Port + "/auth/google/callback"
	}

	return cfg, nil
}
