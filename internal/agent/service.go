package agent

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

type LLMProvider interface {
	Generate(ctx context.Context, prompt string, model string) (string, int, error)
}

type SearchProvider interface {
	Search(ctx context.Context, query string, topK int) ([]CodeChunk, error)
}

type AgentService struct {
	llm      LLMProvider
	searcher SearchProvider
	analyzer *ComplexityAnalyzer
	logger   *slog.Logger
}

func NewAgentService(llm LLMProvider, searcher SearchProvider, logger *slog.Logger) *AgentService {
	return &AgentService{
		llm:      llm,
		searcher: searcher,
		analyzer: NewComplexityAnalyzer(),
		logger:   logger,
	}
}

func (s *AgentService) Execute(ctx context.Context, projectID string, query string, config PipelineConfig) (*PipelineExecution, error) {
	execution := &PipelineExecution{
		ID:        uuid.New(),
		ProjectID: projectID,
		Stages:    make([]StageResult, 0),
		CreatedAt: time.Now(),
	}
	startTime := time.Now()

	// 1. Complexity Analysis
	analysis := s.analyzer.Analyze(query)
	execution.Complexity = analysis.Complexity

	// Determine Mode
	mode := s.determineMode(analysis.Complexity, config)
	execution.Mode = mode

	execution.Stages = append(execution.Stages, StageResult{
		Stage:    StageComplexityAnalysis,
		Output:   analysis.Reasoning,
		Success:  true,
		Metadata: map[string]any{"mode": mode, "confidence": analysis.Confidence},
	})

	var err error
	switch mode {
	case ModeFastPath:
		err = s.executeFastPath(ctx, query, config, execution)
	case ModeHybrid:
		err = s.executeHybrid(ctx, query, config, execution)
	case ModeFull:
		err = s.executeFull(ctx, query, config, execution)
	}

	execution.TotalDuration = time.Since(startTime)
	execution.Success = err == nil
	return execution, err
}

func (s *AgentService) determineMode(complexity QueryComplexity, config PipelineConfig) ExecutionMode {
	if config.ForcedMode != nil {
		return *config.ForcedMode
	}
	if !config.AutoSelectMode {
		return ModeFull
	}
	switch complexity {
	case ComplexitySimple: return ModeFastPath
	case ComplexityMedium: return ModeHybrid
	default: return ModeFull
	}
}

func (s *AgentService) executeFastPath(ctx context.Context, query string, cfg PipelineConfig, exec *PipelineExecution) error {
	// 1. Retrieval (konsistent mit anderen Modi)
	chunks, err := s.executeRetrieval(ctx, query, cfg.EmbedderModel, 3)
	if err != nil {
		return err
	}

	// 2. Direct Coding (Spezialfall für FastPath, daher keine Methode in stages.go vorhanden)
	// Hier bleibt der direkte Aufruf legitim, oder man ergänzt executeDirectCoding in stages.go
	code, tokens, err := s.llm.Generate(ctx, s.buildDirectCodingPrompt(query, chunks), cfg.CoderModel)
	if err != nil {
		return err
	}

	exec.Stages = append(exec.Stages, StageResult{
		Stage:      StageCoding,
		Output:     code,
		TokensUsed: tokens,
		Success:    true,
	})
	exec.FinalOutput = code
	return nil
}

func (s *AgentService) executeHybrid(ctx context.Context, query string, cfg PipelineConfig, exec *PipelineExecution) error {
	// 1. Retrieval: Nutzt jetzt die dedizierte Stage-Methode
	// Wir reichen das EmbedderModel aus der Config weiter (falls executeRetrieval es braucht)
	chunks, err := s.executeRetrieval(ctx, query, cfg.EmbedderModel, 7)
	if err != nil {
		return err
	}

	// 2. Planning: Erstellt den Plan basierend auf Query + Chunks
	plan, pTokens, err := s.executePlanning(ctx, query, chunks, cfg.PlannerModel)
	if err != nil {
		return err
	}

	// Ergebnis der Planning-Stage speichern
	exec.Stages = append(exec.Stages, StageResult{
		Stage:      StagePlanning,
		Output:     plan,
		TokensUsed: pTokens,
		Success:    true,
	})

	// 3. Coding: Setzt den Plan in Code um
	code, cTokens, err := s.executeCoding(ctx, plan, chunks, cfg.CoderModel)
	if err != nil {
		return err
	}

	// Ergebnis der Coding-Stage speichern
	exec.Stages = append(exec.Stages, StageResult{
		Stage:      StageCoding,
		Output:     code,
		TokensUsed: cTokens,
		Success:    true,
	})

	// Finalen Output setzen
	exec.FinalOutput = code
	return nil
}

func (s *AgentService) executeFull(ctx context.Context, query string, cfg PipelineConfig, exec *PipelineExecution) error {
	// Wiederverwendung des sauberen Hybrid-Flows für die ersten Schritte
	if err := s.executeHybrid(ctx, query, cfg, exec); err != nil {
		return err
	}

	// 4. Validation Stage (nur im Full Mode)
	if cfg.EnableValidation {
		// Wir nutzen executeValidation aus stages.go
		valResult, err := s.executeValidation(ctx, exec.FinalOutput, query, cfg.PlannerModel)

		if err == nil {
			exec.Stages = append(exec.Stages, StageResult{
				Stage:   StageValidation,
				Output:  valResult,
				Success: true,
			})
			// Optional: Man könnte hier entscheiden, ob das Validierungsergebnis den FinalOutput überschreibt
			// oder nur als Anmerkung dient.
		} else {
			// Fehler beim Validieren loggen, aber den Prozess nicht zwingend abbrechen
			s.logger.Warn("Validation stage failed", "error", err)
		}
	}
	return nil
}
