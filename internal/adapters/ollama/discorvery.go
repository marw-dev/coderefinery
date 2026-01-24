package ollama

import (
	"coderefinery/internal/models"
	"regexp"
	"sort"
	"strings"
)

type ModelClassifier struct {
	embedPatterns   []*regexp.Regexp
	plannerPatterns []*regexp.Regexp
	coderPatterns   []*regexp.Regexp
}

func NewModelClassifier() *ModelClassifier {
	return &ModelClassifier{
		embedPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)embed`),
			regexp.MustCompile(`(?i)nomic`),
			regexp.MustCompile(`(?i)mxbai`),
			regexp.MustCompile(`(?i)bge-`),
			regexp.MustCompile(`(?i)e5-`),
		},
		plannerPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)r1`),
			regexp.MustCompile(`(?i)reasoning`),
			regexp.MustCompile(`(?i)deepseek.*r1`),
			regexp.MustCompile(`(?i)o1`),
			regexp.MustCompile(`(?i)qwen.*plus`),
		},
		coderPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)coder`),
			regexp.MustCompile(`(?i)code`),
			regexp.MustCompile(`(?i)qwen.*coder`),
			regexp.MustCompile(`(?i)deepseek.*coder`),
			regexp.MustCompile(`(?i)codellama`),
			regexp.MustCompile(`(?i)starcoder`),
			regexp.MustCompile(`(?i)wizard.*coder`),
		},
	}
}

func (c *ModelClassifier) ClassifyModel(modelName string) models.ModelRole {
	for _, pattern := range c.embedPatterns {
		if pattern.MatchString(modelName) {
			return models.RoleEmbedder
		}
	}
	for _, pattern := range c.plannerPatterns {
		if pattern.MatchString(modelName) {
			return models.RolePlanner
		}
	}
	for _, pattern := range c.coderPatterns {
		if pattern.MatchString(modelName) {
			return models.RoleCoder
		}
	}
	return models.RoleGeneralist
}

// ExtractMetadata extrahiert Details aus dem Ollama Model Info Objekt
// Hinweis: 'OllamaModel' Struktur muss analog zur API Response definiert sein
func (c *ModelClassifier) ExtractMetadata(modelName string, tags []string) models.ModelCapability {
	paramSize := extractParameterSize(modelName, tags)
	version := extractVersion(modelName)
	family := extractFamily(modelName)

	return models.ModelCapability{
		Name:          modelName,
		Role:          c.ClassifyModel(modelName),
		ParameterSize: paramSize,
		ContextWindow: extractContextWindow(modelName), // Vereinfacht ohne API Call
		Family:        family,
		Version:       version,
		Tags:          tags,
	}
}

// Scoring-System für Embedder
type EmbedderScore struct {
	Model   models.ModelCapability
	Score   float64
	Reasons []string
}

func (c *ModelClassifier) ScoreEmbedder(model models.ModelCapability) EmbedderScore {
	score := 0.0
	reasons := []string{}
	nameLower := strings.ToLower(model.Name)

	if strings.Contains(nameLower, "nomic-embed-text") {
		score += 15
		reasons = append(reasons, "Industry standard for code embedding")
	}
	if strings.Contains(nameLower, "mxbai-embed-large") {
		score += 12
		reasons = append(reasons, "Excellent multilingual support")
	}
	if model.ContextWindow >= 8192 {
		score += 5
		reasons = append(reasons, "Large context window (8K+)")
	}

	return EmbedderScore{Model: model, Score: score, Reasons: reasons}
}

func (c *ModelClassifier) ScorePlanner(model models.ModelCapability) EmbedderScore {
	score := 0.0
	reasons := []string{}
	nameLower := strings.ToLower(model.Name)

	if strings.Contains(nameLower, "deepseek") && strings.Contains(nameLower, "r1") {
		score += 20
		reasons = append(reasons, "Specialized reasoning model")
	}
	if strings.Contains(nameLower, "qwen") && (strings.Contains(nameLower, "plus") || strings.Contains(nameLower, "max")) {
		score += 15
		reasons = append(reasons, "Advanced reasoning capabilities")
	}
	if model.ContextWindow >= 32000 {
		score += 8
		reasons = append(reasons, "Very large context (32K+)")
	}

	return EmbedderScore{Model: model, Score: score, Reasons: reasons}
}

func (c *ModelClassifier) ScoreCoder(model models.ModelCapability) EmbedderScore {
	score := 0.0
	reasons := []string{}
	nameLower := strings.ToLower(model.Name)

	if strings.Contains(nameLower, "qwen") && strings.Contains(nameLower, "coder") {
		score += 18
		reasons = append(reasons, "Specialized code generation model")
	}
	if strings.Contains(nameLower, "deepseek") && strings.Contains(nameLower, "coder") {
		score += 16
		reasons = append(reasons, "Strong code synthesis")
	}
	if strings.Contains(model.ParameterSize, "32B") {
		score += 8
		reasons = append(reasons, "Optimal size for code (32B)")
	}

	return EmbedderScore{Model: model, Score: score, Reasons: reasons}
}

func (c *ModelClassifier) GenerateRecommendations(classification *models.ModelClassification) models.ModelRecommendations {
	recs := models.ModelRecommendations{}

	// Embedder
	var eScores []EmbedderScore
	for _, m := range classification.Embedders {
		eScores = append(eScores, c.ScoreEmbedder(m))
	}
	sort.Slice(eScores, func(i, j int) bool { return eScores[i].Score > eScores[j].Score })
	if len(eScores) > 0 {
		recs.Embedder = models.ModelRecommendation{ModelName: eScores[0].Model.Name, Score: eScores[0].Score, Reasons: eScores[0].Reasons}
	}

	// Planner
	var pScores []EmbedderScore
	for _, m := range classification.Planners {
		pScores = append(pScores, c.ScorePlanner(m))
	}
	if len(pScores) == 0 {
		for _, m := range classification.Generalists {
			pScores = append(pScores, c.ScorePlanner(m))
		}
	}
	sort.Slice(pScores, func(i, j int) bool { return pScores[i].Score > pScores[j].Score })
	if len(pScores) > 0 {
		recs.Planner = models.ModelRecommendation{ModelName: pScores[0].Model.Name, Score: pScores[0].Score, Reasons: pScores[0].Reasons}
	}

	// Coder
	var cScores []EmbedderScore
	for _, m := range classification.Coders {
		cScores = append(cScores, c.ScoreCoder(m))
	}
	if len(cScores) == 0 {
		for _, m := range classification.Generalists {
			cScores = append(cScores, c.ScoreCoder(m))
		}
	}
	sort.Slice(cScores, func(i, j int) bool { return cScores[i].Score > cScores[j].Score })
	if len(cScores) > 0 {
		recs.Coder = models.ModelRecommendation{ModelName: cScores[0].Model.Name, Score: cScores[0].Score, Reasons: cScores[0].Reasons}
	}

	return recs
}

// Helper Functions (vereinfacht)
func extractParameterSize(name string, tags []string) string {
	paramPattern := regexp.MustCompile(`(\d+\.?\d*)[BM]`)
	for _, tag := range tags {
		if matches := paramPattern.FindStringSubmatch(tag); len(matches) > 0 {
			return matches[0]
		}
	}
	if matches := paramPattern.FindStringSubmatch(name); len(matches) > 0 {
		return matches[0]
	}
	return "unknown"
}

func extractVersion(name string) string {
	versionPattern := regexp.MustCompile(`v(\d+\.?\d*)`)
	if matches := versionPattern.FindStringSubmatch(name); len(matches) > 1 {
		return "v" + matches[1]
	}
	return "unknown"
}

func extractFamily(name string) string {
	nameLower := strings.ToLower(name)
	families := []string{"llama", "qwen", "deepseek", "mistral", "gemma", "nomic"}
	for _, f := range families {
		if strings.Contains(nameLower, f) {
			return f
		}
	}
	return "unknown"
}

func extractContextWindow(name string) int {
	nameLower := strings.ToLower(name)
	if strings.Contains(nameLower, "128k") { return 128000 }
	if strings.Contains(nameLower, "32k") { return 32000 }
	if strings.Contains(nameLower, "16k") { return 16000 }
	if strings.Contains(nameLower, "8k") { return 8192 }
	return 4096
}
