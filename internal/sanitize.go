package internal

import (
	"errors"
	"fmt"
	"regexp"
)

// SanitizationError represents an error during code sanitization
type SanitizationError struct {
	Message string
	Details string
}

func (e *SanitizationError) Error() string {
	return e.Message + ": " + e.Details
}

// PatternCategory represents a category of dangerous patterns
type PatternCategory struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Patterns    []string `json:"patterns"`
}

// LanguagePatterns holds all dangerous patterns for a specific language
type LanguagePatterns struct {
	Common   []PatternCategory            `json:"common"`   // Common dangerous patterns
	Language map[string][]PatternCategory `json:"language"` // Language-specific patterns
}

// Simplified pattern matching functions
func SanitizeCode(code, language string, maxCodeLength int) error {
	if len(code) > maxCodeLength {
		return &SanitizationError{
			Message: "Code length exceeds maximum limit",
			Details: fmt.Sprintf("Max length allowed is %d", maxCodeLength),
		}
	}

	// Check common patterns first
	for _, category := range DangerousPatterns.Common {
		if matched := hasMatchingPattern(category.Patterns, code); matched {
			return &SanitizationError{
				Message: fmt.Sprintf("Prohibited operation detected: %s", category.Name),
				Details: category.Description,
			}
		}
	}

	// Check language-specific patterns
	langPatterns, ok := DangerousPatterns.Language[language]
	if !ok {
		return errors.New("unsupported language: " + language)
	}

	for _, category := range langPatterns {
		if matched := hasMatchingPattern(category.Patterns, code); matched {
			return &SanitizationError{
				Message: fmt.Sprintf("Prohibited %s operation detected: %s", language, category.Name),
				Details: category.Description,
			}
		}
	}

	return nil
}


// Simplified pattern matching function
func hasMatchingPattern(patterns []string, code string) bool {
	for _, pattern := range patterns {
		if matched, err := regexp.MatchString(pattern, code); err == nil && matched {
			return true
		}
	}
	return false
}

// DangerousPatterns contains all the patterns as a struct constant
var DangerousPatterns = LanguagePatterns{
	Common: []PatternCategory{
		{
			Name:        "systemOperations",
			Description: "Dangerous system operations",
			Patterns: []string{
				"(?i)(os\\.Remove|os\\.RemoveAll)",
				"(?i)(net\\.Listen|net\\.Dial)",
				"(?i)(exec\\.Command)",
				"(?i)(syscall\\.Exec)",
			},
		},
		{
			Name:        "codeExecution",
			Description: "Dynamic code execution",
			Patterns: []string{
				"eval\\(",
				"exec\\(",
			},
		},
		{
			Name:        "resourceDepletion",
			Description: "Resource depletion attacks",
			Patterns: []string{
				"(?i)while\\s*\\(\\s*true\\s*\\)",
				"(?i)while\\s*\\(\\s*1\\s*\\)",
				"(?i)for\\s*\\(\\s*;;\\s*\\)",
				"(?i)for\\s*\\(;\\s*true\\s*;\\)",
				"(?i)\\.repeat\\s*\\(\\s*Infinity\\s*\\)",
				"\\[\\s*\\.\\.\\.Array\\s*\\(\\s*1e\\d+\\s*\\)\\s*\\]",
				"Array\\s*\\(\\s*1e\\d+\\s*\\)",
				"BigInt\\s*\\(\\s*Number\\.MAX_SAFE_INTEGER\\s*\\)\\s*\\*\\s*BigInt",
				"(?i)setTimeout\\s*\\(\\s*function\\s*\\(\\s*\\)\\s*{\\s*while\\s*\\(\\s*true\\s*\\)",
			},
		},
		{
			Name:        "forkBombs",
			Description: "Fork bomb attacks",
			Patterns: []string{
				"(?i)while\\s*\\(\\s*true\\s*\\)\\s*{\\s*fork\\s*\\(\\s*\\)",
				"(?i)for\\s*\\(;;\\)\\s*{\\s*fork\\s*\\(\\s*\\)",
				":\\s*\\(\\)\\s*{\\s*:\\|:\\s*&\\s*}\\s*;\\s*:",  // Bash fork bomb
				"define\\s+f\\s+\\(\\)\\s+\\(f\\)&\\s*f",         // Lisp fork bomb
				"(?i)while\\s+1;\\s+do\\s+sh\\s+-c\\s+\"\\$0\\s+&\"",
				"Process\\.fork\\(\\)",
				"cluster\\.fork\\(\\)",
				"multiprocessing\\.Process",
				"pthread_create",
			},
		},
	},
	Language: map[string][]PatternCategory{
		"python": {
			{
				Name:        "dangerousModules",
				Description: "Dangerous Python modules",
				Patterns: []string{
					"import\\s+os\\s*$",
					"from\\s+os\\s+import\\s+(system|popen|execl|execle|execlp|execv|execve|execvp|execvpe|spawn)",
					"import\\s+subprocess",
					"import\\s+shutil",
					"import\\s+ctypes",
					"import\\s+sys",
					"__import__\\(['\"]os['\"]",
				},
			},
			{
				Name:        "dangerousOperations",
				Description: "Dangerous Python operations",
				Patterns: []string{
					"open\\(.+,\\s*['\"]w['\"]",
					"__import__\\(",
					"globals\\(\\)\\.",
					"locals\\(\\)\\.",
					"os\\.system\\(",
					"os\\.exec\\(",
					"subprocess\\.Popen\\(",
					"os\\.fork\\(",
					"threading\\.Thread\\s*\\(.*bomb\\(\\)",
					"for\\s*\\(.*\\s*os\\.fork\\(\\)",
					"while\\s*True\\s*:\\s*os\\.fork\\(\\)",
				},
			},
			{
				Name:        "pythonResourceDepletion",
				Description: "Python resource depletion attacks",
				Patterns: []string{
					"while\\s+True\\s*:",
					"[[]\\s*0\\s*\\]\\s*\\*\\s*10\\*\\*\\d+",
					"range\\s*\\(\\s*10\\s*\\*\\*\\s*\\d{2,}\\s*\\)",
					"'\\s*'\\s*\\.join\\s*\\(\\s*\\[\\s*'A'\\s*\\]\\s*\\*\\s*10\\*\\*\\d+\\s*\\)",
					"multiprocessing\\.Pool\\s*\\(\\s*processes\\s*=\\s*\\d{3,}\\s*\\)",
					"threading\\.Thread\\s*\\(\\s*target\\s*=\\s*.+\\s*\\)\\s*\\.start\\s*\\(\\s*\\)",
					"\\{\\s*\\.\\*\\s*\\.\\*\\s*\\.\\*\\s*\\.\\*\\s*\\}",  // Regex denial of service
				},
			},
		},
		"go": {
			{
				Name:        "infiniteLoops",
				Description: "Potential infinite loops",
				Patterns: []string{
					"for\\s*{",
					"for\\s+true\\s*{",
					"for\\s+;\\s*;\\s*{",
				},
			},
			{
				Name:        "dangerousOsFunctions",
				Description: "Dangerous OS functions",
				Patterns: []string{
					"os\\.Remove", "os\\.RemoveAll",
					"os\\.Chdir", "os\\.Chmod",
					"os\\.Chown", "os\\.Exit",
					"os\\.Link", "os\\.MkdirAll",
					"os\\.Rename", "os\\.Symlink",
				},
			},
			{
				Name:        "goResourceDepletion",
				Description: "Go resource depletion attacks",
				Patterns: []string{
					"make\\s*\\(\\s*\\[\\]\\w+\\s*,\\s*\\d{8,}\\s*\\)",
					"go\\s+func\\s*\\(\\s*\\)\\s*{\\s*for\\s*{",
					"for\\s*\\i\\s*:=\\s*0\\s*;\\s*;\\s*i\\+\\+",
					"runtime\\.GOMAXPROCS\\s*\\(\\s*\\d{3,}\\s*\\)",
					"len\\s*\\(\\s*make\\s*\\(\\s*\\[\\]byte\\s*,\\s*1<<\\d{2,}\\s*\\)\\s*\\)",
				},
			},
		},
		"js": {
			{
				Name:        "dangerousModules",
				Description: "Dangerous JS modules",
				Patterns: []string{
					"require\\(['\"]fs['\"]",
					"require\\(['\"]child_process['\"]",
					"require\\(['\"]http['\"]",
					"require\\(['\"]https['\"]",
					"require\\(['\"]os['\"]",
					"import\\s+.*\\s+from\\s+['\"]fs['\"]",
					"import\\s+.*\\s+from\\s+['\"]child_process['\"]",
				},
			},
			{
				Name:        "dangerousOperations",
				Description: "Dangerous JS operations",
				Patterns: []string{
					"process\\.exit",
					"Function\\(.*\\)",
					"new Function",
					"window\\.",
					"document\\.",
					"localStorage",
					"sessionStorage",
					"indexedDB",
					"WebSocket",
				},
			},
			{
				Name:        "jsResourceDepletion",
				Description: "JavaScript resource depletion attacks",
				Patterns: []string{
					"while\\s*\\(\\s*true\\s*\\)",
					"for\\s*\\(\\s*;;\\s*\\)",
					"setTimeout\\s*\\(\\s*function\\s*\\(\\s*\\)\\s*{\\s*location\\.reload\\s*\\(\\s*\\)",
					"\\.repeat\\s*\\(\\s*1e\\d+\\s*\\)",
					"Array\\s*\\(\\s*1e\\d+\\s*\\)",
					"new\\s+Array\\s*\\(\\s*1e\\d+\\s*\\)",
					"\\[\\s*\\.\\.\\.Array\\s*\\(\\s*1e\\d+\\s*\\)\\s*\\]",
					"(?i)\\(\\+\\[\\]\\+\\[\\]\\+\\[\\]\\+\\[\\]\\+\\[\\]\\+\\[\\]\\+\\[\\]",  
				},
			},
		},
		"cpp": {
			{
				Name:        "dangerousOperations",
				Description: "Dangerous C++ operations",
				Patterns: []string{
					"system\\(",
					"exec\\(",
					"fork\\(",
					"popen\\(",
					"delete\\s+.*\\s+;",
					"new\\s+.*\\s*;",
					"std::system",
				},
			},
			{
				Name:        "cppResourceDepletion",
				Description: "C++ resource depletion attacks",
				Patterns: []string{
					"while\\s*\\(\\s*true\\s*\\)",
					"for\\s*\\(\\s*;;\\s*\\)",
					"malloc\\s*\\(\\s*UINT_MAX\\s*\\)",
					"calloc\\s*\\(\\s*UINT_MAX",
					"new\\s+char\\s*\\[\\s*UINT_MAX\\s*\\]",
					"std::vector<\\w+>\\s*\\(\\s*\\d{9,}\\s*\\)",
					"std::thread\\s*\\(\\s*\\[\\]\\s*\\(\\s*\\)\\s*{\\s*while\\s*\\(\\s*true\\s*\\)",
					"#include\\s*<fork.h>",
				},
			},
		},
	},
}