package llm

import (
	"context"

	"local_review/internal/domain"
)

// MockClient provides a simulated LLM response for code reviews.
// It implements domain.LLMClient.
type MockClient struct{}

// NewMockClient creates a new mock LLM client.
func NewMockClient() *MockClient {
	return &MockClient{}
}

// Review simulates sending the diff to an LLM and parsing a response.
// In a production environment, this would format an API request,
// manage retries, handle auth, and decode the JSON response.
func (c *MockClient) Review(ctx context.Context, diff string, standards string) (*domain.ReviewReport, error) {
	if diff == "" {
		return &domain.ReviewReport{
			Summary:  "No changes detected.",
			Severity: "INFO",
			Details:  "The provided branches have identical trees.",
		}, nil
	}

	return &domain.ReviewReport{
		Summary:  "Mock Review Complete",
		Severity: "LOW",
		Details: "This is a simulated review. The static checks passed successfully.\n" +
			"In a full integration, this section would contain semantically reasoned feedback based on the CODING_STANDARDS.",
	}, nil
}
