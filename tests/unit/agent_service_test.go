package unit

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	"coderefinery/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Mocks ---

type MockLLM struct {
	mock.Mock
}

func (m *MockLLM) Generate(ctx context.Context, prompt string, model string) (string, int, error) {
	args := m.Called(ctx, prompt, model)
	return args.String(0), args.Int(1), args.Error(2)
}

type MockSearcher struct {
	mock.Mock
}

func (m *MockSearcher) Search(ctx context.Context, query string, topK int) ([]agent.CodeChunk, error) {
	args := m.Called(ctx, query, topK)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]agent.CodeChunk), args.Error(1)
}

// --- Tests ---

func TestAgentService_Execute_Hybrid_HappyPath(t *testing.T) {
	// Setup
	mockLLM := new(MockLLM)
	mockSearcher := new(MockSearcher)
	// Logger auf Discard setzen, damit der Test-Output sauber bleibt (oder os.Stdout für Debug)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	service := agent.NewAgentService(mockLLM, mockSearcher, logger)
	ctx := context.Background()

	query := "Add structured logging to the auth service"

	hybridMode := agent.ModeHybrid

	config := agent.PipelineConfig{
		EmbedderModel:    "nomic-embed-text",
		PlannerModel:     "llama3",
		CoderModel:       "codellama",
		ForcedMode:       &hybridMode,
		EnableValidation: false,
	}

	mockChunks := []agent.CodeChunk{
		{FilePath: "auth.go", Content: "func Login() {}", Score: 0.9},
	}

	// 1. Expect Search (Retrieval Stage) -> Hybrid nutzt limit 7
	mockSearcher.On("Search", ctx, query, 7).Return(mockChunks, nil)

	// 2. Expect Planning
	mockLLM.On("Generate", ctx, mock.MatchedBy(func(prompt string) bool {
		return strings.Contains(prompt, "Senior Software Architekt")
	}), "llama3").Return("PLAN: 1. Add import. 2. Init logger.", 50, nil)

	// 3. Expect Coding
	mockLLM.On("Generate", ctx, mock.MatchedBy(func(prompt string) bool {
		return strings.Contains(prompt, "Senior Developer")
	}), "codellama").Return("func Login() { log.Info(...) }", 100, nil)

	// Ausführung
	result, err := service.Execute(ctx, "test-project", query, config)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, agent.ModeHybrid, result.Mode)
	assert.Equal(t, "func Login() { log.Info(...) }", result.FinalOutput)

	// Hybrid Mode hat 3 Stages im Result array (Complexity, Planning, Coding)
	assert.Len(t, result.Stages, 3)
	assert.Equal(t, agent.StagePlanning, result.Stages[1].Stage)
	assert.Equal(t, agent.StageCoding, result.Stages[2].Stage)

	mockLLM.AssertExpectations(t)
	mockSearcher.AssertExpectations(t)
}

func TestAgentService_Execute_FastPath(t *testing.T) {
	mockLLM := new(MockLLM)
	mockSearcher := new(MockSearcher)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	service := agent.NewAgentService(mockLLM, mockSearcher, logger)
	ctx := context.Background()

	query := "fix typo in comment"

	// Hier verlassen wir uns auf AutoSelectMode, da die Query eindeutig "Simple" ist
	config := agent.PipelineConfig{
		EmbedderModel:  "nomic",
		CoderModel:     "codellama",
		AutoSelectMode: true,
	}

	// FastPath nutzt limit 3
	mockSearcher.On("Search", ctx, query, 3).Return([]agent.CodeChunk{}, nil)

	// Direct Coding
	mockLLM.On("Generate", ctx, mock.MatchedBy(func(prompt string) bool {
		return strings.Contains(prompt, "effizienter Coder")
	}), "codellama").Return("Fixed typo", 10, nil)

	result, err := service.Execute(ctx, "proj-1", query, config)

	assert.NoError(t, err)
	assert.Equal(t, agent.ModeFastPath, result.Mode)
	assert.Len(t, result.Stages, 2) // Complexity + Coding
}

func TestAgentService_Execute_Full_WithValidation(t *testing.T) {
	mockLLM := new(MockLLM)
	mockSearcher := new(MockSearcher)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	service := agent.NewAgentService(mockLLM, mockSearcher, logger)
	ctx := context.Background()

	// "Architecture" keyword erzwingt Complexity -> Full Mode
	query := "Refactor the entire Clean Architecture dependency injection"

	config := agent.PipelineConfig{
		PlannerModel:     "gpt-4",
		CoderModel:       "gpt-4",
		AutoSelectMode:   true,
		EnableValidation: true,
	}

	mockSearcher.On("Search", ctx, query, 7).Return([]agent.CodeChunk{}, nil)

	// 1. Plan
	mockLLM.On("Generate", ctx, mock.MatchedBy(func(p string) bool {
		return strings.Contains(p, "Architekt")
	}), "gpt-4").Return("Complex Plan", 100, nil)

	// 2. Code
	mockLLM.On("Generate", ctx, mock.MatchedBy(func(p string) bool {
		return strings.Contains(p, "Developer")
	}), "gpt-4").Return("Complex Code", 500, nil)

	// 3. Validation
	mockLLM.On("Generate", ctx, mock.MatchedBy(func(p string) bool {
		return strings.Contains(p, "Reviewer")
	}), "gpt-4").Return("[PASS] Looks good", 50, nil)

	result, err := service.Execute(ctx, "proj-1", query, config)

	assert.NoError(t, err)
	assert.Equal(t, agent.ModeFull, result.Mode)
	assert.Len(t, result.Stages, 4) // Complexity, Planning, Coding, Validation
}
