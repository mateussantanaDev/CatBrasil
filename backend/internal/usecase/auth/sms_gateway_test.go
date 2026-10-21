package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"catbrasil/backend/internal/usecase/auth"
)

func TestHTTPGatewayContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer private-token" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("cabeçalhos ou método incorretos")
		}
		var body struct {
			Phone   string `json:"phone"`
			Message string `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.Phone != "+5511999999999" || body.Message != "teste" {
			t.Errorf("payload incorreto: %+v", body)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	if err := (&auth.HTTPGateway{URL: server.URL, Token: "private-token"}).Send(context.Background(), "+5511999999999", "teste"); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPGatewayRejectsRedirectWithoutForwardingCode(t *testing.T) {
	forwarded := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwarded = true
		w.WriteHeader(http.StatusAccepted)
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", target.URL)
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	if err := (&auth.HTTPGateway{URL: source.URL, Token: "private-token"}).Send(context.Background(), "+5511999999999", "código 123456"); err == nil {
		t.Fatal("redirecionamento do gateway foi aceito")
	}
	if forwarded {
		t.Fatal("código foi encaminhado a outro destino")
	}
}
