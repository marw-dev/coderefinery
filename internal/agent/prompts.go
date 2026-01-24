package agent

import (
	"fmt"
	"strings"
)

func (s *AgentService) formatChunks(chunks []CodeChunk) string {
	if len(chunks) == 0 {
		return "[Kein Code gefunden]"
	}
	var sb strings.Builder
	for _, c := range chunks {
		fmt.Fprintf(&sb, "\n--- FILE: %s (Lines %d-%d) ---\n%s\n", c.FilePath, c.LineStart, c.LineEnd, c.Content)
	}
	return sb.String()
}

func (s *AgentService) buildPlanningPrompt(query string, chunks []CodeChunk) string {
	return fmt.Sprintf(`Du bist ein Senior Software Architekt.
AUFGABE: Erstelle einen Implementierungsplan für folgende Anfrage.
ANFRAGE: %s

KONTEXT (Vorhandener Code):
%s

REGELN:
1. Schreibe KEINEN Code. Nur Pseudocode oder Schritte.
2. Analysiere, welche Dateien geändert werden müssen.
3. Plane Edge Cases ein.
4. Strukturiere die Antwort: 1. Analyse, 2. Strategie, 3. Schritte.

Antworte nur mit dem Plan.`, query, s.formatChunks(chunks))
}

func (s *AgentService) buildCodingPrompt(plan string, chunks []CodeChunk) string {
	return fmt.Sprintf(`Du bist ein Senior Developer.
AUFGABE: Implementiere den Code basierend auf diesem Plan.

PLAN:
%s

KONTEXT:
%s

REGELN:
1. Gib NUR den fertigen Code aus (Markdown Format).
2. Keine Erklärungen davor/danach, außer Kommentare im Code.
3. Halte dich strikt an Syntax und Best Practices.
4. Füge Error Handling hinzu.`, plan, s.formatChunks(chunks))
}

func (s *AgentService) buildDirectCodingPrompt(query string, chunks []CodeChunk) string {
	return fmt.Sprintf(`Du bist ein effizienter Coder.
AUFGABE: Löse das Problem direkt mit Code.

ANFRAGE: %s

KONTEXT:
%s

Gib nur den Code zurück.`, query, s.formatChunks(chunks))
}

func (s *AgentService) buildValidationPrompt(code string, query string) string {
	return fmt.Sprintf(`Du bist Code Reviewer.
ANFRAGE WAR: %s
GENERIERTER CODE:
%s

Bewerte den Code:
- Erfüllt er die Anfrage?
- Gibt es Bugs?
Antworte kurz mit [PASS] oder [FAIL] + Begründung.`, query, code)
}
