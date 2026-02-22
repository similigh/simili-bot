// Author: Sachindu Nethmin
// GitHub: https://github.com/Sachindu-Nethmin
// Created: 2026-02-22
// Last Modified: 2026-02-22

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
	autoCloseRepo    string
	autoCloseDry     bool
	graceMinutesFlag int
)

// autoCloseCmd represents the auto-close command
var autoCloseCmd = &cobra.Command{
	Use:   "auto-close",
	Short: "Auto-close confirmed duplicate issues after their grace period",
	Long: `Scans issues labeled 'potential-duplicate' and closes those whose
grace period has expired and have no human activity since labeling.

The grace period is configured via auto_close.grace_period_hours in the
config file (default: 72 hours).

Usage:
  simili auto-close --repo owner/name [--config path] [--dry-run] [--verbose]

Environment variables:
  GITHUB_TOKEN   Required. Token with issues:write permission.`,
	Run: func(cmd *cobra.Command, args []string) {
		runAutoClose()
	},
}

func init() {
	rootCmd.AddCommand(autoCloseCmd)

	autoCloseCmd.Flags().StringVar(&autoCloseRepo, "repo", "", "Repository in owner/name format (or set GITHUB_REPOSITORY)")
	autoCloseCmd.Flags().BoolVar(&autoCloseDry, "dry-run", false, "Log actions without executing them")
	autoCloseCmd.Flags().IntVar(&graceMinutesFlag, "grace-minutes", 0, "Override grace period in minutes (for testing; 0 = use config)")
}

func runAutoClose() {
	// 1. Resolve org/repo
	org, repo := resolveAutoCloseRepo(autoCloseRepo)
	if org == "" || repo == "" {
		fmt.Println("Error: --repo owner/name is required (or set GITHUB_REPOSITORY)")
		os.Exit(1)
	}

	// 2. Load config
	cfg := loadAutoCloseConfig()

	// 3. Initialize GitHub client
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		fmt.Println("Error: GITHUB_TOKEN environment variable is required")
		os.Exit(1)
	}

	ctx := context.Background()
	ghClient := github.NewClient(ctx, token)

	// 4. Apply CLI override for grace period
	if graceMinutesFlag > 0 {
		cfg.AutoClose.GracePeriodMinutesOverride = graceMinutesFlag
		if verbose {
			fmt.Printf("Grace period overridden to %d minutes via --grace-minutes\n", graceMinutesFlag)
		}
	}

	// 5. Run auto-closer
	closer := steps.NewAutoCloser(ghClient, cfg, autoCloseDry, verbose)

	fmt.Printf("[Simili-Bot] Running auto-close for %s/%s...\n", org, repo)
	result, err := closer.Run(ctx, org, repo)
	if err != nil {
		fmt.Printf("❌ Auto-close failed: %v\n", err)
		os.Exit(1)
	}

	// 5. Print summary
	fmt.Printf("\n=== Auto-Close Summary ===\n")
	fmt.Printf("Processed: %d\n", result.Processed)
	fmt.Printf("Closed:    %d\n", result.Closed)
	fmt.Printf("Skipped (grace period):    %d\n", result.SkippedGrace)
	fmt.Printf("Skipped (human activity):  %d\n", result.SkippedHuman)
	if len(result.Errors) > 0 {
		fmt.Printf("Errors:    %d\n", len(result.Errors))
	}

	resultBytes, err := json.MarshalIndent(result, "", "  ")
	if err == nil {
		fmt.Println("\n=== Detailed Result ===")
		fmt.Println(string(resultBytes))
	}

	if len(result.Errors) > 0 {
		os.Exit(1)
	}
}

func resolveAutoCloseRepo(flagRepo string) (string, string) {
	if flagRepo != "" && strings.Contains(flagRepo, "/") {
		parts := strings.SplitN(flagRepo, "/", 2)
		if len(parts) == 2 {
			return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		}
	}

	if ghRepo := strings.TrimSpace(os.Getenv("GITHUB_REPOSITORY")); ghRepo != "" {
		parts := strings.SplitN(ghRepo, "/", 2)
		if len(parts) == 2 {
			return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		}
	}

	return "", ""
}

func loadAutoCloseConfig() *config.Config {
	actualCfgPath := cfgFile
	if actualCfgPath == "" {
		actualCfgPath = config.FindConfigPath("")
	}

	if actualCfgPath == "" {
		if verbose {
			fmt.Println("No configuration file found. Using defaults.")
		}
		cfg := &config.Config{}
		return cfg
	}

	// Build fetcher for config inheritance
	var configToken string
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		configToken = token
	}

	fetcher := func(ref string) ([]byte, error) {
		org, repo, branch, path, err := config.ParseExtendsRef(ref)
		if err != nil {
			return nil, err
		}
		if configToken == "" {
			return nil, fmt.Errorf("GITHUB_TOKEN required to fetch remote config %s", ref)
		}
		ghClient := github.NewClient(context.Background(), configToken)
		return ghClient.GetFileContent(context.Background(), org, repo, path, branch)
	}

	cfg, err := config.LoadWithInheritance(actualCfgPath, fetcher)
	if err != nil {
		fmt.Printf("Warning: Failed to load config: %v. Using defaults.\n", err)
		return &config.Config{}
	}

	if verbose {
		fmt.Printf("Loaded config from %s\n", actualCfgPath)
	}
	return cfg
}
