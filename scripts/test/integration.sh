#!/usr/bin/env bash
# scripts/test/integration.sh
# Portão de testes de integração reais com PostgreSQL efêmero em Docker.
# Não toca volumes, redes ou serviços do Compose existentes.
# Higiene estrita: credenciais e DSNs nunca são exibidos em logs ou console.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
BACKEND_DIR="${ROOT_DIR}/backend"

if ! command -v docker >/dev/null 2>&1; then
    echo "ERRO: docker não encontrado no PATH." >&2
    exit 1
fi

if ! command -v go >/dev/null 2>&1; then
    echo "ERRO: go não encontrado no PATH." >&2
    exit 1
fi

RUN_ID="$(od -vAn -N4 -tu4 < /dev/urandom 2>/dev/null | tr -d ' ' || echo "$$")"
CONTAINER_NAME="catbrasil_test_pg_${RUN_ID}"
PG_USER="catbrasil_test_user"
PG_PASSWORD="catbrasil_test_pass_${RUN_ID}"
PG_DB="catbrasil_test_db"
PG_IMAGE="postgres:16-alpine"

cleanup() {
    local exit_code=$?
    if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
        docker rm -f "${CONTAINER_NAME}" >/dev/null 2>&1 || true
    fi
    exit "${exit_code}"
}
trap cleanup EXIT INT TERM

# Inicia container isolado sem montar volumes de dados
docker run -d \
    --name "${CONTAINER_NAME}" \
    --rm \
    -e POSTGRES_USER="${PG_USER}" \
    -e POSTGRES_PASSWORD="${PG_PASSWORD}" \
    -e POSTGRES_DB="${PG_DB}" \
    -p "127.0.0.1:0:5432" \
    "${PG_IMAGE}" >/dev/null

# Extrai porta alocada dinamicamente pelo kernel no host
HOST_PORT="$(docker port "${CONTAINER_NAME}" 5432/tcp | head -n 1 | sed 's/.*://')"

if [ -z "${HOST_PORT}" ]; then
    echo "ERRO: Falha ao determinar a porta alocada para o container de teste." >&2
    exit 1
fi

echo "==> PostgreSQL efêmero iniciado em porta dinâmica 127.0.0.1:${HOST_PORT}"

# Aguarda prontidão do PostgreSQL com timeout
MAX_RETRIES=40
READY=0
for i in $(seq 1 "${MAX_RETRIES}"); do
    if docker exec "${CONTAINER_NAME}" pg_isready -U "${PG_USER}" -d "${PG_DB}" -q >/dev/null 2>&1; then
        READY=1
        break
    fi
    sleep 0.5
done

if [ "${READY}" -ne 1 ]; then
    echo "ERRO: PostgreSQL não atingiu prontidão dentro do tempo limite." >&2
    exit 1
fi

echo "==> PostgreSQL pronto para execução de testes."

# DSNs passadas exclusivamente via variáveis de ambiente mascaradas no processo filho
CATBRASIL_TEST_DATABASE_URL="postgres://${PG_USER}:${PG_PASSWORD}@127.0.0.1:${HOST_PORT}/${PG_DB}?sslmode=disable"
export CATBRASIL_TEST_DATABASE_URL
export TEST_DATABASE_URL="${CATBRASIL_TEST_DATABASE_URL}"

# Executa suite de testes Go (unitários + integração reais, migrações, concorrência e webhooks)
if [ "$#" -gt 0 ]; then
    echo "==> Executando testes Go com argumentos customizados: $*"
    (
        cd "${BACKEND_DIR}"
        go test "$@"
    )
else
    echo "==> Executando suíte Go completa com banco PostgreSQL efêmero..."
    (
        cd "${BACKEND_DIR}"
        go test -count=1 ./...
    )

    echo "==> Criando banco isolado limpo para a fase de concorrência/race..."
    docker exec "${CONTAINER_NAME}" createdb -U "${PG_USER}" "${PG_DB}_race"
    CATBRASIL_TEST_DATABASE_URL="postgres://${PG_USER}:${PG_PASSWORD}@127.0.0.1:${HOST_PORT}/${PG_DB}_race?sslmode=disable"
    export CATBRASIL_TEST_DATABASE_URL
    export TEST_DATABASE_URL="${CATBRASIL_TEST_DATABASE_URL}"

    echo "==> Executando verificação de concorrência financeira com race detector (-race)..."
    (
        cd "${BACKEND_DIR}"
        go test -race -count=1 ./internal/repository/postgres ./internal/usecase/payment ./pkg/id
    )
fi

echo "==> Suíte de testes finalizada com sucesso."
