package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"catbrasil/backend/internal/delivery/http/middleware"
	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/usecase/auth"
)

type AuthHandler struct {
	authUC *auth.AuthUseCase
}

func NewAuthHandler(authUC *auth.AuthUseCase) *AuthHandler {
	return &AuthHandler{authUC: authUC}
}

func (h *AuthHandler) SendSMS(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phone string `json:"phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Phone == "" {
		middleware.RespondError(w, http.StatusBadRequest, "Informe o número de celular", "INVALID_PHONE")
		return
	}

	err := h.authUC.SendSMSCode(r.Context(), req.Phone)
	if err != nil {
		status, errorCode := http.StatusServiceUnavailable, "SMS_UNAVAILABLE"
		if errors.Is(err, domain.ErrInvalidPhone) {
			status, errorCode = http.StatusBadRequest, "INVALID_PHONE"
		}
		if errors.Is(err, domain.ErrSMSCooldownActive) {
			status, errorCode = http.StatusTooManyRequests, "SMS_COOLDOWN"
		}
		middleware.RespondError(w, status, err.Error(), errorCode)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Se o número estiver disponível, o código de verificação será enviado.",
	}
	middleware.RespondJSON(w, http.StatusOK, response)
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req auth.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.RespondError(w, http.StatusBadRequest, "Payload JSON inválido", "INVALID_PAYLOAD")
		return
	}

	req.IPAddress = middleware.ClientIPFromRequest(r)

	resp, err := h.authUC.Register(r.Context(), req)
	if err != nil {
		middleware.RespondError(w, http.StatusBadRequest, err.Error(), "REGISTER_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusCreated, resp)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req auth.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.RespondError(w, http.StatusBadRequest, "Payload JSON inválido", "INVALID_PAYLOAD")
		return
	}

	req.IPAddress = middleware.ClientIPFromRequest(r)

	resp, err := h.authUC.Login(r.Context(), req)
	if err != nil {
		code := "LOGIN_FAILED"
		if errors.Is(err, domain.ErrTwoFactorRequired) {
			code = "TWO_FACTOR_REQUIRED"
		}
		if errors.Is(err, domain.ErrInvalid2FACode) {
			code = "INVALID_2FA_CODE"
		}
		middleware.RespondError(w, http.StatusUnauthorized, err.Error(), code)
		return
	}

	middleware.RespondJSON(w, http.StatusOK, resp)
}

func (h *AuthHandler) ERPLogin(w http.ResponseWriter, r *http.Request) {
	var req auth.ERPLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.RespondError(w, http.StatusBadRequest, "Payload JSON inválido", "INVALID_PAYLOAD")
		return
	}

	resp, err := h.authUC.ERPLogin(r.Context(), req)
	if err != nil {
		code := "ERP_AUTH_REQUIRED"
		status := http.StatusUnauthorized
		if errors.Is(err, domain.ErrTwoFactorRequired) {
			code = "TWO_FACTOR_REQUIRED"
		}
		if errors.Is(err, domain.ErrInvalid2FACode) {
			code = "INVALID_2FA_CODE"
		}
		if errors.Is(err, domain.ErrTwoFactorNotConfigured) {
			code = "TWO_FACTOR_NOT_CONFIGURED"
			status = http.StatusForbidden
		}
		middleware.RespondError(w, status, err.Error(), code)
		return
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Autenticação aprovada",
		"data": map[string]interface{}{
			"token": resp.Token,
			"gestor": map[string]interface{}{
				"id":       resp.UserID,
				"username": resp.Username,
				"role":     resp.Role,
			},
		},
	})
}

func (h *AuthHandler) ERPLogout(w http.ResponseWriter, r *http.Request) {
	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Sessão do gestor encerrada com sucesso.",
	})
}

func (h *AuthHandler) Setup2FA(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.RespondError(w, http.StatusUnauthorized, "Não autenticado", "UNAUTHORIZED")
		return
	}

	resp, err := h.authUC.Setup2FA(r.Context(), userID)
	if err != nil {
		middleware.RespondError(w, http.StatusBadRequest, err.Error(), "2FA_SETUP_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, resp)
}

func (h *AuthHandler) Verify2FA(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.RespondError(w, http.StatusUnauthorized, "Não autenticado", "UNAUTHORIZED")
		return
	}

	var req struct {
		Secret string `json:"secret"`
		Code   string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.RespondError(w, http.StatusBadRequest, "Payload inválido", "INVALID_PAYLOAD")
		return
	}

	err := h.authUC.VerifyAndEnable2FA(r.Context(), userID, req.Code)
	if err != nil {
		middleware.RespondError(w, http.StatusBadRequest, err.Error(), "2FA_VERIFY_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Autenticação 2FA ativada com sucesso!",
	})
}
