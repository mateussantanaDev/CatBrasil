package domain_test

import (
	"testing"

	"catbrasil/backend/internal/domain"
)

func TestUser_CanLevelUp(t *testing.T) {
	tests := []struct {
		name                 string
		user                 domain.User
		activeReferralsCount int
		expected             bool
	}{
		{
			name: "All requirements met",
			user: domain.User{
				ColetasCompleted: 5,
				TratorRented:     true,
			},
			activeReferralsCount: 3,
			expected:             true,
		},
		{
			name: "Insufficient coletas",
			user: domain.User{
				ColetasCompleted: 4,
				TratorRented:     true,
			},
			activeReferralsCount: 3,
			expected:             false,
		},
		{
			name: "Trator D8T not rented",
			user: domain.User{
				ColetasCompleted: 5,
				TratorRented:     false,
			},
			activeReferralsCount: 3,
			expected:             false,
		},
		{
			name: "Insufficient active referrals",
			user: domain.User{
				ColetasCompleted: 5,
				TratorRented:     true,
			},
			activeReferralsCount: 2,
			expected:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.user.CanLevelUp(tt.activeReferralsCount)
			if got != tt.expected {
				t.Errorf("CanLevelUp() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestUser_CalculateYieldWithLevelBonus(t *testing.T) {
	tests := []struct {
		name      string
		nivel     int
		baseYield float64
		expected  float64
	}{
		{
			name:      "Nivel 1 (no bonus)",
			nivel:     1,
			baseYield: 110.00,
			expected:  110.00,
		},
		{
			name:      "Nivel 8 (+35% bonus: 110 * 1.35 = 148.50)",
			nivel:     8,
			baseYield: 110.00,
			expected:  148.50,
		},
		{
			name:      "Nivel 10 (+45% bonus: 75 * 1.45 = 108.75)",
			nivel:     10,
			baseYield: 75.00,
			expected:  108.75,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := domain.User{Nivel: tt.nivel}
			got := u.CalculateYieldWithLevelBonus(tt.baseYield)
			if got != tt.expected {
				t.Errorf("CalculateYieldWithLevelBonus() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestUser_RolloverProgressPercent(t *testing.T) {
	tests := []struct {
		name     string
		req      float64
		curr     float64
		expected float64
	}{
		{
			name:     "No rollover required",
			req:      0,
			curr:     0,
			expected: 100.0,
		},
		{
			name:     "50% completed (500 of 1000)",
			req:      1000.0,
			curr:     500.0,
			expected: 50.0,
		},
		{
			name:     "100% completed (1200 of 1000)",
			req:      1000.0,
			curr:     1200.0,
			expected: 100.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := domain.User{
				RequiredRollover: tt.req,
				CurrentRollover:  tt.curr,
			}
			got := u.RolloverProgressPercent()
			if got != tt.expected {
				t.Errorf("RolloverProgressPercent() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestUser_PartsAssembly(t *testing.T) {
	u := domain.User{
		MotorC15Count:         1,
		EsteiraReforcadaCount: 1,
		FiltroOleoCount:       1,
		TurboGarrettCount:     1,
	}

	if !u.HasCompleteParts() {
		t.Errorf("expected HasCompleteParts() to be true")
	}

	success := u.ConsumePartsForAssembly()
	if !success {
		t.Errorf("expected ConsumePartsForAssembly() to succeed")
	}

	if u.MotorC15Count != 0 || u.EsteiraReforcadaCount != 0 || u.FiltroOleoCount != 0 || u.TurboGarrettCount != 0 {
		t.Errorf("expected all parts to be 0, got %+v", u.GetPartsInventory())
	}

	if u.HasCompleteParts() {
		t.Errorf("expected HasCompleteParts() to be false after consumption")
	}
}
