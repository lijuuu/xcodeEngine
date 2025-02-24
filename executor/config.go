package executor

import (
	"fmt"
	"strings"
	"time"
)

// LanguageConfig defines execution settings for a language
type LanguageConfig struct {
	Timeout time.Duration
	Args    func(code string) []string
}

// languageConfigs holds execution configurations for supported languages
var languageConfigs = map[string]LanguageConfig{
	"go": {
		Timeout: 10 * time.Second,
		Args: func(code string) []string {
			return []string{"sh", "-c", fmt.Sprintf(`echo '%s' > /app/temp/code.go && go run /app/temp/code.go`,
				strings.ReplaceAll(code, "'", "'\\''"))}
		},
	},
	"js": {
		Timeout: 10 * time.Second,
		Args: func(code string) []string {
			return []string{"sh", "-c", fmt.Sprintf(`echo '%s' > /app/temp/code.js && node /app/temp/code.js`,
				strings.ReplaceAll(code, "'", "'\\''"))}
		},
	},
	"python": {
		Timeout: 10 * time.Second,
		Args: func(code string) []string {
			return []string{"sh", "-c", fmt.Sprintf(`echo '%s' > /app/temp/code.py && python3 /app/temp/code.py`,
				strings.ReplaceAll(code, "'", "'\\''"))}
		},
	},
	"cpp": {
		Timeout: 10 * time.Second,
		Args: func(code string) []string {
			return []string{"sh", "-c", fmt.Sprintf(`echo '%s' > /app/temp/code.cpp && g++ -o /app/temp/exe /app/temp/code.cpp && /app/temp/exe`,
				strings.ReplaceAll(code, "'", "'\\''"))}
		},
	},
}

// GetLanguageConfig retrieves the configuration for a given language
func GetLanguageConfig(language string) (LanguageConfig, bool) {
	config, ok := languageConfigs[language]
	return config, ok
}