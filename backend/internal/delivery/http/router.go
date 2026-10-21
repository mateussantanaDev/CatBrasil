package http

import (
	"context"
	"net/http"
	"net/netip"
	"os"
	"strings"
	"time"

	"catbrasil/backend/internal/delivery/http/handler"
	"catbrasil/backend/internal/delivery/http/middleware"
	"catbrasil/backend/internal/delivery/ws"
	"catbrasil/backend/internal/domain"
	"catbrasil/backend/pkg/jwt"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

type RouterConfig struct {
	TokenManager    *jwt.TokenManager
	UserRepo        domain.UserRepository
	AuthHandler     *handler.AuthHandler
	UserHandler     *handler.UserHandler
	GarageHandler   *handler.GarageHandler
	ShopHandler     *handler.ShopHandler
	FinHandler      *handler.FinancialHandler
	RefHandler      *handler.ReferralHandler
	RoulHandler     *handler.RouletteHandler
	RankHandler     *handler.RankingHandler
	AnnHandler      *handler.AnnouncementHandler
	AdminHandler    *handler.AdminHandler
	WSHub           *ws.Hub
	EnableDevRoutes bool
	SMSEnabled          bool
	MetricsToken        string
	CronMetricsCallback CronMetricsCallback
	ReadinessCheck      func(context.Context) error
	TrustedProxies      []netip.Prefix
	RateStore           middleware.RateStore
}

func NewRouter(cfg RouterConfig) http.Handler {
	r := chi.NewRouter()

	// 1. Middlewares Básicos de Infraestrutura
	r.Use(chimiddleware.RequestID)
	r.Use(middleware.TrustedClientIP(cfg.TrustedProxies))
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.SecurityHeaders)
	metrics := newRequestMetrics(cfg.MetricsToken, cfg.CronMetricsCallback)
	r.Use(metrics.middleware)

	// 2. Configuração Avançada de CORS
	allowedOrigins := []string{"http://localhost:3000", "http://localhost:3002"}
	if configured := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS")); configured != "" {
		allowedOrigins = nil
		for _, origin := range strings.Split(configured, ",") {
			if trimmed := strings.TrimSpace(origin); trimmed != "" && trimmed != "*" {
				allowedOrigins = append(allowedOrigins, trimmed)
			}
		}
	}
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Device-Fingerprint", "X-Webhook-Signature", "X-Signature"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// 3. Limitadores de Taxa de Requisições
	authLimiter := middleware.NewRateLimiterWithStore(60*time.Second, 15, "Muitas tentativas de autenticação. Aguarde 1 minuto.", cfg.RateStore)
	smsSendLimiter := middleware.NewRateLimiterWithStore(time.Hour, 20, "Muitas solicitações de SMS. Aguarde uma hora.", cfg.RateStore)
	securityPasswordLimiter := middleware.NewRateLimiterWithStore(60*time.Second, 5, "Muitas tentativas de alterar a senha financeira. Aguarde 1 minuto.", cfg.RateStore)
	securityPasswordIPLimiter := middleware.NewRateLimiterWithStore(60*time.Second, 5, "Muitas tentativas de alterar a senha financeira. Aguarde 1 minuto.", cfg.RateStore)
	generalLimiter := middleware.NewRateLimiterWithStore(60*time.Second, 300, "Limite de requisições excedido. Reduza a frequência de chamadas.", cfg.RateStore)

	r.Use(generalLimiter.Handler())

	// 4. Healthcheck
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"status":  "online",
			"service": "catbrasil-api-golang",
			"version": "3.0.0",
		})
	})
	r.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		if cfg.ReadinessCheck == nil {
			middleware.RespondError(w, http.StatusServiceUnavailable, "Serviço não pronto", "NOT_READY")
			return
		}
		checkCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := cfg.ReadinessCheck(checkCtx); err != nil {
			middleware.RespondError(w, http.StatusServiceUnavailable, "Banco ou schema indisponível", "NOT_READY")
			return
		}
		middleware.RespondJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	if cfg.MetricsToken != "" {
		r.Get("/internal/metrics", metrics.ServeHTTP)
		r.Get("/metrics", metrics.ServeHTTP)
	}

	// 5. WebSocket Hub
	if cfg.WSHub != nil {
		r.Get("/ws", cfg.WSHub.HandleWS(cfg.TokenManager))
		r.Get("/api/v1/ws", cfg.WSHub.HandleWS(cfg.TokenManager))
	}

	// 6. Grupo de Rotas /api/v1
	r.Route("/api/v1", func(api chi.Router) {
		// Rotas Públicas de Autenticação
		api.With(authLimiter.Handler()).Group(func(pubAuth chi.Router) {
			if cfg.SMSEnabled {
				pubAuth.With(smsSendLimiter.Handler()).Post("/auth/send-sms", cfg.AuthHandler.SendSMS)
				pubAuth.Post("/auth/register", cfg.AuthHandler.Register)
			}
			pubAuth.Post("/auth/login", cfg.AuthHandler.Login)
			pubAuth.Post("/admin/auth/login", cfg.AuthHandler.ERPLogin)
		})

		// Webhook PIX (Público com verificação criptográfica interna HMAC-SHA256 e SuitPay)
		api.Post("/webhooks/pix", cfg.FinHandler.HandlePixWebhook)
		api.Post("/webhook/suitpay", cfg.FinHandler.HandleSuitPayWebhook)
		api.Post("/webhooks/suitpay", cfg.FinHandler.HandleSuitPayWebhook)
		if cfg.EnableDevRoutes {
			api.With(middleware.AuthMiddleware(cfg.TokenManager), middleware.RequireAdmin()).Post("/dev/simulate-webhook", cfg.FinHandler.SimulateWebhook)
		}

		// Consultas Públicas (Catálogo, Roleta Config, Comunicados)
		api.Get("/store/machines", cfg.ShopHandler.GetCatalog)
		api.Get("/shop/catalog", cfg.ShopHandler.GetCatalog)
		api.Get("/roulette/config", cfg.RoulHandler.GetConfig)
		api.Get("/announcements/active", cfg.AnnHandler.GetActiveAnnouncements)

		// Rotas Protegidas do Operador / Cliente (Requer JWT Bearer)
		api.Group(func(clientAuth chi.Router) {
			clientAuth.Use(middleware.AuthMiddleware(cfg.TokenManager))

			// 2FA TOTP
			clientAuth.Post("/auth/2fa/setup", cfg.AuthHandler.Setup2FA)
			clientAuth.Post("/auth/2fa/verify", cfg.AuthHandler.Verify2FA)

			// Perfil & Carreira & Passe de Batalha
			clientAuth.Get("/user", cfg.UserHandler.GetUserProfile)
			clientAuth.Get("/user/profile", cfg.UserHandler.GetUserProfile)
			clientAuth.Post("/user/identity", cfg.UserHandler.SubmitIdentity)
			clientAuth.Get("/user/battlepass", cfg.UserHandler.GetBattlePass)
			clientAuth.Post("/user/battlepass/levelup", cfg.UserHandler.LevelUp)
			clientAuth.Post("/user/insurance/recover", cfg.UserHandler.RecoverInsurance)
			clientAuth.With(securityPasswordIPLimiter.Handler(), securityPasswordLimiter.HandlerByUser()).Post("/user/security-password", cfg.UserHandler.SaveSecurityPassword)
			clientAuth.Post("/user/levelup", cfg.UserHandler.LevelUp)
			clientAuth.Post("/user/level-up", cfg.UserHandler.LevelUp)

			// Garagem & Mineração
			clientAuth.Get("/garage", cfg.GarageHandler.GetMachines)
			clientAuth.Get("/garage/machines", cfg.GarageHandler.GetMachines)
			clientAuth.Post("/garage/collect", cfg.GarageHandler.Collect)
			clientAuth.Post("/garage/collect-all", cfg.GarageHandler.Collect)
			if cfg.EnableDevRoutes {
				clientAuth.Post("/garage/simulate", cfg.GarageHandler.Simulate)
			}
			clientAuth.Post("/garage/levelup", cfg.UserHandler.LevelUp)

			// Loja de Frotas
			clientAuth.Post("/shop/rent", cfg.ShopHandler.RentMachine)
			clientAuth.Post("/garage/rent", cfg.ShopHandler.RentMachine)

			// Operações Financeiras & PIX
			clientAuth.Post("/financial/deposit/pix", cfg.FinHandler.GeneratePixDeposit)
			clientAuth.Post("/pix/generate", cfg.FinHandler.GeneratePixDeposit)
			clientAuth.Post("/financial/withdraw", cfg.FinHandler.RequestWithdrawal)
			clientAuth.Post("/transactions/withdraw", cfg.FinHandler.RequestWithdrawal)
			clientAuth.Get("/financial/transactions", cfg.FinHandler.GetTransactions)
			clientAuth.Get("/transactions", cfg.FinHandler.GetTransactions)
			clientAuth.Get("/user/transactions", cfg.FinHandler.GetTransactions)

			// Rede de Afiliados
			clientAuth.Get("/referrals", cfg.RefHandler.GetReferrals)
			clientAuth.Get("/user/referrals", cfg.RefHandler.GetReferrals)
			if cfg.EnableDevRoutes {
				clientAuth.Post("/referrals/simulate", cfg.RefHandler.Simulate)
			}

			// Roleta & Oficina de Peças
			clientAuth.Post("/roulette/spin", cfg.RoulHandler.Spin)
			clientAuth.Post("/gamification/roulette/spin", cfg.RoulHandler.Spin)
			clientAuth.Post("/parts/assemble", cfg.RoulHandler.AssembleParts)
			clientAuth.Post("/roulette/assemble", cfg.RoulHandler.AssembleParts)
			if cfg.EnableDevRoutes {
				clientAuth.Post("/roulette/reset", cfg.RoulHandler.ResetSpin)
			}

			// Ranking Semanal
			clientAuth.Get("/ranking/weekly", cfg.RankHandler.GetWeeklyRanking)
			clientAuth.Get("/ranking", cfg.RankHandler.GetWeeklyRanking)
		})

		// Rotas Protegidas de Gestão ERP (Requer JWT + Role Admin/Gestor/Diretoria)
		api.Route("/admin", func(admin chi.Router) {
			admin.Use(middleware.AuthMiddleware(cfg.TokenManager))
			admin.Use(middleware.RequireAdminActive(cfg.UserRepo))

			// Sessão do Gestor
			admin.Get("/profile", cfg.UserHandler.GetUserProfile)
			admin.Post("/auth/logout", cfg.AuthHandler.ERPLogout)

			// DRE & Métricas
			admin.Get("/dre/metrics", cfg.AdminHandler.GetDRE)
			admin.Get("/dre", cfg.AdminHandler.GetDRE)

			// Gestão de Operadores
			admin.Get("/users", cfg.AdminHandler.ListUsers)
			admin.Get("/identity/pending", cfg.AdminHandler.ListPendingIdentities)
			admin.Post("/users/{userId}/identity/inspect", cfg.AdminHandler.InspectIdentity)
			admin.Patch("/users/{userId}/identity", cfg.AdminHandler.ReviewIdentity)
			admin.Post("/users/{userId}/adjust-balance", cfg.AdminHandler.AdjustBalance)
			admin.Put("/users/{userId}/level", cfg.AdminHandler.UpdateUserLevel)
			admin.Patch("/users/{userId}/toggle-withdraw-lock", cfg.AdminHandler.ToggleWithdrawLock)
			admin.Patch("/users/{userId}/toggle-commission-lock", cfg.AdminHandler.ToggleCommissionLock)
			admin.Post("/users/{userId}/inject-machine", cfg.AdminHandler.InjectMachine)
			admin.Post("/users/{userId}/reset-spin", cfg.AdminHandler.ResetUserSpin)

			// Fila de Saques & Moderação
			admin.Get("/withdrawals", cfg.AdminHandler.ListWithdrawals)
			admin.Get("/finance/reconciliation", cfg.AdminHandler.ListFinancialReconciliation)
			admin.Get("/transactions", cfg.AdminHandler.ListWithdrawals)
			admin.Post("/withdrawals/{txId}/approve", cfg.AdminHandler.ApproveWithdrawal)
			admin.Post("/withdraw/approve", cfg.AdminHandler.ApproveWithdrawal)
			admin.Post("/withdrawals/{txId}/reject", cfg.AdminHandler.RejectWithdrawal)
			admin.Post("/withdraw/reject", cfg.AdminHandler.RejectWithdrawal)

			// Roleta & Probabilidades
			admin.Get("/roulette/matrix", cfg.AdminHandler.GetRouletteMatrix)
			admin.Put("/roulette/matrix", cfg.AdminHandler.UpdateRouletteMatrix)
			admin.Post("/roulette/config", cfg.AdminHandler.UpdateRouletteMatrix)

			// Passe de Batalha & Gestão dos 12 Níveis
			admin.Get("/levels", cfg.AdminHandler.GetLevels)
			admin.Put("/levels/{level}", cfg.AdminHandler.UpdateLevel)
			admin.Put("/levels/maintenance", cfg.AdminHandler.UpdateLevel12Maintenance)
			admin.Post("/season/reset", cfg.AdminHandler.ResetSeason)

			// Configurações Globais do Sistema
			admin.Get("/config", cfg.AdminHandler.GetSystemConfig)
			admin.Put("/config/gateway", cfg.AdminHandler.UpdateGatewayConfig)
			admin.Post("/config/toggle-global-lock", cfg.AdminHandler.ToggleGlobalLock)
			admin.Put("/config/auto-approve", cfg.AdminHandler.UpdateAutoApprove)
			admin.Put("/config/referrals", cfg.AdminHandler.UpdateReferralsConfig)

			// Comunicados Rich Media
			admin.Get("/announcements", cfg.AdminHandler.ListAnnouncements)
			admin.Post("/announcements", cfg.AdminHandler.CreateAnnouncement)
			admin.Patch("/announcements/{id}/toggle", cfg.AdminHandler.ToggleAnnouncement)
			admin.Delete("/announcements/{id}", cfg.AdminHandler.DeleteAnnouncement)

			// Catálogo de Frota & Promos
			admin.Put("/fleet/catalog", cfg.AdminHandler.SaveFleetCatalog)
			admin.Post("/fleet/catalog", cfg.AdminHandler.SaveFleetCatalog)
			admin.Post("/fleet/promos", cfg.AdminHandler.CreatePromo)
			admin.Delete("/fleet/promos/{id}", cfg.AdminHandler.DeletePromo)

			// Trilhas de Auditoria e Segurança Imutáveis
			admin.Get("/logs/security", cfg.AdminHandler.GetSecurityLogs)
			admin.Get("/logs/usage", cfg.AdminHandler.GetUsageLogs)
		})
	})

	return r
}
