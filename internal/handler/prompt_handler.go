package handler

import (
	"auth-microservice/internal/pkg"
	"auth-microservice/internal/repository"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type SuggestPrompt struct {
	BrandName string `json:"brand_name"`
	Domain    string `json:"domain"`
	Country   string `json:"country"`
}

func (h *Handler) GetPromptSuggestions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}

	//Get user email from context
	email, ok := pkg.GetEmailFromContext(r.Context())
	if !ok || email == "" {
		http.Error(w, "unauthorized: missing email", http.StatusUnauthorized)
		return
	}

	//Get saved domain & country from user service
	userData, err := h.usvc.GetUserByEmail(r.Context(), email)
	if err != nil {
		http.Error(w, "failed to get user data: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if userData.Domain == "" || userData.Country == "" {
		http.Error(w, "domain and country not set for this user", http.StatusBadRequest)
		return
	}

	// 3. Generate prompts
	prompts, err := h.p.GeneratePrompts(r.Context(), userData.Domain, userData.Country)
	if err != nil {
		http.Error(w, "failed to generate prompts: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 5. Prepare response with country for each prompt
	type PromptWithCountry struct {
		Prompt  string `json:"prompt"`
		Country string `json:"country"`
	}

	respPrompts := make([]PromptWithCountry, len(prompts))
	for i, p := range prompts {
		respPrompts[i] = PromptWithCountry{
			Prompt:  p,
			Country: userData.Country,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(respPrompts)
}

type PromptRequest struct {
	Prompts []struct {
		Prompt  string `json:"prompt" validate:"required"`
		Country string `json:"country" validate:"required"`
	} `json:"prompts" validate:"required,dive"`
}

func (h *Handler) HandlePromptsEntry(w http.ResponseWriter, r *http.Request) {
	// 1️⃣ Enforce POST method
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}

	// 2️⃣ Get user email from context
	email, ok := pkg.GetEmailFromContext(r.Context())
	if !ok || email == "" {
		http.Error(w, "unauthorized: missing email", http.StatusUnauthorized)
		return
	}

	// 3️⃣ Parse request body
	var req PromptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// 4️⃣ Validate struct
	if err := h.validate.Struct(&req); err != nil {
		http.Error(w, "validation error: "+err.Error(), http.StatusBadRequest)
		return
	}

	// 5️⃣ Context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	// 6️⃣ Fetch brand & competitors from MongoDB
	userData, err := h.usvc.GetUserByEmail(ctx, email)
	if err != nil {
		http.Error(w, "failed to fetch user data: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if userData == nil || userData.BrandName == "" {
		http.Error(w, "brand not configured for this user", http.StatusBadRequest)
		return
	}

	// 1️⃣ Collect results from OpenAI
	var results []pkg.PromptResponse
	for _, p := range req.Prompts {
		respText, err := h.p.SendToOpenAI(ctx, email, p.Prompt, p.Country)
		if err != nil {
			http.Error(w, "OpenAI API error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		results = append(results, pkg.PromptResponse{Prompt: p.Prompt, Response: respText})
	}

	// 2️⃣ Store prompt responses as before and get IDs
	var responseEntries []repository.PromptResponseEntry
	for _, r := range results {
		responseEntries = append(responseEntries, repository.PromptResponseEntry{
			UserEmail: email,
			Prompt:    r.Prompt,
			Response:  r.Response,
			Country:   req.Prompts[0].Country,
			Added:     time.Now().UTC(),
		})
	}

	promptIDs, err := h.p.StorePromptResponses(ctx, responseEntries)
	if err != nil {
		http.Error(w, "failed to store prompt responses: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 3️⃣ Generate brand aliases and analyze responses
	brandAliases := pkg.GenerateAliases(userData.BrandName)
	competitorMap := make(map[string][]string)
	for _, c := range userData.Competitor {
		competitorMap[c.TrackedName] = pkg.GenerateAliases(c.TrackedName)
	}
	analysisResults := pkg.AnalyzeResponses(results, req.Prompts[0].Country, userData.BrandName, brandAliases, competitorMap)

	// 4️⃣ Store analyses split across tables using promptIDs
	var (
		promptEntries []repository.PromptMeta
		brandEntries  []repository.BrandAnalysis
		domainEntries []repository.DomainAnalysis
	)

	for i, a := range analysisResults {
		promptID := promptIDs[i] // use ID from stored prompt response

		// ✅ Prompt table (meta-level info)
		promptEntries = append(promptEntries, repository.PromptMeta{
			PromptID:  promptID,
			UserEmail: email,
			Prompt:    a.Prompt,
			Mentions:  a.Mentions,
			Volume:    a.Volume,
			Tags:      a.Tags,
			Location:  a.Location,
			Added:     time.Now().UTC(),
		})

		// ✅ Brand table
		for _, b := range a.Brands {
			brandEntries = append(brandEntries, repository.BrandAnalysis{
				PromptID:   promptID,
				UserEmail:  email,
				BrandName:  b.BrandName,
				Visibility: b.Visibility,
				Sentiment:  b.Sentiment,
				Position:   b.Position,
				Added:      time.Now().UTC(),
			})
		}

		// ✅ Domain table
		for _, d := range a.Domains {
			domainEntries = append(domainEntries, repository.DomainAnalysis{
				PromptID:     promptID,
				Domain:       d.Domain,
				Used:         d.Used,
				AvgCitations: d.AvgCitations,
				Type:         d.Type,
			})
		}
	}

	// 5️⃣ Store in bulk
	if err := h.p.StorePromptMeta(ctx, promptEntries); err != nil {
		http.Error(w, "failed to store prompt metadata: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := h.p.StoreBrandAnalyses(ctx, brandEntries); err != nil {
		http.Error(w, "failed to store brand analyses: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := h.p.StoreDomainAnalyses(ctx, domainEntries); err != nil {
		http.Error(w, "failed to store domain analyses: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"message":"prompts processed and analyzed successfully"}`)
}
func (h *Handler) GetPromptResponses(w http.ResponseWriter, r *http.Request) {
	// 1️⃣ Enforce GET method
	if r.Method != http.MethodGet {
		http.Error(w, "use GET", http.StatusMethodNotAllowed)
		return
	}

	// 2️⃣ Get user email from context
	email, ok := pkg.GetEmailFromContext(r.Context())
	if !ok || email == "" {
		http.Error(w, "unauthorized: missing email", http.StatusUnauthorized)
		return
	}

	// 3️⃣ Parse query params for pagination
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 10
	}

	// 4️⃣ Call service
	prompts, err := h.p.GetPromptResponses(r.Context(), email, page, limit)
	if err != nil {
		http.Error(w, "failed to get prompt responses: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 5️⃣ Return JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(prompts); err != nil {
		http.Error(w, "failed to encode response: "+err.Error(), http.StatusInternalServerError)
		return
	}
}
func (h *Handler) GetBrandAnalysis(w http.ResponseWriter, r *http.Request) {
	// 1️⃣ Allow only GET
	if r.Method != http.MethodGet {
		http.Error(w, "use GET", http.StatusMethodNotAllowed)
		return
	}

	// 2️⃣ Extract user email from context
	email, ok := pkg.GetEmailFromContext(r.Context())
	if !ok || email == "" {
		http.Error(w, "unauthorized: missing email", http.StatusUnauthorized)
		return
	}

	// 3️⃣ Parse pagination query params
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		page = 1
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 10
	}

	// 4️⃣ Fetch brand analyses
	analyses, err := h.p.GetBrandAnalyses(r.Context(), email, page, limit)
	if err != nil {
		http.Error(w, "failed to get brand analyses: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 5️⃣ Return JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(analyses); err != nil {
		http.Error(w, "failed to encode response: "+err.Error(), http.StatusInternalServerError)
	}
}
func (h *Handler) GetDomainAnalysis(w http.ResponseWriter, r *http.Request) {
	// 1️⃣ Enforce GET method
	if r.Method != http.MethodGet {
		http.Error(w, "use GET", http.StatusMethodNotAllowed)
		return
	}

	// 2️⃣ Extract user email from context
	email, ok := pkg.GetEmailFromContext(r.Context())
	if !ok || email == "" {
		http.Error(w, "unauthorized: missing email", http.StatusUnauthorized)
		return
	}

	// 3️⃣ Parse query params for pagination
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		page = 1
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 10
	}

	// 4️⃣ Fetch domain analyses
	analyses, err := h.p.GetDomainAnalyses(r.Context(), email, page, limit)
	if err != nil {
		http.Error(w, "failed to get domain analyses: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 5️⃣ Return JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(analyses); err != nil {
		http.Error(w, "failed to encode response: "+err.Error(), http.StatusInternalServerError)
	}
}
func (h *Handler) GetBrandOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "use GET", http.StatusMethodNotAllowed)
		return
	}

	email, ok := pkg.GetEmailFromContext(r.Context())
	if !ok || email == "" {
		http.Error(w, "unauthorized: missing email", http.StatusUnauthorized)
		return
	}

	overview, err := h.p.GetBrandOverview(r.Context(), email)
	if err != nil {
		http.Error(w, "failed to get brand overview: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(overview); err != nil {
		http.Error(w, "failed to encode response: "+err.Error(), http.StatusInternalServerError)
	}
}
func (h *Handler) GetPromptMeta(w http.ResponseWriter, r *http.Request) {
	// 1️⃣ Allow only GET
	if r.Method != http.MethodGet {
		http.Error(w, "use GET", http.StatusMethodNotAllowed)
		return
	}

	// 2️⃣ Get user email from context
	email, ok := pkg.GetEmailFromContext(r.Context())
	if !ok || email == "" {
		http.Error(w, "unauthorized: missing email", http.StatusUnauthorized)
		return
	}

	// 3️⃣ Parse query params for pagination
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		page = 1
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 10
	}

	offset := (page - 1) * limit

	// 4️⃣ Fetch from service
	metas, err := h.p.GetPromptMetaByEmail(r.Context(), email, limit, offset)
	if err != nil {
		http.Error(w, "failed to get prompt metadata: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 5️⃣ Send JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metas); err != nil {
		http.Error(w, "failed to encode response: "+err.Error(), http.StatusInternalServerError)
	}
}
func (h *Handler) GetBrandOverviewByPrompt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "use GET", http.StatusMethodNotAllowed)
		return
	}

	// Get email from context
	email, ok := pkg.GetEmailFromContext(r.Context())
	if !ok || email == "" {
		http.Error(w, "unauthorized: missing email", http.StatusUnauthorized)
		return
	}

	// Get prompt_id from query param
	promptIDStr := r.URL.Query().Get("prompt_id")
	if promptIDStr == "" {
		http.Error(w, "missing prompt_id", http.StatusBadRequest)
		return
	}

	promptID, err := strconv.Atoi(promptIDStr)
	if err != nil {
		http.Error(w, "invalid prompt_id", http.StatusBadRequest)
		return
	}

	// Call service
	overview, err := h.p.GetBrandOverviewByPrompt(r.Context(), email, promptID)
	if err != nil {
		http.Error(w, "failed to get brand overview: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(overview); err != nil {
		http.Error(w, "failed to encode response: "+err.Error(), http.StatusInternalServerError)
	}
}
func (h *Handler) GetDomainOverviewByPrompt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "use GET", http.StatusMethodNotAllowed)
		return
	}

	// Get email from context
	email, ok := pkg.GetEmailFromContext(r.Context())
	if !ok || email == "" {
		http.Error(w, "unauthorized: missing email", http.StatusUnauthorized)
		return
	}

	// Get prompt_id from query param
	promptIDStr := r.URL.Query().Get("prompt_id")
	if promptIDStr == "" {
		http.Error(w, "missing prompt_id", http.StatusBadRequest)
		return
	}

	promptID, err := strconv.Atoi(promptIDStr)
	if err != nil {
		http.Error(w, "invalid prompt_id", http.StatusBadRequest)
		return
	}

	// Call service
	overview, err := h.p.GetDomainOverviewByPrompt(r.Context(), email, promptID)
	if err != nil {
		http.Error(w, "failed to get domain overview: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(overview); err != nil {
		http.Error(w, "failed to encode response: "+err.Error(), http.StatusInternalServerError)
	}
}
func (h *Handler) AddPrompt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}

	email, ok := pkg.GetEmailFromContext(r.Context())
	if !ok || email == "" {
		http.Error(w, "unauthorized: missing email", http.StatusUnauthorized)
		return
	}

	// Parse single prompt request
	var req struct {
		Prompt  string   `json:"prompt" validate:"required"`
		Country string   `json:"country" validate:"required"`
		Tags    []string `json:"tags,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		http.Error(w, "validation error: "+err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	// Get user info
	userData, err := h.usvc.GetUserByEmail(ctx, email)
	if err != nil {
		http.Error(w, "failed to fetch user data: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if userData == nil || userData.BrandName == "" {
		http.Error(w, "brand not configured for this user", http.StatusBadRequest)
		return
	}

	// Send prompt to OpenAI
	respText, err := h.p.SendToOpenAI(ctx, email, req.Prompt, req.Country)
	if err != nil {
		http.Error(w, "OpenAI API error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Store prompt response
	entry := repository.PromptResponseEntry{
		UserEmail: email,
		Prompt:    req.Prompt,
		Response:  respText,
		Country:   req.Country,
		Added:     time.Now().UTC(),
	}

	promptIDs, err := h.p.StorePromptResponses(ctx, []repository.PromptResponseEntry{entry})
	if err != nil {
		http.Error(w, "failed to store prompt response: "+err.Error(), http.StatusInternalServerError)
		return
	}
	promptID := promptIDs[0]

	// Generate brand aliases & competitor aliases
	brandAliases := pkg.GenerateAliases(userData.BrandName)
	competitorMap := make(map[string][]string)
	for _, c := range userData.Competitor {
		competitorMap[c.TrackedName] = pkg.GenerateAliases(c.TrackedName)
	}

	// Analyze response
	analysisResults := pkg.AnalyzeResponses(
		[]pkg.PromptResponse{{Prompt: req.Prompt, Response: respText}},
		req.Country,
		userData.BrandName,
		brandAliases,
		competitorMap,
	)

	// Store analyses
	for _, a := range analysisResults {
		// Prompt metadata
		promptMeta := repository.PromptMeta{
			PromptID:  promptID,
			UserEmail: email,
			Prompt:    a.Prompt,
			Mentions:  a.Mentions,
			Volume:    a.Volume,
			Tags:      a.Tags,
			Location:  a.Location,
			Added:     time.Now().UTC(),
		}
		if err := h.p.StorePromptMeta(ctx, []repository.PromptMeta{promptMeta}); err != nil {
			http.Error(w, "failed to store prompt metadata: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Brand analysis
		var brandEntries []repository.BrandAnalysis
		for _, b := range a.Brands {
			brandEntries = append(brandEntries, repository.BrandAnalysis{
				PromptID:   promptID,
				UserEmail:  email,
				BrandName:  b.BrandName,
				Visibility: b.Visibility,
				Sentiment:  b.Sentiment,
				Position:   b.Position,
				Added:      time.Now().UTC(),
			})
		}
		if err := h.p.StoreBrandAnalyses(ctx, brandEntries); err != nil {
			http.Error(w, "failed to store brand analyses: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Domain analysis
		var domainEntries []repository.DomainAnalysis
		for _, d := range a.Domains {
			domainEntries = append(domainEntries, repository.DomainAnalysis{
				PromptID:     promptID,
				Domain:       d.Domain,
				Used:         d.Used,
				AvgCitations: d.AvgCitations,
				Type:         d.Type,
			})
		}
		if err := h.p.StoreDomainAnalyses(ctx, domainEntries); err != nil {
			http.Error(w, "failed to store domain analyses: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"message":"prompt processed and analyzed successfully"}`)
}
