package cli

import (
	"github.com/dl-alexandre/cli-tools/update"
	"github.com/dl-alexandre/cli-tools/version"
)

// UpdateCheckCmd wraps cli-tools update functionality
type UpdateCheckCmd struct {
	Force bool `help:"Force check, bypassing cache" flag:"force"`
}

// Run executes the update check
func (c *UpdateCheckCmd) Run(globals *Globals) error {
	checker := update.New(update.Config{
		CurrentVersion: version.Version,
		BinaryName:     version.BinaryName,
		GitHubRepo:     "dl-alexandre/UPS-CLI",
		InstallCommand: "brew upgrade ups",
	})

	info, err := checker.Check(c.Force)
	if err != nil {
		return err
	}

	// Use format from globals (table, json, markdown)
	return update.DisplayUpdate(info, version.BinaryName, globals.Format)
}

// AutoUpdateCheck performs a background update check (for use at startup)
// It returns immediately and doesn't block
func AutoUpdateCheck(cacheInstance interface{}) {
	checker := update.New(update.Config{
		CurrentVersion: version.Version,
		BinaryName:     version.BinaryName,
		GitHubRepo:     "dl-alexandre/UPS-CLI",
		InstallCommand: "brew upgrade ups",
	})
	checker.AutoCheck()
}

// UpdateInfo is re-exported from cli-tools for backward compatibility
type UpdateInfo = update.Info
