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
	"auth-microservice/internal/service" //http://10.63.118.197:7070/send-verify

	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/bson/primitive"
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

	// Authenticated routes (requires JWT)
	mux.Handle("/me", middleware.JWTAuth(h.cfg.AccessSecret, http.HandlerFunc(h.Me)))
	//Onbaoridng
	mux.Handle("/user/brand",
		middleware.JWTAuth(h.cfg.AccessSecret, http.HandlerFunc(h.AddBrandDetails)))
	mux.Handle("/competitor/generate",
		middleware.JWTAuth(h.cfg.AccessSecret, http.HandlerFunc(h.GetCompetitorSuggestions)))
	mux.Handle("/user/competitor",
		middleware.JWTAuth(h.cfg.AccessSecret, http.HandlerFunc(h.AddCompetitor)))
	mux.Handle("/prompts/generate",
		middleware.JWTAuth(h.cfg.AccessSecret, http.HandlerFunc(h.GetPromptSuggestions))) // generate prompts
	mux.Handle("/prompts/analysis",
		middleware.JWTAuth(h.cfg.AccessSecret, http.HandlerFunc(h.HandlePromptsEntry))) // store prompt
	// User competitor management
	mux.Handle("/user/getcompetitor",
		middleware.JWTAuth(h.cfg.AccessSecret, http.HandlerFunc(h.GetCompetitor)))
	//prompts page
	mux.Handle("/prompt/meta/get",
		middleware.JWTAuth(h.cfg.AccessSecret, http.HandlerFunc(h.GetPromptMeta))) // get promptmeta // get promptResponse
	//Overview
	mux.Handle("/analyse/brand/get",
		middleware.JWTAuth(h.cfg.AccessSecret, http.HandlerFunc(h.GetBrandOverview))) // get brands
	mux.Handle("/analyse/domain/get",
		middleware.JWTAuth(h.cfg.AccessSecret, http.HandlerFunc(h.GetDomainAnalysis))) // get domain
	mux.Handle("/prompts/get",
		middleware.JWTAuth(h.cfg.AccessSecret, http.HandlerFunc(h.GetPromptResponses))) // get promptResponse
}

type UserProfile struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"` // MongoDB ObjectID
	Email      string             `bson:"email" json:"email"`      // required at creation
	IsVerified bool               `bson:"is_verified" json:"-"`
	BrandName  string             `bson:"brand_name,omitempty" json:"brand_name,omitempty"` // optional
	Country    string             `bson:"country,omitempty" json:"country,omitempty"`       // optional
	Competitor []string           `bson:"competitor,omitempty" json:"competitor,omitempty"` // optional
}

// --- handlers ---

func (h *Handler) SendVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Email   string `json:"email" validate:"required,email"`
		BaseURL string `json:"base_url"`
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
	email, ok := pkg.GetEmailFromContext(r.Context())
	if !ok {
		http.Error(w, "email not found in context", http.StatusUnauthorized)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"email": email})
}
