package domain_test

import (
	"testing"

	"catbrasil/backend/internal/domain"
)

func TestRoulette_CalculateExpectedValue(t *testing.T) {
	slices := []*domain.RouletteSlice{
		{ID: 1, Label: "Motor C15", Amount: 0.00, Probability: 20.0},
		{ID: 2, Label: "Esteira CAT", Amount: 0.00, Probability: 20.0},
		{ID: 3, Label: "Filtro Óleo", Amount: 0.00, Probability: 25.0},
		{ID: 4, Label: "Turbo Garrett", Amount: 0.00, Probability: 10.0},
		{ID: 5, Label: "Caixa CAT", Amount: 0.00, Probability: 15.0},
		{ID: 6, Label: "R$ 5,00", Amount: 5.00, Probability: 10.0},
	}

	ev := domain.CalculateExpectedValue(slices)
	// EV: (0 * 0.20) + (0 * 0.20) + (0 * 0.25) + (0 * 0.10) + (0 * 0.15) + (5.00 * 0.10) = 0.50
	expected := 0.50

	if ev != expected {
		t.Errorf("CalculateExpectedValue() = %v, want %v", ev, expected)
	}
}

func TestRoulette_ValidateProbabilities(t *testing.T) {
	validSlices := []*domain.RouletteSlice{
		{Probability: 20.0},
		{Probability: 20.0},
		{Probability: 25.0},
		{Probability: 10.0},
		{Probability: 15.0},
		{Probability: 10.0},
	}

	total, isValid := domain.ValidateProbabilities(validSlices)
	if !isValid || total != 100.0 {
		t.Errorf("ValidateProbabilities(validSlices) total = %v, isValid = %v; want 100.0, true", total, isValid)
	}

	invalidSlices := []*domain.RouletteSlice{
		{Probability: 50.0},
		{Probability: 40.0},
	}

	totalInvalid, isInvalid := domain.ValidateProbabilities(invalidSlices)
	if isInvalid || totalInvalid != 90.0 {
		t.Errorf("ValidateProbabilities(invalidSlices) total = %v, isValid = %v; want 90.0, false", totalInvalid, isInvalid)
	}
}
