package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"catbrasil/backend/internal/repository/postgres"
	"catbrasil/backend/migrations"

	"github.com/joho/godotenv"
)

func main() {
	// Carrega variáveis de ambiente
	_ = godotenv.Load("../../.env")
	_ = godotenv.Load(".env")
	_ = godotenv.Load()

	action := flag.String("action", "up", "Ação de migração: up | status")
	flag.Parse()

	if len(flag.Args()) > 0 {
		*action = flag.Args()[0]
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL é obrigatório")
	}

	log.Println("🐘 CatBrasil Migration CLI: Conectando ao banco...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	db, err := postgres.NewDB(ctx, databaseURL)
	if err != nil {
		log.Fatalf("❌ Erro fatal ao conectar ao PostgreSQL: %v", postgres.SanitizeDBError(err))
	}
	defer db.Close()

	switch *action {
	case "up":
		result, err := migrations.Apply(ctx, db.Pool)
		if err != nil {
			log.Fatalf("❌ Falha ao aplicar migrações: %v", postgres.SanitizeDBError(err))
		}
		log.Printf("✅ Migrações concluídas. Baseline legado: %v; aplicadas: %v", result.Baselined, result.Applied)

	case "status":
		applied, available, err := migrations.Status(ctx, db.Pool)
		if err != nil {
			log.Fatalf("❌ Falha ao consultar migrações: %v", postgres.SanitizeDBError(err))
		}
		log.Printf("📊 Migrações registradas: %d/%d", applied, available)

	default:
		log.Fatalf("❌ Ação desconhecida: %s. Use 'up' ou 'status'.", *action)
	}
}
