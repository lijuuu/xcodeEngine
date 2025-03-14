package internal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const TogetherAPIURL = "https://api.together.xyz/v1/completions"

type TogetherRequest struct {
	Model       string  `json:"model"`
	Prompt      string  `json:"prompt"`
	MaxTokens   int     `json:"max_tokens"`
	Temperature float64 `json:"temperature"`
}

type TogetherResponse struct {
	Choices []struct {
		Text string `json:"text"`
	} `json:"choices"`
}

// Calls Together AI API for code analysis
func CallLLMAPI(code, language string, TOGETHER_API_KEY string) (bool, string, error) {

	timeStart := time.Now()

	prompt := "Analyze this " + language + " code for bad practices, resource usage that is not efficient, and security issues:\n\n" + code + "\n\nProvide a concise analysis."

	requestBody, err := json.Marshal(TogetherRequest{
		Model:       "meta-llama/Llama-3.3-70B-Instruct-Turbo", // Free-tier model (check Together AI docs for latest)
		Prompt:      prompt,
		MaxTokens:   500,
		Temperature: 0.7,
	})
	if err != nil {
		return false, "", err
	}

	req, err := http.NewRequest("POST", TogetherAPIURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return false, "", err
	}

	req.Header.Set("Authorization", "Bearer "+TOGETHER_API_KEY)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return false, "", errors.New("Together AI API call failed with status: " + resp.Status)
	}

	var togetherResp TogetherResponse
	if err := json.NewDecoder(resp.Body).Decode(&togetherResp); err != nil {
		return false, "", err
	}

	if len(togetherResp.Choices) == 0 {
		return false, "", errors.New("empty response from Together AI")
	}

	analysis := togetherResp.Choices[0].Text
	if containsBadPractices(analysis) {
		return false, analysis, nil
	}

	timeTaken := time.Since(timeStart)
	fmt.Println("Time taken to analyze code: ", timeTaken)

	return true, "", nil
}

// Simple function to check if response contains "bad practice" or "security issue"
func containsBadPractices(response string) bool {
	responseLower := strings.ToLower(response)
	return strings.Contains(responseLower, "bad practice") ||
		strings.Contains(responseLower, "security issue") ||
		strings.Contains(responseLower, "vulnerability")
}
