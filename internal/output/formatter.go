package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dl-alexandre/UPS-CLI/internal/api"
	"github.com/rodaine/table"
)

// Printer handles output formatting
type Printer struct {
	format   string
	useColor bool
}

// NewPrinter creates a new output printer
func NewPrinter(format string, useColor bool) *Printer {
	return &Printer{
		format:   format,
		useColor: useColor,
	}
}

// PrintItems prints a list of items in the specified format
func (p *Printer) PrintItems(items *api.ListResponse) error {
	switch p.format {
	case "json":
		return p.printJSON(items)
	case "markdown":
		return p.printMarkdown(items)
	case "table":
		return p.printTable(items)
	default:
		return fmt.Errorf("unsupported format: %s", p.format)
	}
}

// PrintItem prints a single item in the specified format
func (p *Printer) PrintItem(item *api.Item) error {
	switch p.format {
	case "json":
		return p.printJSON(item)
	case "markdown":
		return p.printItemMarkdown(item)
	case "table":
		return p.printItemTable(item)
	default:
		return fmt.Errorf("unsupported format: %s", p.format)
	}
}

// printJSON outputs data as formatted JSON
func (p *Printer) printJSON(data interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// printTable outputs items as a formatted table
func (p *Printer) printTable(items *api.ListResponse) error {
	if len(items.Items) == 0 {
		fmt.Println("No items found.")
		return nil
	}

	tbl := table.New("ID", "Name", "Description", "Created", "Updated").
		WithWriter(os.Stdout)

	if p.useColor {
		tbl.WithHeaderFormatter(func(format string, vals ...interface{}) string {
			return fmt.Sprintf("\033[1m%s\033[0m", fmt.Sprintf(format, vals...))
		})
	}

	for _, item := range items.Items {
		tbl.AddRow(
			item.ID,
			truncate(item.Name, 30),
			truncate(item.Description, 40),
			formatTime(item.CreatedAt),
			formatTime(item.UpdatedAt),
		)
	}

	tbl.Print()
	fmt.Printf("\nShowing %d of %d items\n", len(items.Items), items.Total)

	return nil
}

// printMarkdown outputs items as markdown
func (p *Printer) printMarkdown(items *api.ListResponse) error {
	if len(items.Items) == 0 {
		fmt.Println("No items found.")
		return nil
	}

	fmt.Println("# Items")
	fmt.Println()

	for _, item := range items.Items {
		fmt.Printf("## %s\n\n", item.Name)
		fmt.Printf("**ID:** %s\n\n", item.ID)

		if item.Description != "" {
			fmt.Printf("**Description:** %s\n\n", item.Description)
		}

		fmt.Printf("**Created:** %s\n\n", item.CreatedAt.Format(time.RFC3339))
		fmt.Printf("**Updated:** %s\n\n", item.UpdatedAt.Format(time.RFC3339))

		if len(item.Metadata.Tags) > 0 {
			fmt.Println("**Tags:**")
			for _, tag := range item.Metadata.Tags {
				fmt.Printf("- %s\n", tag)
			}
			fmt.Println()
		}
	}

	fmt.Printf("---\nTotal: %d items\n", items.Total)

	return nil
}

// printItemTable prints a single item as a table
func (p *Printer) printItemTable(item *api.Item) error {
	tbl := table.New("Property", "Value").WithWriter(os.Stdout)

	if p.useColor {
		tbl.WithHeaderFormatter(func(format string, vals ...interface{}) string {
			return fmt.Sprintf("\033[1m%s\033[0m", fmt.Sprintf(format, vals...))
		})
	}

	tbl.AddRow("ID", item.ID)
	tbl.AddRow("Name", item.Name)
	tbl.AddRow("Description", item.Description)
	tbl.AddRow("Created", formatTime(item.CreatedAt))
	tbl.AddRow("Updated", formatTime(item.UpdatedAt))

	if len(item.Metadata.Tags) > 0 {
		tbl.AddRow("Tags", fmt.Sprintf("%v", item.Metadata.Tags))
	}

	if len(item.Metadata.Attributes) > 0 {
		for k, v := range item.Metadata.Attributes {
			tbl.AddRow(k, v)
		}
	}

	tbl.Print()

	return nil
}

// printItemMarkdown prints a single item as markdown
func (p *Printer) printItemMarkdown(item *api.Item) error {
	fmt.Printf("# %s\n\n", item.Name)
	fmt.Printf("**ID:** %s\n\n", item.ID)

	if item.Description != "" {
		fmt.Printf("**Description:** %s\n\n", item.Description)
	}

	fmt.Printf("**Created:** %s\n\n", item.CreatedAt.Format(time.RFC3339))
	fmt.Printf("**Updated:** %s\n\n", item.UpdatedAt.Format(time.RFC3339))

	if len(item.Metadata.Tags) > 0 {
		fmt.Println("**Tags:**")
		for _, tag := range item.Metadata.Tags {
			fmt.Printf("- %s\n", tag)
		}
		fmt.Println()
	}

	if len(item.Metadata.Attributes) > 0 {
		fmt.Println("**Attributes:**")
		for k, v := range item.Metadata.Attributes {
			fmt.Printf("- %s: %s\n", k, v)
		}
		fmt.Println()
	}

	return nil
}

// PrintTracking prints tracking information in the specified format
func (p *Printer) PrintTracking(tracking *api.TrackingResponse, format string) error {
	if format == "" {
		format = p.format
	}

	switch format {
	case "json":
		return p.printJSON(tracking)
	case "markdown":
		return p.printTrackingMarkdown(tracking)
	case "table":
		return p.printTrackingTable(tracking)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// printTrackingTable prints tracking info as a formatted table
func (p *Printer) printTrackingTable(tracking *api.TrackingResponse) error {
	// Main tracking info
	tbl := table.New("Property", "Value").WithWriter(os.Stdout)

	if p.useColor {
		tbl.WithHeaderFormatter(func(format string, vals ...interface{}) string {
			return fmt.Sprintf("\033[1m%s\033[0m", fmt.Sprintf(format, vals...))
		})
	}

	tbl.AddRow("Tracking Number", tracking.TrackingNumber)
	tbl.AddRow("Status", tracking.StatusDesc)
	tbl.AddRow("Status Code", tracking.StatusCode)
	if tracking.ServiceDesc != "" {
		tbl.AddRow("Service", tracking.ServiceDesc)
	} else if tracking.Service != "" {
		tbl.AddRow("Service", tracking.Service)
	}
	if tracking.PickupDate != "" {
		tbl.AddRow("Pickup Date", tracking.PickupDate)
	}
	if tracking.ScheduledDate != "" {
		tbl.AddRow("Scheduled Delivery", tracking.ScheduledDate)
	}
	if tracking.ActualDate != "" {
		tbl.AddRow("Delivered On", tracking.ActualDate)
	}

	tbl.Print()

	// Current status if different from overall status
	if tracking.CurrentStatus != nil && tracking.CurrentStatus.Description != tracking.StatusDesc {
		fmt.Println()
		fmt.Printf("Current Location: %s\n", tracking.CurrentStatus.Location)
		fmt.Printf("Last Scan: %s\n", tracking.CurrentStatus.Timestamp)
	}

	// Shipment events
	if len(tracking.ShipmentEvents) > 0 {
		fmt.Println()
		fmt.Println("Recent Events:")
		fmt.Println()

		eventTbl := table.New("Date/Time", "Location", "Event").WithWriter(os.Stdout)
		if p.useColor {
			eventTbl.WithHeaderFormatter(func(format string, vals ...interface{}) string {
				return fmt.Sprintf("\033[1m%s\033[0m", fmt.Sprintf(format, vals...))
			})
		}

		// Show last 10 events
		startIdx := 0
		if len(tracking.ShipmentEvents) > 10 {
			startIdx = len(tracking.ShipmentEvents) - 10
		}

		for i := len(tracking.ShipmentEvents) - 1; i >= startIdx; i-- {
			event := tracking.ShipmentEvents[i]
			location := event.Location
			if location == "" && (event.City != "" || event.State != "" || event.Country != "") {
				location = fmt.Sprintf("%s, %s %s", event.City, event.State, event.Country)
				location = strings.TrimSpace(strings.TrimSuffix(location, ","))
			}
			eventTbl.AddRow(
				formatTrackingTime(event.Timestamp),
				location,
				truncate(event.Description, 50),
			)
		}

		eventTbl.Print()
	}

	return nil
}

// printTrackingMarkdown prints tracking info as markdown
func (p *Printer) printTrackingMarkdown(tracking *api.TrackingResponse) error {
	fmt.Printf("# Tracking: %s\n\n", tracking.TrackingNumber)

	fmt.Printf("**Status:** %s (%s)\n\n", tracking.StatusDesc, tracking.StatusCode)

	if tracking.ServiceDesc != "" {
		fmt.Printf("**Service:** %s\n\n", tracking.ServiceDesc)
	}

	if tracking.PickupDate != "" {
		fmt.Printf("**Pickup Date:** %s\n\n", tracking.PickupDate)
	}

	if tracking.ScheduledDate != "" {
		fmt.Printf("**Scheduled Delivery:** %s\n\n", tracking.ScheduledDate)
	}

	if tracking.ActualDate != "" {
		fmt.Printf("**Delivered On:** %s\n\n", tracking.ActualDate)
	}

	if tracking.CurrentStatus != nil {
		fmt.Printf("**Current Location:** %s\n\n", tracking.CurrentStatus.Location)
		fmt.Printf("**Last Scan:** %s\n\n", tracking.CurrentStatus.Timestamp)
	}

	if len(tracking.ShipmentEvents) > 0 {
		fmt.Println("## Shipment Events")
		fmt.Println()

		for _, event := range tracking.ShipmentEvents {
			fmt.Printf("### %s\n\n", formatTrackingTime(event.Timestamp))
			fmt.Printf("- **Event:** %s\n", event.Description)
			if event.Location != "" {
				fmt.Printf("- **Location:** %s\n", event.Location)
			} else if event.City != "" || event.State != "" || event.Country != "" {
				fmt.Printf("- **Location:** %s, %s %s\n", event.City, event.State, event.Country)
			}
			fmt.Println()
		}
	}

	return nil
}

// formatTrackingTime formats a tracking timestamp
func formatTrackingTime(timestamp string) string {
	// Try to parse various time formats
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"01/02/2006 15:04",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timestamp); err == nil {
			return t.Format("2006-01-02 15:04")
		}
	}

	// Return original if parsing fails
	return timestamp
}

// formatTime formats a time for display
func formatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04")
}

// ValidateFormat checks if a format is supported
func ValidateFormat(format string, allowed []string) error {
	for _, f := range allowed {
		if f == format {
			return nil
		}
	}
	return fmt.Errorf("invalid format '%s', must be one of: %v", format, allowed)
}

// ParseBool parses a boolean string
func ParseBool(s string) (bool, error) {
	return strconv.ParseBool(s)
}

// truncate shortens a string to max length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// PrintRate prints rate information in the specified format
func (p *Printer) PrintRate(rate *api.RateResponse, format string) error {
	if format == "" {
		format = p.format
	}

	switch format {
	case "json":
		return p.printJSON(rate)
	case "markdown":
		return p.printRateMarkdown(rate)
	case "table":
		return p.printRateTable(rate)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// printRateTable prints rate info as a formatted table
func (p *Printer) printRateTable(rate *api.RateResponse) error {
	if len(rate.RateResponse.RatedShipment) == 0 {
		fmt.Println("No rates available for this shipment.")
		return nil
	}

	tbl := table.New("Service", "Days", "Total", "Currency").WithWriter(os.Stdout)

	if p.useColor {
		tbl.WithHeaderFormatter(func(format string, vals ...interface{}) string {
			return fmt.Sprintf("\033[1m%s\033[0m", fmt.Sprintf(format, vals...))
		})
	}

	for _, shipment := range rate.RateResponse.RatedShipment {
		days := ""
		if shipment.GuaranteedDelivery != nil {
			days = shipment.GuaranteedDelivery.BusinessDaysInTransit
		}

		tbl.AddRow(
			shipment.Service.Description,
			days,
			shipment.TotalCharges.MonetaryValue,
			shipment.TotalCharges.CurrencyCode,
		)
	}

	tbl.Print()
	fmt.Printf("\nShowing %d service options\n", len(rate.RateResponse.RatedShipment))

	return nil
}

// printRateMarkdown prints rate info as markdown
func (p *Printer) printRateMarkdown(rate *api.RateResponse) error {
	if len(rate.RateResponse.RatedShipment) == 0 {
		fmt.Println("No rates available for this shipment.")
		return nil
	}

	fmt.Println("# Shipping Rates")
	fmt.Println()

	for _, shipment := range rate.RateResponse.RatedShipment {
		fmt.Printf("## %s\n\n", shipment.Service.Description)
		fmt.Printf("- **Service Code:** %s\n", shipment.Service.Code)
		fmt.Printf("- **Total:** %s %s\n", shipment.TotalCharges.MonetaryValue, shipment.TotalCharges.CurrencyCode)

		if shipment.GuaranteedDelivery != nil {
			if shipment.GuaranteedDelivery.BusinessDaysInTransit != "" {
				fmt.Printf("- **Transit Time:** %s business days\n", shipment.GuaranteedDelivery.BusinessDaysInTransit)
			}
			if shipment.GuaranteedDelivery.DeliveryBy != "" {
				fmt.Printf("- **Delivery By:** %s\n", shipment.GuaranteedDelivery.DeliveryBy)
			}
		}

		fmt.Println()
	}

	return nil
}
