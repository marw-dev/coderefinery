package llm

import (
	ollama "coderefinery/internal/adapters/ollama"
	"coderefinery/internal/models"
	"context"
	"fmt"

	"github.com/tmc/langchaingo/llms"
	prov "github.com/tmc/langchaingo/llms/ollama"
)

// OllamaProvider vereint Embedder, Generator und Discovery
type OllamaProvider struct {
	*OllamaEmbedder // Erbt Embed-Methoden
	// Wir brauchen separate Clients für Generation, da Models wechseln können
	host string
}

func NewOllamaProvider(embedder *OllamaEmbedder, host string) *OllamaProvider {
	return &OllamaProvider{
		OllamaEmbedder: embedder,
		host:           host,
	}
}

// Generate implementiert das LLMProvider Interface für den Agenten
func (p *OllamaProvider) Generate(ctx context.Context, prompt string, model string) (string, int, error) {
	// Neuen Client für das spezifische Modell erstellen (leichtgewichtig)
	llmClient, err := prov.New(
		prov.WithServerURL(p.host),
		prov.WithModel(model),
	)
	if err != nil {
		return "", 0, fmt.Errorf("failed to create client for model %s: %w", model, err)
	}

	// Generation Options (Temperature niedrig für Code/Planung)
	completion, err := llmClient.Call(ctx, prompt,
		llms.WithTemperature(0.1),
	)
	if err != nil {
		return "", 0, err
	}

	// Token Estimation (einfache Heuristik: chars / 4)
	// Für echte Token-Counts bräuchten wir den Tokenizer des Modells
	tokens := len(completion) / 4

	return completion, tokens, nil
}

// DiscoverAndClassifyModels implementiert die Auto-Discovery
func (p *OllamaProvider) DiscoverAndClassifyModels(ctx context.Context) (*models.ModelClassification, error) {
	// 1. Liste holen
	modelNames, err := p.ListModels(ctx)
	if err != nil {
		return nil, err
	}

	classifier := ollama.NewModelClassifier()
	classification := &models.ModelClassification{
		Embedders:   []models.ModelCapability{},
		Planners:    []models.ModelCapability{},
		Coders:      []models.ModelCapability{},
		Generalists: []models.ModelCapability{},
	}

	// 2. Jedes Modell klassifizieren
	for _, name := range modelNames {
		// Hole Tags (hier vereinfacht nur der Name, idealerweise via API 'show')
		tags := []string{name}
		capability := classifier.ExtractMetadata(name, tags)

		switch capability.Role {
		case models.RoleEmbedder:
			classification.Embedders = append(classification.Embedders, capability)
		case models.RolePlanner:
			classification.Planners = append(classification.Planners, capability)
		case models.RoleCoder:
			classification.Coders = append(classification.Coders, capability)
		default:
			classification.Generalists = append(classification.Generalists, capability)
		}
	}

	// 3. Empfehlungen generieren
	classification.Recommendations = classifier.GenerateRecommendations(classification)

	return classification, nil
}
