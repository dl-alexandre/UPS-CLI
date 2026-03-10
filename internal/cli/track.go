package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/dl-alexandre/UPS-CLI/internal/api"
	"github.com/dl-alexandre/UPS-CLI/internal/config"
)

// TrackCmd handles the track command
type TrackCmd struct {
	TrackingNumber string `arg:"" help:"UPS tracking number to lookup"`
	Raw            bool   `help:"Output raw JSON response" flag:"raw"`
	Format         string `help:"Output format: table, json, markdown" default:"" kong:"-"`
}

func (c *TrackCmd) Run(globals *Globals) error {
	// Validate tracking number
	if c.TrackingNumber == "" {
		return fmt.Errorf("tracking number is required")
	}

	// Get config - check if we have UPS credentials
	if globals.Config == nil {
		// Try to load config with just the essentials
		flags := config.Flags{
			ConfigFile: globals.ConfigFile,
			APIURL:     globals.APIURL,
			Verbose:    globals.Verbose,
			Debug:      globals.Debug,
		}
		cfg, err := config.Load(flags)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		globals.Config = cfg
	}

	upsConfig := globals.Config.UPS
	if upsConfig.ClientID == "" || upsConfig.ClientSecret == "" {
		return fmt.Errorf("UPS credentials not configured. Set UPS_CLIENT_ID and UPS_CLIENT_SECRET environment variables, or configure in config file")
	}

	// Create token provider
	tokenProvider := api.NewTokenProvider(
		upsConfig.ClientID,
		upsConfig.ClientSecret,
		upsConfig.BaseURL,
	)

	// Create API client
	client := api.NewClient(api.ClientOptions{
		BaseURL: upsConfig.BaseURL,
		Timeout: 30,
		Verbose: globals.Verbose,
		Debug:   globals.Debug,
	})
	client.SetTokenProvider(tokenProvider)

	// Execute tracking request
	ctx := context.Background()

	if c.Raw {
		// Output raw JSON
		body, err := client.TrackRaw(ctx, c.TrackingNumber)
		if err != nil {
			return fmt.Errorf("tracking failed: %w", err)
		}
		fmt.Fprintln(os.Stdout, string(body))
		return nil
	}

	// Get formatted tracking response
	response, err := client.Track(ctx, c.TrackingNumber)
	if err != nil {
		return fmt.Errorf("tracking failed: %w", err)
	}

	// Format output
	format := c.Format
	if format == "" {
		format = globals.Format
	}

	printer := globals.GetPrinter()
	return printer.PrintTracking(response, format)
}
