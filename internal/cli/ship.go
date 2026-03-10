package cli

import (
	"context"
	"fmt"
	"os"
	"unicode"

	"github.com/dl-alexandre/UPS-CLI/internal/api"
	"github.com/dl-alexandre/UPS-CLI/internal/config"
)

// ShipCmd handles the ship command
type ShipCmd struct {
	// Address fields
	FromPostal  string `required:"" help:"Origin postal code" flag:"from"`
	ToPostal    string `required:"" help:"Destination postal code" flag:"to"`
	Shipper     string `help:"Shipper postal code (defaults to --from)" flag:"shipper"`
	CountryFrom string `help:"Origin country code (2 letters)" flag:"country-from" default:"US"`
	CountryTo   string `help:"Destination country code (2 letters)" flag:"country-to" default:"US"`

	// Package fields
	Weight     string `required:"" help:"Package weight" flag:"weight"`
	WeightUnit string `help:"Weight unit: LBS or KGS" flag:"weight-unit" default:"LBS" enum:"LBS,KGS"`

	// Service and billing
	Service       string `help:"Service code" flag:"service"`
	AccountNumber string `help:"UPS account number for billing" flag:"account"`

	// Control flags
	Validate bool   `help:"Validate shipment only (do not create)" flag:"validate"`
	Format   string `help:"Output format (overrides global)" kong:"-"`
	Raw      bool   `help:"Output raw JSON response" flag:"raw"`
}

func (c *ShipCmd) Run(globals *Globals) error {
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

	// Log environment in verbose mode
	if globals.Verbose {
		fmt.Fprintf(os.Stderr, "Environment: %s\n", upsConfig.Env)
		fmt.Fprintf(os.Stderr, "Base URL: %s\n", upsConfig.BaseURL)
		fmt.Fprintf(os.Stderr, "From: %s, %s -> To: %s, %s\n", c.FromPostal, c.CountryFrom, c.ToPostal, c.CountryTo)
		fmt.Fprintf(os.Stderr, "Weight: %s %s\n", c.Weight, c.WeightUnit)
		if c.Validate {
			fmt.Fprintf(os.Stderr, "Mode: Validate only\n")
		}
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

	// Build shipment request
	req, err := api.NewShipmentRequest(
		c.FromPostal,
		c.ToPostal,
		c.Shipper,
		c.CountryFrom,
		c.CountryTo,
		c.Weight,
		c.WeightUnit,
		c.Service,
		c.AccountNumber,
	)
	if err != nil {
		return fmt.Errorf("failed to build shipment request: %w", err)
	}

	// Execute shipment validation
	ctx := context.Background()
	locale := upsConfig.Locale
	if locale == "" {
		locale = "en_US"
	}

	if c.Validate {
		// Validation mode
		if c.Raw {
			body, err := client.ShipValidateRaw(ctx, req, locale)
			if err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}
			fmt.Fprintln(os.Stdout, string(body))
			return nil
		}

		response, err := client.ShipValidate(ctx, req, locale)
		if err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}

		// Print validation result
		format := c.Format
		if format == "" {
			format = globals.Format
		}

		printer := globals.GetPrinter()
		return printer.PrintShipmentValidation(response, format)
	}

	// Full shipment mode (not yet implemented)
	return fmt.Errorf("full shipment creation not yet implemented. Use --validate flag for validation only")
}

// validate validates all ship command inputs
func (c *ShipCmd) validate() error {
	// Validate postal codes are non-empty
	if isBlank(c.FromPostal) {
		return fmt.Errorf("origin postal code cannot be empty")
	}
	if isBlank(c.ToPostal) {
		return fmt.Errorf("destination postal code cannot be empty")
	}

	// Validate country codes are 2 letters
	if !isValidCountryCode(c.CountryFrom) {
		return fmt.Errorf("invalid origin country code: %s (must be 2 letters)", c.CountryFrom)
	}
	if !isValidCountryCode(c.CountryTo) {
		return fmt.Errorf("invalid destination country code: %s (must be 2 letters)", c.CountryTo)
	}

	// Validate weight
	if isBlank(c.Weight) {
		return fmt.Errorf("weight is required")
	}

	// Validate weight unit
	unit := toUpper(c.WeightUnit)
	if unit != "LBS" && unit != "KGS" {
		return fmt.Errorf("invalid weight unit: %s (must be LBS or KGS)", c.WeightUnit)
	}

	return nil
}

func isBlank(s string) bool {
	for _, r := range s {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

func toUpper(s string) string {
	result := make([]rune, len(s))
	for i, r := range s {
		result[i] = unicode.ToUpper(r)
	}
	return string(result)
}
