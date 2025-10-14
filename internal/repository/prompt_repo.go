package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type MinimalAnalysis struct {
	Prompt     string           `json:"prompt"`
	Response   string           `json:"response"`
	Tags       []string         `json:"tags"` // frontend tags
	Sentiment  int              `json:"sentiment"`
	Position   int              `json:"position"`
	Mentions   map[string]int   `json:"mentions"` // brand & competitor mentions
	Visibility float64          `json:"visibility"`
	Domains    []DomainAnalysis `json:"domains"`
	Volume     int              `json:"volume"`
	Location   string           `json:"location"`
	Brands     []BrandAnalysis  `json:"brands"`
	Added      time.Time        `json:"added"`
}
type PromptRepo struct {
	db *pgxpool.Pool
}

func NewPromptRepo(db *pgxpool.Pool) *PromptRepo {
	return &PromptRepo{db: db}
}

type PromptResponseEntry struct {
	ID        int       `json:"id"`
	UserEmail string    `json:"user_email"`
	Prompt    string    `json:"prompt"`
	Response  string    `json:"response"`
	Country   string    `json:"country"`
	Added     time.Time `json:"added"`
}

// AddPromptResponse inserts a single record
func (r *PromptRepo) StorePromptResponses(ctx context.Context, entries []PromptResponseEntry) ([]int, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	query := `
		INSERT INTO prompt_response_entry (user_email, prompt, response, country, added)
		VALUES %s
		RETURNING id
	`

	valueStrings := make([]string, 0, len(entries))
	valueArgs := make([]interface{}, 0, len(entries)*5)

	for i, e := range entries {
		idx := i*5 + 1
		valueStrings = append(valueStrings, fmt.Sprintf("($%d,$%d,$%d,$%d,$%d)", idx, idx+1, idx+2, idx+3, idx+4))
		valueArgs = append(valueArgs, e.UserEmail, e.Prompt, e.Response, e.Country, e.Added)
	}

	finalQuery := fmt.Sprintf(query, strings.Join(valueStrings, ","))

	rows, err := r.db.Query(ctx, finalQuery, valueArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]int, 0, len(entries))
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return ids, nil
}

// GetPromptResponsesByEmail retrieves paginated records
func (r *PromptRepo) GetPromptResponsesByEmail(ctx context.Context, email string, limit, offset int) ([]PromptResponseEntry, error) {
	query := `
		SELECT id, user_email, prompt, response, country, added
		FROM prompt_response_entry	
		WHERE user_email = $1
		ORDER BY added DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(ctx, query, email, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []PromptResponseEntry
	for rows.Next() {
		var e PromptResponseEntry
		if err := rows.Scan(&e.ID, &e.UserEmail, &e.Prompt, &e.Response, &e.Country, &e.Added); err != nil {
			return nil, err
		}
		results = append(results, e)
	}
	return results, nil
}

//Final

// 🧩 2️⃣ PromptMeta
// High-level prompt info (acts as root for analyses)
type PromptMeta struct {
	ID        int            `json:"id"`
	PromptID  int            `json:"prompt_id"`
	UserEmail string         `json:"user_email"`
	Prompt    string         `json:"prompt"`
	Mentions  map[string]int `json:"mentions"`
	Volume    int            `json:"volume"`
	Tags      []string       `json:"tags"`
	Location  string         `json:"location"`
	Added     time.Time      `json:"added"`
}

// 🧩 3️⃣ BrandAnalysis
// Per-brand analysis results
type BrandAnalysis struct {
	ID         int       `json:"id"`
	PromptID   int       `json:"prompt_id"`
	UserEmail  string    `json:"user_email"`
	BrandName  string    `json:"brand_name"`
	Visibility float64   `json:"visibility"`
	Sentiment  int       `json:"sentiment"`
	Position   int       `json:"position"`
	Added      time.Time `json:"added"`
}

// 🧩 4️⃣ DomainAnalysis
// Per-domain metrics
type DomainAnalysis struct {
	ID           int       `json:"id"`
	PromptID     int       `json:"prompt_id"`
	Domain       string    `json:"domain"`
	Used         int       `json:"used"`
	AvgCitations float64   `json:"avg_citations"`
	Type         string    `json:"type"`
	Added        time.Time `json:"added"`
}

// 🧩 Store Prompt Meta
func (r *PromptRepo) StorePromptMeta(ctx context.Context, entries []PromptMeta) error {
	if len(entries) == 0 {
		return nil
	}

	query := `
		INSERT INTO prompt_meta (prompt_id, user_email, prompt, mentions, volume, tags, location, added)
		VALUES %s
	`

	valueStrings := make([]string, 0, len(entries))
	valueArgs := make([]interface{}, 0, len(entries)*8) // 8 columns now, including prompt_id

	for i, e := range entries {
		idx := i*8 + 1
		valueStrings = append(valueStrings,
			fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
				idx, idx+1, idx+2, idx+3, idx+4, idx+5, idx+6, idx+7,
			))
		valueArgs = append(valueArgs, e.PromptID, e.UserEmail, e.Prompt, e.Mentions, e.Volume, e.Tags, e.Location, e.Added)
	}

	finalQuery := fmt.Sprintf(query, strings.Join(valueStrings, ","))

	_, err := r.db.Exec(ctx, finalQuery, valueArgs...)
	return err
}

func (r *PromptRepo) StoreBrandAnalyses(ctx context.Context, entries []BrandAnalysis) error {
	if len(entries) == 0 {
		return nil
	}

	query := `
		INSERT INTO brand_analysis (prompt_id, user_email, brand_name, visibility, sentiment, position, added)
		VALUES %s
	`

	valueStrings := make([]string, 0, len(entries))
	valueArgs := make([]interface{}, 0, len(entries)*7)

	for i, e := range entries {
		idx := i*7 + 1
		valueStrings = append(valueStrings,
			fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d)",
				idx, idx+1, idx+2, idx+3, idx+4, idx+5, idx+6,
			))
		valueArgs = append(valueArgs,
			e.PromptID, e.UserEmail, e.BrandName, e.Visibility, e.Sentiment, e.Position, e.Added,
		)
	}

	finalQuery := fmt.Sprintf(query, strings.Join(valueStrings, ","))

	_, err := r.db.Exec(ctx, finalQuery, valueArgs...)
	return err
}

// 🧩 Store Domain Analyses
func (r *PromptRepo) StoreDomainAnalyses(ctx context.Context, entries []DomainAnalysis) error {
	if len(entries) == 0 {
		return nil
	}

	query := `
		INSERT INTO domain_analysis (prompt_id, domain, used, avg_citations, type, added)
		VALUES %s
	`

	valueStrings := make([]string, 0, len(entries))
	valueArgs := make([]interface{}, 0, len(entries)*6)

	for i, e := range entries {
		idx := i*6 + 1
		valueStrings = append(valueStrings,
			fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d)", idx, idx+1, idx+2, idx+3, idx+4, idx+5))
		valueArgs = append(valueArgs,
			e.PromptID, e.Domain, e.Used, e.AvgCitations, e.Type, e.Added,
		)
	}

	finalQuery := fmt.Sprintf(query, strings.Join(valueStrings, ","))

	_, err := r.db.Exec(ctx, finalQuery, valueArgs...)
	return err
}

// GetBrandAnalysesByEmail retrieves paginated brand analyses by user email
func (r *PromptRepo) GetBrandAnalysesByEmail(ctx context.Context, email string, limit, offset int) ([]BrandAnalysis, error) {
	query := `
		SELECT id, prompt_id, user_email, brand_name, visibility, sentiment, position, added
		FROM brand_analysis
		WHERE user_email = $1
		ORDER BY added DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(ctx, query, email, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query brand analyses: %w", err)
	}
	defer rows.Close()

	var analyses []BrandAnalysis
	for rows.Next() {
		var a BrandAnalysis
		if err := rows.Scan(
			&a.ID,
			&a.PromptID,
			&a.UserEmail,
			&a.BrandName,
			&a.Visibility,
			&a.Sentiment,
			&a.Position,
			&a.Added,
		); err != nil {
			return nil, fmt.Errorf("scan brand analysis: %w", err)
		}
		analyses = append(analyses, a)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate brand analyses: %w", err)
	}

	return analyses, nil
}

// GetDomainAnalysesByEmail retrieves paginated domain analyses for a user
func (r *PromptRepo) GetDomainAnalysesByEmail(ctx context.Context, email string, limit, offset int) ([]DomainAnalysis, error) {
	query := `
	SELECT da.id, da.prompt_id, da.domain, da.used, da.avg_citations, da.type, da.added
	FROM domain_analysis AS da
	JOIN prompt_response_entry AS pr ON da.prompt_id = pr.id
	WHERE pr.user_email = $1
	ORDER BY da.added DESC
	LIMIT $2 OFFSET $3
`

	rows, err := r.db.Query(ctx, query, email, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query domain analyses: %w", err)
	}
	defer rows.Close()

	var analyses []DomainAnalysis
	for rows.Next() {
		var a DomainAnalysis
		if err := rows.Scan(
			&a.ID,
			&a.PromptID,
			&a.Domain,
			&a.Used,
			&a.AvgCitations,
			&a.Type,
			&a.Added,
		); err != nil {
			return nil, fmt.Errorf("scan domain analysis: %w", err)
		}
		analyses = append(analyses, a)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate domain analyses: %w", err)
	}

	return analyses, nil
}

type BrandOverview struct {
	BrandName     string  `json:"brand_name"`
	AvgVisibility float64 `json:"avg_visibility"`
	AvgPosition   float64 `json:"avg_position"`
	AvgSentiment  float64 `json:"avg_sentiment"`
}

func (r *PromptRepo) GetBrandOverviewByEmail(ctx context.Context, email string) ([]BrandOverview, error) {
	query := `
		SELECT 
			ba.brand_name,
			AVG(ba.visibility) AS avg_visibility,
			AVG(ba.position) AS avg_position,
			AVG(ba.sentiment) AS avg_sentiment
		FROM brand_analysis AS ba
		JOIN prompt_response_entry AS pr ON ba.prompt_id = pr.id
		WHERE pr.user_email = $1
		GROUP BY ba.brand_name
		ORDER BY avg_visibility DESC
		`

	rows, err := r.db.Query(ctx, query, email)
	if err != nil {
		return nil, fmt.Errorf("query brand overview: %w", err)
	}
	defer rows.Close()

	var overviews []BrandOverview
	for rows.Next() {
		var o BrandOverview
		if err := rows.Scan(
			&o.BrandName,
			&o.AvgVisibility,
			&o.AvgPosition,
			&o.AvgSentiment,
		); err != nil {
			return nil, fmt.Errorf("scan brand overview: %w", err)
		}
		overviews = append(overviews, o)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate brand overview: %w", err)
	}

	return overviews, nil
}
func (r *PromptRepo) GetPromptMetaByEmail(ctx context.Context, email string, limit, offset int) ([]PromptMeta, error) {
	query := `
		SELECT id, prompt_id, user_email, prompt, mentions, volume, tags, location, added
		FROM prompt_meta
		WHERE user_email = $1
		ORDER BY added DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(ctx, query, email, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query prompt meta: %w", err)
	}
	defer rows.Close()

	var metas []PromptMeta
	for rows.Next() {
		var m PromptMeta
		var mentionsJSON []byte

		if err := rows.Scan(
			&m.ID,
			&m.PromptID,
			&m.UserEmail,
			&m.Prompt,
			&mentionsJSON,
			&m.Volume,
			&m.Tags,
			&m.Location,
			&m.Added,
		); err != nil {
			return nil, fmt.Errorf("scan prompt meta: %w", err)
		}

		if err := json.Unmarshal(mentionsJSON, &m.Mentions); err != nil {
			m.Mentions = map[string]int{}
		}

		metas = append(metas, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate prompt meta: %w", err)
	}

	return metas, nil
}
func (r *PromptRepo) GetBrandOverviewByPrompt(ctx context.Context, email string, promptID int) ([]BrandOverview, error) {
	query := `
		SELECT 
			ba.brand_name,
			ba.visibility AS avg_visibility,
			ba.position AS avg_position,
			ba.sentiment AS avg_sentiment
		FROM brand_analysis AS ba
		JOIN prompt_response_entry AS pr ON ba.prompt_id = pr.id
		WHERE pr.user_email = $1 AND pr.id = $2
		ORDER BY ba.visibility DESC
	`

	rows, err := r.db.Query(ctx, query, email, promptID)
	if err != nil {
		return nil, fmt.Errorf("query brand overview by prompt: %w", err)
	}
	defer rows.Close()

	var overviews []BrandOverview
	for rows.Next() {
		var o BrandOverview
		if err := rows.Scan(
			&o.BrandName,
			&o.AvgVisibility,
			&o.AvgPosition,
			&o.AvgSentiment,
		); err != nil {
			return nil, fmt.Errorf("scan brand overview by prompt: %w", err)
		}
		overviews = append(overviews, o)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate brand overview by prompt: %w", err)
	}

	return overviews, nil
}
func (r *PromptRepo) GetDomainOverviewByPrompt(ctx context.Context, email string, promptID int) ([]DomainAnalysis, error) {
	query := `
		SELECT 
			da.domain,
			da.used,
			da.avg_citations,
			da.type,
			da.added
		FROM domain_analysis AS da
		JOIN prompt_response_entry AS pr ON da.prompt_id = pr.id
		WHERE pr.user_email = $1 AND da.prompt_id = $2
		ORDER BY da.avg_citations DESC
	`

	rows, err := r.db.Query(ctx, query, email, promptID)
	if err != nil {
		return nil, fmt.Errorf("query domain overview by prompt: %w", err)
	}
	defer rows.Close()

	var domainOverview []DomainAnalysis
	for rows.Next() {
		var o DomainAnalysis
		if err := rows.Scan(
			&o.Domain,
			&o.Used,
			&o.AvgCitations,
			&o.Type,
			&o.Added,
		); err != nil {
			return nil, fmt.Errorf("scan domain overview by prompt: %w", err)
		}
		domainOverview = append(domainOverview, o)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate domain overview by prompt: %w", err)
	}

	return domainOverview, nil
}
