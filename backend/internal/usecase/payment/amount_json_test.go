package payment_test

import (
	"encoding/json"
	"testing"

	"catbrasil/backend/internal/usecase/payment"
)

func TestFinancialRequestJSONRejectsFractionalCentLexeme(t *testing.T) {
	for _, raw := range []string{`{"amount":0.01000005,"pixKey":"x"}`, `{"amount":9999999999999.995,"pixKey":"x"}`} {
		var payout payment.PayoutRequest
		if err := json.Unmarshal([]byte(raw), &payout); err == nil {
			t.Fatalf("accepted payout %s", raw)
		}
	}
	var payout payment.PayoutRequest
	if err := json.Unmarshal([]byte(`{"amount":1e-2,"pixKey":"x"}`), &payout); err != nil || payout.Amount != 0.01 {
		t.Fatalf("exact exponent rejected: %+v, %v", payout, err)
	}
	var webhook payment.WebhookPayload
	if err := json.Unmarshal([]byte(`{"paidAmount":0.01000005,"txId":"x"}`), &webhook); err == nil {
		t.Fatal("accepted sub-cent webhook")
	}
}
