package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/dl-alexandre/UPS-CLI/internal/api"
	"github.com/dl-alexandre/UPS-CLI/internal/config"
)

// RateCmd handles the rate command
type RateCmd struct {
	FromPostal  string `required:"" help:"Origin postal code" flag:"from"`
	ToPostal    string `required:"" help:"Destination postal code" flag:"to"`
	Weight      string `required:"" help:"Package weight" flag:"weight"`
	CountryFrom string `help:"Origin country code (2 letters)" flag:"country-from" default:"US"`
	CountryTo   string `help:"Destination country code (2 letters)" flag:"country-to" default:"US"`
	WeightUnit  string `help:"Weight unit: LBS or KGS" flag:"weight-unit" default:"LBS" enum:"LBS,KGS"`
	Service     string `help:"Service code filter (optional)" flag:"service"`
	Shipper     string `help:"Shipper postal code (defaults to --from)" flag:"shipper"`
	Format      string `help:"Output format (overrides global)" kong:"-"`
	Raw         bool   `help:"Output raw JSON response" flag:"raw"`
}

func (c *RateCmd) Run(globals *Globals) error {
	// Validate inputs
	if err := c.validate(); err != nil {
		return err
	}

	// Get config
	if globals.Config == nil {
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
		upsConfig.Env,
		upsConfig.BaseURL,
		globals.Config.GetCacheDir(),
	)

	// Create API client
	client := api.NewClient(api.ClientOptions{
		BaseURL: upsConfig.BaseURL,
		Timeout: 30,
		Verbose: globals.Verbose,
		Debug:   globals.Debug,
	})
	client.SetTokenProvider(tokenProvider)

	// Log environment in verbose mode
	if globals.Verbose {
		fmt.Fprintf(os.Stderr, "Environment: %s\n", upsConfig.Env)
		fmt.Fprintf(os.Stderr, "Base URL: %s\n", upsConfig.BaseURL)
		fmt.Fprintf(os.Stderr, "From: %s, %s -> To: %s, %s\n", c.FromPostal, c.CountryFrom, c.ToPostal, c.CountryTo)
		fmt.Fprintf(os.Stderr, "Weight: %s %s\n", c.Weight, c.WeightUnit)
	}

	// Build rate request
	req, err := api.NewRateRequest(
		c.FromPostal,
		c.ToPostal,
		c.Shipper,
		c.CountryFrom,
		c.CountryTo,
		c.Weight,
		c.WeightUnit,
		c.Service,
	)
	if err != nil {
		return fmt.Errorf("failed to build rate request: %w", err)
	}

	// Execute rate request
	ctx := context.Background()
	locale := upsConfig.Locale
	if locale == "" {
		locale = "en_US"
	}

	if c.Raw {
		body, err := client.RateRaw(ctx, req, locale)
		if err != nil {
			return fmt.Errorf("rate request failed: %w", err)
		}
		fmt.Fprintln(os.Stdout, string(body))
		return nil
	}

	response, err := client.Rate(ctx, req, locale)
	if err != nil {
		return fmt.Errorf("rate request failed: %w", err)
	}

	// Format output
	format := c.Format
	if format == "" {
		format = globals.Format
	}

	printer := globals.GetPrinter()
	return printer.PrintRate(response, format)
}

// validate validates all rate command inputs
func (c *RateCmd) validate() error {
	// Validate weight is positive number
	weight, err := strconv.ParseFloat(c.Weight, 64)
	if err != nil {
		return fmt.Errorf("invalid weight: %s (must be a number)", c.Weight)
	}
	if weight <= 0 {
		return fmt.Errorf("weight must be greater than 0")
	}

	// Validate postal codes are non-empty
	if strings.TrimSpace(c.FromPostal) == "" {
		return fmt.Errorf("origin postal code cannot be empty")
	}
	if strings.TrimSpace(c.ToPostal) == "" {
		return fmt.Errorf("destination postal code cannot be empty")
	}

	// Validate country codes are 2 letters
	if !isValidCountryCode(c.CountryFrom) {
		return fmt.Errorf("invalid origin country code: %s (must be 2 letters)", c.CountryFrom)
	}
	if !isValidCountryCode(c.CountryTo) {
		return fmt.Errorf("invalid destination country code: %s (must be 2 letters)", c.CountryTo)
	}

	// Validate weight unit
	unit := strings.ToUpper(c.WeightUnit)
	if unit != "LBS" && unit != "KGS" {
		return fmt.Errorf("invalid weight unit: %s (must be LBS or KGS)", c.WeightUnit)
	}

	// Service code is optional passthrough
	// If provided, we'll validate it's not empty
	if c.Service != "" && strings.TrimSpace(c.Service) == "" {
		return fmt.Errorf("service code cannot be empty if provided")
	}

	return nil
}

// isValidCountryCode checks if a string is a valid 2-letter country code
func isValidCountryCode(code string) bool {
	if len(code) != 2 {
		return false
	}
	for _, r := range code {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}
