package agent

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

type QueryComplexity int

const (
	ComplexitySimple  QueryComplexity = iota // 1-2s
	ComplexityMedium                         // 5-8s
	ComplexityComplex                        // 10-15s
)

type ComplexityAnalyzer struct {
	simplePatterns    []*regexp.Regexp
	complexPatterns   []*regexp.Regexp
	complexKeywords   []string
	architectKeywords []string
}

func NewComplexityAnalyzer() *ComplexityAnalyzer {
	return &ComplexityAnalyzer{
		simplePatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)^(fix|correct|change)\s+\w+\s+(typo|spelling|comment)`),
			regexp.MustCompile(`(?i)^add\s+a?\s?(comment|log|print)`),
		},
		complexPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(refactor|redesign|restructure|rewrite)`),
			regexp.MustCompile(`(?i)(implement|create|build|develop)\s+(a|an|the)\s+\w+`),
		},
		complexKeywords: []string{
			"architecture", "pattern", "design", "security", "optimization",
		},
		architectKeywords: []string{
			"dependency injection", "clean architecture", "CQRS", "event sourcing",
		},
	}
}

type ComplexityAnalysis struct {
	Complexity QueryComplexity `json:"complexity"`
	Confidence float64         `json:"confidence"`
	Indicators []string        `json:"indicators"`
	Reasoning  string          `json:"reasoning"`
}

func (a *ComplexityAnalyzer) Analyze(query string) ComplexityAnalysis {
	queryLower := strings.ToLower(query)
	indicators := []string{}
	score := 0.0

	// Simple Indicators
	if len(query) < 50 {
		score -= 2.0
		indicators = append(indicators, "Very short query")
	}
	for _, pattern := range a.simplePatterns {
		if pattern.MatchString(query) {
			score -= 3.0
			indicators = append(indicators, "Simple action pattern")
			break
		}
	}

	// Complex Indicators
	if len(query) > 150 {
		score += 2.0
		indicators = append(indicators, "Long detailed query")
	}
	for _, pattern := range a.complexPatterns {
		if pattern.MatchString(query) {
			score += 3.0
			indicators = append(indicators, "Complex action pattern")
			break
		}
	}
	for _, kw := range a.complexKeywords {
		if strings.Contains(queryLower, kw) {
			score += 1.0
			indicators = append(indicators, fmt.Sprintf("Keyword: %s", kw))
		}
	}
	for _, kw := range a.architectKeywords {
		if strings.Contains(queryLower, kw) {
			score += 4.0
			indicators = append(indicators, "Architectural request")
			break
		}
	}

	// Classification
	var complexity QueryComplexity
	var confidence float64
	var reasoning string

	if score <= -2.0 {
		complexity = ComplexitySimple
		confidence = math.Min(0.9, 0.6+math.Abs(score)*0.1)
		reasoning = "Simple single-action task"
	} else if score >= 3.0 {
		complexity = ComplexityComplex
		confidence = math.Min(0.95, 0.6+score*0.1)
		reasoning = "Complex architectural task"
	} else {
		complexity = ComplexityMedium
		confidence = 0.7
		reasoning = "Standard coding task"
	}

	return ComplexityAnalysis{
		Complexity: complexity,
		Confidence: confidence,
		Indicators: indicators,
		Reasoning:  reasoning,
	}
}
