package middleware_test

import (
	"net/http/httptest"
	"net/netip"
	"testing"

	"catbrasil/backend/internal/delivery/http/middleware"
)

func TestClientIPOnlyTrustsConfiguredProxy(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("172.30.240.2/32")}
	req := httptest.NewRequest("GET", "/api/v1/auth/login", nil)
	req.Header.Set("X-Forwarded-For", "198.51.100.25")
	req.Header.Set("X-Real-IP", "203.0.113.99")
	req.RemoteAddr = "192.0.2.10:43210"
	if got := middleware.ClientIP(req, trusted); got != "192.0.2.10" {
		t.Fatalf("cliente direto forjou IP: %s", got)
	}
	req.RemoteAddr = "172.30.240.2:43567"
	if got := middleware.ClientIP(req, trusted); got != "198.51.100.25" {
		t.Fatalf("proxy confiável não encaminhou IP: %s", got)
	}
	req.Header.Set("X-Forwarded-For", "198.51.100.25, 203.0.113.1")
	if got := middleware.ClientIP(req, trusted); got != "172.30.240.2" {
		t.Fatalf("cadeia não canônica foi aceita: %s", got)
	}
	req.Header.Set("X-Forwarded-For", "bad-ip")
	if got := middleware.ClientIP(req, trusted); got != "172.30.240.2" {
		t.Fatalf("IP inválido foi aceito: %s", got)
	}
}
