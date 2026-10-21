package money

import "testing"

func TestParseJSONNumberPreservesSubCentLexeme(t *testing.T) {
	for raw, want := range map[string]Cents{
		"0.01": 1, "0.010000": 1, "1e-2": 1, "100e-4": 1,
		"0": 0, "-0.01": -1, "9999999999999.99": MaxCents,
	} {
		got, err := ParseJSONNumber(raw)
		if err != nil || got != want {
			t.Fatalf("%s => %d, %v; want %d", raw, got, err, want)
		}
	}
	for _, raw := range []string{"0.01000005", "0.005", "9999999999999.995", "10000000000000", "1e-3", "1e1000", "01", `"1.00"`} {
		if _, err := ParseJSONNumber(raw); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
