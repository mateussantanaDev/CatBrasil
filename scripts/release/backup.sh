#!/usr/bin/env bash
# scripts/release/backup.sh
#
# Backup lógico fail-closed e criptografado com age para Produção / Staging.
#
# Regras de Segurança:
# 1. Alvo explícito e conferido: projeto Compose, arquivo de ambiente e banco inspecionados antes do dump.
# 2. Cifragem gratuita self-hosted com age: streaming direto do pg_dump para o age, NUNCA persistindo plaintext no host.
# 3. Checksum obrigatório: SHA-256 gerado automaticamente e conferido no restore.
# 4. Saída segura: umask 077, permissões 0600 e limpeza de arquivos parciais via trap EXIT.
# 5. Fail-closed: ausência do age, chave de cifragem ausente, erro de conexão ou divergência abortam com código != 0.

set -euo pipefail

usage() {
  cat <<'EOF'
Uso:
  scripts/release/backup.sh [opções] /caminho/fora-do-repositorio/catbrasil-YYYYMMDD.dump.age

Opções:
  -p, --project <nome>        Projeto Compose alvo (obrigatório: ex. catbrasil-prod ou catbrasil-staging)
  -e, --env-file <arquivo>    Arquivo de ambiente Compose (obrigatório: ex. .prod.env)
  -f, --compose-file <arq>    Arquivo Compose adicional (pode ser repetido; padrão por projeto)
  -s, --service <nome>        Nome do serviço PostgreSQL (padrão: postgres)
  -r, --recipient <chave>     Chave pública age do destinatário (ou $AGE_RECIPIENT)
  -R, --recipient-file <arq>  Arquivo contendo a chave pública age (ou $AGE_RECIPIENT_FILE)
  -h, --help                  Exibe esta ajuda e sai

Variáveis de Ambiente Reconhecidas:
  AGE_RECIPIENT / BACKUP_AGE_RECIPIENT   Chave pública age para cifragem
  AGE_RECIPIENT_FILE                     Arquivo com chave pública age
EOF
  exit 2
}

PROJECT=""
ENV_FILE=""
COMPOSE_FILES=()
SERVICE="postgres"
RECIPIENT="${AGE_RECIPIENT:-${BACKUP_AGE_RECIPIENT:-}}"
RECIPIENT_FILE="${AGE_RECIPIENT_FILE:-}"
OUTPUT=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    -p|--project)
      PROJECT="${2:-}"
      shift 2 || usage
      ;;
    -e|--env-file)
      ENV_FILE="${2:-}"
      shift 2 || usage
      ;;
    -f|--compose-file)
      COMPOSE_FILES+=("${2:-}")
      shift 2 || usage
      ;;
    -s|--service)
      SERVICE="${2:-}"
      shift 2 || usage
      ;;
    -r|--recipient)
      RECIPIENT="${2:-}"
      shift 2 || usage
      ;;
    -R|--recipient-file)
      RECIPIENT_FILE="${2:-}"
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
      if [[ -z "$OUTPUT" ]]; then
        OUTPUT="$1"
        shift
      else
        echo "Argumento posicional extra inesperado: $1" >&2
        usage
      fi
      ;;
  esac
done

if [[ -z "$OUTPUT" ]]; then
  echo "ERRO DE BLOQUEIO: Caminho de saída do backup é obrigatório." >&2
  usage
fi

# Exige projeto e env-file explícitos na invocação (sem defaults do shell ou .prod.env)
if [[ -z "$PROJECT" ]]; then
  echo "ERRO DE BLOQUEIO: Projeto Compose deve ser informado explicitamente via -p ou --project." >&2
  exit 2
fi

if [[ -z "$ENV_FILE" ]]; then
  echo "ERRO DE BLOQUEIO: Arquivo de ambiente deve ser informado explicitamente via -e ou --env-file." >&2
  exit 2
fi

# Mantém -f padrão por projeto conhecido caso não informado
if [[ ${#COMPOSE_FILES[@]} -eq 0 ]]; then
  if [[ "$PROJECT" == "catbrasil-prod" ]]; then
    COMPOSE_FILES=("docker-compose.yml" "compose.prod.yml")
  elif [[ "$PROJECT" == "catbrasil-staging" ]]; then
    COMPOSE_FILES=("docker-compose.yml" "compose.staging.yml")
  else
    COMPOSE_FILES=("docker-compose.yml")
  fi
fi

# 1. Validação de destino
if [[ -e "$OUTPUT" ]]; then
  echo "ERRO DE BLOQUEIO: Arquivo de destino já existe: $OUTPUT" >&2
  exit 2
fi

if [[ -e "${OUTPUT}.sha256" ]]; then
  echo "ERRO DE BLOQUEIO: Arquivo de checksum de destino já existe: ${OUTPUT}.sha256" >&2
  exit 2
fi

# 2. Validação da política de cifragem obrigatória (sem plaintext mesmo em staging)
if [[ "$OUTPUT" != *.age ]]; then
  echo "ERRO DE BLOQUEIO: Destino de backup cifrado deve ter extensão .age (ex: $OUTPUT.age)." >&2
  exit 2
fi

if ! command -v age >/dev/null 2>&1; then
  echo "ERRO DE BLOQUEIO: Dependência obrigatória 'age' não encontrada no host." >&2
  echo "A cifragem de backups é obrigatória para impedir a persistência de dados em plaintext." >&2
  echo "Instale o age (open-source e gratuito):" >&2
  echo "  Debian/Ubuntu: apt-get install -y age" >&2
  echo "  macOS:         brew install age" >&2
  echo "  Binário:       https://github.com/FiloSottile/age/releases" >&2
  exit 2
fi

if [[ -z "$RECIPIENT" && -z "$RECIPIENT_FILE" ]]; then
  echo "ERRO DE BLOQUEIO: Chave pública de cifragem age ausente." >&2
  echo "Informe a chave pública via --recipient <chave>, --recipient-file <arquivo> ou defina AGE_RECIPIENT." >&2
  echo "Para gerar um par de chaves age: age-keygen -o /caminho/seguro/catbrasil-backup.key" >&2
  exit 2
fi

if [[ -n "$RECIPIENT_FILE" && ! -f "$RECIPIENT_FILE" ]]; then
  echo "ERRO DE BLOQUEIO: Arquivo de chave pública age não encontrado: $RECIPIENT_FILE" >&2
  exit 2
fi

# 3. Validação do utilitário de checksum SHA-256
SHA_CMD=""
SHA_CHECK_CMD=""
if command -v sha256sum >/dev/null 2>&1; then
  SHA_CMD="sha256sum"
  SHA_CHECK_CMD="sha256sum -c"
elif command -v shasum >/dev/null 2>&1; then
  SHA_CMD="shasum -a 256"
  SHA_CHECK_CMD="shasum -a 256 -c"
else
  echo "ERRO DE BLOQUEIO: Nenhum utilitário SHA-256 ('sha256sum' ou 'shasum') disponível no host." >&2
  exit 2
fi

# 4. Validação do arquivo de ambiente e comando Docker Compose
if [[ ! -f "$ENV_FILE" ]]; then
  echo "ERRO DE BLOQUEIO: Arquivo de ambiente '$ENV_FILE' não encontrado." >&2
  exit 2
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "ERRO DE BLOQUEIO: Utilitário 'docker' não encontrado no host." >&2
  exit 2
fi

COMPOSE_CMD=(docker compose -p "$PROJECT" --env-file "$ENV_FILE")
for cf in "${COMPOSE_FILES[@]}"; do
  COMPOSE_CMD+=(-f "$cf")
done

# 5. Conferência do contêiner alvo (alvo de projeto/env explícito e conferido)
CONTAINER_ID=$("${COMPOSE_CMD[@]}" ps -q "$SERVICE" 2>/dev/null || true)
if [[ -z "$CONTAINER_ID" ]]; then
  echo "ERRO DE BLOQUEIO: Serviço '$SERVICE' não encontrado ou inativo no projeto Compose '$PROJECT'." >&2
  exit 2
fi

ACTUAL_PROJECT=$(docker inspect -f '{{ index .Config.Labels "com.docker.compose.project" }}' "$CONTAINER_ID" 2>/dev/null || true)
ACTUAL_SERVICE=$(docker inspect -f '{{ index .Config.Labels "com.docker.compose.service" }}' "$CONTAINER_ID" 2>/dev/null || true)

if [[ "$ACTUAL_PROJECT" != "$PROJECT" ]]; then
  echo "ERRO DE BLOQUEIO: Contêiner '$CONTAINER_ID' pertence ao projeto '$ACTUAL_PROJECT', divergindo do projeto esperado '$PROJECT'." >&2
  exit 2
fi

if [[ "$ACTUAL_SERVICE" != "$SERVICE" ]]; then
  echo "ERRO DE BLOQUEIO: Contêiner '$CONTAINER_ID' pertence ao serviço '$ACTUAL_SERVICE', divergindo do serviço esperado '$SERVICE'." >&2
  exit 2
fi

# Inspeciona volume montado em produção: inspeciona EXCLUSIVAMENTE o mount em Destination=/var/lib/postgresql/data e exige Type=volume e Name=catbrasil_prod_pgdata
if [[ "$PROJECT" == "catbrasil-prod" ]]; then
  PGDATA_MOUNT=$(docker inspect -f '{{ range .Mounts }}{{ if eq .Destination "/var/lib/postgresql/data" }}{{ .Type }}:{{ .Name }}{{ end }}{{ end }}' "$CONTAINER_ID" 2>/dev/null || true)
  if [[ "$PGDATA_MOUNT" != "volume:catbrasil_prod_pgdata" ]]; then
    echo "ERRO DE BLOQUEIO: Contêiner de produção '$CONTAINER_ID' deve montar estritamente o volume 'catbrasil_prod_pgdata' (Type=volume) em Destination=/var/lib/postgresql/data (encontrado: '${PGDATA_MOUNT:-nenhum}')." >&2
    exit 2
  fi
fi

# Conferência de prontidão do PostgreSQL usando credenciais do contêiner
if ! "${COMPOSE_CMD[@]}" exec -T "$SERVICE" sh -ceu 'exec pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB" -q' >/dev/null 2>&1; then
  echo "ERRO DE BLOQUEIO: PostgreSQL não está pronto para conexões (pg_isready falhou no contêiner '$CONTAINER_ID')." >&2
  exit 2
fi

# Conferência do banco ativo contra o projeto alvo usando POSTGRES_USER e POSTGRES_DB explícitos
CURRENT_DB=$("${COMPOSE_CMD[@]}" exec -T "$SERVICE" sh -ceu 'exec psql -X -A -t -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "SELECT current_database();"' 2>/dev/null | tr -d '\r\n' || true)
if [[ -z "$CURRENT_DB" ]]; then
  echo "ERRO DE BLOQUEIO: Não foi possível obter o nome do banco de dados ativo no contêiner '$CONTAINER_ID'." >&2
  exit 2
fi

if [[ "$PROJECT" == "catbrasil-prod" && "$CURRENT_DB" != "catbrasil_prod" ]]; then
  echo "ERRO DE BLOQUEIO: Banco de dados ativo '$CURRENT_DB' diverge do banco de produção esperado 'catbrasil_prod'." >&2
  exit 2
fi

if [[ "$PROJECT" == "catbrasil-staging" && "$CURRENT_DB" != "catbrasil_staging" ]]; then
  echo "ERRO DE BLOQUEIO: Banco de dados ativo '$CURRENT_DB' diverge do banco de staging esperado 'catbrasil_staging'." >&2
  exit 2
fi

echo "================================================================="
echo "  CATBRASIL — BACKUP LÓGICO FAIL-CLOSED"
echo "  Projeto: $PROJECT | Serviço: $SERVICE | Banco: $CURRENT_DB"
echo "  Env File: $ENV_FILE | Cifragem age: SIM (streaming)"
echo "================================================================="

# 6. Execução do backup em streaming (sem persistir plaintext no host)
umask 077
mkdir -p "$(dirname "$OUTPUT")"
TEMP=$(mktemp "${OUTPUT}.partial.XXXXXX")
TEMP_SHA=$(mktemp "${OUTPUT}.sha256.partial.XXXXXX")
BACKUP_SUCCESS=0

cleanup() {
  local exit_code=$?
  trap - EXIT INT TERM
  rm -f "$TEMP" "$TEMP_SHA"
  if [[ "$BACKUP_SUCCESS" -ne 1 ]]; then
    rm -f "$OUTPUT" "${OUTPUT}.sha256"
  fi
  if [[ $exit_code -eq 0 && "$BACKUP_SUCCESS" -ne 1 ]]; then
    exit 1
  elif [[ $exit_code -ne 0 ]]; then
    exit "$exit_code"
  fi
}
trap cleanup EXIT INT TERM

DUMP_CMD=('sh' '-ceu' 'exec pg_dump --format=custom --no-owner --no-acl -U "$POSTGRES_USER" -d "$POSTGRES_DB"')

AGE_CMD=(age -e)
if [[ -n "$RECIPIENT_FILE" ]]; then
  AGE_CMD+=(-R "$RECIPIENT_FILE")
else
  AGE_CMD+=(-r "$RECIPIENT")
fi

"${COMPOSE_CMD[@]}" exec -T "$SERVICE" "${DUMP_CMD[@]}" | "${AGE_CMD[@]}" > "$TEMP"

if [[ ! -s "$TEMP" ]]; then
  echo "ERRO FATAL: O arquivo de backup gerado está vazio." >&2
  exit 1
fi

# 7. Geração e validação do Checksum SHA-256 ANTES de publicar o artefato final
BASE_NAME="$(basename "$OUTPUT")"
HASH_VAL=$($SHA_CMD "$TEMP" 2>/dev/null | awk '{print $1}')
if [[ -z "$HASH_VAL" || ${#HASH_VAL} -ne 64 ]]; then
  echo "ERRO FATAL: Falha ao gerar hash SHA-256 do arquivo temporário de backup." >&2
  exit 1
fi

printf "%s  %s\n" "$HASH_VAL" "$BASE_NAME" > "$TEMP_SHA"

# Validação do checksum gerado contra o arquivo temporário
if ! (cd "$(dirname "$TEMP")" && printf "%s  %s\n" "$HASH_VAL" "$(basename "$TEMP")" | $SHA_CHECK_CMD - >/dev/null 2>&1); then
  echo "ERRO FATAL: Falha na conferência de integridade do checksum SHA-256 antes da publicação." >&2
  exit 1
fi

# Publicação dos artefatos finais apenas após checksum existir e ser válido
mv "$TEMP" "$OUTPUT"
mv "$TEMP_SHA" "${OUTPUT}.sha256"

# Pós-validação obrigatória do checksum no artefato final publicado; em caso de falha, trap remove ambos
if ! (cd "$(dirname "$OUTPUT")" && $SHA_CHECK_CMD "$BASE_NAME.sha256" >/dev/null 2>&1); then
  echo "ERRO FATAL: Validação pós-publicação do checksum SHA-256 falhou. Removendo artefatos parciais e finais." >&2
  exit 1
fi

chmod 600 "$OUTPUT" "${OUTPUT}.sha256"

BACKUP_SUCCESS=1
trap - EXIT INT TERM

echo "Backup concluído com sucesso."
echo "  Artefato: $OUTPUT"
echo "  Checksum: ${OUTPUT}.sha256"
echo ""
echo "Atenção: Execute scripts/release/restore-verify.sh para validar a restauração em contêiner descartável antes de aprovar o corte."
