.PHONY: all up down logs dev-backend dev-client dev-admin db-up db-down migrate migrate-status test clean

# Inicia todos os serviços via Docker Compose
up:
	docker compose up -d --build

# Para todos os serviços
down:
	docker compose down

# Exibe logs em tempo real
logs:
	docker compose logs -f

# Sobe apenas o PostgreSQL para desenvolvimento local
db-up:
	docker compose up -d postgres

# Para o banco local
db-down:
	docker compose stop postgres

# Executa o Backend Go em modo desenvolvimento local
dev-backend:
	cd backend && go run cmd/api/main.go

# Executa o Worker Cron Go
dev-cron:
	cd backend && go run cmd/cron/main.go

# Executa o App Cliente Svelte 5 (Porta 3000)
dev-client:
	cd frontend/apps/client && npm run dev

# Executa o App Gestão Svelte 5 (Porta 3002)
dev-admin:
	cd frontend/apps/admin && npm run dev

# Executa as migrações SQL no PostgreSQL
migrate:
	docker compose run --rm backend-migrate

# Consulta as versões aplicadas
migrate-status:
	docker compose run --rm backend-migrate /app/migrate status

# Executa todos os testes unitários e de integração em Go
test:
	cd backend && go test -v ./...

# Limpa binários e caches
clean:
	rm -rf backend/bin frontend/apps/client/dist frontend/apps/admin/dist
