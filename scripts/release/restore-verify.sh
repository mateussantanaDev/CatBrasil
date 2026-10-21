#!/usr/bin/env bash
# scripts/release/restore-verify.sh
#
# Prova de restauração isolada e validação de integridade de schema (28 tabelas)
# para backups lógicos da CatBrasil.
#
# Regras de Segurança:
# 1. Checksum obrigatório: rejeita sumariamente se o arquivo .sha256 não existir ou divergir.
# 2. Cifragem com age: decodificação streaming diretamente para o contêiner isolado;
#    NUNCA persiste plaintext no filesystem do host.
# 3. Isolamento estrito: executa contêiner descartável sem rede (--network none),
#    sem portas expostas e sem volumes montados no host.
# 4. Validação completa de schema: exige sequência exata de migrações 1..14 e todas as 28 tabelas
#    definidas no banco Greenfield.
#
set -euo pipefail

usage() {
  cat <<'EOF'
Uso:
  scripts/release/restore-verify.sh [opções] /caminho/catbrasil-YYYYMMDD.dump.age

Opções:
  -i, --identity <arquivo>       Arquivo de identidade privada age para decifrar (ou $AGE_IDENTITY_FILE)
      --image <imagem>           Imagem Docker do PostgreSQL (padrão: postgres:16-alpine)
  -h, --help                     Exibe esta ajuda e sai

Variáveis de Ambiente Reconhecidas:
  AGE_IDENTITY_FILE / RESTORE_AGE_IDENTITY_FILE  Caminho da chave privada age
EOF
  exit 2
}

ARCHIVE=""
IDENTITY_FILE="${AGE_IDENTITY_FILE:-${RESTORE_AGE_IDENTITY_FILE:-}}"
IMAGE="postgres:16-alpine"

while [[ $# -gt 0 ]]; do
  case "$1" in
    -i|--identity)
      IDENTITY_FILE="${2:-}"
      shift 2 || usage
      ;;
    --image)
      IMAGE="${2:-postgres:16-alpine}"
      shift 2 || usage
      ;;
    -h|--help)
      usage
      ;;
    -*)
      echo "Opção desconhecida: $1" >&2
      usage
      ;;
    *)
      if [[ -z "$ARCHIVE" ]]; then
        ARCHIVE="$1"
        shift
      else
        echo "Argumento posicional extra inesperado: $1" >&2
        usage
      fi
      ;;
  esac
done

if [[ -z "$ARCHIVE" ]]; then
  echo "ERRO DE BLOQUEIO: Caminho do arquivo de backup não informado." >&2
  usage
fi

if [[ ! -f "$ARCHIVE" ]]; then
  echo "ERRO DE BLOQUEIO: Arquivo de backup não encontrado: $ARCHIVE" >&2
  exit 2
fi

# 1. Conferência obrigatória de Checksum SHA-256
CHECKSUM_FILE="${ARCHIVE}.sha256"
if [[ ! -f "$CHECKSUM_FILE" ]]; then
  echo "ERRO DE BLOQUEIO: Checksum obrigatório não encontrado: $CHECKSUM_FILE" >&2
  echo "Todo backup válido deve possuir um arquivo de verificação .sha256 gerado no momento do dump." >&2
  exit 2
fi

SHA_CHECK_CMD=""
if command -v sha256sum >/dev/null 2>&1; then
  SHA_CHECK_CMD="sha256sum -c"
elif command -v shasum >/dev/null 2>&1; then
  SHA_CHECK_CMD="shasum -a 256 -c"
else
  echo "ERRO DE BLOQUEIO: Nenhum utilitário de conferência SHA-256 ('sha256sum' ou 'shasum') disponível." >&2
  exit 2
fi

if ! (cd "$(dirname "$ARCHIVE")" && $SHA_CHECK_CMD "$(basename "$CHECKSUM_FILE")"); then
  echo "ERRO FATAL: Checksum SHA-256 do arquivo '$ARCHIVE' diverge do registro '$CHECKSUM_FILE'." >&2
  exit 2
fi

# 2. Detecção de cifragem e validação de identidade age
IS_ENCRYPTED=0
if [[ "$ARCHIVE" == *.age ]]; then
  IS_ENCRYPTED=1
elif [[ -f "$ARCHIVE" ]] && [[ "$(head -c 18 "$ARCHIVE" 2>/dev/null || true)" == "age-encryption.org" ]]; then
  IS_ENCRYPTED=1
fi

if [[ "$IS_ENCRYPTED" -ne 1 ]]; then
  echo "ERRO DE BLOQUEIO: O arquivo '$ARCHIVE' não está cifrado com age." >&2
  echo "Backups sem cifragem são estritamente proibidos em todos os ambientes (inclusive staging)." >&2
  exit 2
fi

if ! command -v age >/dev/null 2>&1; then
  echo "ERRO DE BLOQUEIO: Ferramenta 'age' não encontrada no host para decifrar o backup." >&2
  echo "Instale o age (open-source e gratuito):" >&2
  echo "  Debian/Ubuntu: apt-get install -y age" >&2
  echo "  macOS:         brew install age" >&2
  echo "  Binário:       https://github.com/FiloSottile/age/releases" >&2
  exit 2
fi

if [[ -z "$IDENTITY_FILE" ]]; then
  echo "ERRO DE BLOQUEIO: Arquivo de identidade age (chave privada) obrigatório para decifrar o backup." >&2
  echo "Informe via --identity <arquivo> ou defina a variável AGE_IDENTITY_FILE." >&2
  exit 2
fi

if [[ ! -f "$IDENTITY_FILE" ]]; then
  echo "ERRO DE BLOQUEIO: Arquivo de identidade age não encontrado: $IDENTITY_FILE" >&2
  exit 2
fi

# 3. Preflight de ferramentas Docker
if ! command -v docker >/dev/null 2>&1; then
  echo "ERRO DE BLOQUEIO: Utilitário 'docker' não encontrado no PATH." >&2
  exit 2
fi

if ! docker info >/dev/null 2>&1; then
  echo "ERRO DE BLOQUEIO: Docker daemon inacessível ou inativo." >&2
  exit 2
fi

# 4. Inicialização do contêiner isolado e efêmero
CONTAINER="catbrasil-restore-proof-$(date +%s)-$$"
cleanup() {
  docker rm -f "$CONTAINER" > /dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

echo "Iniciando contêiner isolado de restauração descartável..."
docker run --rm --network none --name "$CONTAINER" \
  -e POSTGRES_HOST_AUTH_METHOD=trust -e POSTGRES_DB=restore_proof \
  -d "$IMAGE" > /dev/null

READY=0
for _ in {1..30}; do
  if [[ "$(docker exec "$CONTAINER" psql -X -q -t -A -v ON_ERROR_STOP=1 -U postgres -d restore_proof -c 'SELECT 1' 2>/dev/null || true)" == "1" ]]; then
    READY=1
    break
  fi
  sleep 1
done

if [[ "$READY" -ne 1 ]]; then
  echo "ERRO FATAL: PostgreSQL isolado não iniciou no contêiner '$CONTAINER'." >&2
  exit 1
fi

# 5. Restauração via streaming direto (plaintext NUNCA persiste no disco do host)
echo "Restaurando dump lógico via streaming direto para pg_restore..."
age -d -i "$IDENTITY_FILE" "$ARCHIVE" | docker exec -i "$CONTAINER" pg_restore \
  --exit-on-error --no-owner --no-acl -U postgres -d restore_proof

# 6. Validação rigorosa de Schema: 28 tabelas e sequência exata de migrações 1..14
echo "Executando validação de schema e integridade relacional..."
docker exec -i "$CONTAINER" psql -X -v ON_ERROR_STOP=1 -U postgres -d restore_proof <<SQL
DO \$\$
DECLARE
  v_expected_versions int[] := ARRAY[1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14];
  v_applied_versions int[];
  v_missing text[] := ARRAY[]::text[];
  v_tables text[] := ARRAY[
    'schema_migrations',
    'system_config',
    'fleet_catalog',
    'level_configs',
    'roulette_slices',
    'announcements',
    'users',
    'transactions',
    'financial_events',
    'payout_gateway_events',
    'sms_verifications',
    'user_identities',
    'identity_events',
    'machines',
    'referrals',
    'user_promos',
    'battle_pass_season_history',
    'ranking_season_history',
    'mining_race_weekly',
    'usage_logs',
    'security_logs',
    'deposit_gateway_events',
    'cashin_deposit_intents',
    'cashin_event_queue',
    'payout_submissions',
    'payout_event_queue',
    'http_rate_limits',
    'cron_executions'
  ];
  t text;
BEGIN
  FOREACH t IN ARRAY v_tables LOOP
    IF to_regclass('public.' || t) IS NULL THEN
      v_missing := array_append(v_missing, t);
    END IF;
  END LOOP;

  IF array_length(v_missing, 1) > 0 THEN
    RAISE EXCEPTION 'Tabelas obrigatorias ausentes no backup: % (encontradas: % de 28)',
      array_to_string(v_missing, ', '),
      (28 - array_length(v_missing, 1));
  END IF;

  SELECT array_agg(version ORDER BY version) INTO v_applied_versions FROM schema_migrations;
  IF v_applied_versions IS NULL OR v_applied_versions != v_expected_versions THEN
    RAISE EXCEPTION 'Sequencia de migracoes invalida no backup: % (esperado exatamente [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14])',
      COALESCE(v_applied_versions::text, '[]');
  END IF;
END \$\$;

SELECT 'schema_version=' || COALESCE(MAX(version), 0) FROM schema_migrations;
SELECT 'applied_versions=' || COALESCE(array_to_string(array_agg(version ORDER BY version), ','), '') FROM schema_migrations;
SELECT 'users=' || COUNT(*) FROM users;
SELECT 'transactions=' || COUNT(*) FROM transactions;
SELECT 'financial_events=' || COUNT(*) FROM financial_events;
SELECT 'user_identities=' || COUNT(*) FROM user_identities;
SELECT 'identity_events=' || COUNT(*) FROM identity_events;
SELECT 'cashin_deposit_intents=' || COUNT(*) FROM cashin_deposit_intents;
SELECT 'payout_submissions=' || COUNT(*) FROM payout_submissions;
SQL

echo "================================================================="
echo "  RESTAURAÇÃO ISOLADA VALIDADA COM SUCESSO"
echo "  Schema validado: 28 tabelas e sequência de migrações 1..14 confirmadas."
echo "  O contêiner descartável '$CONTAINER' será finalizado."
echo "================================================================="
