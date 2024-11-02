package pkg

import (
	"errors"
	"regexp"
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

	
	systemPatterns := []string{
		`(?i)(os\.|subprocess|exec|system|shell|eval|child_process)`,
		`(?i)(os\..*file|fs\.|io/ioutil|open|read|write|file)`,
		`(?i)(http|net|socket|request|fetch|urllib|axios)`,
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
		restrictedPatterns = []string{
			`^import\s+(?!math|random|datetime|json|re|string|collections|itertools|functools|typing).*$`,
			`^from\s+(?!math|random|datetime|json|re|string|collections|itertools|functools|typing)\s+import.*$`,
			`__import__`, `globals|locals|vars`, `getattr|setattr|delattr`,
			`pip|setuptools|pkg_resources`,
		}
	case "go":
		restrictedPatterns = []string{
			`import\s+\([^)]*\)`, `import\s+"(?!fmt|strings|strconv|math|time|encoding/json|errors)"`,
			`unsafe`, `reflect`, `plugin`, `go/ast`,
		}
	case "nodejs":
		restrictedPatterns = []string{
			`require\(.*\)`, `import\s+.*\s+from`, `import\s*{.*}`,
			`process`, `global`, `Buffer`, `__dirname`, `__filename`,
		}
	default:
		return errors.New("unsupported language: " + language)
	}

	if matched, err := matchPatterns(restrictedPatterns, code); err != nil || matched {
		return &SanitizationError{
			Message: "Prohibited " + language + " code pattern detected",
			Details: "Unauthorized module or operation",
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
