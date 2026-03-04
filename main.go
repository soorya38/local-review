package main

import (
	"context"
	"log"
	"os"

	"local_review/internal/cli"
)

// main initializes the CLI application utilizing Clean Architecture principles.
//
// Infrastructure wiring (Git, Checkers, LLM) is deferred to the CLI layer so
// the Groq API key is resolved from flags / env before engine construction.
func main() {
	ctx := context.Background()

	rootCLI := cli.NewCLI(os.Stdout)

	if err := rootCLI.Execute(ctx); err != nil {
		log.Fatalf("Command failed: %v", err)
	}
}
