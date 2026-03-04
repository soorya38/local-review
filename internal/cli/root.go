package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"

	"local_review/internal/app"
	"local_review/internal/config"
	"local_review/internal/domain"
	"local_review/internal/infra/git"
	"local_review/internal/infra/llm"

	"github.com/spf13/cobra"
)

// CLI configures and executes the command-line interface.
type CLI struct {
	out io.Writer
}

// NewCLI creates a new CLI instance.
func NewCLI(out io.Writer) *CLI {
	if out == nil {
		out = os.Stdout
	}
	return &CLI{out: out}
}

// Execute parses arguments and runs the root command.
func (c *CLI) Execute(ctx context.Context) error {
	var baseBranch string
	var doReview bool
	var groqKey string
	var standardsPath string

	rootCmd := &cobra.Command{
		Use:   "lr --branch <base-branch> <target-branch>",
		Short: "Local branch-to-branch code review CLI",
		Long: `lr performs a local code review by diffing two git branches.
It executes deterministic checks (go fmt, go vet, etc.) before invoking an LLM.

Arguments:
  -b, --branch <base-branch>   the branch you are comparing FROM (required)
  <target-branch>              the branch you are comparing TO (required, positional)

Flags:
  -r, --review                 trigger a branch-to-branch review
  -k, --groq-key <api-key>     Groq API key (overrides GROQ_API_KEY env var and config file)
  -s, --standards <path>       path to coding standards file (overrides config file)

Priority (highest → lowest):
  flag  >  env var (GROQ_API_KEY)  >  config file (~/.config/lr/config)  >  default

One-time setup (persists across sessions):
  lr config set groq-key gsk_xxxxxxxxxxxxxxxxxxxx
  lr config set standards ./CODING_STANDARDS.md

Inspect saved config:
  lr config list
  lr config get groq-key
  lr config unset groq-key

Behaviour:
  If you are already on <base-branch>, lr runs:
    git diff <target-branch>

  Otherwise it runs:
    git diff <base-branch> <target-branch>`,
		Example: `  # One-time setup
  lr config set groq-key gsk_xxxxxxxxxxxxxxxxxxxx
  lr config set standards ./CODING_STANDARDS.md

  # Run reviews without repeating flags
  lr -r -b main feature/auth
  lr -r -b main feature/payments

  # Override key or standards for a single run
  lr -r -k gsk_xxx -b main feature/auth
  lr -r -s ./other/STANDARDS.md -b main feature/auth

  # Inline env var (no config needed)
  GROQ_API_KEY=gsk_xxx lr -r -b main feature/auth`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !doReview {
				fmt.Fprintln(c.out, "Use -r or --review to trigger a deterministic-first review pipeline.")
				return nil
			}

			// Load persisted config (best-effort; errors are non-fatal).
			cfg, _ := config.Load()

			// Resolve Groq API key: flag > env var > config file
			// keySource is printed (masked) on every review run to aid debugging.
			var keySource string
			resolvedKey := groqKey
			if resolvedKey != "" {
				keySource = "flag (-k)"
			}
			if resolvedKey == "" {
				resolvedKey = os.Getenv("GROQ_API_KEY")
				if resolvedKey != "" {
					keySource = "env var (GROQ_API_KEY)"
				}
			}
			if resolvedKey == "" && cfg != nil {
				resolvedKey = cfg.GroqKey
				if resolvedKey != "" {
					keySource = "config file"
				}
			}
			if resolvedKey == "" {
				return fmt.Errorf(
					"Groq API key is required.\n" +
						"  Persist it:  lr config set groq-key gsk_...\n" +
						"  Env var:     export GROQ_API_KEY=gsk_...\n" +
						"  Flag:        lr -r -k gsk_... -b main feature/x",
				)
			}
			fmt.Fprintf(c.out, "Using API key from %s: %s\n", keySource, maskKey(resolvedKey))

			// Resolve standards path: flag > config file > default
			resolvedStandards := standardsPath
			if resolvedStandards == "" && cfg != nil && cfg.Standards != "" {
				resolvedStandards = cfg.Standards
			}
			if resolvedStandards == "" {
				resolvedStandards = "CODING_STANDARDS.md"
			}

			// Read coding standards file (best-effort; empty string is acceptable).
			var standards string
			if raw, err := os.ReadFile(resolvedStandards); err == nil {
				standards = string(raw)
			} else {
				fmt.Fprintf(c.out, "Warning: could not read standards file %q: %v\n", resolvedStandards, err)
			}

			// Wire infrastructure
			gitClient := git.NewClient()
			llmClient := llm.NewGroqClient(resolvedKey, "")

			// Build engine
			engine := app.NewReviewEngine(gitClient, llmClient, c.out)

			targetBranch := args[0]
			req := domain.ReviewRequest{
				BaseBranch:   baseBranch,
				TargetBranch: targetBranch,
				StrictMode:   true,
				Standards:    standards,
			}

			report, err := engine.Run(cmd.Context(), req)
			if err != nil {
				return err
			}

			if report != nil {
				fmt.Fprintln(c.out, "\n==============================================")
				fmt.Fprintln(c.out, "REVIEW REPORT")
				fmt.Fprintln(c.out, "==============================================")
				fmt.Fprintf(c.out, "Summary  : %s\n", report.Summary)
				fmt.Fprintf(c.out, "Severity : %s\n", report.Severity)
				fmt.Fprintln(c.out, "----------------------------------------------")
				fmt.Fprintln(c.out, report.Details)
				fmt.Fprintln(c.out, "==============================================")
			}
			return nil
		},
	}

	rootCmd.Flags().StringVarP(&baseBranch, "branch", "b", "", "Base branch to compare from (required)")
	rootCmd.Flags().BoolVarP(&doReview, "review", "r", false, "Trigger branch-to-branch review")
	rootCmd.Flags().StringVarP(&groqKey, "groq-key", "k", "", "Groq API key (overrides GROQ_API_KEY and config file)")
	rootCmd.Flags().StringVarP(&standardsPath, "standards", "s", "", "Path to coding standards file (overrides config file)")
	_ = rootCmd.MarkFlagRequired("branch")

	rootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		cmd.Println(err)
		cmd.Println(cmd.UsageString())
		return fmt.Errorf("see help above")
	})

	// Attach config subcommand
	rootCmd.AddCommand(c.newConfigCmd())

	return rootCmd.ExecuteContext(ctx)
}

// newConfigCmd builds the `lr config` subcommand tree.
func (c *CLI) newConfigCmd() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage persistent lr settings (~/.config/lr/config)",
		Long: `Manage persistent lr settings stored in ~/.config/lr/config.

Settings saved here are used as fallbacks when the corresponding flag or
environment variable is not provided. Priority (highest → lowest):
  flag  >  env var  >  config file  >  default

Supported keys:
  groq-key    Groq API key
  standards   Path to the coding standards file`,
	}

	// lr config set <key> <value>
	setCmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Persist a setting to the config file",
		Example: `  lr config set groq-key gsk_xxxxxxxxxxxxxxxxxxxx
  lr config set standards /path/to/CODING_STANDARDS.md`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, value := args[0], args[1]
			if key != config.KeyGroqKey && key != config.KeyStandards {
				return fmt.Errorf("unknown key %q — valid keys: %s, %s", key, config.KeyGroqKey, config.KeyStandards)
			}
			if err := config.Set(key, value); err != nil {
				return err
			}
			path, _ := config.FilePath()
			fmt.Fprintf(c.out, "Saved: %s = %s  (%s)\n", key, value, path)
			return nil
		},
	}

	// lr config get <key>
	getCmd := &cobra.Command{
		Use:     "get <key>",
		Short:   "Print the current value of a setting",
		Example: `  lr config get groq-key`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			switch args[0] {
			case config.KeyGroqKey:
				fmt.Fprintln(c.out, cfg.GroqKey)
			case config.KeyStandards:
				fmt.Fprintln(c.out, cfg.Standards)
			default:
				return fmt.Errorf("unknown key %q — valid keys: %s, %s", args[0], config.KeyGroqKey, config.KeyStandards)
			}
			return nil
		},
	}

	// lr config list
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all settings in the config file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			pairs, err := config.List()
			if err != nil {
				return err
			}
			if len(pairs) == 0 {
				path, _ := config.FilePath()
				fmt.Fprintf(c.out, "No settings saved yet. Use `lr config set <key> <value>`\nConfig file: %s\n", path)
				return nil
			}
			path, _ := config.FilePath()
			fmt.Fprintf(c.out, "# %s\n", path)
			keys := make([]string, 0, len(pairs))
			for k := range pairs {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Fprintf(c.out, "%s = %s\n", k, pairs[k])
			}
			return nil
		},
	}

	// lr config unset <key>
	unsetCmd := &cobra.Command{
		Use:     "unset <key>",
		Short:   "Remove a setting from the config file",
		Example: `  lr config unset groq-key`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.Set(args[0], ""); err != nil {
				return err
			}
			fmt.Fprintf(c.out, "Unset: %s\n", args[0])
			return nil
		},
	}

	verifyCmd := &cobra.Command{
		Use:   "verify",
		Short: "Show which value will be used for each setting and from which source",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			path, _ := config.FilePath()
			fmt.Fprintf(c.out, "Config file: %s\n\n", path)

			// groq-key
			key, src := "", ""
			if v := os.Getenv("GROQ_API_KEY"); v != "" {
				key, src = v, "env var (GROQ_API_KEY)  ← overrides config file!"
			} else if cfg != nil && cfg.GroqKey != "" {
				key, src = cfg.GroqKey, "config file"
			} else {
				src = "(not set)"
			}
			if key != "" {
				fmt.Fprintf(c.out, "groq-key  : %s  [%s]\n", maskKey(key), src)
			} else {
				fmt.Fprintf(c.out, "groq-key  : (not set)  [%s]\n", src)
			}

			// standards
			std, stdSrc := "", ""
			if cfg != nil && cfg.Standards != "" {
				std, stdSrc = cfg.Standards, "config file"
			} else {
				std, stdSrc = "CODING_STANDARDS.md", "default"
			}
			fmt.Fprintf(c.out, "standards : %s  [%s]\n", std, stdSrc)

			return nil
		},
	}

	configCmd.AddCommand(setCmd, getCmd, listCmd, unsetCmd, verifyCmd)
	return configCmd
}

// maskKey returns the first 8 characters of a key followed by *** to avoid
// logging the full secret while still making the key recognisable for debugging.
func maskKey(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:8] + "***"
}
