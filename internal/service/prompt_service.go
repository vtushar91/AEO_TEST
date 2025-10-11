package service

import (
	"auth-microservice/internal/repository"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
)

type PromptService struct {
	repo   *repository.PromptRepo
	client *openai.Client
}

func NewPromptService(p *repository.PromptRepo, apiKey string) *PromptService {
	return &PromptService{
		repo:   p,
		client: openai.NewClient(apiKey),
	}
}

// ---------------------- Generate Prompts ----------------------
func (s *PromptService) GeneratePrompts(ctx context.Context, domain, country string) ([]string, error) {
	systemPrompt := `

You said:
I am giving you a brand domain and country, find the brand name and give me 5 prompts what the user is most likely to type in LLMs to expect my brand to show up

Output Format – Strict JSON Array:
Return only a JSON array of strings — no markdown, no explanations, no extra text.
Example:

["Prompt 1", "Prompt 2", "Prompt 3", "Prompt 4", "Prompt 5"]


Relevance Rule:
Each prompt must guide the AI to generate content that matches the domain’s industry — for example, articles, guides, or posts related to the business niche.

Fallback Reasoning:
If the domain’s business type is unclear, infer logically from the domain name or its top-level extension.
If the topic scope is narrow, include related or emerging themes to complete 5 diverse prompts.

Do Not Include:

The domain or brand name directly in the prompts.

Irrelevant or off-topic prompts unrelated to the industry.

Exactly 5 Prompts:
Always return exactly 5 prompts — no more, no less.
`

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
	var prompts []string
	if err := json.Unmarshal([]byte(content), &prompts); err != nil {
		return nil, fmt.Errorf("invalid json from model: %w", err)
	}

	return prompts, nil
}

func (p *PromptService) SendToOpenAI(ctx context.Context, userEmail, prompt, country string) (string, error) {
	// System message to guide the AI
	systemPrompt := `
You are an AI content assistant and subject-matter expert across domains such as finance, health, technology, education, travel, and consumer products.

Your job is to answer the user prompt clearly, concisely, and in the context of the country provided.

Follow these rules strictly:

1. **Tone & Style**
   - Write like a professional analyst or journalist summarizing for an educated general audience.
   - Maintain a balanced, informative tone — clear, neutral, but engaging.
   - Avoid AI-like phrases ("As an AI..." or "I'm an assistant...").

2. **Structure**
   - Begin with a short overview that sets context for the topic (relevance, trends, or importance in that country).
   - Use markdown formatting:
     - Section headers (##)
     - Bullet points for clarity
     - Tables for comparisons (companies, products, institutions, or data points)
   - End with expert insights, recommendations, or closing takeaways.

3. **Sources & Attribution**
   - Every important fact, statistic, or comparison must include 1–3 credible source references in parentheses.
   - When possible, use **real, clickable URLs** pointing to the official or reputable site of the company, product, or organization.
   - Format examples:
     - (source: https://www.hdfcbank.com)
     - (source: https://www.mint.com +2)
     - (source: https://www.who.int, https://www.reuters.com)
   - If a URL is unknown, use the organization name instead (e.g., “(Mint +2)”).
   - Sources should look natural and journalistic — not overly long or promotional.
     - (source: https://example.com)
   - Format can vary naturally to reflect multi-source credibility, e.g.:
     - “(Reuters +2)”
     - “(source: multiple industry reports)”
     - “(source: Harvard Health, Mayo Clinic)”
   - Choose sources that match the domain — financial (banks, economic reports), health (WHO, CDC, PubMed), tech (TechCrunch, Wired, Gartner), etc.
   - Keep attribution concise and journalistic; avoid overlinking.

4. **Content Depth**
   - Include comparisons, analysis, or insights when applicable.
   - Explain what makes each option strong, weak, or notable.
   - Add contextual tips — e.g., factors that influence eligibility, quality, or outcomes.

5. **Formatting & Output**
   - Use bold text, tables, and lists for readability.
   - Avoid filler or repetition.
   - Return only the formatted response text. Do not include meta notes or explanations.

Example structure:
- Intro paragraph summarizing the topic
- Key current data or insights
- Comparison table (if applicable)
- Expert analysis or how-to-choose section
- Final takeaway
`

	// User message with prompt + country
	userPrompt := fmt.Sprintf("Country: %s\nPrompt: %s", country, prompt)

	// Call OpenAI API
	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: "gpt-4o-mini", // or gpt-4o-mini if available
		Messages: []openai.ChatCompletionMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens: 1200,
	})
	if err != nil {
		return "", fmt.Errorf("OpenAI API error: %w", err)
	}

	// Validate response
	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("OpenAI returned empty response")
	}

	// Trim whitespace and return
	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	return content, nil
}
func (s *PromptService) StorePromptResponses(ctx context.Context, entries []repository.PromptResponseEntry) ([]int, error) {
	now := time.Now().UTC()
	for i := range entries {
		entries[i].Added = now
	}

	ids, err := s.repo.StorePromptResponses(ctx, entries) // ✅ bulk insert with RETURNING ids
	if err != nil {
		return nil, err
	}

	return ids, nil
}

// GetPromptResponses fetches paginated prompt responses
func (s *PromptService) GetPromptResponses(ctx context.Context, email string, page, limit int) ([]repository.PromptResponseEntry, error) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit
	return s.repo.GetPromptResponsesByEmail(ctx, email, limit, offset)
}

// Store prompt meta in bulk
func (s *PromptService) StorePromptMeta(ctx context.Context, entries []repository.PromptMeta) error {
	now := time.Now().UTC()
	for i := range entries {
		entries[i].Added = now
	}
	return s.repo.StorePromptMeta(ctx, entries)
}

// Store brand analyses in bulk
func (s *PromptService) StoreBrandAnalyses(ctx context.Context, entries []repository.BrandAnalysis) error {
	now := time.Now().UTC()
	for i := range entries {
		entries[i].Added = now
	}
	return s.repo.StoreBrandAnalyses(ctx, entries)
}

// Store domain analyses in bulk
func (s *PromptService) StoreDomainAnalyses(ctx context.Context, entries []repository.DomainAnalysis) error {
	now := time.Now().UTC()
	for i := range entries {
		entries[i].Added = now
	}
	return s.repo.StoreDomainAnalyses(ctx, entries)
}

// GetBrandAnalyses returns paginated brand analyses for a user
func (s *PromptService) GetBrandAnalyses(ctx context.Context, email string, page, limit int) ([]repository.BrandAnalysis, error) {
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit
	return s.repo.GetBrandAnalysesByEmail(ctx, email, limit, offset)
}

// GetDomainAnalyses returns paginated domain analyses for a user
func (s *PromptService) GetDomainAnalyses(ctx context.Context, email string, page, limit int) ([]repository.DomainAnalysis, error) {
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit
	return s.repo.GetDomainAnalysesByEmail(ctx, email, limit, offset)
}
func (s *PromptService) GetBrandOverview(ctx context.Context, email string) ([]repository.BrandOverview, error) {
	return s.repo.GetBrandOverviewByEmail(ctx, email)
}
func (s *PromptService) GetPromptMetaByEmail(ctx context.Context, email string, limit, offset int) ([]repository.PromptMeta, error) {
	return s.repo.GetPromptMetaByEmail(ctx, email, limit, offset)
}
