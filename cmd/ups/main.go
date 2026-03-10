package main

import (
	"fmt"
	"os"
	"time"

	"github.com/alecthomas/kong"
	"github.com/dl-alexandre/cli-template/internal/cache"
	"github.com/dl-alexandre/cli-template/internal/cli"
)

var (
	version   = "dev"
	gitCommit = "unknown"
	buildTime = "unknown"
)

func main() {
	// Set version info in cli package
	cli.Version = version
	cli.BinaryName = "ups"
	cli.GitHubRepo = "UPS-CLI"
	cli.GitCommit = gitCommit
	cli.BuildTime = buildTime

	var c cli.CLI
	ctx := kong.Parse(&c,
		kong.Name("ups"),
		kong.Description("CLI for UPS APIs (Tracking, Rating, Shipping)"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
		}),
		kong.Vars{
			"version": version,
		},
	)

	if ctx.Command() == "version" {
		fmt.Printf("ups %s (commit: %s) built %s\n", version, gitCommit, buildTime)
		os.Exit(0)
	}

	// Run auto-update check in background (after initialization)
	// This runs asynchronously and won't block the main command
	go func() {
		// Small delay to not interfere with command output
		time.Sleep(100 * time.Millisecond)

		// Use a minimal cache for update checks
		updateCache := cache.New(cache.DefaultCacheDir(), 24*time.Hour)
		cli.AutoUpdateCheck(updateCache)
	}()

	if err := ctx.Run(&c.Globals); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
