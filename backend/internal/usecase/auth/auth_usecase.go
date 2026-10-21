package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"catbrasil/backend/internal/domain"
	"catbrasil/backend/pkg/crypto"
	"catbrasil/backend/pkg/id"
	"catbrasil/backend/pkg/identity"
	"catbrasil/backend/pkg/jwt"
)

type RegisterRequest struct {
	Username    string `json:"username"`
	CPF         string `json:"cpf"`
	LegalName   string `json:"legalName"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	SMSCode     string `json:"smsCode"`
	Password    string `json:"password"`
	InviteCode  string `json:"inviteCode"`
	Fingerprint string `json:"fingerprint"`
	IPAddress   string `json:"ipAddress"`
}

type LoginRequest struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	TwoFactorCode string `json:"twoFactorCode"`
	Fingerprint   string `json:"fingerprint"`
	IPAddress     string `json:"ipAddress"`
}

type ERPLoginRequest struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	PIN           string `json:"pin"`
	TwoFactorCode string `json:"twoFactorCode"`
}

type AuthResponse struct {
	Token            string       `json:"token"`
	UserID           string       `json:"userId"`
	Username         string       `json:"username"`
	Role             string       `json:"role"`
	MultiAccountFlag bool         `json:"multiAccountFlag"`
	Message          string       `json:"message"`
	User             *domain.User `json:"user,omitempty"`
}

type TwoFactorSetupResponse struct {
	Secret     string `json:"secret"`
	OTPAuthURL string `json:"otpauthUrl"`
}

type AuthUseCase struct {
	userRepo     domain.UserRepository
	identityRepo domain.IdentityRepository
	machineRepo  domain.MachineRepository
	referralRepo domain.ReferralRepository
	smsRepo      domain.SMSRepository
	smsSender    SMSSender
	logRepo      domain.LogRepository
	tokenManager *jwt.TokenManager
}

func (uc *AuthUseCase) SetSMSSender(sender SMSSender)                        { uc.smsSender = sender }
func (uc *AuthUseCase) SetIdentityRepository(repo domain.IdentityRepository) { uc.identityRepo = repo }

func ValidateIdentityPerson(person domain.IdentityPerson) error {
	_, err := identity.NormalizePerson(identity.Person(person))
	return err
}

func NormalizeBrazilPhone(input string) (string, error) {
	digits := strings.Builder{}
	for _, char := range input {
		if char >= '0' && char <= '9' {
			digits.WriteRune(char)
		} else if char != '+' && char != ' ' && char != '(' && char != ')' && char != '-' {
			return "", domain.ErrInvalidPhone
		}
	}
	number := digits.String()
	if strings.HasPrefix(number, "55") && len(number) >= 12 {
		number = number[2:]
	}
	if len(number) != 11 || number[2] != '9' {
		return "", fmt.Errorf("%w: informe um celular brasileiro com DDD", domain.ErrInvalidPhone)
	}
	return "+55" + number, nil
}

func NewAuthUseCase(
	userRepo domain.UserRepository,
	machineRepo domain.MachineRepository,
	referralRepo domain.ReferralRepository,
	smsRepo domain.SMSRepository,
	logRepo domain.LogRepository,
	tokenManager *jwt.TokenManager,
) *AuthUseCase {
	return &AuthUseCase{
		userRepo:     userRepo,
		machineRepo:  machineRepo,
		referralRepo: referralRepo,
		smsRepo:      smsRepo,
		logRepo:      logRepo,
		tokenManager: tokenManager,
	}
}

// SendSMSCode registra o desafio e envia o código ao número informado.
func (uc *AuthUseCase) SendSMSCode(ctx context.Context, phone string) error {
	var err error
	phone, err = NormalizeBrazilPhone(phone)
	if err != nil {
		return err
	}
	if uc.smsRepo == nil || uc.smsSender == nil {
		return fmt.Errorf("verificação SMS indisponível")
	}
	code := crypto.GenerateRandomNumericCode(6)
	expiresAt := time.Now().Add(10 * time.Minute)
	if err := uc.smsRepo.CreateCode(ctx, phone, code, expiresAt); err != nil {
		return err
	}
	if err := uc.smsSender.Send(ctx, phone, "Seu código de verificação CatBrasil é "+code+". Válido por 10 minutos."); err != nil {
		// Um código não entregue jamais pode continuar válido para cadastro.
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if invalidateErr := uc.smsRepo.InvalidateCode(cleanupCtx, phone, code); invalidateErr != nil {
			return fmt.Errorf("envio SMS falhou e o código não pôde ser invalidado: %w", invalidateErr)
		}
		return err
	}
	return nil
}

// Register realiza o cadastro seguro do novo operador com verificação anti-fraude e máquina inicial
func (uc *AuthUseCase) Register(ctx context.Context, req RegisterRequest) (*AuthResponse, error) {
	username := strings.TrimSpace(req.Username)
	person := domain.IdentityPerson{CPF: req.CPF, LegalName: req.LegalName, Email: req.Email}
	if uc.identityRepo != nil {
		if err := ValidateIdentityPerson(person); err != nil {
			return nil, err
		}
	}
	if len(username) < 3 {
		return nil, fmt.Errorf("o nome de usuário deve ter no mínimo 3 caracteres")
	}
	if len(req.Password) < 8 {
		return nil, fmt.Errorf("a senha deve ter no mínimo 8 caracteres")
	}

	existing, err := uc.userRepo.GetByUsername(ctx, username)
	if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
		return nil, err
	}
	if existing != nil {
		return nil, domain.ErrUserAlreadyExists
	}
	if uc.smsRepo == nil || uc.smsSender == nil {
		return nil, fmt.Errorf("verificação SMS indisponível")
	}
	phone, err := NormalizeBrazilPhone(req.Phone)
	if err != nil {
		return nil, err
	}
	existingPhone, err := uc.userRepo.GetByPhone(ctx, phone)
	if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
		return nil, err
	}
	if existingPhone != nil {
		return nil, domain.ErrUserAlreadyExists
	}
	if len(req.SMSCode) != 6 {
		return nil, domain.ErrInvalidSMSCode
	}
	valid, err := uc.smsRepo.VerifyCode(ctx, phone, req.SMSCode)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, domain.ErrInvalidSMSCode
	}

	relatedUsers, _ := uc.userRepo.GetByFingerprintOrIP(ctx, req.Fingerprint, req.IPAddress)
	isMultiAccount := len(relatedUsers) >= 2

	hash, err := crypto.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	idBytes := make([]byte, 8)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, err
	}
	userID := "CB-" + strings.ToUpper(hex.EncodeToString(idBytes))
	now := time.Now()

	newUser := &domain.User{
		ID:                userID,
		Username:          username,
		Phone:             phone,
		PasswordHash:      hash,
		Role:              domain.RoleUser,
		Balance:           0.0,
		XP:                0,
		Nivel:             1,
		ColetasCompleted:  0,
		MultiAccountFlag:  isMultiAccount,
		IPAddress:         req.IPAddress,
		DeviceFingerprint: req.Fingerprint,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	initialMachID, err := id.New("mach")
	if err != nil {
		return nil, err
	}

	var refID string
	if req.InviteCode != "" && !isMultiAccount {
		refID, err = id.New("ref")
		if err != nil {
			return nil, err
		}
	}

	var createErr error
	if uc.identityRepo != nil {
		createErr = uc.identityRepo.CreateUserWithIdentity(ctx, newUser, person)
	} else {
		createErr = uc.userRepo.Create(ctx, newUser)
	}
	if createErr != nil {
		return nil, fmt.Errorf("erro ao salvar operador no banco: %w", createErr)
	}

	// Concede 1x Máquina Escavadeira CAT 320 inicial com primeira coleta liberada
	initialMachine := &domain.Machine{
		ID:              initialMachID,
		UserID:          userID,
		Name:            "Escavadeira CAT 320",
		Cost:            2500.0,
		YieldAmount:     110.0,
		LastCollectedAt: now.Add(-24 * time.Hour), // Disponível de imediato
		State:           domain.MachineStateDisponivel,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	_ = uc.machineRepo.Create(ctx, initialMachine)

	// Vincula convite se informado e não for multi-conta fraudulenta
	if req.InviteCode != "" && !isMultiAccount {
		referrer, _ := uc.userRepo.GetByID(ctx, strings.ToUpper(strings.TrimSpace(req.InviteCode)))
		if referrer != nil {
			ref := &domain.Referral{
				ID:         refID,
				ReferrerID: referrer.ID,
				Nome:       username,
				Status:     domain.ReferralStatusInativo,
				Deposito:   0.0,
				CreatedAt:  now,
				UpdatedAt:  now,
			}
			_ = uc.referralRepo.Create(ctx, ref)

			// Atualiza timestamp de indicação do padrinho e reativa bônus de Nível 12 se estiver suspenso
			referrer.LastReferralCreatedAt = &now
			referrer.Level12BonusSuspended = false
			_ = uc.userRepo.Update(ctx, referrer)
		}
	}

	eventType := "USER_REGISTERED"
	if isMultiAccount {
		eventType = "MULTI_ACCOUNT_DETECTED"
	}
	_ = uc.logRepo.LogSecurity(ctx, &userID, eventType, req.IPAddress, req.Fingerprint, "")

	token, err := uc.tokenManager.GenerateToken(newUser)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		Token:            token,
		UserID:           newUser.ID,
		Username:         newUser.Username,
		Role:             string(newUser.Role),
		MultiAccountFlag: isMultiAccount,
		Message:          "Operador registrado com sucesso na CatBrasil!",
		User:             newUser,
	}, nil
}

// Login autentica o operador com verificação estrita de hash bcrypt
func (uc *AuthUseCase) Login(ctx context.Context, req LoginRequest) (*AuthResponse, error) {
	username := strings.TrimSpace(req.Username)
	if username == "" || req.Password == "" {
		return nil, fmt.Errorf("informe o usuário e a senha")
	}

	user, err := uc.userRepo.GetByUsername(ctx, username)
	if err != nil || user == nil {
		return nil, domain.ErrInvalidCredentials
	}
	if user.IsAdmin() {
		return nil, domain.ErrInvalidCredentials
	}

	if !crypto.CheckPassword(req.Password, user.PasswordHash) {
		_ = uc.logRepo.LogSecurity(ctx, &user.ID, "FAILED_LOGIN_ATTEMPT", req.IPAddress, req.Fingerprint, "")
		return nil, domain.ErrInvalidCredentials
	}
	if user.TwoFactorEnabled && !crypto.VerifyTOTPCode(user.TwoFactorSecret, strings.TrimSpace(req.TwoFactorCode)) {
		if strings.TrimSpace(req.TwoFactorCode) == "" {
			return nil, domain.ErrTwoFactorRequired
		}
		return nil, domain.ErrInvalid2FACode
	}

	user.IPAddress = req.IPAddress
	if req.Fingerprint != "" {
		user.DeviceFingerprint = req.Fingerprint
	}
	_ = uc.userRepo.Update(ctx, user)

	_ = uc.logRepo.LogSecurity(ctx, &user.ID, "USER_LOGGED_IN", req.IPAddress, req.Fingerprint, "")

	token, err := uc.tokenManager.GenerateToken(user)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		Token:            token,
		UserID:           user.ID,
		Username:         user.Username,
		Role:             string(user.Role),
		MultiAccountFlag: user.MultiAccountFlag,
		Message:          "Autenticação realizada com sucesso!",
		User:             user,
	}, nil
}

// ERPLogin autentica diretores e gestores com verificação de cargo administrativo
func (uc *AuthUseCase) ERPLogin(ctx context.Context, req ERPLoginRequest) (*AuthResponse, error) {
	expectedPIN := os.Getenv("ADMIN_SECRET_PIN")
	if len(expectedPIN) < 6 || subtle.ConstantTimeCompare([]byte(req.PIN), []byte(expectedPIN)) != 1 {
		return nil, domain.ErrInvalidCredentials
	}
	username := strings.TrimSpace(req.Username)
	user, err := uc.userRepo.GetByUsername(ctx, username)
	if err != nil || user == nil {
		return nil, domain.ErrInvalidCredentials
	}

	if !user.IsAdmin() {
		return nil, domain.ErrForbidden
	}

	if !crypto.CheckPassword(req.Password, user.PasswordHash) {
		return nil, domain.ErrInvalidCredentials
	}
	if !user.TwoFactorEnabled || user.TwoFactorSecret == "" {
		return nil, domain.ErrTwoFactorNotConfigured
	}
	if !crypto.VerifyTOTPCode(user.TwoFactorSecret, strings.TrimSpace(req.TwoFactorCode)) {
		if strings.TrimSpace(req.TwoFactorCode) == "" {
			return nil, domain.ErrTwoFactorRequired
		}
		return nil, domain.ErrInvalid2FACode
	}

	token, err := uc.tokenManager.GenerateTokenWithTTL(user, 8*time.Hour)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		Token:    token,
		UserID:   user.ID,
		Username: user.Username,
		Role:     string(user.Role),
		Message:  "Sessão Administrativa iniciada com sucesso!",
		User:     user,
	}, nil
}

// UpdateSecurityPassword requires account reauthentication, the current PIN on rotation,
// and TOTP when enabled. Only a bcrypt hash is persisted.
func (uc *AuthUseCase) UpdateSecurityPassword(ctx context.Context, userID, password, currentPassword, accountPassword, twoFactorCode string) error {
	password = strings.TrimSpace(password)
	if len(password) != 6 {
		return domain.ErrSecurityPasswordInvalidLen
	}
	for _, digit := range password {
		if digit < '0' || digit > '9' {
			return domain.ErrSecurityPasswordInvalidLen
		}
	}

	user, err := uc.userRepo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return domain.ErrUserNotFound
	}
	if !crypto.CheckPassword(accountPassword, user.PasswordHash) {
		return domain.ErrInvalidCredentials
	}
	if user.SecurityPassword != "" && !crypto.CheckPassword(currentPassword, user.SecurityPassword) {
		return domain.ErrInvalidSecurityPassword
	}
	if user.TwoFactorEnabled && !crypto.VerifyTOTPCode(user.TwoFactorSecret, strings.TrimSpace(twoFactorCode)) {
		return domain.ErrInvalid2FACode
	}

	hash, err := crypto.HashPassword(password)
	if err != nil {
		return err
	}
	user.SecurityPassword = hash
	return uc.userRepo.Update(ctx, user)
}

// Setup2FA gera um novo segredo e URL para QR Code de 2FA
func (uc *AuthUseCase) Setup2FA(ctx context.Context, userID string) (*TwoFactorSetupResponse, error) {
	user, err := uc.userRepo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return nil, domain.ErrUserNotFound
	}
	if user.TwoFactorEnabled {
		return nil, fmt.Errorf("2FA já está ativo")
	}

	secret := user.TwoFactorSecret
	if secret == "" {
		secret, err = crypto.GenerateTOTPSecret()
		if err != nil {
			return nil, err
		}
		user.TwoFactorSecret = secret
		if err := uc.userRepo.Update(ctx, user); err != nil {
			return nil, err
		}
	}

	return &TwoFactorSetupResponse{
		Secret:     secret,
		OTPAuthURL: crypto.GenerateTOTPURL(user.Username, secret),
	}, nil
}

// VerifyAndEnable2FA valida o token gerado no aplicativo autenticador e ativa o 2FA
func (uc *AuthUseCase) VerifyAndEnable2FA(ctx context.Context, userID, code string) error {
	user, err := uc.userRepo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return domain.ErrUserNotFound
	}

	if user.TwoFactorSecret == "" {
		return fmt.Errorf("nenhum segredo 2FA configurado previamente")
	}

	if !crypto.VerifyTOTPCode(user.TwoFactorSecret, strings.TrimSpace(code)) {
		return domain.ErrInvalid2FACode
	}

	user.TwoFactorEnabled = true
	return uc.userRepo.Update(ctx, user)
}
