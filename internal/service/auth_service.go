package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"auth-microservice/internal/auth"
	"auth-microservice/internal/config"
	"auth-microservice/internal/repository"
)

type AuthService struct {
	users  *repository.UserRepo
	tokens *repository.TokenRepo
	cfg    *config.Config
}

func NewAuthService(u *repository.UserRepo, t *repository.TokenRepo, cfg *config.Config) *AuthService {
	return &AuthService{users: u, tokens: t, cfg: cfg}
}

// SendEmailVerification generates a magic link and sends email
func (s *AuthService) SendEmailVerification(ctx context.Context, email, baseURL string) (string, error) {
	// generate random token
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)

	// save token record in DB
	rec := &repository.TokenRecord{
		Token:     token,
		Email:     email,
		Purpose:   "verify_email",
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}
	if err := s.tokens.Create(ctx, rec); err != nil {
		return "", err
	}

	// construct magic link
	verifyURL := fmt.Sprintf("%s/verify?token=%s", baseURL, token)

	// send email if configured
	if s.cfg.Email != "" && s.cfg.EmailKey != "" {
		err := auth.SendVerificationEmail(s.cfg.Email, s.cfg.EmailKey, email, verifyURL)
		if err != nil {
			// log the error and return it
			fmt.Println("SendGrid email error:", err)
			return "", fmt.Errorf("failed to send verification email: %w", err)
		}
	}

	return verifyURL, nil
}

// checks token
func (s *AuthService) VerifyEmailToken(ctx context.Context, token string) (*repository.TokenRecord, error) {
	rec, err := s.tokens.FindValid(ctx, token, "verify_email")
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, errors.New("invalid or expired token")
	}
	_ = s.DeleteToken(ctx, token)
	return rec, nil

}

func (s *AuthService) GetUserByEmail(ctx context.Context, email string) (*repository.User, error) {
	return s.users.FindByEmail(ctx, email)
}
func (s *AuthService) SignupUser(ctx context.Context, email string) (*repository.User, error) {
	// Create a new verified user
	user, err := s.users.CreateUser(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// GenerateAccessToken creates JWT for the given email
func (s *AuthService) GenerateAccessToken(email, userID string) (string, error) {
	return auth.GenerateAccessToken(s.cfg.AccessSecret, email, userID, 24*time.Hour)
}
func (s *AuthService) DeleteToken(ctx context.Context, token string) error {
	// Optional: add any business logic here, e.g., logging
	if token == "" {
		return errors.New("token cannot be empty")
	}

	if err := s.tokens.Delete(ctx, token); err != nil {
		return fmt.Errorf("failed to delete token: %w", err)
	}

	return nil
}
func (s *AuthService) SignupOAuthUser(ctx context.Context, email, provider, providerID string) (*repository.User, error) {
	user, err := s.users.UpsertOAuthUser(ctx, email, provider, providerID)
	if err != nil {
		return nil, fmt.Errorf("failed to signup OAuth user: %w", err)
	}

	return user, nil
}
