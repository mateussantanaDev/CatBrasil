package config

import (
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"strings"
)

type Runtime struct {
	Environment        string
	PixMode            string
	DatabaseURL        string
	JWTSecret          string
	WebhookSecret      string
	AdminPIN           string
	SMSGatewayURL      string
	SMSGatewayToken    string
	SMSCodeSecret      string
	PixSandboxURL      string
	MetricsToken       string
	IdentitySecret     string
	SuitPayLiveEnabled bool
	Port               string
	TrustedProxyCIDRs  []netip.Prefix
}

func LoadRuntime() (Runtime, error) {
	cfg := Runtime{
		Environment:        strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV"))),
		PixMode:            strings.ToLower(strings.TrimSpace(os.Getenv("PIX_MODE"))),
		DatabaseURL:        strings.TrimSpace(os.Getenv("DATABASE_URL")),
		JWTSecret:          strings.TrimSpace(os.Getenv("JWT_SECRET")),
		WebhookSecret:      strings.TrimSpace(os.Getenv("PIX_WEBHOOK_SECRET")),
		AdminPIN:           strings.TrimSpace(os.Getenv("ADMIN_SECRET_PIN")),
		SMSGatewayURL:      strings.TrimSpace(os.Getenv("SMS_GATEWAY_URL")),
		SMSGatewayToken:    strings.TrimSpace(os.Getenv("SMS_GATEWAY_TOKEN")),
		SMSCodeSecret:      strings.TrimSpace(os.Getenv("SMS_CODE_SECRET")),
		PixSandboxURL:      strings.TrimSpace(os.Getenv("PIX_SANDBOX_URL")),
		MetricsToken:       strings.TrimSpace(os.Getenv("OBSERVABILITY_TOKEN")),
		IdentitySecret:     strings.TrimSpace(os.Getenv("IDENTITY_SECRET")),
		SuitPayLiveEnabled: strings.EqualFold(strings.TrimSpace(os.Getenv("SUITPAY_LIVE_ENABLED")), "true"),
		Port:               strings.TrimSpace(os.Getenv("PORT")),
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	if cfg.Environment != "development" && cfg.Environment != "staging" && cfg.Environment != "production" {
		return cfg, fmt.Errorf("APP_ENV deve ser development, staging ou production")
	}
	if cfg.PixMode != "simulated" && cfg.PixMode != "suitpay" {
		return cfg, fmt.Errorf("PIX_MODE deve ser simulated ou suitpay")
	}
	if cfg.Environment == "production" && cfg.PixMode != "suitpay" {
		return cfg, fmt.Errorf("PIX_MODE=simulated não é permitido em production")
	}
	proxyCIDRs := strings.TrimSpace(os.Getenv("TRUSTED_PROXY_CIDRS"))
	if cfg.Environment != "development" && proxyCIDRs == "" {
		return cfg, fmt.Errorf("TRUSTED_PROXY_CIDRS é obrigatório fora de development")
	}
	if proxyCIDRs != "" {
		for _, item := range strings.Split(proxyCIDRs, ",") {
			prefix, err := netip.ParsePrefix(strings.TrimSpace(item))
			if err != nil || prefix.Addr().Is4In6() || (prefix.Addr().Is4() && prefix.Bits() != 32) || (prefix.Addr().Is6() && prefix.Bits() != 128) {
				return cfg, fmt.Errorf("TRUSTED_PROXY_CIDRS deve conter apenas IPs individuais (/32 ou /128)")
			}
			cfg.TrustedProxyCIDRs = append(cfg.TrustedProxyCIDRs, prefix.Masked())
		}
	}
	if cfg.DatabaseURL == "" {
		return cfg, fmt.Errorf("DATABASE_URL é obrigatório")
	}
	if len(cfg.JWTSecret) < 32 {
		return cfg, fmt.Errorf("JWT_SECRET deve conter pelo menos 32 caracteres")
	}
	if len(cfg.WebhookSecret) < 32 {
		return cfg, fmt.Errorf("PIX_WEBHOOK_SECRET deve conter pelo menos 32 caracteres")
	}
	if len(cfg.AdminPIN) < 6 {
		return cfg, fmt.Errorf("ADMIN_SECRET_PIN deve conter pelo menos 6 caracteres")
	}
	if (cfg.Environment != "development" || cfg.IdentitySecret != "") && len(cfg.IdentitySecret) < 32 {
		return cfg, fmt.Errorf("IDENTITY_SECRET deve conter pelo menos 32 caracteres")
	}
	if cfg.SMSGatewayURL != "" {
		parsed, err := url.Parse(cfg.SMSGatewayURL)
		if err != nil || parsed == nil {
			return cfg, fmt.Errorf("SMS_GATEWAY_URL deve ser uma URL HTTPS válida")
		}
		stagingSandbox := cfg.Environment == "staging" && parsed.Scheme == "http" && parsed.Host == "sms-sandbox:8080" && parsed.Path == "/send"
		if parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || (parsed.Scheme != "https" && !(cfg.Environment == "development" && parsed.Scheme == "http") && !stagingSandbox) {
			return cfg, fmt.Errorf("SMS_GATEWAY_URL deve ser uma URL HTTPS válida")
		}
	}
	if cfg.PixSandboxURL != "" {
		if cfg.Environment != "staging" || cfg.PixMode != "suitpay" || cfg.PixSandboxURL != "http://pix-sandbox:8080/api/v1/gateway" {
			return cfg, fmt.Errorf("PIX_SANDBOX_URL só é permitido em staging no serviço pix-sandbox interno")
		}
	}
	if cfg.MetricsToken != "" && len(cfg.MetricsToken) < 32 {
		return cfg, fmt.Errorf("OBSERVABILITY_TOKEN deve conter pelo menos 32 caracteres")
	}
	if cfg.Environment == "production" && cfg.SMSGatewayURL == "" {
		return cfg, fmt.Errorf("SMS_GATEWAY_URL é obrigatório em production para cadastro por SMS")
	}
	if cfg.SMSGatewayURL != "" && (cfg.SMSGatewayToken == "" || len(cfg.SMSCodeSecret) < 32) {
		return cfg, fmt.Errorf("SMS_GATEWAY_TOKEN e SMS_CODE_SECRET (32+ caracteres) são obrigatórios com SMS_GATEWAY_URL")
	}
	if cfg.SMSCodeSecret == "" {
		cfg.SMSCodeSecret = cfg.JWTSecret
	}
	if cfg.Environment == "production" && strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS")) == "" {
		return cfg, fmt.Errorf("CORS_ALLOWED_ORIGINS é obrigatório em production")
	}
	return cfg, nil
}
