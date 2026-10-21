package gateway

import (
	"bytes"
	"catbrasil/backend/internal/money"
	"catbrasil/backend/pkg/identity"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	SuitPayProdURL    = "https://ws.suitpay.app/api/v1/gateway"
	SuitPaySandboxURL = "https://sandbox.ws.suitpay.app/api/v1/gateway"
)

type SuitPayClient struct {
	ClientID     string
	ClientSecret string
	Sandbox      bool
	CallbackURL  string
	BaseURL      string
	HTTPClient   *http.Client
}

func NewSuitPayClient(clientID, clientSecret string, sandbox bool, callbackURL string) *SuitPayClient {
	return &SuitPayClient{
		ClientID:     strings.TrimSpace(clientID),
		ClientSecret: strings.TrimSpace(clientSecret),
		Sandbox:      sandbox,
		CallbackURL:  strings.TrimSpace(callbackURL),
		HTTPClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *SuitPayClient) IsConfigured() bool {
	return c.ClientID != "" && c.ClientSecret != ""
}

func (c *SuitPayClient) getBaseURL() string {
	if c.BaseURL != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	if c.Sandbox {
		return SuitPaySandboxURL
	}
	return SuitPayProdURL
}

// ----------------------------------------------------------------------------
// CASH-IN: Geração de Cobrança PIX
// ----------------------------------------------------------------------------

type SuitPayClientData struct {
	Name        string `json:"name"`
	Document    string `json:"document"`
	Email       string `json:"email"`
	PhoneNumber string `json:"phoneNumber"`
}

type SuitPayPixPaymentRequest struct {
	RequestNumber string            `json:"requestNumber"`
	DueDate       string            `json:"dueDate"`
	Amount        float64           `json:"amount"`
	CallbackURL   string            `json:"callbackUrl"`
	Client        SuitPayClientData `json:"client"`
}

type SuitPayPixPaymentResponse struct {
	Response          string `json:"response"`
	IDTransaction     string `json:"idTransaction"`
	PaymentCode       string `json:"paymentCode"`       // Código Copia e Cola
	PaymentCodeBase64 string `json:"paymentCodeBase64"` // Imagem Base64 QR Code
	Message           string `json:"message,omitempty"`
}

// CreatePixPayment envia requisição de cobrança PIX para o gateway SuitPay
func (c *SuitPayClient) CreatePixPayment(ctx context.Context, req SuitPayPixPaymentRequest) (*SuitPayPixPaymentResponse, error) {
	if !c.IsConfigured() {
		return nil, fmt.Errorf("credenciais SuitPay não configuradas")
	}

	if req.CallbackURL == "" {
		req.CallbackURL = c.CallbackURL
	}
	if req.DueDate == "" {
		req.DueDate = time.Now().Add(24 * time.Hour).Format("2006-01-02")
	}

	url := fmt.Sprintf("%s/request-qrcode", c.getBaseURL())
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("falha ao serializar payload SuitPay: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("falha ao criar requisição SuitPay: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("ci", c.ClientID)
	httpReq.Header.Set("cs", c.ClientSecret)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("falha na conexão com gateway SuitPay: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("falha ao ler resposta da SuitPay: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("erro da API SuitPay (HTTP %d)", resp.StatusCode)
	}

	var suitPayResp SuitPayPixPaymentResponse
	if err := json.Unmarshal(respBody, &suitPayResp); err != nil {
		return nil, fmt.Errorf("falha ao decodificar JSON da SuitPay: %w", err)
	}

	return &suitPayResp, nil
}

// ----------------------------------------------------------------------------
// CASH-OUT: Transferência PIX (Saque Automático)
// ----------------------------------------------------------------------------

type SuitPayPixTransferRequest struct {
	Value              float64 `json:"value"`
	Key                string  `json:"key"`
	TypeKey            string  `json:"typeKey"` // "document", "email", "phoneNumber", "randomKey"
	DocumentValidation string  `json:"documentValidation"`
	CallbackURL        string  `json:"callbackUrl,omitempty"`
	ExternalID         string  `json:"externalId"`
}

type SuitPayPixTransferResponse struct {
	Response      string `json:"response"`
	IDTransaction string `json:"idTransaction"`
	Status        string `json:"status"`
	Message       string `json:"message,omitempty"`
}

// DetectPixKeyType identifica automaticamente o formato da chave PIX
func DetectPixKeyType(key string) string {
	k := strings.TrimSpace(key)
	if strings.Contains(k, "@") {
		return "email"
	}
	clean := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(k, ".", ""), "-", ""), "/", "")
	if len(clean) == 11 && !strings.HasPrefix(clean, "+") {
		if _, err := identity.NormalizeCPF(clean); err == nil {
			return "document"
		}
		if clean[2] == '9' {
			return "phoneNumber"
		}
	}
	if len(clean) == 14 {
		return "document"
	}
	if strings.HasPrefix(k, "+") || (len(clean) >= 10 && len(clean) <= 13) {
		return "phoneNumber"
	}
	return "randomKey"
}

// TransferPix executa transferência PIX de saída através da SuitPay
func (c *SuitPayClient) TransferPix(ctx context.Context, req SuitPayPixTransferRequest) (*SuitPayPixTransferResponse, error) {
	if !c.IsConfigured() {
		return nil, fmt.Errorf("credenciais SuitPay não configuradas")
	}

	doc := strings.TrimSpace(req.DocumentValidation)
	if doc == "" {
		return nil, fmt.Errorf("documentValidation é obrigatório para transferência PIX")
	}
	cleanDoc := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(doc, ".", ""), "-", ""), "/", ""), " ", "")
	var normalizedDoc string
	if len(cleanDoc) == 11 {
		cpf, err := identity.NormalizeCPF(cleanDoc)
		if err != nil {
			return nil, fmt.Errorf("documentValidation inválido para transferência PIX")
		}
		normalizedDoc = cpf
	} else if len(cleanDoc) == 14 {
		for _, r := range cleanDoc {
			if r < '0' || r > '9' {
				return nil, fmt.Errorf("documentValidation inválido para transferência PIX")
			}
		}
		normalizedDoc = cleanDoc
	} else {
		return nil, fmt.Errorf("documentValidation inválido para transferência PIX")
	}
	req.DocumentValidation = normalizedDoc

	extID := strings.TrimSpace(req.ExternalID)
	if extID == "" {
		return nil, fmt.Errorf("externalId é obrigatório para transferência PIX")
	}
	req.ExternalID = extID

	key := strings.TrimSpace(req.Key)
	if key == "" {
		return nil, fmt.Errorf("chave PIX é obrigatória para transferência")
	}
	req.Key = key

	if req.Value <= 0 {
		return nil, fmt.Errorf("valor de transferência PIX inválido")
	}

	if req.TypeKey == "" {
		req.TypeKey = DetectPixKeyType(req.Key)
	}
	if req.CallbackURL == "" {
		req.CallbackURL = c.CallbackURL
	}

	url := fmt.Sprintf("%s/pix-payment", c.getBaseURL())
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("falha ao serializar payload de transferência SuitPay: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("falha ao criar requisição de transferência SuitPay: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("ci", c.ClientID)
	httpReq.Header.Set("cs", c.ClientSecret)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("falha na conexão de transferência com SuitPay: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("falha ao ler resposta de transferência SuitPay: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("erro na transferência SuitPay (HTTP %d)", resp.StatusCode)
	}

	var transferResp SuitPayPixTransferResponse
	if err := json.Unmarshal(respBody, &transferResp); err != nil {
		return nil, fmt.Errorf("falha ao decodificar JSON de transferência: %w", err)
	}

	return &transferResp, nil
}

// ----------------------------------------------------------------------------
// WEBHOOK CALLBACK: Processamento de Notificação SuitPay
// ----------------------------------------------------------------------------

type SuitPayWebhookCallback struct {
	// IDTransaction preserva o identificador externo exato: números chegam como
	// json.Number (nunca float64), evitando perda de precisão acima de 2^53 que
	// transformaria IDs em notação científica via fmt.Sprint.
	IDTransaction     interface{} `json:"idTransaction"` // string ou json.Number
	TypeTransaction   string      `json:"typeTransaction"`
	StatusTransaction string      `json:"statusTransaction"` // "PAID_OUT", "PAID", "CHARGEBACK", "CANCELED"
	Value             float64     `json:"value"`
	RequestNumber     string      `json:"requestNumber"`
}

func (w *SuitPayWebhookCallback) UnmarshalJSON(data []byte) error {
	type alias SuitPayWebhookCallback
	var decoded alias
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	var raw struct {
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	cents, err := money.ParseJSONNumber(string(raw.Value))
	if err != nil {
		return err
	}
	*w = SuitPayWebhookCallback(decoded)
	w.Value = cents.Float64()
	return nil
}

// VerifySuitPayWebhook valida o hash SHA-256 documentado pela SuitPay.
// Os valores são concatenados na ordem em que aparecem no JSON, inclusive campos futuros.
func VerifySuitPayWebhook(raw []byte, clientSecret string) (*SuitPayWebhookCallback, error) {
	if clientSecret == "" {
		return nil, fmt.Errorf("Client Secret SuitPay não configurado")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	start, err := dec.Token()
	if err != nil || start != json.Delim('{') {
		return nil, fmt.Errorf("webhook SuitPay inválido")
	}
	var values strings.Builder
	var signature string
	seen := make(map[string]bool)
	for dec.More() {
		keyToken, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("webhook SuitPay inválido: %w", err)
		}
		key, ok := keyToken.(string)
		if !ok || seen[key] {
			return nil, fmt.Errorf("campos duplicados ou inválidos no webhook SuitPay")
		}
		seen[key] = true
		var value interface{}
		if err := dec.Decode(&value); err != nil {
			return nil, fmt.Errorf("valor inválido no webhook SuitPay: %w", err)
		}
		str, ok := suitPayScalar(value)
		if !ok {
			return nil, fmt.Errorf("campo não escalar no webhook SuitPay: %s", key)
		}
		if key == "hash" {
			signature = str
		} else {
			values.WriteString(str)
		}
	}
	if _, err := dec.Token(); err != nil {
		return nil, fmt.Errorf("webhook SuitPay inválido: %w", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("dados extras no webhook SuitPay")
	}
	provided, err := hex.DecodeString(signature)
	if err != nil || len(provided) != sha256.Size {
		return nil, fmt.Errorf("hash SuitPay ausente ou inválido")
	}
	expected := sha256.Sum256([]byte(clientSecret + values.String()))
	if subtle.ConstantTimeCompare(provided, expected[:]) != 1 {
		return nil, fmt.Errorf("hash SuitPay não confere")
	}
	var callback SuitPayWebhookCallback
	// Decode with UseNumber so numeric idTransaction values keep their exact
	// decimal representation; json.Unmarshal would coerce them to float64 and
	// lose precision for UnixNano-sized identifiers.
	cbDec := json.NewDecoder(bytes.NewReader(raw))
	cbDec.UseNumber()
	if err := cbDec.Decode(&callback); err != nil {
		return nil, fmt.Errorf("payload SuitPay inválido: %w", err)
	}
	return &callback, nil
}

func suitPayScalar(value interface{}) (string, bool) {
	switch v := value.(type) {
	case string:
		return v, true
	case json.Number:
		return v.String(), true
	case bool:
		if v {
			return "true", true
		}
		return "false", true
	default:
		return "", false
	}
}

func (w *SuitPayWebhookCallback) IsPaid() bool {
	st := strings.ToUpper(strings.TrimSpace(w.StatusTransaction))
	return st == "PAID_OUT" || st == "PAID" || st == "PAGO" || st == "SUCESSO"
}
