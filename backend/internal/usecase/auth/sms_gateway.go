package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type SMSSender interface {
	Send(ctx context.Context, phone, message string) error
}

// HTTPGateway posts a minimal JSON contract to the configured SMS provider.
type HTTPGateway struct {
	URL    string
	Token  string
	Client *http.Client
}

func (g *HTTPGateway) Send(ctx context.Context, phone, message string) error {
	body, err := json.Marshal(map[string]string{"phone": phone, "message": message})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.Token)
	client := http.Client{Timeout: 5 * time.Second}
	if g.Client != nil {
		client = *g.Client
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("gateway SMS indisponível: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("gateway SMS rejeitou envio: HTTP %d", resp.StatusCode)
	}
	return nil
}
