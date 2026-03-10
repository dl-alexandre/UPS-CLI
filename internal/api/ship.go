package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// ShipmentRequest represents a UPS shipment creation request
type ShipmentRequest struct {
	ShipmentRequest *ShipmentRequestDetail `json:"ShipmentRequest,omitempty"`
}

type ShipmentRequestDetail struct {
	Request  *ShipmentReqInfo `json:"Request,omitempty"`
	Shipment *ShipmentData    `json:"Shipment,omitempty"`
}

type ShipmentReqInfo struct {
	RequestOption        string          `json:"RequestOption,omitempty"` // "validate" or "nonvalidate"
	TransactionReference *TransactionRef `json:"TransactionReference,omitempty"`
}

type ShipmentData struct {
	Description        string       `json:"Description,omitempty"`
	Shipper            *Party       `json:"Shipper,omitempty"`
	ShipTo             *Party       `json:"ShipTo,omitempty"`
	ShipFrom           *Party       `json:"ShipFrom,omitempty"`
	PaymentInformation *PaymentInfo `json:"PaymentInformation,omitempty"`
	Service            *Service     `json:"Service,omitempty"`
	Package            []Package    `json:"Package,omitempty"`
}

type PaymentInfo struct {
	ShipmentCharge *ShipmentCharge `json:"ShipmentCharge,omitempty"`
}

type ShipmentCharge struct {
	Type        string       `json:"Type,omitempty"` // "01" for transportation
	BillShipper *BillShipper `json:"BillShipper,omitempty"`
}

type BillShipper struct {
	AccountNumber string `json:"AccountNumber,omitempty"`
}

// ShipmentResponse represents the UPS shipment response
type ShipmentResponse struct {
	ShipmentResponse struct {
		Response struct {
			ResponseStatus struct {
				Code        string `json:"Code,omitempty"`
				Description string `json:"Description,omitempty"`
			} `json:"ResponseStatus,omitempty"`
			Alert []struct {
				Code        string `json:"Code,omitempty"`
				Description string `json:"Description,omitempty"`
			} `json:"Alert,omitempty"`
			Error []struct {
				ErrorSeverity string `json:"ErrorSeverity,omitempty"`
				ErrorCode     string `json:"ErrorCode,omitempty"`
				ErrorMessage  string `json:"ErrorMessage,omitempty"`
			} `json:"Error,omitempty"`
		} `json:"Response,omitempty"`
		ShipmentResults *ShipmentResults `json:"ShipmentResults,omitempty"`
	} `json:"ShipmentResponse,omitempty"`
}

type ShipmentResults struct {
	ShipmentCharges       *ShipmentCharges       `json:"ShipmentCharges,omitempty"`
	NegotiatedRateCharges *NegotiatedRateCharges `json:"NegotiatedRateCharges,omitempty"`
	PackageResults        []PackageResult        `json:"PackageResults,omitempty"`
}

type ShipmentCharges struct {
	TransportationCharges TotalCharges `json:"TransportationCharges,omitempty"`
	ServiceOptionsCharges TotalCharges `json:"ServiceOptionsCharges,omitempty"`
	TotalCharges          TotalCharges `json:"TotalCharges,omitempty"`
}

type NegotiatedRateCharges struct {
	TotalCharge TotalCharges `json:"TotalCharge,omitempty"`
}

type PackageResult struct {
	TrackingNumber string         `json:"TrackingNumber,omitempty"`
	ShippingLabel  *ShippingLabel `json:"ShippingLabel,omitempty"`
}

type ShippingLabel struct {
	ImageFormat  string `json:"ImageFormat,omitempty"`
	GraphicImage string `json:"GraphicImage,omitempty"`
}

// ShipValidate validates a shipment without creating it
// Uses SafeRetryConfig: only retries on 429
func (c *Client) ShipValidate(ctx context.Context, req *ShipmentRequest, locale string) (*ShipmentResponse, error) {
	endpoint := "/ship/v1/shipments"

	// Set request option to validate only
	if req.ShipmentRequest != nil && req.ShipmentRequest.Request != nil {
		req.ShipmentRequest.Request.RequestOption = "validate"
	}

	// Add locale query parameter
	if locale == "" {
		locale = "en_US"
	}
	endpoint = endpoint + "?locale=" + locale

	// Use safe retry config (only retry on 429)
	resp, err := c.doRequestWithRetry(ctx, "POST", endpoint, req, SafeRetryConfig())
	if err != nil {
		return nil, err
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, ParseUPSError(resp)
	}

	var result ShipmentResponse
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("failed to decode shipment response: %w", err)
	}

	// Check for UPS API errors in response body
	if len(result.ShipmentResponse.Response.Error) > 0 {
		errInfo := result.ShipmentResponse.Response.Error[0]
		return nil, &UPSError{
			StatusCode: resp.StatusCode(),
			Code:       errInfo.ErrorCode,
			Message:    errInfo.ErrorMessage,
		}
	}

	return &result, nil
}

// ShipValidateRaw returns the raw validation response body (for debugging)
func (c *Client) ShipValidateRaw(ctx context.Context, req *ShipmentRequest, locale string) ([]byte, error) {
	endpoint := "/ship/v1/shipments"

	// Set request option to validate only
	if req.ShipmentRequest != nil && req.ShipmentRequest.Request != nil {
		req.ShipmentRequest.Request.RequestOption = "validate"
	}

	// Add locale query parameter
	if locale == "" {
		locale = "en_US"
	}
	endpoint = endpoint + "?locale=" + locale

	resp, err := c.doRequestWithRetry(ctx, "POST", endpoint, req, SafeRetryConfig())
	if err != nil {
		return nil, err
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, ParseUPSError(resp)
	}

	return resp.Body(), nil
}

// NewShipmentRequest creates a shipment request with the required fields
func NewShipmentRequest(fromPostal, toPostal, shipperPostal, countryFrom, countryTo, weight, weightUnit, serviceCode, accountNumber string) (*ShipmentRequest, error) {
	// Validate and format weight
	weightFloat, err := strconv.ParseFloat(weight, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid weight: %w", err)
	}
	if weightFloat <= 0 {
		return nil, fmt.Errorf("weight must be greater than 0")
	}
	// Format with 2 decimal places
	weightStr := fmt.Sprintf("%.2f", weightFloat)

	// Set defaults
	if shipperPostal == "" {
		shipperPostal = fromPostal
	}
	if countryFrom == "" {
		countryFrom = "US"
	}
	if countryTo == "" {
		countryTo = "US"
	}
	if weightUnit == "" {
		weightUnit = "LBS"
	}
	// Normalize weight unit to uppercase
	weightUnit = strings.ToUpper(weightUnit)

	req := &ShipmentRequest{
		ShipmentRequest: &ShipmentRequestDetail{
			Request: &ShipmentReqInfo{
				RequestOption: "validate", // Default to validate
				TransactionReference: &TransactionRef{
					CustomerContext: "ups-cli shipment validation",
				},
			},
			Shipment: &ShipmentData{
				Description: "UPS CLI Shipment",
				Shipper: &Party{
					Address: &Address{
						PostalCode:  shipperPostal,
						CountryCode: countryFrom,
					},
				},
				ShipFrom: &Party{
					Address: &Address{
						PostalCode:  fromPostal,
						CountryCode: countryFrom,
					},
				},
				ShipTo: &Party{
					Address: &Address{
						PostalCode:  toPostal,
						CountryCode: countryTo,
					},
				},
				Package: []Package{
					{
						PackagingType: &PackagingType{
							Code: "02", // Package
						},
						PackageWeight: &PackageWeight{
							UnitOfMeasurement: &UnitOfMeasurement{
								Code: weightUnit,
							},
							Weight: weightStr,
						},
					},
				},
			},
		},
	}

	// Add service code if specified
	if serviceCode != "" {
		req.ShipmentRequest.Shipment.Service = &Service{
			Code: serviceCode,
		}
	}

	// Add payment info if account number provided
	if accountNumber != "" {
		req.ShipmentRequest.Shipment.PaymentInformation = &PaymentInfo{
			ShipmentCharge: &ShipmentCharge{
				Type: "01", // Transportation
				BillShipper: &BillShipper{
					AccountNumber: accountNumber,
				},
			},
		}
	}

	return req, nil
}
