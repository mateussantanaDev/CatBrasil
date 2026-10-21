package config

import "testing"

func TestLoadRuntimeRequiresExplicitSecureConfiguration(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("PIX_MODE", "simulated")
	t.Setenv("DATABASE_URL", "postgres://db/test")
	t.Setenv("JWT_SECRET", "abcdefghijklmnopqrstuvwxyz012345")
	t.Setenv("PIX_WEBHOOK_SECRET", "abcdefghijklmnopqrstuvwxyz012345")
	t.Setenv("ADMIN_SECRET_PIN", "secure-pin-987654")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.test")
	t.Setenv("SMS_GATEWAY_URL", "https://sms.example.test/send")
	t.Setenv("SMS_GATEWAY_TOKEN", "test-token")
	t.Setenv("SMS_CODE_SECRET", "sms-code-secret-32-characters-long!")
	t.Setenv("IDENTITY_SECRET", "identity-secret-32-characters-long!")
	t.Setenv("TRUSTED_PROXY_CIDRS", "172.30.240.2/32")
	if _, err := LoadRuntime(); err == nil {
		t.Fatal("production deve rejeitar PIX simulado")
	}
	t.Setenv("PIX_MODE", "suitpay")
	if _, err := LoadRuntime(); err != nil {
		t.Fatalf("configuração explícita deveria ser aceita: %v", err)
	}
	t.Setenv("TRUSTED_PROXY_CIDRS", "0.0.0.0/0")
	if _, err := LoadRuntime(); err == nil {
		t.Fatal("CIDR amplo de proxy confiável deveria ser rejeitado")
	}
	t.Setenv("TRUSTED_PROXY_CIDRS", "172.30.240.0/24")
	if _, err := LoadRuntime(); err == nil {
		t.Fatal("rede inteira não deve ser considerada proxy confiável")
	}
	t.Setenv("TRUSTED_PROXY_CIDRS", "172.30.240.2/32")
	t.Setenv("SMS_GATEWAY_URL", "")
	if _, err := LoadRuntime(); err == nil {
		t.Fatal("production aceitou cadastro sem gateway SMS")
	}
	t.Setenv("SMS_GATEWAY_URL", "http://sms.example.test/send")
	if _, err := LoadRuntime(); err == nil {
		t.Fatal("production aceitou gateway SMS sem HTTPS")
	}
	t.Setenv("SMS_GATEWAY_URL", "https://sms.example.test/send")
	t.Setenv("JWT_SECRET", "weak")
	if _, err := LoadRuntime(); err == nil {
		t.Fatal("segredo JWT fraco deveria ser rejeitado")
	}
}

func TestStagingSandboxURLsCannotEscapeToProduction(t *testing.T) {
	t.Setenv("APP_ENV", "staging")
	t.Setenv("PIX_MODE", "suitpay")
	t.Setenv("DATABASE_URL", "postgres://db/test")
	t.Setenv("JWT_SECRET", "abcdefghijklmnopqrstuvwxyz012345")
	t.Setenv("PIX_WEBHOOK_SECRET", "abcdefghijklmnopqrstuvwxyz012345")
	t.Setenv("ADMIN_SECRET_PIN", "secure-pin-987654")
	t.Setenv("SMS_GATEWAY_URL", "http://sms-sandbox:8080/send")
	t.Setenv("SMS_GATEWAY_TOKEN", "test-token")
	t.Setenv("SMS_CODE_SECRET", "sms-code-secret-32-characters-long!")
	t.Setenv("IDENTITY_SECRET", "identity-secret-32-characters-long!")
	t.Setenv("PIX_SANDBOX_URL", "http://pix-sandbox:8080/api/v1/gateway")
	t.Setenv("OBSERVABILITY_TOKEN", "metrics-token-32-characters-long!")
	t.Setenv("TRUSTED_PROXY_CIDRS", "172.30.241.2/32")
	if _, err := LoadRuntime(); err != nil {
		t.Fatalf("sandbox interno de staging deveria ser aceito: %v", err)
	}
	t.Setenv("APP_ENV", "production")
	if _, err := LoadRuntime(); err == nil {
		t.Fatal("production aceitou gateway interno de staging")
	}
	t.Setenv("APP_ENV", "staging")
	t.Setenv("SMS_GATEWAY_URL", "http://evil.example/send")
	if _, err := LoadRuntime(); err == nil {
		t.Fatal("staging aceitou gateway SMS HTTP externo")
	}
	t.Setenv("SMS_GATEWAY_URL", "http://sms-sandbox:8080/send")
	t.Setenv("PIX_SANDBOX_URL", "http://evil.example/api/v1/gateway")
	if _, err := LoadRuntime(); err == nil {
		t.Fatal("staging aceitou gateway PIX HTTP externo")
	}
}
