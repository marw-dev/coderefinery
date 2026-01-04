package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"coderefinery/internal/config"
)

type OllamaEmbedder struct {
	baseURL string
	model   string
	client  *http.Client
}

func NewOllamaEmbedder(cfg config.OllamaConfig) (*OllamaEmbedder, error) {
	return &OllamaEmbedder{
		baseURL: cfg.BaseURL,
		model:   cfg.Model,
		client:  &http.Client{Timeout: cfg.Timeout},
	}, nil
}

func (e *OllamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := map[string]interface{}{
		"model":  e.model,
		"prompt": text,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", 
		e.baseURL+"/api/embeddings", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var result struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Embedding, nil
}

func (e *OllamaEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
    embeddings := make([][]float32, len(texts))
    
    // Semaphore: Maximal 5 parallele Requests an Ollama, um Überlastung zu vermeiden.
    // Kann man konfigurierbar machen, aber 5 ist ein guter Wert für lokale LLMs.
    maxConcurrency := 5
    sem := make(chan struct{}, maxConcurrency) 
    
    var wg sync.WaitGroup
    var mu sync.Mutex 
    var firstErr error

    for i, text := range texts {
        wg.Add(1)
        
        // Start Goroutine
        go func(idx int, t string) {
            defer wg.Done()

            // Token holen (wartet, wenn Channel voll ist)
            sem <- struct{}{} 
            defer func() { <-sem }() // Token zurückgeben

            // Abbrechen, falls schon ein Fehler passierte
            mu.Lock()
            if firstErr != nil {
                mu.Unlock()
                return
            }
            mu.Unlock()

            // Eigentlicher Request
            emb, err := e.Embed(ctx, t)
            if err != nil {
                mu.Lock()
                if firstErr == nil {
                    firstErr = err
                }
                mu.Unlock()
                return
            }

            // Sicheres Zuweisen (jeder Index ist einzigartig)
            embeddings[idx] = emb
        }(i, text)
    }

    wg.Wait()

    if firstErr != nil {
        return nil, firstErr
    }

    return embeddings, nil
}

func (e *OllamaEmbedder) Dimensions() int {
	return 768 // nomic-embed-text dimensions
}