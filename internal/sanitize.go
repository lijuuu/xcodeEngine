package internal

import (
	"errors"
	"fmt"
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

func SanitizeCode(code, language string, maxCodeLength int) error {
	if len(code) > maxCodeLength {
		return &SanitizationError{
			Message: "Code length exceeds maximum limit",
			Details: fmt.Sprintf("Max length allowed is %d", maxCodeLength),
		}
	}

	// Check for obvious dangerous operations regardless of language
	dangerousPatterns := []string{
		`(?i)(os\.Remove|os\.RemoveAll)`,
		`(?i)(net\.Listen|net\.Dial)`,
		`(?i)(exec\.Command)`,
		`(?i)(syscall\.Exec)`,
	}

	if matched, err := matchPatterns(dangerousPatterns, code); err != nil || matched {
		return &SanitizationError{
			Message: "Prohibited dangerous operation detected",
			Details: "Code contains potentially harmful system operations",
		}
	}

	switch language {
	case "python":
		return sanitizePython(code)
	case "go":
		return sanitizeGo(code)
	case "js":
		return sanitizeJS(code)
	case "cpp":
		return sanitizeCPP(code)
	default:
		return errors.New("unsupported language: " + language)
	}
}

func sanitizePython(code string) error {
	// Blacklist clearly dangerous modules
	dangerousModules := []string{
		`import\s+os\s*$`,
		`from\s+os\s+import\s+(system|popen|execl|execle|execlp|execv|execve|execvp|execvpe|spawn)`,
		`import\s+subprocess`,
		`import\s+shutil`,
		`import\s+ctypes`,
		`import\s+sys`,
		`__import__\(['"]os['"]`,
	}

	if matched, err := matchPatterns(dangerousModules, code); err != nil || matched {
		return &SanitizationError{
			Message: "Prohibited Python module detected",
			Details: "Code attempts to import restricted system modules",
		}
	}

	// Look for potentially dangerous operations
	dangerousOps := []string{
		`open\(.+,\s*['"]w['"]`, // Writing to files
		`__import__\(`,
		`eval\(`,
		`exec\(`,
		`globals\(\)\.`,
		`locals\(\)\.`,
	}

	if matched, err := matchPatterns(dangerousOps, code); err != nil || matched {
		return &SanitizationError{
			Message: "Prohibited Python operation detected",
			Details: "Code attempts to perform potentially unsafe operations",
		}
	}

	return nil
}

func sanitizeGo(code string) error {
	// Define packages that require special scrutiny
	restrictedPackages := map[string]bool{
		"os":        true,
		"syscall":   true,
		"unsafe":    true,
		"net":       true,
		"net/http":  true,
		"io/ioutil": true,
		"plugin":    true,
		"runtime":   true,
	}

	// Check imports
	importRegex := regexp.MustCompile(`import\s+(?:[\w\d_]+\s+)?"([^"]+)"`)
	matches := importRegex.FindAllStringSubmatch(code, -1)

	for _, match := range matches {
		if len(match) > 1 {
			pkg := match[1]
			pkgBase := strings.Split(pkg, "/")[0]

			if restrictedPackages[pkg] || restrictedPackages[pkgBase] {
				// Allow specific safe functions from restricted packages
				if pkg == "os" && !containsOSDangerousFunctions(code) {
					// We're explicitly allowing certain os functions
					continue
				}

				return &SanitizationError{
					Message: "Prohibited Go package detected",
					Details: "Package not allowed: " + pkg,
				}
			}
		}
	}

	// Check for goroutine overuse
	goroutineCount := len(regexp.MustCompile(`go\s+func`).FindAllString(code, -1))
	if goroutineCount > 5 {
		return &SanitizationError{
			Message: "Excessive goroutine usage",
			Details: fmt.Sprintf("Found %d goroutines, maximum allowed is 5", goroutineCount),
		}
	}

	// Check for infinite loops
	infiniteLoopPatterns := []string{
		`for\s*{`,
		`for\s+true\s*{`,
		`for\s+;\s*;\s*{`,
	}

	// Only check for these if there's no indication of a break statement
	if !strings.Contains(code, "break") {
		if matched, err := matchPatterns(infiniteLoopPatterns, code); err != nil || matched {
			return &SanitizationError{
				Message: "Potential infinite loop detected",
				Details: "Make sure loops have proper exit conditions",
			}
		}
	}

	return nil
}

// Helper function to check if code uses dangerous OS functions
func containsOSDangerousFunctions(code string) bool {
	dangerousFuncs := []string{
		`os\.Remove`, `os\.RemoveAll`,
		`os\.Chdir`, `os\.Chmod`,
		`os\.Chown`, `os\.Exit`,
		`os\.Link`, `os\.MkdirAll`,
		`os\.Rename`, `os\.Symlink`,
	}

	for _, pattern := range dangerousFuncs {
		if matched, _ := regexp.MatchString(pattern, code); matched {
			return true
		}
	}

	// Allow these OS functions (they're generally safe)
	// os.Getenv, os.Environ, os.Hostname, os.UserHomeDir, etc.

	return false
}

func sanitizeJS(code string) error {
	// Blacklist dangerous imports/requires
	dangerousModules := []string{
		`require\(['"]fs['"]`,
		`require\(['"]child_process['"]`,
		`require\(['"]http['"]`,
		`require\(['"]https['"]`,
		`require\(['"]os['"]`,
		`import\s+.*\s+from\s+['"]fs['"]`,
		`import\s+.*\s+from\s+['"]child_process['"]`,
	}

	if matched, err := matchPatterns(dangerousModules, code); err != nil || matched {
		return &SanitizationError{
			Message: "Prohibited JS module detected",
			Details: "Code attempts to import restricted system modules",
		}
	}

	// Check for dangerous operations
	dangerousOps := []string{
		`process\.exit`,
		`eval\(`,
		`Function\(.*\)`,
		`new Function`,
		`window\.`,
		`document\.`,
		`localStorage`,
		`sessionStorage`,
		`indexedDB`,
		`WebSocket`,
	}

	if matched, err := matchPatterns(dangerousOps, code); err != nil || matched {
		return &SanitizationError{
			Message: "Prohibited JS operation detected",
			Details: "Code attempts to perform potentially unsafe operations",
		}
	}

	return nil
}

func sanitizeCPP(code string) error {
	// Blacklist potentially dangerous operations or imports
	dangerousPatterns := []string{
		`system\(`,        // Disallow system calls
		`exec\(`,          // Disallow exec calls
		`fork\(`,          // Disallow forking processes
		`popen\(`,         // Disallow opening processes
		`delete\s+.*\s+;`, // Disallow dynamic memory deletion
		`new\s+.*\s*;`,    // Disallow dynamic memory allocation
		`std::system`,     // Disallow system calls in std namespace
	}

	if matched, err := matchPatterns(dangerousPatterns, code); err != nil || matched {
		return &SanitizationError{
			Message: "Prohibited C++ operation detected",
			Details: "Code attempts to perform potentially unsafe operations",
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
