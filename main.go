package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"

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
			&cli.StringSliceFlag{
				Name:    "branch",
				Aliases: []string{"b"},
				Usage:   "Specify two branches to compare: -b branchA -b branchB",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if !cmd.Bool("review") {
				fmt.Println("Use -r or --review to trigger review")
				return nil
			}

			branches := cmd.StringSlice("branch")
			if len(branches) != 2 {
				return fmt.Errorf("please provide exactly two branches using -b")
			}

			branchA := branches[0]
			branchB := branches[1]

			fmt.Printf("Comparing %s ↔ %s\n\n", branchA, branchB)

			// git diff branchA...branchB
			gitCmd := exec.CommandContext(
				ctx,
				"git",
				"diff",
				fmt.Sprintf("%s...%s", branchA, branchB),
			)

			var out bytes.Buffer
			var stderr bytes.Buffer
			gitCmd.Stdout = &out
			gitCmd.Stderr = &stderr

			if err := gitCmd.Run(); err != nil {
				return fmt.Errorf("git diff failed: %v\n%s", err, stderr.String())
			}

			fmt.Println(out.String())
			return nil
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
