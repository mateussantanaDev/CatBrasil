package handler

import (
	"strings"
	"testing"
)

func TestAdminMoneyDecoderRejectsSubCentBeforeFloat(t *testing.T) {
	for _, tc := range []struct{ body, path string }{
		{`{"amount":0.01000005}`, "amount"},
		{`{"catalog":[{"price":1,"dailyYield":0.01000005}]}`, "catalog[].dailyYield"},
		{`{"slices":[{"amount":0.01000005}]}`, "slices[].amount"},
		{`{"customPrice":0.01000005}`, "customPrice"},
		{`{"withdrawFeeFixed":0.01000005}`, "withdrawFeeFixed"},
		{`{"autoApproveWithdrawalsUnder":0.01000005}`, "autoApproveWithdrawalsUnder"},
		{`{"bonusPerMutiraoAmount":0.01000005}`, "bonusPerMutiraoAmount"},
	} {
		var decoded map[string]any
		if err := decodeExactMonetaryJSON(strings.NewReader(tc.body), &decoded, tc.path); err == nil {
			t.Fatalf("accepted fractional cent at %s: %s", tc.path, tc.body)
		}
	}
	var decoded struct {
		Amount float64 `json:"amount"`
	}
	if err := decodeExactMonetaryJSON(strings.NewReader(`{"amount":1e-2}`), &decoded, "amount"); err != nil || decoded.Amount != 0.01 {
		t.Fatalf("exact cent exponent rejected: %+v, %v", decoded, err)
	}
}
