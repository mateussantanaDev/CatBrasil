package money

import (
	"math"
	"math/rand"
	"testing"
)

func TestBoundaryAndFractionalCents(t *testing.T) {
	for _, tc := range []struct {
		value    float64
		expected Cents
	}{
		{0.01, 1}, {0.1 + 0.2, 30}, {9_999_999_999_999.99, MaxCents}, {-9_999_999_999_999.99, -MaxCents},
	} {
		got, err := FromFloat64(tc.value)
		if err != nil || got != tc.expected || got.String() == "" {
			t.Fatalf("value %v: got %v, %v", tc.value, got, err)
		}
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), 1.001, 9_999_999_999_999.995, 10_000_000_000_000, -10_000_000_000_000} {
		if _, err := FromFloat64(value); err == nil {
			t.Fatalf("accepted %v", value)
		}
	}
}

func TestPercentExactRounding(t *testing.T) {
	for _, tc := range []struct {
		amount   Cents
		bps      int64
		expected Cents
	}{
		{101, 500, 5}, {110, 500, 6}, {MaxCents, 500, 50_000_000_000_000},
		{MaxCents, 10_000, MaxCents}, {MaxCents, 9_300, 929_999_999_999_999},
	} {
		got, err := tc.amount.Percent(tc.bps)
		if err != nil || got != tc.expected {
			t.Fatalf("percent %d/%d: %d, %v", tc.amount, tc.bps, got, err)
		}
	}
}

func TestEverySampledCentSurvivesLegacyFloatBoundary(t *testing.T) {
	values := []Cents{0, 1, 2, 99, 100, MaxCents - 2, MaxCents - 1, MaxCents}
	random := rand.New(rand.NewSource(20261020))
	for i := 0; i < 100_000; i++ {
		values = append(values, Cents(random.Int63n(MaxCents+1)))
	}
	for _, value := range values {
		got, err := FromFloat64(value.Float64())
		if err != nil || got != value {
			t.Fatalf("centavo %d perdido na fronteira float64: %d, %v", value, got, err)
		}
	}
}
