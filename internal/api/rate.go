package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// RateRequest represents a minimal UPS rate request
// Using pointers and omitempty to avoid sending empty fields
type RateRequest struct {
	RateRequest *RateRequestDetail `json:"RateRequest,omitempty"`
}

type RateRequestDetail struct {
	Request  *RequestInfo  `json:"Request,omitempty"`
	Shipment *ShipmentInfo `json:"Shipment,omitempty"`
}

type RequestInfo struct {
	TransactionReference *TransactionRef `json:"TransactionReference,omitempty"`
}

type TransactionRef struct {
	CustomerContext       string `json:"CustomerContext,omitempty"`
	TransactionIdentifier string `json:"TransactionIdentifier,omitempty"`
}

type ShipmentInfo struct {
	Shipper  *Party    `json:"Shipper,omitempty"`
	ShipTo   *Party    `json:"ShipTo,omitempty"`
	ShipFrom *Party    `json:"ShipFrom,omitempty"`
	Service  *Service  `json:"Service,omitempty"`
	Package  []Package `json:"Package,omitempty"`
}

type Party struct {
	Address *Address `json:"Address,omitempty"`
}

type Address struct {
	PostalCode  string `json:"PostalCode,omitempty"`
	CountryCode string `json:"CountryCode,omitempty"`
}

type Service struct {
	Code string `json:"Code,omitempty"`
}

type Package struct {
	PackagingType *PackagingType `json:"PackagingType,omitempty"`
	PackageWeight *PackageWeight `json:"PackageWeight,omitempty"`
}

type PackagingType struct {
	Code string `json:"Code,omitempty"`
}

type PackageWeight struct {
	UnitOfMeasurement *UnitOfMeasurement `json:"UnitOfMeasurement,omitempty"`
	Weight            string             `json:"Weight,omitempty"`
}

type UnitOfMeasurement struct {
	Code string `json:"Code,omitempty"`
}

// RateResponse represents the UPS rate response
type RateResponse struct {
	RateResponse struct {
		RatedShipment []RatedShipment `json:"RatedShipment,omitempty"`
	} `json:"RateResponse,omitempty"`
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
}

// RatedShipment represents a rated shipping option
type RatedShipment struct {
	Service              ServiceDetail         `json:"Service,omitempty"`
	RatedShipmentDetails []RatedShipmentDetail `json:"RatedShipmentDetails,omitempty"`
	TotalCharges         TotalCharges          `json:"TotalCharges,omitempty"`
	GuaranteedDelivery   *GuaranteedDelivery   `json:"GuaranteedDelivery,omitempty"`
	RatedPackage         []RatedPackage        `json:"RatedPackage,omitempty"`
}

type ServiceDetail struct {
	Code        string `json:"Code,omitempty"`
	Description string `json:"Description,omitempty"`
}

type RatedShipmentDetail struct {
	RateType     string       `json:"RateType,omitempty"`
	TotalCharges TotalCharges `json:"TotalCharges,omitempty"`
}

type TotalCharges struct {
	CurrencyCode  string `json:"CurrencyCode,omitempty"`
	MonetaryValue string `json:"MonetaryValue,omitempty"`
}

type GuaranteedDelivery struct {
	BusinessDaysInTransit string `json:"BusinessDaysInTransit,omitempty"`
	DeliveryBy            string `json:"DeliveryBy,omitempty"`
}

type RatedPackage struct {
	PackageWeight         *PackageWeight `json:"PackageWeight,omitempty"`
	TransportationCharges TotalCharges   `json:"TransportationCharges,omitempty"`
}

// Rate calculates shipping rates for a shipment
func (c *Client) Rate(ctx context.Context, req *RateRequest, locale string) (*RateResponse, error) {
	endpoint := "/rating/v1/Rate"

	// Add locale query parameter
	if locale == "" {
		locale = "en_US"
	}
	endpoint = endpoint + "?locale=" + locale

	resp, err := c.doRequest(ctx, "POST", endpoint, req)
	if err != nil {
		return nil, err
	}

	// Check for error status codes
	if resp.StatusCode() != http.StatusOK {
		return nil, ParseUPSError(resp)
	}

	var result RateResponse
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("failed to decode rate response: %w", err)
	}

	// Check for UPS API errors in response body
	if len(result.Response.Error) > 0 {
		errInfo := result.Response.Error[0]
		return nil, &UPSError{
			StatusCode: resp.StatusCode(),
			Code:       errInfo.ErrorCode,
			Message:    errInfo.ErrorMessage,
		}
	}

	return &result, nil
}

// RateRaw returns the raw rate response body (for debugging)
func (c *Client) RateRaw(ctx context.Context, req *RateRequest, locale string) ([]byte, error) {
	endpoint := "/rating/v1/Rate"

	// Add locale query parameter
	if locale == "" {
		locale = "en_US"
	}
	endpoint = endpoint + "?locale=" + locale

	resp, err := c.doRequest(ctx, "POST", endpoint, req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, ParseUPSError(resp)
	}

	return resp.Body(), nil
}

// NewRateRequest creates a minimal rate request with the required fields
func NewRateRequest(fromPostal, toPostal, shipperPostal, countryFrom, countryTo, weight, weightUnit, serviceCode string) *RateRequest {
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

	req := &RateRequest{
		RateRequest: &RateRequestDetail{
			Request: &RequestInfo{
				TransactionReference: &TransactionRef{
					CustomerContext: "ups-cli rate request",
				},
			},
			Shipment: &ShipmentInfo{
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
							Weight: weight,
						},
					},
				},
			},
		},
	}

	// Add service code if specified
	if serviceCode != "" {
		req.RateRequest.Shipment.Service = &Service{
			Code: serviceCode,
		}
	}

	return req
}
