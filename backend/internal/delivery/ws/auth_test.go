package ws_test

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"catbrasil/backend/internal/delivery/ws"
	"catbrasil/backend/internal/domain"
	"catbrasil/backend/pkg/jwt"

	"github.com/gorilla/websocket"
)

func TestWebSocketFinancialEventEnvelope(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	tokens := jwt.NewTokenManager("test-secret-for-websocket-123456789", 1)
	token, err := tokens.GenerateToken(&domain.User{ID: "payer", Role: domain.RoleUser})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(hub.HandleWS(tokens))
	defer server.Close()
	dialer := websocket.Dialer{Subprotocols: []string{"bearer", token}}
	conn, _, err := dialer.Dial("ws"+server.URL[4:], nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	time.Sleep(20 * time.Millisecond)

	for _, tc := range []struct {
		event   string
		payload map[string]any
		field   string
		want    float64
	}{
		{"balance_updated", map[string]any{"newBalance": 120.5}, "newBalance", 120.5},
		{"pix_confirmed", map[string]any{"paidAmount": 50.0, "outcome": "applied"}, "paidAmount", 50},
		{"withdraw_paid", map[string]any{"paidAmount": 75.0, "outcome": "applied"}, "paidAmount", 75},
	} {
		hub.NotifyUser("payer", tc.event, tc.payload)
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		var message struct {
			Event   string          `json:"event"`
			Payload json.RawMessage `json:"payload"`
			UserID  string          `json:"userId"`
		}
		if err := conn.ReadJSON(&message); err != nil {
			t.Fatal(err)
		}
		if message.Event != tc.event || message.UserID != "payer" {
			t.Fatalf("envelope incorreto: %+v", message)
		}
		var payload map[string]any
		if err := json.Unmarshal(message.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		if payload[tc.field] != tc.want {
			t.Fatalf("payload de %s incorreto: %+v", tc.event, payload)
		}
	}
}

func TestWebSocketRequiresTokenAndIgnoresForgedRoom(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	tokens := jwt.NewTokenManager("test-secret-for-websocket-123456789", 1)
	server := httptest.NewServer(hub.HandleWS(tokens))
	defer server.Close()
	url := "ws" + server.URL[4:]

	if conn, response, err := websocket.DefaultDialer.Dial(url, nil); err == nil || response == nil || response.StatusCode != http.StatusUnauthorized {
		if conn != nil {
			conn.Close()
		}
		t.Fatalf("conexão sem token deve retornar 401: resposta=%v erro=%v", response, err)
	}

	token, err := tokens.GenerateToken(&domain.User{ID: "user-1", Role: domain.RoleUser})
	if err != nil {
		t.Fatal(err)
	}
	dialer := websocket.Dialer{Subprotocols: []string{"bearer", token}}
	if denied, response, err := dialer.Dial(url, http.Header{"Origin": []string{"https://evil.example.test"}}); err == nil || response == nil || response.StatusCode != http.StatusForbidden {
		if denied != nil {
			denied.Close()
		}
		t.Fatalf("origem cruzada deveria ser recusada: resposta=%v erro=%v", response, err)
	}
	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.WriteJSON(map[string]string{"event": "join_room", "userId": "user-2"}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	hub.NotifyUser("user-2", "forbidden", nil)
	hub.NotifyUser("user-1", "allowed", nil)
	conn.SetReadDeadline(time.Now().Add(time.Second))
	var message ws.WebSocketMessage
	if err := conn.ReadJSON(&message); err != nil {
		t.Fatal(err)
	}
	if message.Event != "allowed" || message.UserID != "user-1" {
		t.Fatalf("mensagem de outro usuário recebida: %+v", message)
	}
	hub.NotifyUser("user-1", "first", map[string]int{"sequence": 1})
	hub.NotifyUser("user-1", "second", map[string]int{"sequence": 2})
	for _, expected := range []string{"first", "second"} {
		var next ws.WebSocketMessage
		if err := conn.ReadJSON(&next); err != nil {
			t.Fatalf("cada evento deve ter seu próprio frame JSON: %v", err)
		}
		if next.Event != expected {
			t.Fatalf("evento %s, esperado %s", next.Event, expected)
		}
	}
}

func TestWebSocketClosesAtTokenExpiry(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	tokens := jwt.NewTokenManager("test-secret-for-websocket-123456789", 1)
	token, err := tokens.GenerateTokenWithTTL(&domain.User{ID: "expiring-user", Role: domain.RoleUser}, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(hub.HandleWS(tokens))
	defer server.Close()
	dialer := websocket.Dialer{Subprotocols: []string{"bearer", token}}
	conn, _, err := dialer.Dial("ws"+server.URL[4:], nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Fatal("WebSocket permaneceu aberto após expiração do JWT")
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		t.Fatalf("WebSocket não fechou no vencimento do JWT: %v", err)
	}
}
