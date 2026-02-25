// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-02-25

package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/similigh/simili-bot/internal/core/config"
	"github.com/similigh/simili-bot/internal/integrations/github"
	"github.com/similigh/simili-bot/internal/steps"
)

var (
	autoCloseRepo            string
	autoCloseDryRun          bool
	autoCloseGracePeriodMins int
	autoCloseGraceMinChanged bool // set in PreRun to detect explicit 0
)

var autoCloseCmd = &cobra.Command{
	Use:   "auto-close",
	Short: "Close confirmed duplicate issues after the grace period expires",
	Long: `Scan all open issues labelled "potential-duplicate".

Close those whose grace period has expired and show no human activity.

Human activity signals (any of these prevents auto-close):
  1. A negative reaction (👎 -1 or 😕 confused) on the bot's triage comment
     by a non-bot user.
  2. The issue was reopened by a human after the label was applied.
  3. A non-bot comment posted after the label was applied.

Grace period order of precedence:
  --grace-period-minutes flag  >  config auto_close.grace_period_hours  >  72 h default`,
	PreRun: func(cmd *cobra.Command, args []string) {
		autoCloseGraceMinChanged = cmd.Flags().Changed("grace-period-minutes")
	},
	Run: runAutoClose,
}

func init() {
	rootCmd.AddCommand(autoCloseCmd)

	autoCloseCmd.Flags().StringVar(&autoCloseRepo, "repo", "", "Repository to scan (owner/name); falls back to GITHUB_REPOSITORY env var")
	autoCloseCmd.Flags().BoolVar(&autoCloseDryRun, "dry-run", false, "Print what would be closed without making any changes")
	autoCloseCmd.Flags().IntVar(&autoCloseGracePeriodMins, "grace-period-minutes", 0,
		"Override grace period in minutes (0 means instant-expire; use for testing)")
}

func runAutoClose(cmd *cobra.Command, args []string) {
	// Resolve org/repo
	repo := autoCloseRepo
	if repo == "" {
		repo = os.Getenv("GITHUB_REPOSITORY")
	}
	if repo == "" {
		fmt.Fprintln(os.Stderr, "Error: --repo or GITHUB_REPOSITORY is required")
		os.Exit(1)
	}
	org, repoOnly, ok := strings.Cut(repo, "/")
	if !ok || org == "" || repoOnly == "" {
		fmt.Fprintf(os.Stderr, "Error: invalid repository format %q (expected owner/name)\n", repo)
		os.Exit(1)
	}

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "Error: GITHUB_TOKEN is required")
		os.Exit(1)
	}

	// Load config
	actualCfgPath := cfgFile
	if actualCfgPath == "" {
		actualCfgPath = config.FindConfigPath("")
	}

	var cfg *config.Config
	var err error
	if actualCfgPath != "" {
		fetcher := func(ref string) ([]byte, error) {
			o, r, branch, path, ferr := config.ParseExtendsRef(ref)
			if ferr != nil {
				return nil, ferr
			}
			ghc := github.NewClient(context.Background(), token)
			return ghc.GetFileContent(context.Background(), o, r, path, branch)
		}
		cfg, err = config.LoadWithInheritance(actualCfgPath, fetcher)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to load config from %s: %v — using defaults\n", actualCfgPath, err)
			cfg = &config.Config{}
		} else if verbose {
			fmt.Printf("Loaded config from %s\n", actualCfgPath)
		}
	} else {
		if verbose {
			fmt.Fprintln(os.Stderr, "No config file found, using defaults")
		}
		cfg = &config.Config{}
	}

	// Apply grace-period-minutes override when the flag was explicitly provided.
	// auto_closer.go only acts on GracePeriodMinutesOverride when it is > 0,
	// so passing 0 on the CLI (meaning "expire instantly for tests") is stored
	// as 1 minute — the smallest meaningful positive value.
	if autoCloseGraceMinChanged {
		if autoCloseGracePeriodMins <= 0 {
			cfg.AutoClose.GracePeriodMinutesOverride = 1
		} else {
			cfg.AutoClose.GracePeriodMinutesOverride = autoCloseGracePeriodMins
		}
	}

	if verbose && autoCloseGraceMinChanged {
		fmt.Printf("Grace period override: %d min(s)\n", cfg.AutoClose.GracePeriodMinutesOverride)
	}

	// Run auto-closer
	ghClient := github.NewClient(context.Background(), token)
	closer := steps.NewAutoCloser(ghClient, cfg, autoCloseDryRun, verbose)

	result, err := closer.Run(context.Background(), org, repoOnly)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Print result as JSON to stdout
	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling result: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(out))
}
