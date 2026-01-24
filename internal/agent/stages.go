package agent

import (
	"context"
)

// executeRetrieval holt relevanten Code
func (s *AgentService) executeRetrieval(ctx context.Context, query string, embedderModel string, topK int) ([]CodeChunk, error) {
	// Hinweis: In einem echten System müsste hier der 'searcher' verwendet werden,
	// der evtl. die Query vorher noch embedden muss.
	// Wir nehmen an s.searcher abstrahiert das.
	chunks, err := s.searcher.Search(ctx, query, topK)
	if err != nil {
		return nil, err
	}
	return chunks, nil
}

// executePlanning erstellt den Plan
func (s *AgentService) executePlanning(ctx context.Context, query string, chunks []CodeChunk, plannerModel string) (string, int, error) {
	prompt := s.buildPlanningPrompt(query, chunks)
	return s.llm.Generate(ctx, prompt, plannerModel)
}

// executeCoding setzt den Plan um
func (s *AgentService) executeCoding(ctx context.Context, plan string, chunks []CodeChunk, coderModel string) (string, int, error) {
	prompt := s.buildCodingPrompt(plan, chunks)
	return s.llm.Generate(ctx, prompt, coderModel)
}

// executeValidation prüft das Ergebnis
func (s *AgentService) executeValidation(ctx context.Context, code string, originalQuery string, validatorModel string) (string, error) {
	prompt := s.buildValidationPrompt(code, originalQuery)
	res, _, err := s.llm.Generate(ctx, prompt, validatorModel)
	return res, err
}
