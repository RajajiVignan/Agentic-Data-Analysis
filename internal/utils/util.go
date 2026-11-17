package utils

import (
	"os"
	"strings"
)

func QuoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// GetEnv returns the value of the environment variable or fallback if unset.
func GetEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// IsProduction reports whether any of the standard environment markers is
// set to "production".
func IsProduction() bool {
	for _, name := range []string{"APP_ENV", "GO_ENV", "NODE_ENV"} {
		if strings.EqualFold(os.Getenv(name), "production") {
			return true
		}
	}
	return false
}

// NormalizeBearer strips any "Bearer " prefix and surrounding quotes from the token.
func NormalizeBearer(key string) string {
	key = strings.TrimSpace(key)
	key = strings.Trim(key, "\"")
	key = strings.Trim(key, "'")
	key = strings.TrimPrefix(key, "Bearer ")
	key = strings.TrimPrefix(key, "bearer ")
	return key
}

// StripCodeFences removes ```python ... ``` or ``` ... ``` wrappers from LLM output.
func StripCodeFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if idx := strings.Index(s[3:], "\n"); idx >= 0 {
			s = s[3+idx+1:]
		}
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	}
	return s
}
