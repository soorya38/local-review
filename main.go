package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/urfave/cli/v3"
)

func main() {
	cmd := &cli.Command{
		Name:  "lr",
		Usage: "Local branch-to-branch code review CLI",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "review",
				Aliases: []string{"r"},
				Usage:   "Trigger branch-to-branch review",
			},
			&cli.StringFlag{
				Name:    "branch",
				Aliases: []string{"b"},
				Usage:   "Base branch to compare from",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {

			if !cmd.Bool("review") {
				fmt.Println("Use -r or --review to trigger review")
				return nil
			}

			branchA := cmd.String("branch")
			if branchA == "" {
				return fmt.Errorf("please provide base branch using -b")
			}

			if cmd.NArg() != 1 {
				return fmt.Errorf("please provide exactly one target branch")
			}

			branchB := cmd.Args().First()

			fmt.Printf("Comparing %s ↔ %s\n\n", branchA, branchB)

			// Step 1: Detect current branch
			currentBranch, err := getCurrentBranch(ctx)
			if err != nil {
				return fmt.Errorf("failed to detect current branch: %w", err)
			}

			var gitCmd *exec.Cmd

			// Step 2: Smart diff behavior
			if currentBranch == branchA {
				// Behave like: git diff main
				gitCmd = exec.CommandContext(ctx, "git", "diff", branchB)
			} else {
				// Fallback to branch-to-branch comparison
				gitCmd = exec.CommandContext(ctx, "git", "diff", branchA, branchB)
			}

			gitCmd.Stdout = os.Stdout
			gitCmd.Stderr = os.Stderr

			return gitCmd.Run()
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
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
