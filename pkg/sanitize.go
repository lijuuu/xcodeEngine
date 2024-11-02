package pkg

import (
	"errors"
	"regexp"
	"strings"
)

type SanitizationError struct {
	Message string
	Details string
}

func (e *SanitizationError) Error() string {
	return e.Message + ": " + e.Details
}

type CodeSanitizer struct {
	maxCodeLength int
}

func NewCodeSanitizer(maxCodeLength int) *CodeSanitizer {
	return &CodeSanitizer{maxCodeLength: maxCodeLength}
}

func (s *CodeSanitizer) SanitizeCode(code, language string) error {
	if len(code) > s.maxCodeLength {
		return &SanitizationError{
			Message: "Code length exceeds maximum limit",
			Details: "Max length allowed is " + string(rune(s.maxCodeLength)),
		}
	}

	// Check system patterns always
	systemPatterns := []string{
		`(?i)(subprocess|exec\.|shell|eval|child_process)`,
		`(?i)(io/ioutil|os\.Open|os\.Create|os\.Remove)`,
		`(?i)(net\.Listen|net\.Dial|http\.|urllib|axios)`,
	}
	if matched, err := matchPatterns(systemPatterns, code); err != nil || matched {
		return &SanitizationError{
			Message: "Prohibited system-level access detected",
			Details: "Code contains restricted system operations",
		}
	}

	var restrictedPatterns []string
	switch language {
	case "python":
		if strings.Contains(code, "import") || strings.Contains(code, "from") {
			restrictedPatterns = []string{
				`^import\s+(?!math|random|datetime|json|re|string|collections|itertools|functools|typing).*$`,
				`^from\s+(?!math|random|datetime|json|re|string|collections|itertools|functools|typing)\s+import.*$`,
			}
		}
		restrictedPatterns = append(restrictedPatterns, []string{
			`__import__`, `globals|locals|vars`, `getattr|setattr|delattr`,
			`pip|setuptools|pkg_resources`,
		}...)
	case "go":
		// Define safe packages that can be imported
		safePackages := []string{
			"fmt",
			"strings",
			"strconv",
			"math",
			"time",
			"encoding/json",
			"errors",
			"sort",
			"regexp",
		}

		// Create a pattern that matches any import that's not in our safe list
		if strings.Contains(code, "import") {
			lines := strings.Split(code, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "import") {
					// Handle single import
					importMatch := regexp.MustCompile(`^import\s+"([^"]+)"`).FindStringSubmatch(line)
					if importMatch != nil {
						pkg := importMatch[1]
						isSafe := false
						for _, safePkg := range safePackages {
							if pkg == safePkg {
								isSafe = true
								break
							}
						}
						if !isSafe {
							return &SanitizationError{
								Message: "Prohibited go code pattern detected",
								Details: "Unauthorized import: " + pkg,
							}
						}
					}
				}
			}
		}

		// Always check for dangerous package usage regardless of imports
		restrictedPatterns = []string{
			`unsafe\.`,
			`reflect\.`,
			`plugin\.`,
			`go/ast`,
			`syscall\.`,
			`debug\.`,
			`runtime\.`,
			`os\.Exit`,
			`panic\(`,
		}
	case "nodejs":
		if strings.Contains(code, "require") || strings.Contains(code, "import") {
			restrictedPatterns = []string{
				`require\(.*\)`, `import\s+.*\s+from`, `import\s*{.*}`,
			}
		}
		restrictedPatterns = append(restrictedPatterns, []string{
			`process`, `global`, `Buffer`,
			`__proto__`, `prototype`, 
			`fs`, `child_process`, 
			`eval`, `Function`, 
			`process\.env`, 
		}...)
	default:
		return errors.New("unsupported language: " + language)
	}

	if len(restrictedPatterns) > 0 {
		if matched, err := matchPatterns(restrictedPatterns, code); err != nil || matched {
			return &SanitizationError{
				Message: "Prohibited " + language + " code pattern detected",
				Details: "Unauthorized module or operation",
			}
		}
	}

	return nil
}

func matchPatterns(patterns []string, code string) (bool, error) {
	for _, pattern := range patterns {
		matched, err := regexp.MatchString(pattern, code)
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}
