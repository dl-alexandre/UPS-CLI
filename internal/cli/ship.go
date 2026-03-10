package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	// Idempotency
	RequestID string `help:"Unique request ID for idempotency. Sent to UPS as transaction reference. May help detect duplicates but UPS may still create duplicates on ambiguous failures." flag:"request-id"`

	// Label output
	LabelFormat string `help:"Label format: PDF" flag:"label-format" enum:"PDF" default:"PDF"`
	LabelOut    string `help:"Label output file path" flag:"label-out"`

	// Control flags
	Validate bool `help:"Validate shipment only (do not create)" flag:"validate"`
	Create   bool `help:"Create shipment and generate label" flag:"create"`

	// Production confirmations (required when UPS_ENV=production and --create)
	ConfirmProduction bool `help:"Confirm production environment" flag:"confirm-production"`
	ConfirmCharge     bool `help:"Confirm charges will be applied" flag:"confirm-charge"`

	Format string `help:"Output format (overrides global)" kong:"-"`
	Raw    bool   `help:"Output raw JSON response" flag:"raw"`
}

func (c *ShipCmd) Run(globals *Globals) error {
	// Validate mode selection
	if c.Validate && c.Create {
		return fmt.Errorf("cannot use both --validate and --create")
	}
	if !c.Validate && !c.Create {
		return fmt.Errorf("must specify either --validate or --create")
	}

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

	// Check production confirmations for create
	if c.Create && upsConfig.Env == "production" {
		if !c.ConfirmProduction {
			return fmt.Errorf("UPS_ENV=production requires --confirm-production flag")
		}
		if !c.ConfirmCharge {
			return fmt.Errorf("UPS_ENV=production requires --confirm-charge flag")
		}
	}

	// Check request-id for create (idempotency)
	if c.Create && c.RequestID == "" {
		return fmt.Errorf("--create requires --request-id for idempotency")
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
		if c.Create {
			fmt.Fprintf(os.Stderr, "Mode: Create shipment\n")
			fmt.Fprintf(os.Stderr, "Request ID: %s\n", c.RequestID)
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

	// Set request ID for idempotency if creating
	if c.Create && c.RequestID != "" {
		if req.ShipmentRequest != nil && req.ShipmentRequest.Request != nil && req.ShipmentRequest.Request.TransactionReference != nil {
			req.ShipmentRequest.Request.TransactionReference.TransactionIdentifier = c.RequestID
		}
	}

	// Execute
	ctx := context.Background()
	locale := upsConfig.Locale
	if locale == "" {
		locale = "en_US"
	}

	if c.Validate {
		return c.runValidate(ctx, client, req, locale, globals)
	}

	return c.runCreate(ctx, client, req, locale, globals)
}

// runValidate handles validation mode
func (c *ShipCmd) runValidate(ctx context.Context, client *api.Client, req *api.ShipmentRequest, locale string, globals *Globals) error {
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

// runCreate handles create mode
func (c *ShipCmd) runCreate(ctx context.Context, client *api.Client, req *api.ShipmentRequest, locale string, globals *Globals) error {
	var response *api.ShipmentResponse
	var body []byte
	var err error

	if c.Raw {
		body, err = client.ShipCreateRaw(ctx, req, locale)
		if err != nil {
			return c.handleCreateError(err)
		}
		fmt.Fprintln(os.Stdout, string(body))
	} else {
		response, err = client.ShipCreate(ctx, req, locale)
		if err != nil {
			return c.handleCreateError(err)
		}

		// Print creation result (always show alerts for create)
		format := c.Format
		if format == "" {
			format = globals.Format
		}

		printer := globals.GetPrinter()
		if err := printer.PrintShipmentCreate(response, format); err != nil {
			return err
		}
	}

	// Handle label output for create
	if c.Create {
		if c.Raw {
			// Parse from raw body to extract label
			var resp api.ShipmentResponse
			if err := json.Unmarshal(body, &resp); err != nil {
				return fmt.Errorf("failed to parse response for label: %w", err)
			}
			response = &resp
		}

		// Extract and write label
		if err := c.writeLabel(response); err != nil {
			return fmt.Errorf("failed to write label: %w", err)
		}
	}

	return nil
}

// writeLabel extracts and writes the shipping label to file atomically
// Uses temp file + fsync + rename to avoid partial files
func (c *ShipCmd) writeLabel(response *api.ShipmentResponse) error {
	if response == nil || response.ShipmentResponse.ShipmentResults == nil {
		return fmt.Errorf("no shipment results in response")
	}

	results := response.ShipmentResponse.ShipmentResults
	if len(results.PackageResults) == 0 {
		return fmt.Errorf("no package results in response")
	}

	// Get the first package's label
	pkg := results.PackageResults[0]
	if pkg.ShippingLabel == nil || pkg.ShippingLabel.GraphicImage == "" {
		return fmt.Errorf("no label data in response")
	}

	// Determine output path
	outputPath := c.LabelOut
	if outputPath == "" {
		// Default to tracking number as filename
		if pkg.TrackingNumber != "" {
			outputPath = fmt.Sprintf("%s.pdf", pkg.TrackingNumber)
		} else {
			outputPath = "label.pdf"
		}
	}

	// Ensure directory exists
	dir := filepath.Dir(outputPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	// Decode base64
	labelData, err := base64.StdEncoding.DecodeString(pkg.ShippingLabel.GraphicImage)
	if err != nil {
		return fmt.Errorf("failed to decode label data: %w", err)
	}

	// Atomic write: temp file -> fsync -> rename
	tempPath := outputPath + ".tmp"

	// Write to temp file with restrictive permissions
	if err := os.WriteFile(tempPath, labelData, 0600); err != nil {
		return fmt.Errorf("failed to write temp label file: %w", err)
	}

	// Sync to disk to ensure data is written
	if file, err := os.Open(tempPath); err == nil {
		file.Sync()
		file.Close()
	}

	// Atomic rename
	if err := os.Rename(tempPath, outputPath); err != nil {
		// Clean up temp file on error
		os.Remove(tempPath)
		return fmt.Errorf("failed to finalize label file: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Label written to: %s\n", outputPath)
	return nil
}

// handleCreateError handles shipment creation errors with special guidance
// for ambiguous failures that may have succeeded server-side
func (c *ShipCmd) handleCreateError(err error) error {
	errStr := err.Error()

	// Detect ambiguous errors (network timeouts, connection resets, etc.)
	// These may have been processed by UPS even though we got an error
	ambiguousIndicators := []string{
		"timeout",
		"connection reset",
		"no such host",
		"i/o timeout",
		"context deadline exceeded",
		"EOF",
		"connection refused",
	}

	for _, indicator := range ambiguousIndicators {
		if containsIgnoreCase(errStr, indicator) {
			return fmt.Errorf(`%w

⚠️  WARNING: This error may indicate the request was processed by UPS before the connection failed.

DO NOT retry with a new --request-id. Instead:
  1. Check your UPS account for the shipment
  2. If not found, retry with the SAME --request-id: %q
  3. If found, download the label from your UPS account

The shipment may have been created with charges applied.`, err, c.RequestID)
		}
	}

	return fmt.Errorf("shipment creation failed: %w", err)
}

func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
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
