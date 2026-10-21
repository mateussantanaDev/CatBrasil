package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestSuitPayCallbackRejectsFractionalCentLexeme(t *testing.T) {
	var callback SuitPayWebhookCallback
	if err := json.Unmarshal([]byte(`{"idTransaction":9007199254740993,"value":0.01000005,"statusTransaction":"PAID_OUT"}`), &callback); err == nil {
		t.Fatal("accepted sub-cent SuitPay callback")
	}
	if err := json.Unmarshal([]byte(`{"idTransaction":9007199254740993,"value":0.01,"statusTransaction":"PAID_OUT"}`), &callback); err != nil {
		t.Fatal(err)
	}
	if id, ok := callback.IDTransaction.(json.Number); !ok || id.String() != "9007199254740993" {
		t.Fatalf("id externo perdeu precisão: %T %v", callback.IDTransaction, callback.IDTransaction)
	}
}

type captureTransport struct{ payload map[string]any }

func (c *captureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	defer req.Body.Close()
	if err := json.NewDecoder(req.Body).Decode(&c.payload); err != nil {
		return nil, err
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"response":"OK","idTransaction":"provider-123"}`)), Header: make(http.Header)}, nil
}

func TestCashoutIncludesGatewayDeduplicationID(t *testing.T) {
	client := NewSuitPayClient("client", "secret", true, "https://example.com/webhook")
	capture := &captureTransport{}
	client.HTTPClient = &http.Client{Transport: capture}
	response, err := client.TransferPix(context.Background(), SuitPayPixTransferRequest{
		Value:              95,
		Key:                "52998224725",
		DocumentValidation: "529.982.247-25",
		ExternalID:         "WDR-123",
	})
	if err != nil || response.IDTransaction != "provider-123" {
		t.Fatalf("resposta: %+v %v", response, err)
	}
	if capture.payload["externalId"] != "WDR-123" || capture.payload["typeKey"] != "document" {
		t.Fatalf("requisição sem proteção de duplicidade: %+v", capture.payload)
	}
	if capture.payload["documentValidation"] != "52998224725" {
		t.Fatalf("documentValidation ausente ou não normalizado no payload: %+v", capture.payload)
	}
}

func TestTransferPix_DocumentValidationFailClosed(t *testing.T) {
	client := NewSuitPayClient("client", "secret", true, "https://example.com/webhook")
	capture := &captureTransport{}
	client.HTTPClient = &http.Client{Transport: capture}

	// 1. Missing documentValidation fails closed
	_, err := client.TransferPix(context.Background(), SuitPayPixTransferRequest{
		Value:      50,
		Key:        "test@example.com",
		ExternalID: "WDR-M-1",
	})
	if err == nil {
		t.Fatal("esperado erro para documentValidation ausente")
	}

	// 2. Invalid CPF fails closed (bad check digit)
	_, err = client.TransferPix(context.Background(), SuitPayPixTransferRequest{
		Value:              50,
		Key:                "test@example.com",
		DocumentValidation: "11144477700", // invalid check digits
		ExternalID:         "WDR-M-2",
	})
	if err == nil {
		t.Fatal("esperado erro para documentValidation com CPF inválido")
	}

	// 3. Missing externalId fails closed
	_, err = client.TransferPix(context.Background(), SuitPayPixTransferRequest{
		Value:              50,
		Key:                "test@example.com",
		DocumentValidation: "52998224725",
		ExternalID:         "",
	})
	if err == nil {
		t.Fatal("esperado erro para externalId ausente")
	}

	// 4. Ensure no HTTP call was dispatched
	if len(capture.payload) != 0 {
		t.Fatalf("gateway efetuou requisição HTTP indevida após falha de validação: %+v", capture.payload)
	}
}

func TestVerifySuitPayWebhook(t *testing.T) {
	values := "42PIXPAID_OUT100.00PIX-42"
	digest := sha256.Sum256([]byte("client-secret" + values))
	hash := hex.EncodeToString(digest[:])
	raw := []byte(fmt.Sprintf(`{"idTransaction":42,"typeTransaction":"PIX","statusTransaction":"PAID_OUT","value":100.00,"requestNumber":"PIX-42","hash":"%s"}`, hash))
	callback, err := VerifySuitPayWebhook(raw, "client-secret")
	if err != nil || callback.RequestNumber != "PIX-42" || callback.Value != 100 {
		t.Fatalf("callback válido rejeitado: %+v, %v", callback, err)
	}
	if fmt.Sprint(callback.IDTransaction) != "42" {
		t.Fatalf("idTransaction numérico pequeno deveria virar \"42\", obteve %q", fmt.Sprint(callback.IDTransaction))
	}
	if _, err := VerifySuitPayWebhook([]byte(strings.Replace(string(raw), "100.00", "900.00", 1)), "client-secret"); err == nil {
		t.Fatal("alteração do valor deve invalidar hash")
	}
	if _, err := VerifySuitPayWebhook([]byte(strings.Replace(string(raw), hash, "", 1)), "client-secret"); err == nil {
		t.Fatal("hash ausente deve ser rejeitado")
	}
}

func TestVerifySuitPayWebhook_LargeNumericIDKeepsExactExternalID(t *testing.T) {
	// 2^53+1: smallest integer that float64 cannot represent exactly.
	const largeID = "9007199254740993"
	values := largeID + "PIX" + "PAID_OUT" + "150.00" + "PIX-BIG-1"
	digest := sha256.Sum256([]byte("client-secret" + values))
	hash := hex.EncodeToString(digest[:])
	raw := []byte(fmt.Sprintf(`{"idTransaction":%s,"typeTransaction":"PIX","statusTransaction":"PAID_OUT","value":150.00,"requestNumber":"PIX-BIG-1","hash":"%s"}`, largeID, hash))

	callback, err := VerifySuitPayWebhook(raw, "client-secret")
	if err != nil {
		t.Fatalf("callback com id numérico grande rejeitado: %v", err)
	}
	if _, isFloat := callback.IDTransaction.(float64); isFloat {
		t.Fatalf("idTransaction numérico não pode ser decodificado como float64: %#v", callback.IDTransaction)
	}
	got := fmt.Sprint(callback.IDTransaction)
	if got != largeID {
		t.Fatalf("external ID numérico perdeu precisão: esperado %s, obteve %s", largeID, got)
	}
}

func TestVerifySuitPayWebhook_StringIDKeepsExactExternalID(t *testing.T) {
	const stringID = "9007199254740993"
	values := stringID + "PIX" + "CHARGEBACK" + "80.00" + "PIX-STR-1"
	digest := sha256.Sum256([]byte("client-secret" + values))
	hash := hex.EncodeToString(digest[:])
	raw := []byte(fmt.Sprintf(`{"idTransaction":"%s","typeTransaction":"PIX","statusTransaction":"CHARGEBACK","value":80.00,"requestNumber":"PIX-STR-1","hash":"%s"}`, stringID, hash))

	callback, err := VerifySuitPayWebhook(raw, "client-secret")
	if err != nil {
		t.Fatalf("callback com id string rejeitado: %v", err)
	}
	got := fmt.Sprint(callback.IDTransaction)
	if got != stringID {
		t.Fatalf("external ID string alterado: esperado %s, obteve %s", stringID, got)
	}
	if callback.StatusTransaction != "CHARGEBACK" {
		t.Fatalf("status inesperado: %s", callback.StatusTransaction)
	}
}
