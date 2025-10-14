package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"auth-microservice/internal/auth"
	"auth-microservice/internal/config"
	"auth-microservice/internal/middleware"
	"auth-microservice/internal/pkg"
	"auth-microservice/internal/service"

	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type Handler struct {
	svc      *service.AuthService
	usvc     *service.UserService
	p        *service.PromptService
	validate *validator.Validate
	cfg      *config.Config
}

func NewHandler(svc *service.AuthService, usvc *service.UserService, cfg *config.Config, p *service.PromptService) *Handler {
	validate := validator.New()
	return &Handler{
		svc:      svc,
		p:        p,
		usvc:     usvc,
		validate: validate,
		cfg:      cfg,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Public routes
	mux.HandleFunc("/send-verify", h.SendVerify) // accepts email, base_url optional
	mux.HandleFunc("/verify", h.Verify)          // GET ?token=...
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	//oAuth Routes
	mux.HandleFunc("/oauth/google", h.GoogleOAuthRedirect)
	mux.HandleFunc("/oauth/google/callback", h.GoogleOAuthCallback)
	// Authenticated routes (requires JWT)
	mux.Handle("/me", middleware.JWTAuth(h.cfg.AccessSecret, http.HandlerFunc(h.Me)))
	//Onbaoridng
	mux.Handle("/user/brand",
		middleware.JWTAuth(h.cfg.AccessSecret, http.HandlerFunc(h.AddBrandDetails))) //Add Brand details
	mux.Handle("/competitor/generate",
		middleware.JWTAuth(h.cfg.AccessSecret, http.HandlerFunc(h.GetCompetitorSuggestions))) //generate competitor sugg
	mux.Handle("/prompts/generate",
		middleware.JWTAuth(h.cfg.AccessSecret, http.HandlerFunc(h.GetPromptSuggestions))) // generate prompts sugg
	mux.Handle("/prompts/analysis",
		middleware.JWTAuth(h.cfg.AccessSecret, http.HandlerFunc(h.HandlePromptsEntry))) // store prompt & analyse them
	// Competitor page
	mux.Handle("/user/getcompetitor",
		middleware.JWTAuth(h.cfg.AccessSecret, http.HandlerFunc(h.GetCompetitor))) //get competitor
	mux.Handle("/user/competitor",
		middleware.JWTAuth(h.cfg.AccessSecret, http.HandlerFunc(h.AddCompetitor))) //Add competitor
	//prompts page
	mux.Handle("/prompt/meta/get",
		middleware.JWTAuth(h.cfg.AccessSecret, http.HandlerFunc(h.GetPromptMeta))) // get promptmeta
	mux.Handle("/analyse/brand/prompt/get",
		middleware.JWTAuth(h.cfg.AccessSecret, http.HandlerFunc(h.GetBrandOverviewByPrompt))) //get brand per prompt
	mux.Handle("/analyse/domain/prompt/get",
		middleware.JWTAuth(h.cfg.AccessSecret, http.HandlerFunc(h.GetDomainOverviewByPrompt))) //get domain per prompt
	//TODO:Add Prompt Route
	//Overview
	mux.Handle("/analyse/brand/get",
		middleware.JWTAuth(h.cfg.AccessSecret, http.HandlerFunc(h.GetBrandOverview))) // get brands
	mux.Handle("/analyse/domain/get",
		middleware.JWTAuth(h.cfg.AccessSecret, http.HandlerFunc(h.GetDomainAnalysis))) // get domain //TODO:Unique Domain might be
	mux.Handle("/prompts/get",
		middleware.JWTAuth(h.cfg.AccessSecret, http.HandlerFunc(h.GetPromptResponses))) // get promptResponse
}

type UserProfile struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Email      string             `bson:"email" json:"email"`
	IsVerified bool               `bson:"is_verified" json:"-"`
	BrandName  string             `bson:"brand_name,omitempty" json:"brand_name,omitempty"`
	Country    string             `bson:"country,omitempty" json:"country,omitempty"`
	Competitor []string           `bson:"competitor,omitempty" json:"competitor,omitempty"`

	// OAuth Fields
	Provider    string    `bson:"provider,omitempty" json:"provider,omitempty"`       // "google", "github", or empty for email flow
	ProviderID  string    `bson:"provider_id,omitempty" json:"provider_id,omitempty"` // unique ID from provider
	CreatedAt   time.Time `bson:"created_at,omitempty" json:"created_at,omitempty"`
	LastLoginAt time.Time `bson:"last_login_at,omitempty" json:"last_login_at,omitempty"`
}

// --- handlers ---

func (h *Handler) SendVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Email   string `json:"email" validate:"required,email"`
		BaseURL string `json:"baseURL" validate:"required"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := h.validate.Struct(&body); err != nil {
		http.Error(w, "validation: "+err.Error(), http.StatusBadRequest)
		return
	}
	base := body.BaseURL
	if base == "" {
		base = fmt.Sprintf("http://%s", r.Host)
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	verifyURL, err := h.svc.SendEmailVerification(ctx, body.Email, base)
	if err != nil {
		http.Error(w, "error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"verify_url": verifyURL})
}

func (h *Handler) Verify(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		http.Error(w, "use GET", http.StatusMethodNotAllowed)
		return
	}
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, `{"error": "missing token"}`, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	rec, err := h.svc.VerifyEmailToken(ctx, token)
	if err != nil {
		http.Error(w, `{"error": "invalid or expired token"}`, http.StatusBadRequest)
		return
	}

	user, err := h.svc.GetUserByEmail(ctx, rec.Email)
	if err != nil {
		http.Error(w, `{"error": "error fetching user"}`, http.StatusInternalServerError)
		return
	}

	if user == nil {
		// New user → signup
		user, err = h.svc.SignupUser(ctx, rec.Email)
		if err != nil {
			http.Error(w, `{"error": "failed to signup user"}`, http.StatusInternalServerError)
			return
		}
	}

	accessToken, err := auth.GenerateAccessToken(h.cfg.AccessSecret, user.Email, user.ID.Hex(), 24*time.Hour)
	if err != nil {
		http.Error(w, `{"error": "failed to generate access token"}`, http.StatusInternalServerError)
		return
	}

	// Determine action based on whether the user has a brand
	action := "signup"
	if user.BrandName != "" { // single brand string
		action = "login"
	}

	json.NewEncoder(w).Encode(map[string]string{
		"email":        rec.Email,
		"access_token": accessToken,
		"action":       action,
		"message":      "Welcome to AEORANK",
	})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "use GET", http.StatusMethodNotAllowed)
		return
	}

	// Extract email from context (set by JWT middleware)
	email, ok := pkg.GetEmailFromContext(r.Context())
	if !ok {
		http.Error(w, "email not found in context", http.StatusUnauthorized)
		return
	}

	// Fetch full user details
	user, err := h.svc.GetUserByEmail(r.Context(), email)
	if err != nil {
		http.Error(w, "failed to fetch user: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if user == nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(user)
}

// oAuth Routes
func (h *Handler) GoogleOAuthRedirect(w http.ResponseWriter, r *http.Request) {
	conf := &oauth2.Config{
		ClientID:     h.cfg.GoogleClientID,
		ClientSecret: h.cfg.GoogleClientSecret,
		RedirectURL:  h.cfg.GoogleRedirectURL,
		Scopes:       []string{"email"},
		Endpoint:     google.Endpoint,
	}

	url := conf.AuthCodeURL("random-state", oauth2.AccessTypeOffline)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}
func (h *Handler) GoogleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	conf := &oauth2.Config{
		ClientID:     h.cfg.GoogleClientID,
		ClientSecret: h.cfg.GoogleClientSecret,
		RedirectURL:  h.cfg.GoogleRedirectURL,
		Scopes:       []string{"email"},
		Endpoint:     google.Endpoint,
	}

	// Exchange code for token
	token, err := conf.Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, "code exchange failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch user info from Google
	client := conf.Client(context.Background(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		http.Error(w, "fetch user info failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var gUser struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&gUser); err != nil {
		http.Error(w, "decode user info failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Check if user exists or create new
	ctx := r.Context()
	user, err := h.svc.GetUserByEmail(ctx, gUser.Email)
	if err != nil {
		http.Error(w, "error fetching user", http.StatusInternalServerError)
		return
	}
	if user == nil {
		user, err = h.svc.SignupOAuthUser(ctx, gUser.Email, "google", gUser.ID)
		if err != nil {
			http.Error(w, "failed to signup oauth user", http.StatusInternalServerError)
			return
		}
	}

	// Generate AEORANK JWT
	accessToken, err := auth.GenerateAccessToken(h.cfg.AccessSecret, user.Email, user.ID.Hex(), 24*time.Hour)
	if err != nil {
		http.Error(w, "token gen failed", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"email":        user.Email,
		"access_token": accessToken,
		"action":       "oauth_login",
		"message":      "Welcome via Google OAuth!",
	})
}
