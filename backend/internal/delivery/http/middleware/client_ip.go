package middleware

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type clientIPContextKey struct{}

func TrustedClientIP(proxies []netip.Prefix) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := ClientIP(r, proxies)
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), clientIPContextKey{}, ip)))
		})
	}
}

func ClientIPFromRequest(r *http.Request) string {
	if ip, ok := r.Context().Value(clientIPContextKey{}).(string); ok && ip != "" {
		return ip
	}
	return ClientIP(r, nil)
}

// ClientIP trusts an X-Forwarded-For value only when the immediate TCP peer is
// one of the explicitly configured reverse proxies. Caddy overwrites this
// header with a single address before forwarding the request.
func ClientIP(r *http.Request, trustedProxies []netip.Prefix) string {
	peer := remoteIP(r.RemoteAddr)
	if peer == "" {
		return "unknown"
	}
	addr, err := netip.ParseAddr(peer)
	if err == nil {
		for _, prefix := range trustedProxies {
			if prefix.Contains(addr.Unmap()) {
				forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
				if strings.Contains(forwarded, ",") {
					return peer // only a single address from the trusted edge is accepted
				}
				if client, err := netip.ParseAddr(forwarded); err == nil {
					return client.Unmap().String()
				}
				return peer
			}
		}
	}
	return peer
}

func remoteIP(remote string) string {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	if addr, err := netip.ParseAddr(strings.Trim(host, "[]")); err == nil {
		return addr.Unmap().String()
	}
	return ""
}
