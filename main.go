package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var (
	baseBranch string
	doReview   bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "lr --branch <base-branch> <target-branch>",
		Short: "Local branch-to-branch code review CLI",
		Long: `lr performs a local code review by diffing two git branches.

Arguments:
  -b, --branch <base-branch>   the branch you are comparing FROM (required)
  <target-branch>              the branch you are comparing TO (required, positional)

Flags:
  -r, --review                 trigger a branch-to-branch review

Behaviour:
  If you are already on <base-branch>, lr runs:
    git diff <target-branch>

  Otherwise it runs:
    git diff <base-branch> <target-branch>`,
		Example: `  lr -b main feature/my-branch
  lr --branch develop feature/my-branch
  lr -r -b main feature/my-branch`,
		Args: cobra.ExactArgs(1),
		RunE: run,
	}

	rootCmd.Flags().StringVarP(&baseBranch, "branch", "b", "", "Base branch to compare from (required)")
	rootCmd.Flags().BoolVarP(&doReview, "review", "r", false, "Trigger branch-to-branch review")
	rootCmd.MarkFlagRequired("branch")

	rootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		cmd.Println(err)
		cmd.Println(cmd.UsageString())
		return fmt.Errorf("see help above")
	})

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func run(cmd *cobra.Command, args []string) error {
	if !doReview {
		fmt.Println("Use -r or --review to trigger review")
		return nil
	}

	branchB := args[0]
	ctx := context.Background()

	fmt.Printf("Comparing %s ↔ %s\n\n", baseBranch, branchB)

	currentBranch, err := getCurrentBranch(ctx)
	if err != nil {
		return fmt.Errorf("failed to detect current branch: %w", err)
	}

	var gitCmd *exec.Cmd

	if currentBranch == baseBranch {
		gitCmd = exec.CommandContext(ctx, "git", "diff", branchB)
	} else {
		gitCmd = exec.CommandContext(ctx, "git", "diff", baseBranch, branchB)
	}

	gitCmd.Stdout = os.Stdout
	gitCmd.Stderr = os.Stderr

	return gitCmd.Run()
}

func getCurrentBranch(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}
