package utils

import (
	"testing"
)

func TestIsProduction(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     bool
	}{
		{
			name:     "Production lowercase",
			envValue: "production",
			want:     true,
		},
		{
			name:     "Production uppercase",
			envValue: "PRODUCTION",
			want:     true,
		},
		{
			name:     "Prod lowercase",
			envValue: "prod",
			want:     true,
		},
		{
			name:     "Prod uppercase",
			envValue: "PROD",
			want:     true,
		},
		{
			name:     "Development lowercase",
			envValue: "development",
			want:     false,
		},
		{
			name:     "Dev lowercase",
			envValue: "dev",
			want:     false,
		},
		{
			name:     "Empty string",
			envValue: "",
			want:     false,
		},
		{
			name:     "Whitespace trimmed",
			envValue: "  production  ",
			want:     true,
		},
		{
			name:     "Random value",
			envValue: "staging",
			want:     false,
		},
		{
			name:     "Mixed case",
			envValue: "Production",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsProduction(tt.envValue); got != tt.want {
				t.Errorf("IsProduction(%q) = %v, want %v", tt.envValue, got, tt.want)
			}
		})
	}
}

func TestIsDevelopment(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     bool
	}{
		{
			name:     "Development lowercase",
			envValue: "development",
			want:     true,
		},
		{
			name:     "Development uppercase",
			envValue: "DEVELOPMENT",
			want:     true,
		},
		{
			name:     "Dev lowercase",
			envValue: "dev",
			want:     true,
		},
		{
			name:     "Dev uppercase",
			envValue: "DEV",
			want:     true,
		},
		{
			name:     "French spelling",
			envValue: "developpement",
			want:     true,
		},
		{
			name:     "Production lowercase",
			envValue: "production",
			want:     false,
		},
		{
			name:     "Prod lowercase",
			envValue: "prod",
			want:     false,
		},
		{
			name:     "Empty string",
			envValue: "",
			want:     false,
		},
		{
			name:     "Whitespace trimmed",
			envValue: "  dev  ",
			want:     true,
		},
		{
			name:     "Random value",
			envValue: "staging",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDevelopment(tt.envValue); got != tt.want {
				t.Errorf("IsDevelopment(%q) = %v, want %v", tt.envValue, got, tt.want)
			}
		})
	}
}

func TestEnvDetectionConsistency(t *testing.T) {
	tests := []struct {
		name          string
		envValue      string
		wantProd      bool
		wantDev       bool
		shouldBeValid bool
	}{
		{
			name:          "Production",
			envValue:      "production",
			wantProd:      true,
			wantDev:       false,
			shouldBeValid: true,
		},
		{
			name:          "Development",
			envValue:      "development",
			wantProd:      false,
			wantDev:       true,
			shouldBeValid: true,
		},
		{
			name:          "Dev",
			envValue:      "dev",
			wantProd:      false,
			wantDev:       true,
			shouldBeValid: true,
		},
		{
			name:          "Prod",
			envValue:      "prod",
			wantProd:      true,
			wantDev:       false,
			shouldBeValid: true,
		},
		{
			name:          "Unknown environment",
			envValue:      "staging",
			wantProd:      false,
			wantDev:       false,
			shouldBeValid: false,
		},
		{
			name:          "Empty environment",
			envValue:      "",
			wantProd:      false,
			wantDev:       false,
			shouldBeValid: false,
		},
		{
			name:          "French development spelling",
			envValue:      "developpement",
			wantProd:      false,
			wantDev:       true,
			shouldBeValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isProd := IsProduction(tt.envValue)
			isDev := IsDevelopment(tt.envValue)

			if isProd != tt.wantProd {
				t.Errorf("IsProduction(%q) = %v, want %v", tt.envValue, isProd, tt.wantProd)
			}
			if isDev != tt.wantDev {
				t.Errorf("IsDevelopment(%q) = %v, want %v", tt.envValue, isDev, tt.wantDev)
			}

			// Ensure production and development are mutually exclusive
			if isProd && isDev {
				t.Error("IsProduction() and IsDevelopment() should not both be true")
			}
		})
	}
}
