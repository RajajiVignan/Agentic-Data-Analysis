package agent

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Guard validates LLM responses before they reach the UI.
type Guard struct {
	maxNotebookSteps   int
	maxRecommendations int
	forbiddenStrings   []string
}

// NewGuard creates a guard with sensible defaults.
func NewGuard() *Guard {
	return &Guard{
		maxNotebookSteps:   20,
		maxRecommendations: 10,
		forbiddenStrings: []string{
			"SUPABASE_KEY",
			"NVIDIA_API_KEY",
			"API_KEY",
			"PASSWORD",
			"SECRET",
			"-----BEGIN",
		},
	}
}

// ValidateResponse checks an LLM-produced response for safety and shape.
// It returns a list of violations. An empty list means the response is safe.
func (g *Guard) ValidateResponse(resp AnalysisResponse) []string {
	violations := []string{}

	raw, _ := json.Marshal(resp)
	rawStr := string(raw)
	for _, forbidden := range g.forbiddenStrings {
		if strings.Contains(rawStr, forbidden) {
			violations = append(violations, fmt.Sprintf("response may contain sensitive string: %s", forbidden))
		}
	}

	if len(resp.Notebook) > g.maxNotebookSteps {
		violations = append(violations, fmt.Sprintf("notebook has %d steps, max is %d", len(resp.Notebook), g.maxNotebookSteps))
	}

	if len(resp.Dashboard.Recommendations) > g.maxRecommendations {
		violations = append(violations, fmt.Sprintf("too many recommendations: %d", len(resp.Dashboard.Recommendations)))
	}

	return violations
}

// SanitizeForPrompt prepares a string for safe inclusion in an LLM prompt.
func SanitizeForPrompt(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\x00", "")
	s = strings.ReplaceAll(s, "\r", "")
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	return s
}
