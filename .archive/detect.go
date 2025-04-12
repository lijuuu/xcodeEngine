package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"go.uber.org/zap"
)

// SemgrepOutput represents the JSON output structure from Semgrep
type SemgrepOutput struct {
	Results []struct {
		CheckID string `json:"check_id"`
		Extra   struct {
			Message string `json:"message"`
		} `json:"extra"`
	} `json:"results"`
	Errors []struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"errors"`
}

// getRuleFileForLanguage maps a language to its corresponding Semgrep rule file
func getRuleFileForLanguage(language string) string {
	switch language {
	case "python":
		return "/semgrep-rules/python-rules.yaml"
	case "go":
		return "/semgrep-rules/go-rules.yaml"
	case "javascript":
		return "/semgrep-rules/js-rules.yaml"
	case "cpp":
		return "/semgrep-rules/cpp-rules.yaml"
	default:
		return ""
	}
}

// DetectHarmfulCode analyzes the submitted code using Semgrep to detect harmful patterns
func DetectHarmfulCode(code, language string, maxCodeLength int) error {

	logger, err := zap.NewProduction()
	if err != nil {
		return fmt.Errorf("failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Check code length

	if len(code) > maxCodeLength {
		return &SanitizationError{
			Message: "Code length exceeds maximum limit",
			Details: fmt.Sprintf("Max length allowed is %d", maxCodeLength),
		}
	}

	// Determine the rule file based on the language
	ruleFile := getRuleFileForLanguage(language)
	if ruleFile == "" {
		return errors.New("unsupported language: " + language)
	}

	// Write the code to a temporary file
	tempFile, err := os.CreateTemp("", "code-*.txt")
	if err != nil {
		logger.Error("Failed to create temp file", zap.Error(err))
		return fmt.Errorf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	if _, err := tempFile.WriteString(code); err != nil {
		logger.Error("Failed to write code to temp file", zap.Error(err))
		return fmt.Errorf("failed to write code to temp file: %v", err)
	}
	tempFile.Close()

	// Run Semgrep with the specified rule file
	cmd := exec.Command("semgrep", "--config", ruleFile, "--json", tempFile.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("Semgrep execution failed", zap.Error(err), zap.String("output", string(output)))
		return fmt.Errorf("semgrep execution failed: %v", err)
	}

	// Parse Semgrep's JSON output
	var semgrepResult SemgrepOutput
	if err := json.Unmarshal(output, &semgrepResult); err != nil {
		logger.Error("Failed to parse Semgrep output", zap.Error(err))
		return fmt.Errorf("failed to parse semgrep output: %v", err)
	}

	// Check for Semgrep errors
	if len(semgrepResult.Errors) > 0 {
		errMsg := fmt.Sprintf("Semgrep error: %s", semgrepResult.Errors[0].Message)
		logger.Error("Semgrep reported an error", zap.String("error", errMsg))
		return fmt.Errorf(errMsg)
	}

	// Check if harmful patterns were detected
	if len(semgrepResult.Results) > 0 {
		result := semgrepResult.Results[0]
		return &SanitizationError{
			Message: fmt.Sprintf("Prohibited operation detected: %s", result.CheckID),
			Details: result.Extra.Message,
		}
	}

	return nil
}
