package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"auth-microservice/internal/repository"

	"github.com/sashabaranov/go-openai"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type UserService struct {
	users  *repository.UserRepo
	client *openai.Client
}

// Constructor
func NewUserService(users *repository.UserRepo, apiKey string) *UserService {
	return &UserService{
		users:  users,
		client: openai.NewClient(apiKey),
	}
}

// AddCompetitor adds a competitor to a user's record
func (s *UserService) AddCompetitor(ctx context.Context, email string, competitor []repository.Competitor) error {
	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return err
	}
	if user == nil {
		return errors.New("user does not exist")
	}
	return s.users.AddCompetitor(ctx, email, competitor)
}

// GetCompetitor returns a paginated list of competitors for a user
func (s *UserService) GetCompetitor(ctx context.Context, email string, page, limit int) ([]repository.Competitor, int, error) {
	// check if user exists
	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return nil, 0, err
	}
	if user == nil {
		return nil, 0, errors.New("user does not exist")
	}

	// fetch competitors with pagination
	competitors, total, err := s.users.GetCompetitor(ctx, email, page, limit)
	if err != nil {
		return nil, 0, err
	}

	return competitors, total, nil
}

func (s *UserService) UpdateUserProfile(ctx context.Context, email, brandName, domain, country string) error {
	profile := &repository.User{
		BrandName: brandName,
		Domain:    domain,
		Country:   country,
	}

	// Call repo to update DB
	if err := s.users.UpdateProfile(ctx, email, profile); err != nil {
		return err
	}
	return nil
}

type UserDomainCountry struct {
	ID      primitive.ObjectID
	Domain  string
	Country string
}

func (s *UserService) GetUserByEmail(ctx context.Context, email string) (*repository.User, error) {
	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}
	return user, nil
}
func (s *UserService) GenerateCompetitor(ctx context.Context, domain, country string) ([]string, error) {
	systemPrompt := `
You are an expert in market intelligence, brand research, and competitive analysis.
Your task is to generate exactly 5 competitor brand names based on the website domain and country provided by the user. Follow these rules strictly:

Domain-Focused: Analyze the website domain to understand what industry, product, or service it represents.
Example: swiggy.com → online food delivery platform.

Country-Specific: Only list competitors that operate or are popular in the given country.
Example: If the country is India, only show competitors active or relevant in India.

Output Format – Strict JSON Array:
Return only a JSON array of strings — no markdown, no explanations, no punctuation outside JSON.
Example:

["Competitor 1", "Competitor 2", "Competitor 3", "Competitor 4", "Competitor 5"]


Relevance Rule: Each competitor must offer similar products, services, or target audience as the given domain.

Fallback Reasoning:

If the domain’s business type is unclear, infer logically from the domain name or extension.

If there are fewer than 5 direct competitors, include indirect or emerging ones to complete the list.

Do Not Include:

The provided domain itself.

Irrelevant or international-only competitors not present in the target country.

Exactly 5 Names: Always return exactly 5 — no more, no less.`

	userPrompt := "Domain: " + domain + "\nCountry: " + country

	resp, err := s.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: "gpt-4o-mini",
		Messages: []openai.ChatCompletionMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens: 200,
	})
	if err != nil {
		return nil, fmt.Errorf("openai error: %w", err)
	}

	content := resp.Choices[0].Message.Content
	var Competitor []string
	if err := json.Unmarshal([]byte(content), &Competitor); err != nil {
		return nil, fmt.Errorf("invalid json from model: %w", err)
	}

	return Competitor, nil
}
