package pkg

import (
	"auth-microservice/internal/repository"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cdipaolo/sentiment"
)

// PromptResponse holds one prompt and its AI response
type PromptResponse struct {
	Prompt   string
	Response string
}

var model *sentiment.Models // do NOT restore here

// Provide a setter so main.go can initialize it
func SetSentimentModel(m *sentiment.Models) {
	model = m
}

// Returns sentiment as a number from 1 to 100
func AnalyzeSentiment(text string) int {
	if model == nil {
		panic("sentiment model not initialized")
	}
	analysis := model.SentimentAnalysis(text, sentiment.English)

	// Convert the original sentiment score to a 1-100 scale
	// Assuming analysis.Score is usually in range [0, 2] (or similar)
	score := int((float64(analysis.Score)/2.0)*99.0) + 1

	if score < 1 {
		score = 1
	} else if score > 100 {
		score = 100
	}

	return score
}

// GenerateAliases generates lowercase variants of a brand name
func GenerateAliases(name string) []string {
	name = strings.ToLower(strings.TrimSpace(name))
	parts := strings.Fields(name)

	aliasesMap := make(map[string]struct{})
	var aliases []string

	// 1️⃣ Original lowercase
	aliasesMap[name] = struct{}{}

	// 2️⃣ Join without spaces
	noSpace := strings.Join(parts, "")
	aliasesMap[noSpace] = struct{}{}

	// 3️⃣ First word only
	if len(parts) > 0 {
		aliasesMap[parts[0]] = struct{}{}
	}

	// 4️⃣ Initials (e.g., "bajaj finserv" -> "bf")
	if len(parts) > 1 {
		var initials strings.Builder
		for _, p := range parts {
			initials.WriteByte(p[0])
		}
		aliasesMap[initials.String()] = struct{}{}
	}

	// 5️⃣ Variants with hyphens/underscores removed (common in text)
	cleaned := strings.ReplaceAll(noSpace, "-", "")
	cleaned = strings.ReplaceAll(cleaned, "_", "")
	aliasesMap[cleaned] = struct{}{}

	// Convert map keys to slice
	for a := range aliasesMap {
		aliases = append(aliases, a)
	}

	return aliases
}

func ExtractDomains(text string) []repository.DomainAnalysis {
	domainRegex := `([a-zA-Z0-9-]+\.)+[a-zA-Z]{2,}`
	re := regexp.MustCompile(domainRegex)
	matches := re.FindAllString(text, -1)

	// Keep unique domains
	unique := make(map[string]struct{})
	for _, m := range matches {
		unique[m] = struct{}{}
	}

	var domains []repository.DomainAnalysis
	for d := range unique {
		domains = append(domains, repository.DomainAnalysis{
			Domain:       d,
			Used:         1,         // initial usage count
			AvgCitations: 0,         // can calculate later if needed
			Type:         "unknown", // placeholder, can update based on rules
			Added:        time.Now().UTC(),
		})
	}

	return domains
}
func CountBrandMentions(
	text string,
	brandName string,
	brandAliases []string,
	competitorAliases map[string][]string,
) map[string]int {
	counts := make(map[string]int)
	normText := strings.ToLower(text)

	// Helper: count occurrences of an alias using word boundaries
	countMatches := func(t string, alias string) int {
		alias = strings.ToLower(alias)
		alias = strings.TrimSpace(alias)
		alias = regexp.QuoteMeta(alias) // escape regex chars
		re := regexp.MustCompile(`\b` + alias + `\b`)
		return len(re.FindAllStringIndex(t, -1))
	}

	// 1️⃣ Count brand mentions using aliases
	brandCount := 0
	for _, alias := range brandAliases {
		c := countMatches(normText, alias)
		brandCount += c
		// Remove matched alias from text to avoid double-counting
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(strings.ToLower(alias)) + `\b`)
		normText = re.ReplaceAllString(normText, "")
	}
	counts[brandName] = brandCount

	// 2️⃣ Count competitor mentions
	for compName, aliases := range competitorAliases {
		compCount := 0
		for _, alias := range aliases {
			c := countMatches(normText, alias)
			compCount += c
			re := regexp.MustCompile(`\b` + regexp.QuoteMeta(strings.ToLower(alias)) + `\b`)
			normText = re.ReplaceAllString(normText, "")
		}
		counts[compName] = compCount
	}

	return counts
}

// CalculateBrandVisibility returns how visible the brand is among all mentions (in %)
func CalculateBrandVisibility(mentions map[string]int, brandAliases []string) float64 {
	total := 0
	brandCount := 0

	for name, count := range mentions {
		total += count
		// check if this mention belongs to brand aliases
		for _, alias := range brandAliases {
			if strings.EqualFold(name, alias) {
				brandCount += count
				break
			}
		}
	}

	if total == 0 {
		return 0.0
	}
	return (float64(brandCount) / float64(total)) * 100
}

// WordVolume returns total number of words in the text
func WordVolume(text string) int {
	return len(strings.Fields(text))
}
func AnalyzeResponses(
	responses []PromptResponse,
	country string,
	brandName string,
	brandAliases []string,
	competitorAliases map[string][]string,
) []repository.MinimalAnalysis {
	var results []repository.MinimalAnalysis

	for _, r := range responses {
		// Count mentions
		mentions := CountBrandMentions(r.Response, brandName, brandAliases, competitorAliases)

		// Prepare brand analyses
		var brandAnalyses []repository.BrandAnalysis

		// Main brand
		mainPosition := BrandPosition(r.Response, brandName, brandAliases, competitorAliases)
		mainSentiment := AnalyzeSentiment(r.Response)
		mainVisibility := CalculateBrandVisibility(mentions, append([]string{brandName}, brandAliases...))

		brandAnalyses = append(brandAnalyses, repository.BrandAnalysis{
			BrandName:  brandName,
			Sentiment:  mainSentiment,
			Position:   mainPosition,
			Visibility: mainVisibility,
		})

		// Competitors
		for comp, aliases := range competitorAliases {
			compPosition := BrandPosition(r.Response, comp, aliases, competitorAliases)
			compSentiment := AnalyzeSentiment(r.Response)
			compVisibility := CalculateBrandVisibility(mentions, aliases)

			brandAnalyses = append(brandAnalyses, repository.BrandAnalysis{
				BrandName:  comp,
				Sentiment:  compSentiment,
				Position:   compPosition,
				Visibility: compVisibility,
			})
		}

		analysis := repository.MinimalAnalysis{
			Prompt:     r.Prompt,
			Response:   r.Response,
			Sentiment:  mainSentiment, // top-level sentiment still main brand
			Position:   mainPosition,  // top-level position still main brand
			Mentions:   mentions,
			Visibility: mainVisibility, // top-level visibility still main brand
			Domains:    ExtractDomains(r.Response),
			Volume:     WordVolume(r.Response),
			Location:   country,
			Brands:     brandAnalyses, // filled with main + competitors
			Added:      time.Now(),
		}

		results = append(results, analysis)
	}

	return results
}

// BrandPosition calculates the rank (position) of a main brand in a text
// among competitors. Returns 0 if the brand is not mentioned.
func BrandPosition(
	text string,
	brandName string,
	brandAliases []string,
	competitorAliases map[string][]string,
) int {
	textLower := strings.ToLower(text)

	// Map to store first occurrence of each brand/competitor
	brandIndices := map[string]int{}

	// Main brand + aliases
	allBrands := append([]string{brandName}, brandAliases...)
	for _, b := range allBrands {
		if idx := strings.Index(textLower, strings.ToLower(b)); idx != -1 {
			brandIndices[b] = idx
		}
	}

	// Competitor aliases
	for comp, aliases := range competitorAliases {
		for _, alias := range aliases {
			if idx := strings.Index(textLower, strings.ToLower(alias)); idx != -1 {
				brandIndices[comp] = idx
			}
		}
	}

	// Sort all brands by first occurrence
	type brandPos struct {
		Name  string
		Index int
	}
	var positions []brandPos
	for b, idx := range brandIndices {
		positions = append(positions, brandPos{b, idx})
	}
	sort.Slice(positions, func(i, j int) bool { return positions[i].Index < positions[j].Index })

	// Determine rank of main brand
	for i, bp := range positions {
		if strings.EqualFold(bp.Name, brandName) {
			return i + 1 // 1-based ranking
		}
	}

	return 0 // Not mentioned
}
