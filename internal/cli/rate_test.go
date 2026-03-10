package cli

import (
	"testing"
)

func TestRateCmd_validate(t *testing.T) {
	tests := []struct {
		name    string
		cmd     RateCmd
		wantErr bool
	}{
		{
			name: "valid request",
			cmd: RateCmd{
				FromPostal:  "90210",
				ToPostal:    "10001",
				Weight:      "5.5",
				CountryFrom: "US",
				CountryTo:   "US",
				WeightUnit:  "LBS",
			},
			wantErr: false,
		},
		{
			name: "zero weight",
			cmd: RateCmd{
				FromPostal:  "90210",
				ToPostal:    "10001",
				Weight:      "0",
				CountryFrom: "US",
				CountryTo:   "US",
				WeightUnit:  "LBS",
			},
			wantErr: true,
		},
		{
			name: "negative weight",
			cmd: RateCmd{
				FromPostal:  "90210",
				ToPostal:    "10001",
				Weight:      "-5",
				CountryFrom: "US",
				CountryTo:   "US",
				WeightUnit:  "LBS",
			},
			wantErr: true,
		},
		{
			name: "invalid weight format",
			cmd: RateCmd{
				FromPostal:  "90210",
				ToPostal:    "10001",
				Weight:      "abc",
				CountryFrom: "US",
				CountryTo:   "US",
				WeightUnit:  "LBS",
			},
			wantErr: true,
		},
		{
			name: "empty origin postal",
			cmd: RateCmd{
				FromPostal:  "",
				ToPostal:    "10001",
				Weight:      "5",
				CountryFrom: "US",
				CountryTo:   "US",
				WeightUnit:  "LBS",
			},
			wantErr: true,
		},
		{
			name: "empty destination postal",
			cmd: RateCmd{
				FromPostal:  "90210",
				ToPostal:    "",
				Weight:      "5",
				CountryFrom: "US",
				CountryTo:   "US",
				WeightUnit:  "LBS",
			},
			wantErr: true,
		},
		{
			name: "invalid country code - too long",
			cmd: RateCmd{
				FromPostal:  "90210",
				ToPostal:    "10001",
				Weight:      "5",
				CountryFrom: "USA",
				CountryTo:   "US",
				WeightUnit:  "LBS",
			},
			wantErr: true,
		},
		{
			name: "invalid country code - numbers",
			cmd: RateCmd{
				FromPostal:  "90210",
				ToPostal:    "10001",
				Weight:      "5",
				CountryFrom: "U1",
				CountryTo:   "US",
				WeightUnit:  "LBS",
			},
			wantErr: true,
		},
		{
			name: "invalid weight unit",
			cmd: RateCmd{
				FromPostal:  "90210",
				ToPostal:    "10001",
				Weight:      "5",
				CountryFrom: "US",
				CountryTo:   "US",
				WeightUnit:  "OZ",
			},
			wantErr: true,
		},
		{
			name: "KGS weight unit valid",
			cmd: RateCmd{
				FromPostal:  "90210",
				ToPostal:    "10001",
				Weight:      "5",
				CountryFrom: "US",
				CountryTo:   "US",
				WeightUnit:  "KGS",
			},
			wantErr: false,
		},
		{
			name: "lowercase country codes",
			cmd: RateCmd{
				FromPostal:  "90210",
				ToPostal:    "10001",
				Weight:      "5",
				CountryFrom: "us",
				CountryTo:   "ca",
				WeightUnit:  "LBS",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cmd.validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsValidCountryCode(t *testing.T) {
	tests := []struct {
		name string
		code string
		want bool
	}{
		{"valid US", "US", true},
		{"valid CA", "CA", true},
		{"valid lowercase", "us", true},
		{"too short", "U", false},
		{"too long", "USA", false},
		{"with number", "U1", false},
		{"with symbol", "U-", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidCountryCode(tt.code)
			if got != tt.want {
				t.Errorf("isValidCountryCode(%q) = %v, want %v", tt.code, got, tt.want)
			}
		})
	}
}
