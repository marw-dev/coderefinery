package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"coderefinery/internal/config"
	"coderefinery/internal/infrastructure/resilience"

	"github.com/tmc/langchaingo/llms/ollama"
)

type OllamaEmbedder struct {
	client *ollama.LLM
	host   string
	model  string
	cb 	   *resilience.CircuitBreaker
}

// NewOllamaEmbedder nutzt jetzt config.LLMConfig
func NewOllamaEmbedder(cfg config.LLMConfig) (*OllamaEmbedder, error) {
	ollamaUrl, err := url.Parse(cfg.Host)
	if err != nil {
		return nil, fmt.Errorf("invalid ollama host: %w", err)
	}

	llm, err := ollama.New(
		ollama.WithServerURL(ollamaUrl.String()),
		ollama.WithModel(cfg.EmbeddingModel),
	)
	if err != nil {
		return nil, err
	}

	return &OllamaEmbedder{
		client: llm,
		host:   cfg.Host,
		model:  cfg.EmbeddingModel,
		cb: resilience.NewProductionCircuitBreaker("ollama-embedder"),
	}, nil
}

func (o *OllamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	result, err := o.cb.Execute(func() (any, error) {
		embeddings, err := o.client.CreateEmbedding(ctx, []string{text})
		if err != nil {
			return nil, err
		}
		if len(embeddings) == 0 || len(embeddings[0]) == 0 {
			return nil, fmt.Errorf("empty embedding returned")
		}
		return embeddings[0], nil
	})

	if err != nil {
		return nil, err
	}

	return result.([]float32), nil
}

func (o *OllamaEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	// Auch Batch Requests schützen
	result, err := o.cb.Execute(func() (any, error) {
		return o.client.CreateEmbedding(ctx, texts)
	})

	if err != nil {
		return nil, err
	}
	return result.([][]float32), nil
}

func (o *OllamaEmbedder) Dimensions() (int, error) {
    resp, err := http.Post(
        o.host + "/api/show",
        "application/json",
        strings.NewReader(fmt.Sprintf(`{"name": "%s"}`, o.model)),
    )
    if err != nil {
        return 0, err
    }
    defer resp.Body.Close()

    var result struct {
        ModelInfo struct {
            EmbeddingLength int `json:"embedding_length"`
        } `json:"model_info"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return 0, err
    }

    return result.ModelInfo.EmbeddingLength, nil
}


// SetModel initialisiert den Client neu mit einem anderen Modell
func (o *OllamaEmbedder) SetModel(newModel string) error {
	ollamaUrl, _ := url.Parse(o.host) // Host ist valide, da im Konstruktor geprüft

	llm, err := ollama.New(
		ollama.WithServerURL(ollamaUrl.String()),
		ollama.WithModel(newModel),
	)
	if err != nil {
		return err
	}

	o.client = llm
	o.model = newModel
	return nil
}

// ListModels holt verfügbare Modelle direkt von der Ollama API
func (o *OllamaEmbedder) ListModels(ctx context.Context) ([]string, error) {
	// Ollama API Endpoint: GET /api/tags
	resp, err := http.Get(o.host + "/api/tags")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ollama at %s: %w", o.host, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama api returned status: %d", resp.StatusCode)
	}

	// JSON Struktur der Ollama Antwort
	type OllamaModelResponse struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	var result OllamaModelResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var models []string
	for _, m := range result.Models {
		models = append(models, m.Name)
	}
	return models, nil
}

// Helper um aktuelles Modell abzufragen
func (o *OllamaEmbedder) GetCurrentModel() string {
	return o.model
}
