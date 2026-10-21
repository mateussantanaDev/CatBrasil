package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"catbrasil/backend/internal/delivery/http/handler"
	"catbrasil/backend/internal/delivery/http/middleware"
	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/usecase/auth"
	"catbrasil/backend/pkg/crypto"
	"catbrasil/backend/pkg/jwt"
)

type mockUserRepo struct {
	users         map[string]*domain.User
	notFoundError bool
}

func TestERPLoginRequiresConfiguredPIN(t *testing.T) {
	t.Setenv("ADMIN_SECRET_PIN", "private-pin-987654")
	hash, err := crypto.HashPassword("strong-password")
	if err != nil {
		t.Fatal(err)
	}
	admin := &domain.User{ID: "manager-1", Username: "manager", Role: domain.RoleDiretoria, PasswordHash: hash}
	uc := auth.NewAuthUseCase(&mockUserRepo{users: map[string]*domain.User{admin.ID: admin}}, nil, nil, nil, nil, jwt.NewTokenManager("test-secret-key-32-chars-long!", 24))
	if _, err := uc.Login(context.Background(), auth.LoginRequest{Username: admin.Username, Password: "strong-password"}); err == nil {
		t.Fatal("login comum não pode emitir token administrativo sem PIN")
	}
	if _, err := uc.ERPLogin(context.Background(), auth.ERPLoginRequest{Username: admin.Username, Password: "strong-password", PIN: "wrong"}); err == nil {
		t.Fatal("PIN incorreto deve ser rejeitado")
	}
	if _, err := uc.ERPLogin(context.Background(), auth.ERPLoginRequest{Username: admin.Username, Password: "strong-password", PIN: "private-pin-987654"}); err != domain.ErrTwoFactorNotConfigured {
		t.Fatalf("ERP sem 2FA configurado deve ser bloqueado: %v", err)
	}
	secret, err := crypto.GenerateTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	admin.TwoFactorSecret, admin.TwoFactorEnabled = secret, true
	if _, err := uc.ERPLogin(context.Background(), auth.ERPLoginRequest{Username: admin.Username, Password: "strong-password", PIN: "private-pin-987654"}); err != domain.ErrTwoFactorRequired {
		t.Fatalf("ERP com 2FA ativo deve exigir TOTP: %v", err)
	}
	if _, err := uc.ERPLogin(context.Background(), auth.ERPLoginRequest{Username: admin.Username, Password: "strong-password", PIN: "private-pin-987654", TwoFactorCode: "123"}); err != domain.ErrInvalid2FACode {
		t.Fatalf("ERP com TOTP inválido deve ser rejeitado: %v", err)
	}
	code, err := crypto.GenerateTOTPCode(secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := uc.ERPLogin(context.Background(), auth.ERPLoginRequest{Username: admin.Username, Password: "strong-password", PIN: "private-pin-987654", TwoFactorCode: code}); err != nil {
		t.Fatalf("ERP com TOTP válido: %v", err)
	}
	operator := &domain.User{ID: "operator-1", Username: "operator", Role: domain.RoleUser, PasswordHash: hash, TwoFactorSecret: secret, TwoFactorEnabled: true}
	uc = auth.NewAuthUseCase(&mockUserRepo{users: map[string]*domain.User{operator.ID: operator}}, nil, nil, nil, nil, jwt.NewTokenManager("test-secret-key-32-chars-long!", 24))
	opCode, err := crypto.GenerateTOTPCode(secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := uc.ERPLogin(context.Background(), auth.ERPLoginRequest{Username: operator.Username, Password: "strong-password", PIN: "private-pin-987654", TwoFactorCode: opCode}); err != domain.ErrForbidden {
		t.Fatalf("operador comum não pode acessar ERP: %v", err)
	}
}

func TestLoginRequiresTOTPWhenEnabled(t *testing.T) {
	secret, err := crypto.GenerateTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	hash, err := crypto.HashPassword("strong-password")
	if err != nil {
		t.Fatal(err)
	}
	user := &domain.User{ID: "user-2fa", Username: "operator", PasswordHash: hash, TwoFactorSecret: secret, TwoFactorEnabled: true}
	uc := auth.NewAuthUseCase(&mockUserRepo{users: map[string]*domain.User{user.ID: user}}, nil, nil, nil, &mockLogRepo{}, jwt.NewTokenManager("test-secret-key-32-chars-long!", 24))
	if _, err := uc.Login(context.Background(), auth.LoginRequest{Username: user.Username, Password: "strong-password"}); err != domain.ErrTwoFactorRequired {
		t.Fatalf("login sem TOTP: %v", err)
	}
	code, err := crypto.GenerateTOTPCode(secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := uc.Login(context.Background(), auth.LoginRequest{Username: user.Username, Password: "strong-password", TwoFactorCode: code}); err != nil {
		t.Fatalf("login com TOTP: %v", err)
	}
}

func (m *mockUserRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	return m.users[id], nil
}
func (m *mockUserRepo) GetByIDForUpdate(ctx context.Context, id string) (*domain.User, error) {
	return m.users[id], nil
}
func (m *mockUserRepo) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	for _, u := range m.users {
		if u.Username == username {
			return u, nil
		}
	}
	if m.notFoundError {
		return nil, domain.ErrUserNotFound
	}
	return nil, nil
}
func (m *mockUserRepo) GetByPhone(ctx context.Context, phone string) (*domain.User, error) {
	if m.notFoundError {
		return nil, domain.ErrUserNotFound
	}
	return nil, nil
}
func (m *mockUserRepo) GetByFingerprintOrIP(ctx context.Context, fingerprint, ip string) ([]*domain.User, error) {
	var list []*domain.User
	for _, u := range m.users {
		if (fingerprint != "" && u.DeviceFingerprint == fingerprint) || (ip != "" && u.IPAddress == ip) {
			list = append(list, u)
		}
	}
	return list, nil
}
func (m *mockUserRepo) Create(ctx context.Context, user *domain.User) error {
	m.users[user.ID] = user
	return nil
}
func (m *mockUserRepo) Update(ctx context.Context, user *domain.User) error {
	m.users[user.ID] = user
	return nil
}
func (m *mockUserRepo) AdjustBalance(ctx context.Context, id string, deltaB, deltaY float64) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepo) List(ctx context.Context, query, status string, limit, offset int) ([]*domain.User, int, error) {
	return nil, 0, nil
}
func (m *mockUserRepo) Count(ctx context.Context) (int64, error)  { return 1, nil }
func (m *mockUserRepo) ResetDailySpins(ctx context.Context) error { return nil }

type mockMachineRepo struct {
	created []*domain.Machine
}

func (m *mockMachineRepo) GetByID(ctx context.Context, id string) (*domain.Machine, error) {
	return nil, nil
}
func (m *mockMachineRepo) GetByIDForUpdate(ctx context.Context, id string) (*domain.Machine, error) {
	return nil, nil
}
func (m *mockMachineRepo) GetByUserID(ctx context.Context, userID string) ([]*domain.Machine, error) {
	return m.created, nil
}
func (m *mockMachineRepo) Create(ctx context.Context, machine *domain.Machine) error {
	m.created = append(m.created, machine)
	return nil
}
func (m *mockMachineRepo) Update(ctx context.Context, machine *domain.Machine) error { return nil }
func (m *mockMachineRepo) Delete(ctx context.Context, id string) error               { return nil }

type mockReferralRepo struct {
	referrals []*domain.Referral
}

func (m *mockReferralRepo) GetByReferrerID(ctx context.Context, referrerID string) ([]*domain.Referral, error) {
	return m.referrals, nil
}
func (m *mockReferralRepo) GetAll(ctx context.Context) ([]*domain.Referral, error) {
	return m.referrals, nil
}
func (m *mockReferralRepo) GetByID(ctx context.Context, id string) (*domain.Referral, error) {
	return nil, nil
}
func (m *mockReferralRepo) GetByReferredName(ctx context.Context, name string) (*domain.Referral, error) {
	return nil, nil
}
func (m *mockReferralRepo) Create(ctx context.Context, ref *domain.Referral) error {
	m.referrals = append(m.referrals, ref)
	return nil
}
func (m *mockReferralRepo) Update(ctx context.Context, ref *domain.Referral) error { return nil }
func (m *mockReferralRepo) CountActiveByReferrerID(ctx context.Context, referrerID string) (int, error) {
	return 0, nil
}

type mockSMSRepo struct {
	phone, code string
	used        bool
}

func (m *mockSMSRepo) CreateCode(ctx context.Context, phone, code string, expiresAt time.Time) error {
	m.phone, m.code, m.used = phone, code, false
	return nil
}
func (m *mockSMSRepo) InvalidateCode(ctx context.Context, phone, code string) error {
	if m.phone == phone && m.code == code {
		m.used = true
	}
	return nil
}
func (m *mockSMSRepo) VerifyCode(ctx context.Context, phone, code string) (bool, error) {
	valid := !m.used && m.phone == phone && m.code == code
	if valid {
		m.used = true
	}
	return valid, nil
}

type mockSMSSender struct {
	phone, message string
	sendErr        error
}

func (m *mockSMSSender) Send(ctx context.Context, phone, message string) error {
	m.phone, m.message = phone, message
	return m.sendErr
}

func TestRegistrationRequiresSMSWhenGatewayConfigured(t *testing.T) {
	userRepo := &mockUserRepo{users: map[string]*domain.User{}, notFoundError: true}
	sender := &mockSMSSender{}
	smsRepo := &mockSMSRepo{}
	uc := auth.NewAuthUseCase(userRepo, &mockMachineRepo{}, &mockReferralRepo{}, smsRepo, &mockLogRepo{}, jwt.NewTokenManager("test-secret-key-32-chars-long!", 24))
	uc.SetSMSSender(sender)
	if err := uc.SendSMSCode(context.Background(), "(11) 99999-9999"); err != nil {
		t.Fatal(err)
	}
	if sender.phone != "+5511999999999" || sender.message == "" {
		t.Fatalf("envio SMS incorreto: %+v", sender)
	}
	request := auth.RegisterRequest{Username: "sms-operator", Password: "strong-password", Phone: "11999999999"}
	if _, err := uc.Register(context.Background(), request); err != domain.ErrInvalidSMSCode {
		t.Fatalf("cadastro sem SMS foi aceito: %v", err)
	}
	request.SMSCode = regexp.MustCompile(`[0-9]{6}`).FindString(sender.message)
	if request.SMSCode != smsRepo.code {
		t.Fatal("código enviado difere do desafio armazenado")
	}
	if _, err := uc.Register(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if _, err := uc.Register(context.Background(), auth.RegisterRequest{Username: "second-operator", Password: "strong-password", Phone: "11988888888", SMSCode: request.SMSCode}); err != domain.ErrInvalidSMSCode {
		t.Fatalf("código reutilizado para outro número foi aceito: %v", err)
	}
}

func TestSMSHandlerDoesNotRevealCode(t *testing.T) {
	uc := auth.NewAuthUseCase(nil, nil, nil, &mockSMSRepo{}, nil, nil)
	uc.SetSMSSender(&mockSMSSender{})
	h := handler.NewAuthHandler(uc)
	req := httptest.NewRequest("POST", "/api/v1/auth/send-sms", bytes.NewBufferString(`{"phone":"11999999999"}`))
	recorder := httptest.NewRecorder()
	h.SendSMS(recorder, req)
	if recorder.Code != 200 {
		t.Fatalf("envio SMS: %d %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if _, exposed := response["code"]; exposed {
		t.Fatal("código SMS exposto na resposta")
	}
}

func TestSMSRegistrationFailsClosedWithoutDelivery(t *testing.T) {
	ctx := context.Background()
	smsRepo := &mockSMSRepo{}
	uc := auth.NewAuthUseCase(&mockUserRepo{users: map[string]*domain.User{}}, &mockMachineRepo{}, &mockReferralRepo{}, smsRepo, nil, jwt.NewTokenManager("test-secret-key-32-chars-long!", 24))
	if err := uc.SendSMSCode(ctx, "11999999999"); err == nil || smsRepo.code != "" {
		t.Fatalf("envio sem gateway deve falhar antes de criar código: %v", err)
	}
	request := auth.RegisterRequest{Username: "no-gateway", Password: "strong-password", Phone: "11999999999", SMSCode: "123456"}
	if _, err := uc.Register(ctx, request); err == nil {
		t.Fatal("cadastro sem gateway foi aceito")
	}

	sender := &mockSMSSender{sendErr: errors.New("gateway indisponível")}
	uc.SetSMSSender(sender)
	if err := uc.SendSMSCode(ctx, "11999999999"); err == nil || !smsRepo.used {
		t.Fatalf("falha do gateway deixou código ativo: %v", err)
	}
	request.SMSCode = smsRepo.code
	if _, err := uc.Register(ctx, request); err != domain.ErrInvalidSMSCode {
		t.Fatalf("cadastro aceitou código de envio falho: %v", err)
	}
}

type mockLogRepo struct{}

func (m *mockLogRepo) LogUsage(ctx context.Context, eventType string, amount float64) error {
	return nil
}
func (m *mockLogRepo) LogSecurity(ctx context.Context, userID *string, event, ip, fingerprint, details string) error {
	return nil
}
func (m *mockLogRepo) GetUsageLogs(ctx context.Context, limit, offset int) ([]*domain.UsageLog, error) {
	return nil, nil
}
func (m *mockLogRepo) GetSecurityLogs(ctx context.Context, userID *string, limit, offset int) ([]*domain.SecurityLog, error) {
	return nil, nil
}

func TestAuthUseCase_RegisterAndLogin(t *testing.T) {
	ctx := context.Background()
	userRepo := &mockUserRepo{users: make(map[string]*domain.User)}
	machineRepo := &mockMachineRepo{}
	refRepo := &mockReferralRepo{}
	smsRepo := &mockSMSRepo{}
	logRepo := &mockLogRepo{}
	tokenMgr := jwt.NewTokenManager("test-secret-key-32-chars-long!", 24)

	uc := auth.NewAuthUseCase(userRepo, machineRepo, refRepo, smsRepo, logRepo, tokenMgr)
	uc.SetSMSSender(&mockSMSSender{})
	if err := smsRepo.CreateCode(ctx, "+5511999999999", "123456", time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	// Cadastro
	regRes, err := uc.Register(ctx, auth.RegisterRequest{
		Username:    "operador1",
		Password:    "senha123",
		Phone:       "11999999999",
		SMSCode:     "123456",
		Fingerprint: "fp-browser-1",
		IPAddress:   "192.168.1.1",
	})
	if err != nil {
		t.Fatalf("falha no registro: %v", err)
	}

	if regRes.Token == "" || regRes.UserID == "" {
		t.Errorf("token ou userID vazios")
	}

	if len(machineRepo.created) != 1 {
		t.Errorf("esperado 1 máquina inicial concedida no cadastro")
	}

	// Login com sucesso
	loginRes, err := uc.Login(ctx, auth.LoginRequest{
		Username:    "operador1",
		Password:    "senha123",
		Fingerprint: "fp-browser-1",
		IPAddress:   "192.168.1.1",
	})
	if err != nil {
		t.Fatalf("falha no login: %v", err)
	}

	if loginRes.Token == "" {
		t.Errorf("esperado token JWT válido no login")
	}

	// Login com senha errada
	_, errInvalid := uc.Login(ctx, auth.LoginRequest{
		Username: "operador1",
		Password: "senha_incorreta",
	})
	if errInvalid != domain.ErrInvalidCredentials {
		t.Errorf("esperado ErrInvalidCredentials, obteve: %v", errInvalid)
	}
}

func TestAuthUseCase_SecurityPasswordAnd2FA(t *testing.T) {
	ctx := context.Background()
	accountHash, err := crypto.HashPassword("account-secret")
	if err != nil {
		t.Fatal(err)
	}
	user := &domain.User{ID: "USR-1", Username: "operador1", PasswordHash: accountHash}
	userRepo := &mockUserRepo{users: map[string]*domain.User{"USR-1": user}}
	machineRepo := &mockMachineRepo{}
	refRepo := &mockReferralRepo{}
	smsRepo := &mockSMSRepo{}
	logRepo := &mockLogRepo{}
	tokenMgr := jwt.NewTokenManager("test-secret-key-32-chars-long!", 24)

	uc := auth.NewAuthUseCase(userRepo, machineRepo, refRepo, smsRepo, logRepo, tokenMgr)

	// Senha financeira inválida (menos de 6 dígitos)
	errBadPin := uc.UpdateSecurityPassword(ctx, "USR-1", "123", "", "account-secret", "")
	if errBadPin != domain.ErrSecurityPasswordInvalidLen {
		t.Errorf("esperado ErrSecurityPasswordInvalidLen, obteve: %v", errBadPin)
	}

	// Senha financeira válida
	if err := uc.UpdateSecurityPassword(ctx, "USR-1", "123456", "", "wrong-account", ""); err != domain.ErrInvalidCredentials {
		t.Fatalf("cadastro sem senha da conta deveria falhar: %v", err)
	}
	errOk := uc.UpdateSecurityPassword(ctx, "USR-1", "123456", "", "account-secret", "")
	if errOk != nil {
		t.Fatalf("erro ao salvar senha financeira: %v", errOk)
	}
	if user.SecurityPassword == "123456" || !crypto.CheckPassword("123456", user.SecurityPassword) {
		t.Errorf("senha financeira não foi armazenada como hash")
	}
	if err := uc.UpdateSecurityPassword(ctx, "USR-1", "654321", "", "account-secret", ""); err != domain.ErrInvalidSecurityPassword {
		t.Fatalf("troca sem PIN anterior deveria falhar: %v", err)
	}
	if err := uc.UpdateSecurityPassword(ctx, "USR-1", "654321", "123456", "wrong-account", ""); err != domain.ErrInvalidCredentials {
		t.Fatalf("troca sem reautenticação deveria falhar: %v", err)
	}
	if !crypto.CheckPassword("123456", user.SecurityPassword) {
		t.Fatal("tentativa rejeitada alterou o PIN")
	}
	if err := uc.UpdateSecurityPassword(ctx, "USR-1", "654321", "123456", "account-secret", ""); err != nil {
		t.Fatalf("troca com PIN anterior falhou: %v", err)
	}
	if !crypto.CheckPassword("654321", user.SecurityPassword) || crypto.CheckPassword("123456", user.SecurityPassword) {
		t.Fatal("novo PIN não foi persistido corretamente")
	}

	profile := handler.NewUserHandler(uc, nil, userRepo, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/user", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, user.ID))
	recorder := httptest.NewRecorder()
	profile.GetUserProfile(recorder, req)
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusOK || body["hasSecurityPassword"] != true {
		t.Fatalf("perfil deve retornar apenas indicador booleano: %d %v", recorder.Code, body)
	}
	if _, exposed := body["securityPassword"]; exposed {
		t.Fatal("perfil expôs campo da senha financeira")
	}
	if _, exposed := body["passwordHash"]; exposed {
		t.Fatal("perfil expôs hash da conta")
	}
	if _, exposed := body["security_password"]; exposed {
		t.Fatal("perfil expôs hash do PIN")
	}
	serializedUser, err := json.Marshal(user)
	if err != nil {
		t.Fatal(err)
	}
	var authUser map[string]any
	if err := json.Unmarshal(serializedUser, &authUser); err != nil {
		t.Fatal(err)
	}
	if authUser["hasSecurityPassword"] != true || authUser["securityPassword"] != nil || authUser["passwordHash"] != nil {
		t.Fatalf("resposta de autenticação pode revelar PIN: %s", serializedUser)
	}

	requestPINChange := func(accountPassword string) int {
		payload, _ := json.Marshal(map[string]string{
			"password": "987654", "currentPassword": "654321", "accountPassword": accountPassword,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/user/security-password", bytes.NewReader(payload))
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, user.ID))
		recorder := httptest.NewRecorder()
		profile.SaveSecurityPassword(recorder, req)
		return recorder.Code
	}
	if status := requestPINChange(""); status != http.StatusForbidden {
		t.Fatalf("troca sem reautenticação: HTTP %d", status)
	}
	if status := requestPINChange("account-secret"); status != http.StatusOK {
		t.Fatalf("troca autenticada: HTTP %d", status)
	}
	if !crypto.CheckPassword("987654", user.SecurityPassword) {
		t.Fatal("rota de troca não persistiu novo PIN com hash")
	}

	// Setup 2FA
	setupRes, err := uc.Setup2FA(ctx, "USR-1")
	if err != nil {
		t.Fatalf("erro no setup 2FA: %v", err)
	}
	if setupRes.Secret == "" || setupRes.OTPAuthURL == "" {
		t.Errorf("dados de setup 2FA incompletos")
	}
}

func TestSecurityPasswordRequiresEnabledTOTP(t *testing.T) {
	secret, err := crypto.GenerateTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	accountHash, err := crypto.HashPassword("account-secret")
	if err != nil {
		t.Fatal(err)
	}
	user := &domain.User{ID: "pin-totp", PasswordHash: accountHash, TwoFactorEnabled: true, TwoFactorSecret: secret}
	uc := auth.NewAuthUseCase(&mockUserRepo{users: map[string]*domain.User{user.ID: user}}, nil, nil, nil, nil, nil)
	if err := uc.UpdateSecurityPassword(context.Background(), user.ID, "123456", "", "account-secret", ""); err != domain.ErrInvalid2FACode {
		t.Fatalf("PIN sem TOTP: %v", err)
	}
	if user.SecurityPassword != "" {
		t.Fatal("PIN foi salvo sem TOTP")
	}
	code, err := crypto.GenerateTOTPCode(secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := uc.UpdateSecurityPassword(context.Background(), user.ID, "123456", "", "account-secret", code); err != nil {
		t.Fatalf("PIN com TOTP válido: %v", err)
	}
}
