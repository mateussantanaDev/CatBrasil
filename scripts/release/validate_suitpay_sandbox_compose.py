#!/usr/bin/env python3
"""
scripts/release/validate_suitpay_sandbox_compose.py

Validador semântico e read-only da configuração do Docker Compose para o ambiente
isolado e opt-in do Sandbox Oficial SuitPay (P0.5).

Objetivos estritos:
1. Validar que o projeto Compose é exclusivamente 'catbrasil-suitpay-sandbox'.
2. Garantir isolamento total de volumes e rede contra staging ('catbrasil-staging'),
   produção ('catbrasil_prod') e legado ('catbrasil').
3. Garantir ausência total do serviço stub ('pix-sandbox') e dependências associadas.
4. Garantir que APP_ENV='staging', PIX_MODE='suitpay' e PIX_SANDBOX_URL='' (vazio,
   forçando o uso da URL oficial hardcoded: https://sandbox.ws.suitpay.app/api/v1/gateway).
5. Garantir que SUITPAY_LIVE_ENABLED='false' de forma incondicional.
6. Garantir fechamento de portas de banco (postgres) e API (backend-api, frontends) no host.
7. Garantir Caddy configurado com domínios públicos válidos (não localhost) com HTTPS.
8. Garantir que a URL de callback aponta para a rota real '/api/v1/webhooks/suitpay' com HTTPS público.
9. Higiene estrita de segredos: JAMAIS imprime senhas, tokens, JWT ou credenciais.
10. Relatar explicitamente os limites da validação estática local.
"""

import argparse
import ipaddress
import json
from pathlib import Path
import re
import subprocess
import sys
import urllib.parse

EXPECTED_PROJECT = "catbrasil-suitpay-sandbox"
EXPECTED_DB = "catbrasil_suitpay_sandbox"
EXPECTED_VOLUME_PGDATA = "catbrasil_suitpay_sandbox_pgdata"
EXPECTED_VOLUME_CADDY_DATA = "catbrasil_suitpay_sandbox_caddy_data"
EXPECTED_VOLUME_CADDY_CONFIG = "catbrasil_suitpay_sandbox_caddy_config"
EXPECTED_VOLUME_PROMETHEUS = "catbrasil_suitpay_sandbox_prometheus_data"
EXPECTED_NETWORK = "catbrasil_suitpay_sandbox_net"

CRITICAL_SERVICES = [
    "postgres",
    "backend-migrate",
    "backend-api",
    "backend-cron",
    "app-client",
    "app-admin",
    "caddy",
]

PROHIBITED_PROJECTS = {"catbrasil", "catbrasil-staging", "catbrasil-prod"}
PROHIBITED_VOLUMES = {
    "catbrasil_pgdata", "catbrasil-staging_pgdata", "catbrasil_prod_pgdata",
    "catbrasil_caddy_data", "catbrasil-staging_caddy_data", "catbrasil_prod_caddy_data",
    "catbrasil_caddy_config", "catbrasil-staging_caddy_config", "catbrasil_prod_caddy_config",
    "catbrasil_prometheus_data", "catbrasil-staging_prometheus_data", "catbrasil_prod_prometheus_data"
}
PROHIBITED_NETWORKS = {"catbrasil_net", "catbrasil-staging_net", "catbrasil_prod_net"}

DOMAIN_REGEX = re.compile(
    r"^(?!(https?://))([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$"
)
EMAIL_REGEX = re.compile(r"^[^@\s]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$")


def parse_env_file(filepath: Path) -> dict:
    """Lê um arquivo .env sem expor valores ou carregar no ambiente global."""
    env = {}
    if not filepath.is_file():
        raise FileNotFoundError(f"Arquivo de ambiente não encontrado: {filepath.name}")
    try:
        with open(filepath, "r", encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if not line or line.startswith("#"):
                    continue
                if "=" in line:
                    key, val = line.split("=", 1)
                    key = key.strip()
                    val = val.strip().strip("'\"")
                    env[key] = val
    except Exception as exc:
        raise ValueError(f"Falha ao ler arquivo de ambiente {filepath.name}") from exc
    return env


def is_localhost_or_private_host(host: str) -> bool:
    """Verifica se o host é localhost, IP de loopback ou domínio privado/inválido."""
    if not host:
        return True
    host_clean = host.split(":")[0].strip().lower()
    if host_clean in ("localhost", "127.0.0.1", "::1", "0.0.0.0"):
        return True
    if (
        host_clean.endswith(".localhost")
        or host_clean.endswith(".local")
        or host_clean.endswith(".invalid")
        or host_clean.endswith(".test")
    ):
        return True
    return False


def validate_config(data: dict, env_vars: dict = None) -> list:
    """
    Executa a validação semântica da configuração renderizada.
    Retorna uma lista de mensagens de erro (vazia se aprovado).
    Nenhuma mensagem de erro contém segredos, credenciais ou valores de entrada.
    """
    errors = []
    env_vars = env_vars or {}
    services = data.get("services")
    if not isinstance(services, dict):
        errors.append("Seção 'services' ausente ou inválida no documento Compose renderizado")
        services = {}

    volumes = data.get("volumes", {})
    networks = data.get("networks", {})

    # 1. Presença Obrigatória de Serviços Críticos
    for svc in CRITICAL_SERVICES:
        if svc not in services or not isinstance(services.get(svc), dict):
            errors.append(f"Serviço crítico '{svc}' ausente na configuração do Compose")

    # 2. Validação do Projeto Compose
    project_name = data.get("name")
    if not project_name:
        errors.append("Nome do projeto Compose ausente no documento renderizado")
    elif project_name != EXPECTED_PROJECT:
        errors.append(f"Projeto Compose inválido; esperado '{EXPECTED_PROJECT}'")
    if project_name in PROHIBITED_PROJECTS:
        errors.append("Projeto Compose colide com ambiente restrito proibido")

    # 3. Validação de Isolamento de Volumes
    pgdata_vol = volumes.get("pgdata")
    pgdata_name = pgdata_vol.get("name") if isinstance(pgdata_vol, dict) else None
    if pgdata_name != EXPECTED_VOLUME_PGDATA:
        errors.append(f"Volume 'pgdata' inválido; esperado '{EXPECTED_VOLUME_PGDATA}'")
    if pgdata_name in PROHIBITED_VOLUMES:
        errors.append("Volume 'pgdata' colide com volume restrito de outro ambiente")

    caddy_data_vol = volumes.get("caddy_data")
    caddy_data_name = caddy_data_vol.get("name") if isinstance(caddy_data_vol, dict) else None
    if caddy_data_name != EXPECTED_VOLUME_CADDY_DATA:
        errors.append(f"Volume 'caddy_data' inválido; esperado '{EXPECTED_VOLUME_CADDY_DATA}'")
    if caddy_data_name in PROHIBITED_VOLUMES:
        errors.append("Volume 'caddy_data' colide com volume restrito de outro ambiente")

    caddy_cfg_vol = volumes.get("caddy_config")
    caddy_cfg_name = caddy_cfg_vol.get("name") if isinstance(caddy_cfg_vol, dict) else None
    if caddy_cfg_name != EXPECTED_VOLUME_CADDY_CONFIG:
        errors.append(f"Volume 'caddy_config' inválido; esperado '{EXPECTED_VOLUME_CADDY_CONFIG}'")
    if caddy_cfg_name in PROHIBITED_VOLUMES:
        errors.append("Volume 'caddy_config' colide com volume restrito de outro ambiente")

    prom_vol = volumes.get("prometheus_data")
    if prom_vol is not None:
        prom_vol_name = prom_vol.get("name") if isinstance(prom_vol, dict) else None
        if prom_vol_name != EXPECTED_VOLUME_PROMETHEUS:
            errors.append(f"Volume 'prometheus_data' inválido; esperado '{EXPECTED_VOLUME_PROMETHEUS}'")
        if prom_vol_name in PROHIBITED_VOLUMES:
            errors.append("Volume 'prometheus_data' colide com volume restrito de outro ambiente")

    # 4. Validação de Isolamento da Rede
    net_obj = networks.get("catbrasil_net")
    net_name = net_obj.get("name") if isinstance(net_obj, dict) else None
    if net_name != EXPECTED_NETWORK:
        errors.append(f"Rede 'catbrasil_net' inválida; esperada '{EXPECTED_NETWORK}'")
    if net_name in PROHIBITED_NETWORKS:
        errors.append("Rede 'catbrasil_net' colide com rede restrita de outro ambiente")
    if isinstance(net_obj, dict) and net_obj.get("external") is True:
        errors.append("Rede 'catbrasil_net' não deve ser external para garantir criação isolada no sandbox")

    # 5. Proibição de Stub (Ausência absoluta de pix-sandbox)
    if "pix-sandbox" in services:
        errors.append("Serviço proibido 'pix-sandbox' detectado no Compose; o sandbox oficial não utiliza stub")

    api_svc = services.get("backend-api", {})
    api_depends = api_svc.get("depends_on", {}) if isinstance(api_svc, dict) else {}
    if isinstance(api_depends, dict) and "pix-sandbox" in api_depends:
        errors.append("backend-api não deve ter dependência configurada para 'pix-sandbox'")
    elif isinstance(api_depends, list) and "pix-sandbox" in api_depends:
        errors.append("backend-api não deve ter dependência configurada para 'pix-sandbox'")

    # 6. Validação das Variáveis do Backend API
    api_env = api_svc.get("environment", {}) if isinstance(api_svc, dict) else {}
    app_env = api_env.get("APP_ENV")
    if app_env != "staging":
        errors.append("backend-api.environment.APP_ENV inválido; esperado 'staging'")

    pix_mode = api_env.get("PIX_MODE")
    if pix_mode != "suitpay":
        errors.append("backend-api.environment.PIX_MODE inválido; esperado 'suitpay'")

    # PIX_SANDBOX_URL DEVE ser vazio para acionar a URL oficial hardcoded no gateway
    # (backend/pkg/gateway/suitpay.go -> SuitPaySandboxURL = "https://sandbox.ws.suitpay.app/api/v1/gateway")
    pix_sandbox_url = api_env.get("PIX_SANDBOX_URL")
    if pix_sandbox_url is not None and str(pix_sandbox_url).strip() != "":
        errors.append(
            "backend-api.environment.PIX_SANDBOX_URL deve ser vazio para acionar a URL oficial "
            "do sandbox SuitPay ('https://sandbox.ws.suitpay.app/api/v1/gateway')"
        )

    # SUITPAY_LIVE_ENABLED deve ser exatamente 'false'
    suitpay_live_raw = api_env.get("SUITPAY_LIVE_ENABLED")
    if suitpay_live_raw is None:
        errors.append("backend-api.environment.SUITPAY_LIVE_ENABLED ausente; esperado exatamente 'false'")
    else:
        suitpay_live = str(suitpay_live_raw).strip().lower()
        if suitpay_live != "false":
            errors.append("backend-api.environment.SUITPAY_LIVE_ENABLED inválido; esperado exatamente 'false'")

    # 7. Validação do Banco de Dados e Conexão Interna
    pg_svc = services.get("postgres", {}) if isinstance(services, dict) else {}
    pg_env = pg_svc.get("environment", {}) if isinstance(pg_svc, dict) else {}
    pg_db = pg_env.get("POSTGRES_DB")
    if pg_db != EXPECTED_DB:
        errors.append(f"postgres.environment.POSTGRES_DB inválido; esperado '{EXPECTED_DB}'")

    for svc_name in ["backend-migrate", "backend-api", "backend-cron"]:
        s_svc = services.get(svc_name, {}) if isinstance(services, dict) else {}
        s_env = s_svc.get("environment", {}) if isinstance(s_svc, dict) else {}
        db_url = s_env.get("DATABASE_URL")
        if not db_url or not isinstance(db_url, str) or not db_url.strip():
            errors.append(f"{svc_name}: DATABASE_URL ausente; esperado host 'postgres' e base '{EXPECTED_DB}'")
            continue
        try:
            u = urllib.parse.urlparse(db_url)
            if u.scheme not in ("postgres", "postgresql"):
                errors.append(f"{svc_name}: esquema de DATABASE_URL inválido; esperado 'postgres' ou 'postgresql'")
            if u.hostname != "postgres":
                errors.append(f"{svc_name}: host de DATABASE_URL inválido; esperado host interno 'postgres'")
            try:
                port = u.port or 5432
            except Exception:
                port = None
            if port != 5432:
                errors.append(f"{svc_name}: porta de DATABASE_URL inválida; esperada porta 5432")
            path = u.path.strip("/")
            if path != EXPECTED_DB:
                errors.append(f"{svc_name}: base de dados de DATABASE_URL inválida; esperada '{EXPECTED_DB}'")
        except Exception:
            errors.append(f"{svc_name}: falha ao analisar estrutura de DATABASE_URL; formato inválido")

    # 8. Validação de Fechamento Estrito de Portas no Host
    # Banco, API e Frontends não podem expor portas no host
    for svc_name in ["postgres", "backend-api", "backend-migrate", "backend-cron", "app-client", "app-admin"]:
        svc_obj = services.get(svc_name, {}) if isinstance(services, dict) else {}
        ports = svc_obj.get("ports")
        if ports:
            errors.append(f"{svc_name} não deve expor portas no host; esperado tráfego exclusivo via Caddy")

    # Prometheus só pode expor porta em loopback se presente
    prom_svc = services.get("prometheus")
    if prom_svc and isinstance(prom_svc, dict):
        prom_ports = prom_svc.get("ports", []) or []
        for p in prom_ports:
            host_ip = None
            if isinstance(p, dict):
                host_ip = p.get("host_ip")
            elif isinstance(p, str):
                parts = p.split(":")
                if len(parts) >= 2:
                    host_ip = parts[0]
            if host_ip not in ("127.0.0.1", "localhost"):
                errors.append("prometheus deve expor portas exclusivamente na interface loopback (127.0.0.1)")

    # 9. Validação de Portas Publicadas do Caddy (Obrigatório 80 e 443 no host)
    caddy_svc = services.get("caddy", {}) if isinstance(services, dict) else {}
    caddy_ports = caddy_svc.get("ports") if isinstance(caddy_svc, dict) else None
    published_80 = False
    published_443 = False
    caddy_loopback_detected = False

    if isinstance(caddy_ports, list):
        for p in caddy_ports:
            host_ip = None
            published = None
            target = None
            if isinstance(p, dict):
                host_ip = p.get("host_ip") or ""
                published = str(p.get("published") or "")
                target = str(p.get("target") or "")
            elif isinstance(p, str):
                p_clean = p.split("/")[0]
                parts = p_clean.split(":")
                if len(parts) == 3:
                    host_ip = parts[0]
                    published = parts[1]
                    target = parts[2]
                elif len(parts) == 2:
                    published = parts[0]
                    target = parts[1]
                elif len(parts) == 1:
                    target = parts[0]

            if published == "80" and target in ("80", ""):
                published_80 = True
                if str(host_ip).strip() in ("127.0.0.1", "localhost"):
                    caddy_loopback_detected = True
            elif published == "443" and target in ("443", ""):
                published_443 = True
                if str(host_ip).strip() in ("127.0.0.1", "localhost"):
                    caddy_loopback_detected = True

    if not published_80 or not published_443:
        errors.append(
            "Caddy deve publicar obrigatoriamente as portas 80 e 443 no host para terminação TLS e webhooks públicos"
        )
    if caddy_loopback_detected:
        errors.append(
            "Caddy não deve vincular portas 80/443 exclusivamente a loopback; bind público necessário para webhooks"
        )

    # 10. Validação de Caddy e Domínios Públicos HTTPS (Não localhost)
    caddy_env = caddy_svc.get("environment", {}) if isinstance(caddy_svc, dict) else {}

    for dom_key in ["CLIENT_DOMAIN", "ADMIN_DOMAIN", "API_DOMAIN"]:
        domain_val = caddy_env.get(dom_key) or env_vars.get(dom_key)
        if not domain_val or not str(domain_val).strip():
            errors.append(f"Caddy: variável de domínio obrigatória '{dom_key}' ausente ou vazia")
            continue
        domain_str = str(domain_val).strip()
        if is_localhost_or_private_host(domain_str):
            errors.append(
                f"Caddy: domínio '{dom_key}' inválido; "
                "o sandbox SuitPay oficial exige domínio público com HTTPS válido (não localhost ou IP privado)"
            )
        elif not DOMAIN_REGEX.match(domain_str):
            errors.append(
                f"Caddy: domínio '{dom_key}' com formato inválido; "
                "esperado domínio FQDN sem protocolo (ex: api.sandbox.dominio.com)"
            )

    acme_email = caddy_env.get("ACME_EMAIL") or env_vars.get("ACME_EMAIL")
    if not acme_email or not str(acme_email).strip():
        errors.append("Caddy: ACME_EMAIL ausente; obrigatório para emissão automática de certificados TLS")
    else:
        acme_str = str(acme_email).strip()
        if not EMAIL_REGEX.match(acme_str) or is_localhost_or_private_host(acme_str.split("@")[-1]):
            errors.append("Caddy: ACME_EMAIL com formato inválido; esperado endereço de e-mail com domínio público válido")

    # 11. Validação Obrigatória e Estrita da URL de Callback Oficial SuitPay
    callback_url = env_vars.get("SUITPAY_CALLBACK_URL")
    if not callback_url or not str(callback_url).strip():
        errors.append("SUITPAY_CALLBACK_URL ausente ou vazia; obrigatória para recebimento de webhooks SuitPay")
    else:
        cb_str = str(callback_url).strip()
        try:
            parsed = urllib.parse.urlparse(cb_str)
            if parsed.scheme != "https":
                errors.append("SUITPAY_CALLBACK_URL deve usar obrigatoriamente o esquema público HTTPS ('https://')")

            if parsed.username or parsed.password:
                errors.append("SUITPAY_CALLBACK_URL não deve conter credenciais de autenticação (userinfo)")

            netloc_host = parsed.netloc.split("@")[-1]
            if ":" in netloc_host and not (netloc_host.startswith("[") and netloc_host.endswith("]")):
                errors.append("SUITPAY_CALLBACK_URL não deve conter porta alternativa na URL")

            if parsed.query:
                errors.append("SUITPAY_CALLBACK_URL não deve conter parâmetros de consulta (query)")

            if parsed.fragment:
                errors.append("SUITPAY_CALLBACK_URL não deve conter fragmento (#)")

            cb_host = parsed.hostname or ""
            is_ip = False
            try:
                ipaddress.ip_address(cb_host)
                is_ip = True
            except ValueError:
                is_ip = False

            if is_ip:
                errors.append("SUITPAY_CALLBACK_URL não deve utilizar endereço IP; esperado domínio público FQDN")
            elif is_localhost_or_private_host(cb_host):
                errors.append(
                    "SUITPAY_CALLBACK_URL deve usar um domínio público acessível pela SuitPay (não localhost ou IP privado)"
                )

            if parsed.path != "/api/v1/webhooks/suitpay":
                errors.append(
                    "SUITPAY_CALLBACK_URL possui rota incorreta; esperada rota real '/api/v1/webhooks/suitpay'"
                )

            api_domain_val = caddy_env.get("API_DOMAIN") or env_vars.get("API_DOMAIN")
            api_domain_clean = str(api_domain_val).strip() if api_domain_val else ""

            if not api_domain_clean:
                errors.append("SUITPAY_CALLBACK_URL requer API_DOMAIN configurado para correspondência exata")
            elif cb_host != api_domain_clean:
                errors.append(
                    "SUITPAY_CALLBACK_URL deve corresponder exatamente ao domínio configurado em API_DOMAIN"
                )

            if api_domain_clean and cb_str != f"https://{api_domain_clean}/api/v1/webhooks/suitpay":
                if not any("SUITPAY_CALLBACK_URL" in e for e in errors):
                    errors.append(
                        "SUITPAY_CALLBACK_URL deve corresponder exatamente a 'https://{API_DOMAIN}/api/v1/webhooks/suitpay'"
                    )
        except Exception:
            errors.append("SUITPAY_CALLBACK_URL possui formato de URL inválido")

    # 12. Higiene de Segredos Obrigatórios (Apenas verificação de presença e comprimento mínimo)
    secret_specs = [
        ("JWT_SECRET", 32),
        ("PIX_WEBHOOK_SECRET", 32),
        ("ADMIN_SECRET_PIN", 6),
        ("IDENTITY_SECRET", 32),
    ]
    for sec_name, min_len in secret_specs:
        sec_val = api_env.get(sec_name) or env_vars.get(sec_name)
        if not sec_val or len(str(sec_val).strip()) < min_len:
            errors.append(
                f"backend-api.environment.{sec_name} ausente ou comprimento insuficiente "
                f"(mínimo de {min_len} caracteres)"
            )

    return errors


def run_docker_compose_config(env_file_path: Path, compose_files: list, project_name: str) -> dict:
    """Executa 'docker compose config --format json' de forma segura e sem vazar segredos."""
    cmd = ["docker", "compose", "-p", project_name, "--env-file", str(env_file_path)]
    for cf in compose_files:
        cmd.extend(["-f", str(cf)])
    cmd.extend(["config", "--format", "json"])

    try:
        res = subprocess.run(cmd, capture_output=True, text=True, check=True, timeout=30)
        return json.loads(res.stdout)
    except subprocess.CalledProcessError as exc:
        # Higiene estrita: JAMAIS imprimir stderr ou stdout que possam conter interpolação de secrets
        raise RuntimeError(
            f"Comando 'docker compose config' falhou com código {exc.returncode}. "
            "Verifique se o arquivo de variáveis contém todas as chaves obrigatórias sem erros de interpolação."
        ) from None
    except json.JSONDecodeError as exc:
        raise RuntimeError("Falha ao decodificar saída JSON de 'docker compose config'.") from exc
    except OSError as exc:
        raise RuntimeError(f"Docker Compose não disponível no sistema operacional ({type(exc).__name__}).") from exc


def print_success_report():
    print("=" * 80)
    print("RELATÓRIO DE VALIDAÇÃO: CATBRASIL SUITPAY SANDBOX (P0.5)")
    print("=" * 80)
    print("[OK] Projeto Compose isolado: catbrasil-suitpay-sandbox")
    print("[OK] Volumes dedicados sem colisão com staging, legado ou produção")
    print("[OK] Rede privada dedicada: catbrasil_suitpay_sandbox_net")
    print("[OK] Serviços críticos verificados: postgres, backend-migrate, backend-api, backend-cron, app-client, app-admin, caddy")
    print("[OK] Ausência total de stub local (pix-sandbox ausente e sem dependência)")
    print("[OK] Configuração de runtime: APP_ENV=staging, PIX_MODE=suitpay")
    print("[OK] PIX_SANDBOX_URL vazio (aciona gateway oficial SuitPay sandbox)")
    print("[OK] SUITPAY_LIVE_ENABLED=false (travamento financeiro estrito ativo)")
    print("[OK] Portas de banco e API fechadas no host (tráfego exclusivo via Caddy)")
    print("[OK] Caddy com portas públicas 80 e 443 expostas para tráfego web e webhooks")
    print("[OK] Domínios públicos FQDN configurados para TLS automático (não localhost)")
    print("[OK] Webhook HTTPS obrigatório e estrito apontando para rota real /api/v1/webhooks/suitpay")
    print("[OK] Segredos obrigatórios configurados com comprimento mínimo (não expostos)")
    print("-" * 80)
    print("AVISO DE FRONTEIRA E LIMITES TÉCNICOS:")
    print("A aprovação desta configuração estática comprova apenas a integridade e")
    print("o isolamento da infraestrutura local Docker Compose.")
    print("Ela NÃO comprova:")
    print("  1. Validade das credenciais de sandbox junto ao provedor SuitPay.")
    print("  2. Liberação de IP da VPS no painel administrativo da SuitPay.")
    print("  3. Roteamento DNS externo e entrega real de webhooks HTTPS pelo provedor.")
    print("  4. Liquidação bancária, conciliação de eventos ou saque no provedor oficial.")
    print("  5. Gateway de SMS ausente no overlay (cadastro novo e prova E2E dependem de P0.4 ou fixture sintética segregada; não usar seed legado).")
    print("=" * 80)
    print("Configuração do sandbox SuitPay aprovada com sucesso.")


def main():
    parser = argparse.ArgumentParser(
        description="Validador semântico read-only para Docker Compose SuitPay Sandbox (P0.5)."
    )
    parser.add_argument(
        "--env-file",
        type=Path,
        help="Caminho para o arquivo .env (ex: .suitpay-sandbox.env). Não exibe valores.",
    )
    parser.add_argument(
        "--json-input",
        type=Path,
        help="Caminho para arquivo JSON pré-renderizado (para testes ou execução sem daemon Docker).",
    )
    parser.add_argument(
        "--compose-file",
        action="append",
        default=[],
        help="Arquivos compose (padrão: docker-compose.yml e compose.suitpay-sandbox.yml).",
    )
    parser.add_argument(
        "--project-name",
        default=EXPECTED_PROJECT,
        help=f"Nome do projeto esperado (padrão: {EXPECTED_PROJECT}).",
    )
    args = parser.parse_args()

    env_vars = {}
    compose_data = None

    if args.json_input:
        try:
            with open(args.json_input, "r", encoding="utf-8") as f:
                compose_data = json.load(f)
        except Exception as exc:
            print(f"ERRO FATAL: Falha ao carregar JSON de entrada: {exc}", file=sys.stderr)
            sys.exit(1)
    elif not sys.stdin.isatty():
        raw = sys.stdin.read().strip()
        if raw:
            try:
                compose_data = json.loads(raw)
            except Exception as exc:
                print("ERRO FATAL: Falha ao decodificar JSON do Compose via stdin.", file=sys.stderr)
                sys.exit(1)

    if args.env_file:
        try:
            env_vars = parse_env_file(args.env_file)
        except Exception as exc:
            print(f"ERRO FATAL: {exc}", file=sys.stderr)
            sys.exit(1)

        if compose_data is None:
            compose_files = args.compose_file or [
                Path("docker-compose.yml"),
                Path("compose.suitpay-sandbox.yml"),
            ]
            try:
                compose_data = run_docker_compose_config(
                    args.env_file, compose_files, args.project_name
                )
            except RuntimeError as exc:
                print(f"ERRO FATAL: {exc}", file=sys.stderr)
                sys.exit(1)

    if compose_data is None:
        print(
            "ERRO FATAL: Nenhuma entrada fornecida. Especifique --env-file, --json-input ou forneça JSON via stdin.",
            file=sys.stderr,
        )
        sys.exit(1)

    errors = validate_config(compose_data, env_vars)
    if errors:
        print("=" * 80, file=sys.stderr)
        print("FALHA NA VALIDAÇÃO DO COMPOSE SUITPAY SANDBOX (P0.5):", file=sys.stderr)
        print("=" * 80, file=sys.stderr)
        for err in errors:
            print(f" [REJEIÇÃO] {err}", file=sys.stderr)
        print("-" * 80, file=sys.stderr)
        print("Ajuste a configuração antes de tentar qualquer execução.", file=sys.stderr)
        sys.exit(2)

    print_success_report()
    sys.exit(0)


if __name__ == "__main__":
    main()
