package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"local_review/internal/domain"
)

const (
	groqChatEndpoint = "https://api.groq.com/openai/v1/chat/completions"
	groqDefaultModel = "llama-3.3-70b-versatile"
	groqTimeout      = 120 * time.Second
)

// GroqClient sends code review requests to the Groq Chat Completions API.
// It implements domain.LLMClient.
type GroqClient struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewGroqClient creates a new GroqClient with the provided API key.
// model may be left empty to use the default (llama-3.3-70b-versatile).
func NewGroqClient(apiKey, model string) *GroqClient {
	if model == "" {
		model = groqDefaultModel
	}
	return &GroqClient{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: groqTimeout,
		},
	}
}

// --- Request / Response shapes (OpenAI-compatible) ---

type groqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type groqRequest struct {
	Model       string        `json:"model"`
	Messages    []groqMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type groqResponse struct {
	Choices []struct {
		Message groqMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Review implements domain.LLMClient.
// It sends the diff and coding standards to Groq and parses the structured response.
func (c *GroqClient) Review(ctx context.Context, diff string, standards string) (*domain.ReviewReport, error) {
	if diff == "" {
		return &domain.ReviewReport{
			Summary:  "No changes detected.",
			Severity: "INFO",
			Details:  "The provided branches have identical trees.",
		}, nil
	}

	prompt := buildPrompt(diff, standards)

	payload := groqRequest{
		Model: c.model,
		Messages: []groqMessage{
			{
				Role: "system",
				Content: `You are a senior software engineer performing a precise code review.
Your output MUST be valid GitHub-flavoured Markdown.

Structure your response exactly as follows:

## Summary
<One concise sentence describing the overall change.>

## Severity
<Exactly one of: INFO | LOW | MEDIUM | HIGH | CRITICAL>

## Review

For every meaningful finding, create a subsection like:

### <Short finding title>

<Explanation of the issue or praise, referencing the specific lines.>

` + "``` " + `diff
<paste the exact +/- lines from the diff that relate to this finding>
` + "```" + `

> **Suggestion:** <concrete, actionable improvement if applicable>

If there are no significant findings, say so explicitly.
Be specific: always quote the exact changed lines you are commenting on.`,
			},
			{Role: "user", Content: prompt},
		},
		Temperature: 0.2,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("groq: failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, groqChatEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("groq: failed to build HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("groq: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("groq: failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("groq: unexpected status %d: %s", resp.StatusCode, string(rawBody))
	}

	var groqResp groqResponse
	if err := json.Unmarshal(rawBody, &groqResp); err != nil {
		return nil, fmt.Errorf("groq: failed to decode response: %w", err)
	}

	if groqResp.Error != nil {
		return nil, fmt.Errorf("groq API error: %s", groqResp.Error.Message)
	}

	if len(groqResp.Choices) == 0 {
		return nil, fmt.Errorf("groq: no choices returned in response")
	}

	return parseReviewResponse(groqResp.Choices[0].Message.Content), nil
}

// buildPrompt constructs the user prompt from the diff and coding standards.
func buildPrompt(diff, standards string) string {
	var sb strings.Builder
	if standards != "" {
		sb.WriteString("## Coding Standards\n\n")
		sb.WriteString(standards)
		sb.WriteString("\n\n")
	}
	sb.WriteString("## Git Diff to Review\n\n```diff\n")
	sb.WriteString(diff)
	sb.WriteString("\n```\n\n")
	sb.WriteString("Please review the above diff against the coding standards and provide your structured feedback.")
	return sb.String()
}

// parseReviewResponse extracts Summary, Severity, and Details from the markdown
// returned by the LLM. The full markdown is preserved in Details so that the
// caller can render it as-is (e.g. piped to a Markdown viewer or printed raw).
func parseReviewResponse(text string) *domain.ReviewReport {
	report := &domain.ReviewReport{
		Summary:  "Review complete.",
		Severity: "LOW",
		Details:  strings.TrimSpace(text),
	}

	lines := strings.Split(text, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// ## Summary  → next non-empty line is the summary value
		if strings.EqualFold(trimmed, "## Summary") {
			for _, next := range lines[i+1:] {
				if v := strings.TrimSpace(next); v != "" {
					report.Summary = v
					break
				}
			}
			continue
		}

		// ## Severity  → next non-empty line is the severity token
		if strings.EqualFold(trimmed, "## Severity") {
			for _, next := range lines[i+1:] {
				if v := strings.TrimSpace(next); v != "" {
					report.Severity = strings.ToUpper(v)
					break
				}
			}
			continue
		}
	}

	return report
}
