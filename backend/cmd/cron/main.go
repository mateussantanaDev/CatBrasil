package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	deliverycron "catbrasil/backend/internal/delivery/cron"
	"catbrasil/backend/internal/delivery/ws"
	"catbrasil/backend/internal/repository/postgres"
	"catbrasil/backend/internal/usecase/ranking"
	"catbrasil/backend/internal/usecase/roulette"
	"catbrasil/backend/migrations"

	"github.com/joho/godotenv"
)

func main() {
	// Carrega variáveis de ambiente
	_ = godotenv.Load("../../.env")
	_ = godotenv.Load(".env")
	_ = godotenv.Load()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL é obrigatório")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Println("⏱️ CatBrasil Cron Worker conectando ao banco...")
	db, err := postgres.NewDB(ctx, databaseURL)
	if err != nil {
		log.Fatalf("Falha ao conectar ao banco de dados: %v", postgres.SanitizeDBError(err))
	}
	defer db.Close()
	if err := migrations.CheckCurrent(ctx, db.Pool); err != nil {
		log.Fatalf("Schema do banco indisponível para o cron: %v", postgres.SanitizeDBError(err))
	}

	var userRepo *postgres.UserRepository
	var rouletteRepo *postgres.RouletteRepository
	var machineRepo *postgres.MachineRepository
	var txRepo *postgres.TransactionRepository
	var logRepo *postgres.LogRepository
	var rankingRepo *postgres.RankingRepository
	var cronRepo *postgres.CronRepository

	if db != nil {
		userRepo = postgres.NewUserRepository(db.Pool)
		rouletteRepo = postgres.NewRouletteRepository(db.Pool)
		machineRepo = postgres.NewMachineRepository(db.Pool)
		txRepo = postgres.NewTransactionRepository(db.Pool)
		logRepo = postgres.NewLogRepository(db.Pool)
		rankingRepo = postgres.NewRankingRepository(db.Pool)
		cronRepo = postgres.NewCronRepository(db.Pool)
	}

	wsHub := ws.NewHub()
	go wsHub.Run()

	rouletteUC := roulette.NewRouletteUseCase(userRepo, rouletteRepo, machineRepo, txRepo, logRepo)
	rankingUC := ranking.NewRankingUseCase(rankingRepo)

	worker := deliverycron.NewCronWorker(rouletteUC, rankingUC, rankingRepo, machineRepo, logRepo, wsHub, cronRepo)
	go worker.Start(ctx)

	log.Println("⏱️ CatBrasil Cron Worker iniciado com sucesso (Monitorando resets e temporadas)...")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("⏱️ Encerrando Cron Worker...")
	cancel()
	time.Sleep(500 * time.Millisecond)
	log.Println("✅ Cron Worker encerrado.")
}
