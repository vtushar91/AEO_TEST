package handler

import (
	"auth-microservice/internal/pkg"
	"auth-microservice/internal/repository"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"
)

func (h *Handler) AddCompetitor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}

	email, ok := pkg.GetEmailFromContext(r.Context())
	if !ok {
		http.Error(w, "email not found in context", http.StatusUnauthorized)
		return
	}

	var input struct {
		BrandName   string `json:"brand_name"`
		Domain      string `json:"domain"`
		TrackedName string `json:"tracked_name,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if input.BrandName == "" || input.Domain == "" {
		http.Error(w, "brand_name and domain are required", http.StatusBadRequest)
		return
	}

	if input.TrackedName == "" {
		input.TrackedName = input.BrandName
	}

	comp := repository.Competitor{
		DisplayName: input.BrandName,
		Domain:      input.Domain,
		TrackedName: input.TrackedName,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Only call the Service layer
	if err := h.usvc.AddCompetitor(ctx, email, []repository.Competitor{comp}); err != nil {
		http.Error(w, "failed to add competitor: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": fmt.Sprintf("competitor '%s' added successfully", input.BrandName),
	})
}

func (h *Handler) GetCompetitor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "use GET", http.StatusMethodNotAllowed)
		return
	}
	email, ok := pkg.GetEmailFromContext(r.Context())
	if !ok {
		http.Error(w, "email not found in context", http.StatusUnauthorized)
		return
	}

	// Parse pagination parameters from query
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		page = 1 // default page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 10 // default limit = 10
	}

	// Fetch paginated prompts
	competitor, total, err := h.usvc.GetCompetitor(r.Context(), email, page, limit)
	if err != nil {
		http.Error(w, "failed to get prompts: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Build response with pagination metadata
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "competitors fetched successfully",
		"data":    competitor,
		"pagination": map[string]interface{}{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": int(math.Ceil(float64(total) / float64(limit))),
		},
	})
}
func (h *Handler) AddBrandDetails(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}

	// Get email from context
	email, ok := pkg.GetEmailFromContext(r.Context())
	if !ok || email == "" {
		http.Error(w, "unauthorized: missing email", http.StatusUnauthorized)
		return
	}

	// Parse request body
	var req struct {
		BrandName string `json:"brand_name"`
		Domain    string `json:"domain"`
		Country   string `json:"country"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.BrandName == "" || req.Domain == "" || req.Country == "" {
		http.Error(w, "brand_name, domain, and country are required", http.StatusBadRequest)
		return
	}

	// Call service
	err := h.usvc.UpdateUserProfile(r.Context(), email, req.BrandName, req.Domain, req.Country)
	if err != nil {
		http.Error(w, "failed to update profile: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Respond
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "Brand details added",
	})
}
func (h *Handler) GetCompetitorSuggestions(w http.ResponseWriter, r *http.Request) {
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
	competitor, err := h.usvc.GenerateCompetitor(r.Context(), userData.Domain, userData.Country)
	if err != nil {
		http.Error(w, "failed to generate prompts: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 5. Prepare response with country for each prompt
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(competitor)
}
