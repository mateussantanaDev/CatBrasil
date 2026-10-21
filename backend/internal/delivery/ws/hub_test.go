package ws_test

import (
	"testing"
	"time"

	"catbrasil/backend/internal/delivery/ws"
)

func TestHub_CreationAndBroadcast(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()

	// Testa envio de notificação individual sem travar
	hub.NotifyUser("CB-458F", "pix_confirmed", map[string]interface{}{
		"amount": 500.00,
	})

	// Testa broadcast global sem travar
	hub.Broadcast("announcement_new", map[string]interface{}{
		"title": "Novo Comunicado",
	})

	time.Sleep(50 * time.Millisecond)
}
