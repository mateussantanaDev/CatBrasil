#!/usr/bin/env bash
# scripts/release/cutover_orchestrator.sh
#
# Orquestrador de Corte Sequencial Seguro para Produção Greenfield (CatBrasil)
#
# Sequência Obrigatória de Execução:
# 0. Validação de uso e existência do ENV_FILE (.prod.env).
# 1. FASE 1 (somente --execute): PORTÃO DE EVIDÊNCIAS P0.6 (fail-closed, ANTES de qualquer
#    chamada Docker). Exige CUTOVER_OPERATOR (identidade do operador) e o pacote de evidências
#    (CUTOVER_EVIDENCE_MANIFEST, default release_evidence/p0_cutover_evidence.json). O validador
#    exige: manifesto assinado com assinatura OpenSSH detached verificada via
#    'ssh-keygen -Y verify' contra CUTOVER_ALLOWED_SIGNERS (arquivo FORA do repositório);
#    operador == identidade assinante; revisor de p0_4_identity_sms distinto do operador;
#    e, para cada item, um arquivo de evidência REAL fora do repositório com sha256 conferido.
#    Falha => exit 3 e nada foi tocado.
#    LIMITE DECLARADO: a assinatura prova IDENTIDADE DE APROVAÇÃO (quem assumiu a
#    responsabilidade), não a verdade do mundo real do conteúdo das evidências; o portão
#    não audita o conteúdo dos artefatos (consistência != autenticidade).
# 2. FASE 2: Validação Estrita da Configuração FINAL Renderizada (via docker compose config --format json).
#    Valida postgres.environment.POSTGRES_DB == catbrasil_prod.
#    Valida DATABASE_URL em backend-migrate, backend-api, backend-cron e bootstrap-admin (host=postgres, port=5432, db=catbrasil_prod).
#    Valida APP_ENV=production, PIX_MODE=suitpay e SUITPAY_LIVE_ENABLED=false.
#    Valida volumes.pgdata.name == catbrasil_prod_pgdata e networks.catbrasil_net.name == catbrasil_prod_net.
#    Valida imagens imutáveis (@sha256:<64 hex>) para todos os serviços, inclusive alertas.
#    Rejeita precedência indevida de variáveis de ambiente exportadas no shell.
# 3. FASE 3 (somente --execute): disponibilidade das portas 80 e 443 em 127.0.0.1 ANTES de
#    qualquer mutação Docker (host ocupado => exit 3, nada foi tocado).
# 4. FASE 4: No modo --execute: Checa antes se backend-api/backend-cron/caddy já estão rodando; aborta se sim.
# 5. FASE 5: Subida ISOLADA do serviço PostgreSQL (sem API, sem portas no host).
# 6. FASE 6: Execução das migrações (backend-migrate up).
# 7. FASE 7: Execução OBRIGATÓRIA do Portão Técnico de Corte (preflight_cutover.py) com exit code 0.
# 8. FASE 7B: Bootstrap interativo do primeiro diretor, antes de serviço público.
# 9. FASE 8: Inicialização com --no-deps de backend-api e demais serviços para não re-executar migrações.
#
# Códigos de saída: 0 = sucesso; 1 = portão técnico (preflight) rejeitou;
#   2 = erro de configuração/uso/status; 3 = portão de evidências ou portas bloqueou o corte.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

ENV_FILE="${ENV_FILE:-.prod.env}"
MODE="${1:---validate-only}"

# COMPOSE_CMD: --profile observability garante que o serviço prometheus (que tem profile)
# apareça renderizado em 'compose config' e passe pela validação de digest imutável.
COMPOSE_CMD=(
  docker compose
  -p catbrasil-prod
  --env-file "$ENV_FILE"
  -f docker-compose.yml
  -f compose.prod.yml
  --profile observability
  --profile bootstrap
)

echo "================================================================="
echo "  ORQUESTRADOR DE CORTE SEQUENCIAL SEGURO — GREENFIELD CUTOVER"
echo "  Protege o caminho suportado de deploy (catbrasil-prod)"
echo "================================================================="

if [[ "$MODE" != "--validate-only" && "$MODE" != "--dry-run" && "$MODE" != "--execute" ]]; then
  echo "Uso: $0 [--validate-only | --execute]" >&2
  exit 2
fi

if [[ ! -f "$ENV_FILE" ]]; then
  echo "ERRO FATAL: Arquivo de ambiente '$ENV_FILE' não encontrado." >&2
  echo "Copie .prod.env.example para $ENV_FILE e configure as credenciais de produção." >&2
  exit 2
fi

# Lê uma variável de configuração: 1) ambiente do shell, 2) ENV_FILE, 3) valor padrão.
read_env_file_var() {
  local key="$1"
  local line
  line="$(grep -E "^${key}=" "$ENV_FILE" 2>/dev/null | tail -n 1)" || return 1
  [[ -n "$line" ]] || return 1
  printf '%s' "${line#*=}"
}

setting_or_default() {
  local key="$1"
  local def="$2"
  local val="${!key:-}"
  if [[ -z "$val" ]]; then
    val="$(read_env_file_var "$key" || true)"
  fi
  printf '%s' "${val:-$def}"
}

# -----------------------------------------------------------------------------
# FASE 1: PORTÃO DE EVIDÊNCIAS DO CORTE P0.6 (SOMENTE --execute, ANTES DE TUDO)
# -----------------------------------------------------------------------------
# Fail-closed: sem operador identificado, sem manifesto assinado, sem allowed_signers
# fora do repositório, sem artefatos reais com sha256 conferido => exit 3.
# Nenhuma chamada Docker (sequer 'compose config') ocorre antes deste portão.
if [[ "$MODE" == "--execute" ]]; then
  echo "[FASE 1] Portão de Evidências do Corte P0.6 (fail-closed — nenhuma chamada Docker até aqui)..."

  CUTOVER_OPERATOR_VALUE="$(setting_or_default CUTOVER_OPERATOR "")"
  if [[ -z "$CUTOVER_OPERATOR_VALUE" ]]; then
    echo "BLOQUEIO DE EVIDÊNCIAS: CUTOVER_OPERATOR não definido (ambiente ou $ENV_FILE)." >&2
    echo "O corte exige um operador identificado como assinante do pacote de evidências." >&2
    echo "Nada foi tocado: nenhuma chamada Docker foi emitida." >&2
    exit 3
  fi

  EVIDENCE_MANIFEST="$(setting_or_default CUTOVER_EVIDENCE_MANIFEST "release_evidence/p0_cutover_evidence.json")"
  ALLOWED_SIGNERS="$(setting_or_default CUTOVER_ALLOWED_SIGNERS "/etc/catbrasil-release/allowed_signers")"
  EVIDENCE_SIGNATURE="$(setting_or_default CUTOVER_EVIDENCE_SIGNATURE "${EVIDENCE_MANIFEST}.sig")"

  if ! python3 "$ROOT_DIR/scripts/release/validate_compose_config.py" \
      --evidence-manifest "$EVIDENCE_MANIFEST" \
      --operator "$CUTOVER_OPERATOR_VALUE" \
      --allowed-signers "$ALLOWED_SIGNERS" \
      --signature "$EVIDENCE_SIGNATURE"
  then
    echo "=================================================================" >&2
    echo "BLOQUEIO DE EVIDÊNCIAS: pacote de evidências do corte REJEITADO (exit 3)." >&2
    echo "Exigências: manifesto assinado (assinatura OpenSSH detached do operador validada" >&2
    echo "com ssh-keygen -Y verify contra allowed_signers fora do repositório), artefatos" >&2
    echo "reais fora do repositório com sha256 conferido e revisor de P0.4 distinto." >&2
    echo "Nada foi tocado: nenhuma chamada Docker foi emitida." >&2
    echo "=================================================================" >&2
    exit 3
  fi

  echo "[FASE 1] Portão de Evidências APROVADO:"
  echo "  - Operador/assinante identificado e validado"
  echo "  - Itens obrigatórios com artefato externo verificado por sha256"
  echo "  - Assinatura = identidade de aprovação, não prova de verdade do conteúdo das evidências"
fi

# -----------------------------------------------------------------------------
# FASE 2: VALIDAÇÃO DA CONFIGURAÇÃO FINAL RENDERIZADA PELO COMPOSE (JSON)
# -----------------------------------------------------------------------------
echo "[FASE 2] Inspecionando configuração final renderizada (captura em memória, sem vazar segredos)..."

# Renderiza a configuração final em JSON e valida com Python via stdin sem expor segredos
if ! COMPOSE_JSON=$("${COMPOSE_CMD[@]}" config --format json 2>/dev/null); then
  echo "ERRO FATAL: Não foi possível renderizar a configuração do Compose." >&2
  exit 2
fi
if ! printf '%s' "$COMPOSE_JSON" | python3 "$ROOT_DIR/scripts/release/validate_compose_config.py"; then
  echo "ERRO FATAL: Configuração renderizada inválida, sobreposta indevida por variáveis de shell ou imagem sem digest imutável." >&2
  exit 2
fi

echo "[FASE 2] Configuração final renderizada aprovada:"
echo "  - Banco: catbrasil_prod"
echo "  - Volume: catbrasil_prod_pgdata"
echo "  - Rede: catbrasil_prod_net"
echo "  - Host DATABASE_URL: postgres:5432/catbrasil_prod"
echo "  - APP_ENV: production | PIX_MODE: suitpay | SUITPAY_LIVE_ENABLED: false"
echo "  - Imagens fixadas por digest (@sha256) em todos os serviços de produção"

if [[ "$MODE" == "--execute" ]] && ! printf '%s' "$COMPOSE_JSON" | python3 "$ROOT_DIR/scripts/release/validate_compose_config.py" --check-host-paths; then
  echo "BLOQUEIO DE MONITORAMENTO: diretório textfile do host ausente; nenhuma mutação Docker foi executada." >&2
  exit 2
fi

if [[ "$MODE" == "--execute" ]]; then
  BOOTSTRAP_ADMIN_USERNAME_VALUE="$(setting_or_default BOOTSTRAP_ADMIN_USERNAME "")"
  if [[ -z "$BOOTSTRAP_ADMIN_USERNAME_VALUE" ]]; then
    echo "BLOQUEIO DE BOOTSTRAP: BOOTSTRAP_ADMIN_USERNAME é obrigatório antes de qualquer mutação Docker." >&2
    exit 3
  fi
fi

if [[ "$MODE" == "--validate-only" || "$MODE" == "--dry-run" ]]; then
  echo "================================================================="
  echo "Validação concluída com sucesso (--validate-only). Nenhuma alteração aplicada."
  echo "NOTA DE RISCO: Este orquestrador protege o caminho suportado de deploy."
  echo "O modo --execute NÃO é autorizado apenas por este script: ele exige adicionalmente"
  echo "  1) CUTOVER_OPERATOR + pacote de evidências assinado (CUTOVER_EVIDENCE_MANIFEST,"
  echo "     artefatos fora do repositório com sha256 e assinatura OpenSSH verificada"
  echo "     contra CUTOVER_ALLOWED_SIGNERS fora do repositório); e"
  echo "  2) todas as portas publicadas livres nos endereços renderizados pelo Compose."
  echo "Para executar o corte sequencial real quando autorizado:"
  echo "  $0 --execute"
  echo "================================================================="
  exit 0
fi

# -----------------------------------------------------------------------------
# FASE 3: DISPONIBILIDADE DE TODAS AS PORTAS PUBLICADAS (ANTES DE MUTAR)
# -----------------------------------------------------------------------------
echo "[FASE 3] Verificando bind de todas as portas publicadas no Compose renderizado..."
if ! printf '%s' "$COMPOSE_JSON" | python3 "$ROOT_DIR/scripts/release/validate_compose_config.py" --check-ports; then
  echo "BLOQUEIO DE PORTAS: endereço ou porta publicados indisponíveis; nenhuma mutação de serviço foi executada." >&2
  exit 3
fi
echo "[FASE 3] Todos os binds publicados estão disponíveis."

# -----------------------------------------------------------------------------
# FASE 3B: RESOLVER E CONFERIR TODOS OS DIGESTS ANTES DA PRIMEIRA MUTAÇÃO DE SERVIÇO
# -----------------------------------------------------------------------------
echo "[FASE 3B] Baixando e verificando todas as imagens por digest imutável..."
if ! IMAGE_REFS=$(printf '%s' "$COMPOSE_JSON" | python3 "$ROOT_DIR/scripts/release/validate_compose_config.py" --list-images); then
  echo "ERRO FATAL: Falha na lista de imagens imutáveis." >&2
  exit 2
fi
unset COMPOSE_JSON
while IFS= read -r image_ref; do
  [[ -n "$image_ref" ]] || continue
  if ! docker pull "$image_ref" >/dev/null 2>&1; then
    echo "ERRO FATAL: Falha ao baixar imagem pinada; nenhuma mutação de serviço foi executada." >&2
    exit 2
  fi
  if ! repo_digests=$(docker image inspect --format '{{json .RepoDigests}}' "$image_ref" 2>/dev/null); then
    echo "ERRO FATAL: Falha ao inspecionar digest local; nenhuma mutação de serviço foi executada." >&2
    exit 2
  fi
  if ! printf '%s' "$repo_digests" | python3 -c '
import json, sys
ref = sys.argv[1]
repo, digest = ref.rsplit("@", 1)
last = repo.rsplit("/", 1)[-1]
if ":" in last:
    repo = repo[:-(len(last.split(":", 1)[1]) + 1)]
try:
    found = json.load(sys.stdin)
except Exception:
    sys.exit(1)
sys.exit(0 if isinstance(found, list) and repo + "@" + digest in found else 1)
' "$image_ref"; then
    echo "ERRO FATAL: Digest local não corresponde à imagem pinada; nenhuma mutação de serviço foi executada." >&2
    exit 2
  fi
done <<< "$IMAGE_REFS"
unset IMAGE_REFS
echo "[FASE 3B] Todas as imagens pinadas foram baixadas e conferidas."

# -----------------------------------------------------------------------------
# FASE 4: VERIFICAÇÃO DE PRÉ-EXISTÊNCIA DE SERVIÇOS EM EXECUÇÃO
# -----------------------------------------------------------------------------
# NOTA DE RISCO: Este portão protege o caminho suportado de deploy. Um operador
# no host ainda poderia contornar executando comandos docker compose manuais diretamente;
# não se alega impossibilidade física absoluta contra bypass direto de Compose.
echo "[FASE 4] Verificando se serviços de aplicação já estão rodando no projeto..."
if ! RUNNING_SVCS=$("${COMPOSE_CMD[@]}" ps --services --filter "status=running" 2>/dev/null); then
  echo "ERRO FATAL: Falha ao inspecionar status dos serviços do projeto catbrasil-prod via 'docker compose ps'." >&2
  exit 2
fi
for s in backend-api backend-cron app-client app-admin caddy prometheus node-exporter alertmanager alert-relay; do
  if echo "$RUNNING_SVCS" | grep -qx "$s"; then
    echo "BLOQUEIO DE SEGURANÇA: O serviço '$s' já está em execução no projeto catbrasil-prod!" >&2
    echo "O corte sequencial exige que nenhum serviço de aplicação esteja ativo antes da migração e do preflight." >&2
    exit 2
  fi
done
echo "[FASE 4] Nenhum serviço conflitante em execução no caminho suportado."

# -----------------------------------------------------------------------------
# FASE 5: INICIALIZAÇÃO ISOLADA DO POSTGRESQL NOVO (PRIMEIRA MUTAÇÃO DOCKER)
# -----------------------------------------------------------------------------
echo "[FASE 5] Inicializando exclusivamente o serviço PostgreSQL no volume catbrasil_prod_pgdata..."
"${COMPOSE_CMD[@]}" up -d postgres

echo "[FASE 5] Aguardando prontidão saudável do PostgreSQL (healthcheck)..."
healthy=0
for _ in {1..30}; do
  status=$("${COMPOSE_CMD[@]}" ps postgres --format '{{.Health}}' 2>/dev/null || true)
  if [[ "$status" == "healthy" ]]; then
    healthy=1
    break
  fi
  sleep 1
done

if [[ $healthy -ne 1 ]]; then
  echo "ERRO FATAL: PostgreSQL não atingiu estado saudável no tempo limite." >&2
  exit 2
fi
echo "[FASE 5] PostgreSQL operacional e saudável."

# -----------------------------------------------------------------------------
# FASE 6: EXECUÇÃO DAS MIGRAÇÕES (1..11)
# -----------------------------------------------------------------------------
echo "[FASE 6] Aplicando migrações de schema (backend-migrate up)..."
"${COMPOSE_CMD[@]}" run --rm backend-migrate

# -----------------------------------------------------------------------------
# FASE 7: PORTÃO TÉCNICO DE CORTE (PREFLIGHT GATE FAIL-CLOSED)
# -----------------------------------------------------------------------------
echo "[FASE 7] Executando Portão Técnico de Corte (scripts/release/preflight_cutover.py)..."
PG_CONTAINER_ID=$("${COMPOSE_CMD[@]}" ps -q postgres)

if [[ -z "$PG_CONTAINER_ID" ]]; then
  echo "ERRO FATAL: Não foi possível obter o ID do contêiner PostgreSQL." >&2
  exit 2
fi

if ! python3 scripts/release/preflight_cutover.py --container "$PG_CONTAINER_ID"; then
  echo "=================================================================" >&2
  echo "BLOQUEIO CRÍTICO DE SEGURANÇA: O Portão Técnico de Corte REJEITOU o banco!" >&2
  echo "A inicialização da backend-api e dos serviços financeiros foi ABORTADA." >&2
  echo "Nenhum tráfego foi disponibilizado." >&2
  echo "=================================================================" >&2
  exit 1
fi
echo "[FASE 7] Portão Técnico de Corte APROVADO com sucesso (APPROVED_CLEAN_BASELINE)."

echo "[FASE 7B] Criando primeiro diretor com senha exclusiva e TOTP no terminal do operador..."
if ! "${COMPOSE_CMD[@]}" run --rm --no-deps --interactive --tty bootstrap-admin \
  --username "$BOOTSTRAP_ADMIN_USERNAME_VALUE"; then
  echo "BLOQUEIO DE BOOTSTRAP: primeiro diretor não foi criado; serviços públicos não serão iniciados." >&2
  exit 3
fi

# -----------------------------------------------------------------------------
# FASE 8: INICIALIZAÇÃO CONTROLADA E SEQUENCIAL COM --no-deps
# -----------------------------------------------------------------------------
PHASE8_COMPLETE=0
cleanup_phase8_on_exit() {
  local original_status=$?
  if [[ "$PHASE8_COMPLETE" != "1" ]]; then
    echo "[FASE 8] Falha de prontidão: parando serviços de aplicação desta tentativa; PostgreSQL preservado para diagnóstico." >&2
    "${COMPOSE_CMD[@]}" stop --timeout 10 backend-api backend-cron app-client app-admin caddy prometheus node-exporter alertmanager alert-relay >/dev/null 2>&1 || true
  fi
  return "$original_status"
}
trap cleanup_phase8_on_exit EXIT

wait_service_status() {
  local service="$1" expected="$2" format="$3" status
  for _ in {1..30}; do
    status=$("${COMPOSE_CMD[@]}" ps "$service" --format "$format" 2>/dev/null || true)
    if [[ "$status" == "$expected" ]]; then
      return 0
    fi
    sleep 1
  done
  echo "ERRO FATAL: Serviço $service não atingiu estado $expected no tempo limite." >&2
  return 2
}

echo "[FASE 8] Inicializando API e aguardando readiness saudável..."
"${COMPOSE_CMD[@]}" up -d --no-deps backend-api
wait_service_status backend-api healthy '{{.Health}}'

echo "[FASE 8] Inicializando frontends e aguardando healthchecks..."
"${COMPOSE_CMD[@]}" up -d --no-deps app-client app-admin
wait_service_status app-client healthy '{{.Health}}'
wait_service_status app-admin healthy '{{.Health}}'

echo "[FASE 8] Inicializando worker e cadeia de alertas..."
"${COMPOSE_CMD[@]}" up -d --no-deps backend-cron
wait_service_status backend-cron running '{{.State}}'
"${COMPOSE_CMD[@]}" up -d --no-deps alert-relay
wait_service_status alert-relay healthy '{{.Health}}'
"${COMPOSE_CMD[@]}" up -d --no-deps alertmanager
wait_service_status alertmanager healthy '{{.Health}}'
"${COMPOSE_CMD[@]}" up -d --no-deps node-exporter
wait_service_status node-exporter running '{{.State}}'
"${COMPOSE_CMD[@]}" up -d --no-deps prometheus
wait_service_status prometheus healthy '{{.Health}}'
echo "[FASE 8] Inicializando acesso público após prontidão de alertas..."
"${COMPOSE_CMD[@]}" up -d --no-deps caddy
wait_service_status caddy healthy '{{.Health}}'
PHASE8_COMPLETE=1

echo "================================================================="
echo "  CORTE SEQUENCIAL GREENFIELD CONCLUÍDO COM SUCESSO"
echo "  Banco: catbrasil_prod | Volume: catbrasil_prod_pgdata"
echo "  Status Financeiro: SUITPAY_LIVE_ENABLED=false (Retido até P0.2-P0.6)"
echo "================================================================="
