package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Client provides a thin wrapper over the git executable.
// It implements domain.GitProvider to abstract the specifics of git commands.
type Client struct{}

// NewClient creates a new Git client instance.
func NewClient() *Client {
	return &Client{}
}

// CurrentBranch execution retrieves the currently checked out branch using rev-parse.
// Returns the branch name trimmed of whitespace or an error if the command fails.
func (c *Client) CurrentBranch(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("exec git rev-parse failed: %w", err)
	}

	return strings.TrimSpace(out.String()), nil
}

// BranchExists checks whether a specific branch exists in the repository.
// Uses git show-ref to test for the branch's presence.
func (c *Client) BranchExists(ctx context.Context, branch string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)

	// If the command succeeds, the branch exists locally.
	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("exec git show-ref failed: %w", err)
	}

	return true, nil
}

// Diff extracts the raw diff output between two branches.
// If baseBranch is an empty string, it diffs against the target branch only.
func (c *Client) Diff(ctx context.Context, baseBranch, targetBranch string) (string, error) {
	args := []string{"diff"}
	if baseBranch != "" {
		args = append(args, baseBranch)
	}
	args = append(args, targetBranch)

	cmd := exec.CommandContext(ctx, "git", args...)
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("exec git diff failed: %w", err)
	}

	return strings.TrimSpace(out.String()), nil
}
