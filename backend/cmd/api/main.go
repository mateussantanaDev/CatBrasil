package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"catbrasil/backend/internal/config"
	deliveryhttp "catbrasil/backend/internal/delivery/http"
	"catbrasil/backend/internal/delivery/http/handler"
	"catbrasil/backend/internal/delivery/ws"
	"catbrasil/backend/internal/repository/postgres"
	"catbrasil/backend/internal/usecase/admin"
	"catbrasil/backend/internal/usecase/auth"
	"catbrasil/backend/internal/usecase/garage"
	"catbrasil/backend/internal/usecase/payment"
	"catbrasil/backend/internal/usecase/ranking"
	"catbrasil/backend/internal/usecase/roulette"
	"catbrasil/backend/internal/usecase/shop"
	"catbrasil/backend/migrations"
	"catbrasil/backend/pkg/jwt"

	"github.com/joho/godotenv"
)

func main() {
	// 1. Carrega variáveis de ambiente (.env)
	_ = godotenv.Load("../../.env")
	_ = godotenv.Load(".env")
	_ = godotenv.Load()

	runtime, err := config.LoadRuntime()
	if err != nil {
		log.Fatalf("Configuração inválida: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 2. Conexão com PostgreSQL 16 (pgxpool)
	log.Println("🐘 Conectando ao PostgreSQL...")
	db, err := postgres.NewDB(ctx, runtime.DatabaseURL)
	if err != nil {
		log.Fatalf("Falha ao conectar ao banco de dados: %v", postgres.SanitizeDBError(err))
	}
	if db == nil || db.Pool == nil {
		log.Fatal("Falha ao conectar ao banco de dados: pool indisponível")
	}
	defer db.Close()
	if err := migrations.CheckCurrent(ctx, db.Pool); err != nil {
		log.Fatalf("Schema do banco indisponível: %v. Execute as migrações antes da API.", postgres.SanitizeDBError(err))
	}

	// 3. Inicialização dos Repositórios
	var userRepo *postgres.UserRepository
	var machineRepo *postgres.MachineRepository
	var catalogRepo *postgres.CatalogRepository
	var txRepo *postgres.TransactionRepository
	var referralRepo *postgres.ReferralRepository
	var rouletteRepo *postgres.RouletteRepository
	var announceRepo *postgres.AnnouncementRepository
	var configRepo *postgres.ConfigRepository
	var promoRepo *postgres.PromoRepository
	var rankingRepo *postgres.RankingRepository
	var logRepo *postgres.LogRepository
	var dreRepo *postgres.DRERepository
	var smsRepo *postgres.SMSRepository
	var levelRepo *postgres.LevelRepository
	var identityRepo *postgres.IdentityRepository

	userRepo = postgres.NewUserRepository(db.Pool)
	machineRepo = postgres.NewMachineRepository(db.Pool)
	catalogRepo = postgres.NewCatalogRepository(db.Pool)
	txRepo = postgres.NewTransactionRepository(db.Pool)
	referralRepo = postgres.NewReferralRepository(db.Pool)
	rouletteRepo = postgres.NewRouletteRepository(db.Pool)
	announceRepo = postgres.NewAnnouncementRepository(db.Pool)
	configRepo = postgres.NewConfigRepository(db.Pool)
	promoRepo = postgres.NewPromoRepository(db.Pool)
	rankingRepo = postgres.NewRankingRepository(db.Pool)
	logRepo = postgres.NewLogRepository(db.Pool)
	dreRepo = postgres.NewDRERepository(db.Pool)
	smsRepo = postgres.NewSMSRepository(db.Pool, runtime.SMSCodeSecret)
	levelRepo = postgres.NewLevelRepository(db.Pool)
	cronRepo := postgres.NewCronRepository(db.Pool)
	if runtime.IdentitySecret != "" {
		identityRepo, err = postgres.NewIdentityRepository(db.Pool, runtime.IdentitySecret)
		if err != nil {
			log.Fatalf("Identidade indisponível: %v", err)
		}
		if err = identityRepo.CheckKey(ctx); err != nil {
			log.Fatalf("Chave de identidade incorreta ou dados inacessíveis: %v", err)
		}
	}

	// 4. Token Manager JWT
	tokenManager := jwt.NewTokenManager(runtime.JWTSecret, 168)

	// 5. WebSockets Hub em Tempo Real
	wsHub := ws.NewHub()
	go wsHub.Run()
	rateStore := postgres.NewRateLimitStore(db.Pool)
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cleanupCtx, done := context.WithTimeout(ctx, 30*time.Second)
				if err := rateStore.Cleanup(cleanupCtx); err != nil {
					log.Printf("Falha ao limpar contadores de rate limit: %v", err)
				}
				done()
			}
		}
	}()

	// 6. Casos de Uso (UseCases)
	authUC := auth.NewAuthUseCase(userRepo, machineRepo, referralRepo, smsRepo, logRepo, tokenManager)
	authUC.SetIdentityRepository(identityRepo)
	if runtime.SMSGatewayURL != "" {
		authUC.SetSMSSender(&auth.HTTPGateway{URL: runtime.SMSGatewayURL, Token: runtime.SMSGatewayToken})
	}
	garageUC := garage.NewGarageUseCase(userRepo, machineRepo, txRepo, referralRepo, rankingRepo, logRepo, levelRepo, configRepo)
	shopUC := shop.NewShopUseCase(userRepo, catalogRepo, machineRepo, txRepo)
	paymentUC := payment.NewPaymentUseCase(userRepo, txRepo, referralRepo, configRepo, logRepo, runtime.WebhookSecret, payment.Options{
		Mode: payment.Mode(runtime.PixMode), Environment: runtime.Environment, PixSandboxURL: runtime.PixSandboxURL, IdentityRepo: identityRepo, LiveEnabled: runtime.SuitPayLiveEnabled,
	})
	rouletteUC := roulette.NewRouletteUseCase(userRepo, rouletteRepo, machineRepo, txRepo, logRepo)
	rankingUC := ranking.NewRankingUseCase(rankingRepo)
	adminUC := admin.NewAdminUseCase(userRepo, machineRepo, txRepo, dreRepo, catalogRepo, rouletteRepo, announceRepo, configRepo, logRepo)
	adminUC.SetPaymentUseCase(paymentUC)

	// 7. Handlers REST HTTP
	authHandler := handler.NewAuthHandler(authUC)
	userHandler := handler.NewUserHandler(authUC, garageUC, userRepo, referralRepo)
	userHandler.SetIdentityRepository(identityRepo)
	garageHandler := handler.NewGarageHandler(garageUC, wsHub)
	shopHandler := handler.NewShopHandler(shopUC, wsHub)
	finHandler := handler.NewFinancialHandler(paymentUC, wsHub)
	refHandler := handler.NewReferralHandler(referralRepo, userRepo)
	roulHandler := handler.NewRouletteHandler(rouletteUC, wsHub)
	rankHandler := handler.NewRankingHandler(rankingUC)
	annHandler := handler.NewAnnouncementHandler(announceRepo)
	adminHandler := handler.NewAdminHandler(adminUC, userRepo, machineRepo, catalogRepo, txRepo, referralRepo, rouletteRepo, announceRepo, configRepo, promoRepo, logRepo, levelRepo, wsHub)
	adminHandler.SetIdentityRepository(identityRepo)

	// 8. Roteador Central Chi
	router := deliveryhttp.NewRouter(deliveryhttp.RouterConfig{
		TokenManager:    tokenManager,
		UserRepo:        userRepo,
		AuthHandler:     authHandler,
		UserHandler:     userHandler,
		GarageHandler:   garageHandler,
		ShopHandler:     shopHandler,
		FinHandler:      finHandler,
		RefHandler:      refHandler,
		RoulHandler:     roulHandler,
		RankHandler:     rankHandler,
		AnnHandler:      annHandler,
		AdminHandler:    adminHandler,
		WSHub:           wsHub,
		EnableDevRoutes: runtime.Environment == "development" && runtime.PixMode == "simulated",
		SMSEnabled:      runtime.SMSGatewayURL != "",
		MetricsToken:    runtime.MetricsToken,
		CronMetricsCallback: func(ctx context.Context) (*deliveryhttp.CronObservabilityData, error) {
			data, err := cronRepo.GetCronObservabilityMetrics(ctx)
			if err != nil {
				return nil, postgres.SanitizeDBError(err)
			}
			converted := make([]deliveryhttp.CronJobMetric, len(data.Jobs))
			for i, m := range data.Jobs {
				converted[i] = deliveryhttp.CronJobMetric{
					JobType:              m.JobType,
					LastSuccessTimestamp: m.LastSuccessTimestamp,
					RecentFailures:       m.RecentFailures,
				}
			}
			return &deliveryhttp.CronObservabilityData{
				Jobs:                 converted,
				PendingWeeklyPeriods: data.PendingWeeklyPeriods,
				PendingDailyPeriods:  data.PendingDailyPeriods,
			}, nil
		},
		ReadinessCheck: func(ctx context.Context) error {
			if err := db.Pool.Ping(ctx); err != nil {
				return postgres.SanitizeDBError(err)
			}
			if err := migrations.CheckCurrent(ctx, db.Pool); err != nil {
				return postgres.SanitizeDBError(err)
			}
			return nil
		},
		TrustedProxies: runtime.TrustedProxyCIDRs,
		RateStore:      rateStore,
	})

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", runtime.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 9. Inicialização do Servidor HTTP com Encerramento Gracioso
	go func() {
		log.Printf("🚜 CatBrasil Go API rodando na porta %s (REST + WebSockets)...\n", runtime.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Erro fatal ao iniciar servidor HTTP: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Encerrando servidor HTTP graciosamente...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Falha no encerramento do servidor: %v", err)
	}

	log.Println("✅ Servidor encerrado com sucesso.")
}
