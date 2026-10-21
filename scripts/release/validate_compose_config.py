#!/usr/bin/env python3
"""
scripts/release/validate_compose_config.py

Validador semântico da configuração final renderizada do Docker Compose para produção greenfield.
Recebe o JSON de 'docker compose config --format json' via stdin (ou arquivo).
Garante que precedência de variáveis de ambiente do shell ou erros de configuração não desviem
o banco de dados, o volume, a rede ou as variáveis de contenção financeira.

Dois modos:
  1. Modo stdin (padrão): validação da configuração renderizada do Compose.
  2. Modo evidências: --evidence-manifest PATH --operator NAME [--allowed-signers PATH] [--signature PATH]
     Portão fail-closed do corte P0.6. NÃO confia em status/ref autoafirmados do JSON.

Regras Estritas de Segurança (modo stdin):
1. volumes.pgdata.name == 'catbrasil_prod_pgdata'
2. networks.catbrasil_net.name == 'catbrasil_prod_net'
3. services.postgres.environment.POSTGRES_DB == 'catbrasil_prod'
4. services.backend-migrate, backend-api e backend-cron:
   DATABASE_URL scheme em ('postgres', 'postgresql'), hostname == 'postgres', port == 5432, path == '/catbrasil_prod'
5. services.backend-api:
   APP_ENV == 'production'
   PIX_MODE == 'suitpay'
   SUITPAY_LIVE_ENABLED deve ser exatamente 'false' (ausente ou qualquer outro valor bloqueia)
6. Imagens de produção devem ser imutáveis: para postgres, backend-migrate, backend-api,
   backend-cron, app-client, app-admin, caddy e prometheus o campo 'image' renderizado deve
   terminar em '@sha256:<64 hex minúsculo>'. Erros citam apenas o nome do serviço e a expectativa.
7. API sem porta publicada; Caddy em rede proxy dedicada com IP fixo /32 explicitamente
   confiável pela API.

Regras do Portão de Evidências (modo --evidence-manifest):
1. Manifesto JSON deve existir, parsear e ser objeto.
2. approved_by deve ser igual ao --operator (identidade do operador) e approved_at não vazio.
3. Assinatura OpenSSH detached do manifesto deve ser validada com
   'ssh-keygen -Y verify -I <operador> -f <allowed_signers> -n <namespace> -s <assinatura>',
   com o manifesto lido via stdin do ssh-keygen. O allowed_signers deve existir, ser arquivo
   regular e estar FORA do repositório. ssh-keygen indisponível, allowed_signers ausente ou
   verificação falhando => recusa (fail-closed).
4. A identidade do operador (--operator / approved_by) é também a identidade assinante exigida
   na verificação (principals do allowed_signers), ou seja, operador == assinante.
5. 6 itens obrigatórios, cada um um objeto com: status == "approved" (string exata),
   evidence_ref string não vazia (mín. 5 caracteres), verified_by string não vazia e um
   objeto 'artifact' com 'path' absoluto de um arquivo REAL fora do repositório e 'sha256'
   (64 hex minúsculo) que DEVE existir e cujo digest DEVE conferir com o arquivo.
   Itens: p0_4_identity_sms, p0_5_sandbox_official, backup_restored_offhost,
   monitoring_alerts, images_immutable, release_checklist_signed.
6. p0_4_identity_sms.verified_by deve ser distinto do operador aprovador (revisor distinto).
7. Nenhum conteúdo de arquivo, caminho, digest ou valor recebido é impresso; apenas nomes de
   campo e expectativa (higiene de segredos).

Limites declarados (consistência != autenticidade):
- A assinatura OpenSSH prova IDENTIDADE DE APROVAÇÃO (quem assumiu a responsabilidade), não a
  verdade do mundo real do conteúdo das evidências. O portão não autentica nem audita o
  conteúdo dos artefatos; ele exige existência, integridade byte a byte (sha256), localização
  fora do repositório e um aprovador identificado por chave.
- Digests de imagem são validados apenas por FORMATO; não são conferidos contra o registry
  offline (o operador deve obter o digest real antes do corte).
- Higiene absoluta de credenciais: JAMAIS imprime partes de DATABASE_URL, senhas, tokens,
  caminhos de artefatos nem mensagens de exceção de parsing que possam conter dados sensíveis.
"""

import argparse
import hashlib
import ipaddress
import json
import re
import socket
import shutil
import subprocess
import sys
import urllib.parse
from pathlib import Path

EXPECTED_DB = "catbrasil_prod"
EXPECTED_VOLUME = "catbrasil_prod_pgdata"
EXPECTED_NETWORK = "catbrasil_prod_net"
EXPECTED_PROXY_NETWORK = "catbrasil_prod_proxy_net"
EXPECTED_PROXY_IP = "172.30.240.2"
EXPECTED_PROXY_SUBNET = "172.30.240.0/28"

REPO_ROOT = Path(__file__).resolve().parents[2]

# Serviços de produção, inclusive bootstrap sem logs, cuja imagem precisa ser imutável.
PINNED_IMAGE_SERVICES = (
    "postgres",
    "backend-migrate",
    "backend-api",
    "bootstrap-admin",
    "backend-cron",
    "app-client",
    "app-admin",
    "caddy",
    "prometheus",
    "node-exporter",
    "alertmanager",
    "alert-relay",
)
IMAGE_DIGEST_RE = re.compile(r"[A-Za-z0-9][A-Za-z0-9._:/-]*@sha256:[0-9a-f]{64}")
SHA256_HEX_RE = re.compile(r"^[0-9a-f]{64}$")
OPERATOR_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$")

EVIDENCE_ITEMS = (
    "p0_4_identity_sms",
    "p0_5_sandbox_official",
    "backup_restored_offhost",
    "monitoring_alerts",
    "images_immutable",
    "release_checklist_signed",
)
EVIDENCE_SIGN_NAMESPACE = "catbrasil:release:evidence-manifest:v1"
MIN_EVIDENCE_REF_LEN = 5


def validate_config(data: dict) -> list:
    errors = []
    services = data.get("services", {})
    volumes = data.get("volumes", {})
    networks = data.get("networks", {})

    # 1. Volume pgdata renderizado
    pgdata_vol = volumes.get("pgdata", {})
    vol_name = pgdata_vol.get("name") if isinstance(pgdata_vol, dict) else None
    if vol_name != EXPECTED_VOLUME:
        errors.append(f"Volume 'pgdata' inválido; esperado '{EXPECTED_VOLUME}'")

    # 2. Rede catbrasil_net renderizada
    net_obj = networks.get("catbrasil_net", {})
    net_name = net_obj.get("name") if isinstance(net_obj, dict) else None
    if net_name != EXPECTED_NETWORK:
        errors.append(f"Rede 'catbrasil_net' inválida; esperada '{EXPECTED_NETWORK}'")

    # 3. Serviço postgres e POSTGRES_DB
    pg_svc = services.get("postgres", {})
    pg_env = pg_svc.get("environment", {}) if isinstance(pg_svc, dict) else {}
    postgres_db = pg_env.get("POSTGRES_DB")
    if postgres_db != EXPECTED_DB:
        errors.append(f"postgres.environment.POSTGRES_DB inválido; esperado '{EXPECTED_DB}'")
    postgres_password = pg_env.get("POSTGRES_PASSWORD")
    if not isinstance(postgres_password, str) or len(postgres_password) < 24:
        errors.append("postgres.environment.POSTGRES_PASSWORD ausente ou curto; mínimo 24 caracteres")

    # 4. DATABASE_URL em todos os serviços que acessam o banco
    # Higiene estrita: nenhuma exceção nem valor recebido é concatenado na mensagem
    for svc_name in ["backend-migrate", "backend-api", "backend-cron", "bootstrap-admin"]:
        s_obj = services.get(svc_name, {})
        s_env = s_obj.get("environment", {}) if isinstance(s_obj, dict) else {}
        db_url = s_env.get("DATABASE_URL")
        if not db_url or not isinstance(db_url, str) or not db_url.strip():
            errors.append(f"{svc_name}: DATABASE_URL ausente; esperado formato 'postgres://postgres:5432/{EXPECTED_DB}'")
            continue
        try:
            u = urllib.parse.urlparse(db_url)
            if u.scheme not in ("postgres", "postgresql"):
                errors.append(f"{svc_name}: esquema de DATABASE_URL inválido; esperado 'postgres' ou 'postgresql'")
            if u.hostname != "postgres":
                errors.append(f"{svc_name}: host de DATABASE_URL inválido; esperado host 'postgres'")
            try:
                port = u.port or 5432
            except Exception:
                port = None
            if port != 5432:
                errors.append(f"{svc_name}: porta de DATABASE_URL inválida; esperada porta 5432")
            path = u.path.strip("/")
            if path != EXPECTED_DB:
                errors.append(f"{svc_name}: base de dados de DATABASE_URL inválida; esperada '{EXPECTED_DB}'")
            password = urllib.parse.unquote(u.password or "")
            if not password or password != postgres_password:
                errors.append(f"{svc_name}: senha de DATABASE_URL ausente ou diferente de POSTGRES_PASSWORD")
        except Exception:
            errors.append(f"{svc_name}: falha ao analisar estrutura de DATABASE_URL; esperado formato 'postgres://postgres:5432/{EXPECTED_DB}'")

    # 5. Variáveis operacionais em backend-api
    api_obj = services.get("backend-api", {})
    api_env = api_obj.get("environment", {}) if isinstance(api_obj, dict) else {}
    if api_obj.get("ports"):
        errors.append("backend-api: portas publicadas são proibidas")
    proxy_net = networks.get("proxy_net", {})
    if not isinstance(proxy_net, dict) or proxy_net.get("name") != EXPECTED_PROXY_NETWORK:
        errors.append("proxy_net de produção ausente ou inválida")
    proxy_ipam = proxy_net.get("ipam", {}) if isinstance(proxy_net, dict) else {}
    proxy_subnets = [entry.get("subnet") for entry in proxy_ipam.get("config", []) if isinstance(entry, dict)] if isinstance(proxy_ipam, dict) else []
    if EXPECTED_PROXY_SUBNET not in proxy_subnets:
        errors.append("proxy_net: subnet dedicada inválida")
    api_proxy = api_obj.get("networks", {}).get("proxy_net", {}) if isinstance(api_obj.get("networks"), dict) else {}
    if "api-proxy" not in (api_proxy.get("aliases") or []):
        errors.append("backend-api: alias api-proxy ausente na rede dedicada")
    caddy_obj = services.get("caddy", {})
    caddy_proxy = caddy_obj.get("networks", {}).get("proxy_net", {}) if isinstance(caddy_obj.get("networks"), dict) else {}
    if caddy_proxy.get("ipv4_address") != EXPECTED_PROXY_IP:
        errors.append("caddy: IP do proxy dedicado inválido")
    if api_env.get("TRUSTED_PROXY_CIDRS") != EXPECTED_PROXY_IP + "/32":
        errors.append("backend-api: TRUSTED_PROXY_CIDRS deve confiar apenas no Caddy dedicado")
    app_env = api_env.get("APP_ENV")
    if app_env != "production":
        errors.append("backend-api.environment.APP_ENV inválido; esperado 'production'")

    for name, minimum in (("JWT_SECRET", 32), ("PIX_WEBHOOK_SECRET", 32),
                          ("IDENTITY_SECRET", 32), ("SMS_CODE_SECRET", 32),
                          ("ADMIN_SECRET_PIN", 6)):
        value = api_env.get(name)
        if not isinstance(value, str) or len(value.strip()) < minimum:
            errors.append(f"backend-api.environment.{name} ausente ou curto; mínimo {minimum} caracteres")
    if not isinstance(api_env.get("SMS_GATEWAY_TOKEN"), str) or not api_env.get("SMS_GATEWAY_TOKEN").strip():
        errors.append("backend-api.environment.SMS_GATEWAY_TOKEN ausente")
    gateway_url = api_env.get("SMS_GATEWAY_URL")
    if not isinstance(gateway_url, str) or not gateway_url.startswith("https://"):
        errors.append("backend-api.environment.SMS_GATEWAY_URL deve usar HTTPS")
    if not isinstance(api_env.get("CORS_ALLOWED_ORIGINS"), str) or not api_env.get("CORS_ALLOWED_ORIGINS").strip():
        errors.append("backend-api.environment.CORS_ALLOWED_ORIGINS ausente")

    pix_mode = api_env.get("PIX_MODE")
    if pix_mode != "suitpay":
        errors.append("backend-api.environment.PIX_MODE inválido; esperado 'suitpay'")

    # Exigir exatamente 'false' (qualquer valor diferente, desconhecido ou ausente bloqueia)
    suitpay_raw = api_env.get("SUITPAY_LIVE_ENABLED")
    if suitpay_raw is None:
        errors.append("backend-api.environment.SUITPAY_LIVE_ENABLED ausente; esperado exatamente 'false'")
    elif suitpay_raw != "false":
        errors.append("backend-api.environment.SUITPAY_LIVE_ENABLED inválido; esperado exatamente 'false'")

    bootstrap = services.get("bootstrap-admin")
    bootstrap = bootstrap if isinstance(bootstrap, dict) else {}
    bootstrap_env = bootstrap.get("environment", {})
    if bootstrap_env.get("APP_ENV") != "production":
        errors.append("bootstrap-admin.environment.APP_ENV inválido; esperado 'production'")
    if bootstrap_env.get("ADMIN_SECRET_PIN") != api_env.get("ADMIN_SECRET_PIN"):
        errors.append("bootstrap-admin.environment.ADMIN_SECRET_PIN deve ser idêntico ao da API")
    api_image = api_obj.get("image") if isinstance(api_obj, dict) else None
    if bootstrap.get("image") != api_image:
        errors.append("bootstrap-admin: imagem deve ser idêntica à backend-api")
    if bootstrap.get("entrypoint") != ["/app/bootstrap-admin"]:
        errors.append("bootstrap-admin: entrypoint inválido")
    if bootstrap.get("logging", {}).get("driver") != "none":
        errors.append("bootstrap-admin: logging Docker deve estar desativado")
    if bootstrap.get("ports"):
        errors.append("bootstrap-admin: portas publicadas são proibidas")

    relay = services.get("alert-relay", {})
    relay = relay if isinstance(relay, dict) else {}
    relay_env = relay.get("environment", {})
    relay_env = relay_env if isinstance(relay_env, dict) else {}
    if relay.get("ports"):
        errors.append("alert-relay: portas publicadas são proibidas")
    if relay_env.get("APP_ENV") != "production":
        errors.append("alert-relay.environment.APP_ENV inválido; esperado 'production'")
    ntfy_url = relay_env.get("NTFY_URL")
    try:
        destination = urllib.parse.urlsplit(ntfy_url if isinstance(ntfy_url, str) else "")
        host = (destination.hostname or "").lower()
        try:
            local_ip = ipaddress.ip_address(host).is_loopback or ipaddress.ip_address(host).is_unspecified
        except ValueError:
            local_ip = False
        if (destination.scheme != "https" or not destination.hostname or destination.username or
                destination.password or destination.fragment or destination.path in ("", "/") or
                host in ("localhost", "alertmanager", "alert-relay", "host.docker.internal") or
                host.endswith(".localhost") or local_ip or any(c.isspace() for c in ntfy_url)):
            raise ValueError
    except (ValueError, TypeError):
        errors.append("alert-relay.environment.NTFY_URL deve ser HTTPS externo válido, sem credenciais na URL")
    if services.get("alertmanager", {}).get("ports"):
        errors.append("alertmanager: portas publicadas são proibidas")
    if volumes.get("alertmanager_data", {}).get("name") != "catbrasil_prod_alertmanager_data":
        errors.append("Volume 'alertmanager_data' inválido")

    node = services.get("node-exporter", {})
    node = node if isinstance(node, dict) else {}
    if node.get("ports"):
        errors.append("node-exporter: portas publicadas são proibidas")
    textfile_mounts = [mount for mount in node.get("volumes", []) if isinstance(mount, dict) and
                       mount.get("type") == "bind" and mount.get("target") == "/textfile"]
    if len(textfile_mounts) != 1 or textfile_mounts[0].get("source") != "/var/lib/node_exporter/textfile_collector" or \
            not textfile_mounts[0].get("read_only") or textfile_mounts[0].get("bind", {}).get("create_host_path") is not False:
        errors.append("node-exporter: mount textfile deve ser bind somente leitura, sem criar diretório")
    prometheus_env = services.get("prometheus", {}).get("environment", {})
    recovery_target = prometheus_env.get("RECOVERY_NODE_EXPORTER_TARGET") if isinstance(prometheus_env, dict) else None
    try:
        target_host, target_port = recovery_target.rsplit(":", 1)
        target_ip = ipaddress.ip_address(target_host)
        allowed_networks = tuple(ipaddress.ip_network(net) for net in
                                 ("10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "100.64.0.0/10"))
        if not isinstance(target_ip, ipaddress.IPv4Address) or not any(target_ip in net for net in allowed_networks) or \
                not target_port.isascii() or not target_port.isdecimal() or not 1 <= int(target_port) <= 65535:
            raise ValueError
    except (AttributeError, ValueError):
        errors.append("prometheus.environment.RECOVERY_NODE_EXPORTER_TARGET deve ser IPv4 privado:porta")

    # 6. Imagens imutáveis (digest sha256) nos serviços de produção
    for svc_name in PINNED_IMAGE_SERVICES:
        svc_obj = services.get(svc_name)
        image = svc_obj.get("image") if isinstance(svc_obj, dict) else None
        if not isinstance(image, str) or not IMAGE_DIGEST_RE.fullmatch(image):
            errors.append(
                f"{svc_name}: imagem de produção sem digest imutável; "
                "esperado '<repo>:<tag>@sha256:<64 hex minúsculo>'"
            )
        if isinstance(svc_obj, dict) and svc_obj.get("build") is not None:
            errors.append(f"{svc_name}: build proibido no corte; esperado somente imagem imutável")

    return errors


def _sha256_of(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def _is_outside_repo(path: Path) -> bool:
    try:
        path.relative_to(REPO_ROOT)
    except ValueError:
        return True
    return False


def _verify_evidence_signature(manifest_path: Path, manifest_raw: bytes, operator: str,
                               allowed_signers_path, signature_path) -> list:
    errors = []

    if not (signature_path and str(signature_path).strip()):
        return ["assinatura: caminho não informado; esperado assinatura OpenSSH detached do manifesto (--signature)"]
    sig_path = Path(str(signature_path)).expanduser()
    if not sig_path.is_file():
        return ["assinatura: arquivo de assinatura OpenSSH ausente ou ilegível; "
                "esperado assinatura detached criada com 'ssh-keygen -Y sign'"]

    if not (allowed_signers_path and str(allowed_signers_path).strip()):
        return ["allowed_signers: caminho não informado; "
                "esperado arquivo allowed_signers fora do repositório (--allowed-signers)"]
    allowed_path = Path(str(allowed_signers_path)).expanduser()
    if not allowed_path.is_file():
        return ["allowed_signers: arquivo ausente ou ilegível; esperado allowed_signers fora do repositório"]
    try:
        allowed_resolved = allowed_path.resolve()
    except Exception:
        return ["allowed_signers: ilegível; esperado allowed_signers fora do repositório"]
    if not _is_outside_repo(allowed_resolved):
        return ["allowed_signers: arquivo dentro do repositório; esperado allowed_signers fora do repositório"]

    ssh_keygen = shutil.which("ssh-keygen")
    if not ssh_keygen:
        return ["assinatura: ssh-keygen indisponível no host; "
                "esperado verificação fail-closed com 'ssh-keygen -Y verify'"]

    cmd = [
        ssh_keygen, "-Y", "verify",
        "-I", operator,
        "-f", str(allowed_resolved),
        "-n", EVIDENCE_SIGN_NAMESPACE,
        "-s", str(sig_path),
    ]
    try:
        proc = subprocess.run(
            cmd,
            input=manifest_raw,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            timeout=20,
        )
    except Exception:
        return ["assinatura: falha ao executar 'ssh-keygen -Y verify'; esperado verificação concluída"]

    if proc.returncode != 0:
        errors.append(
            "assinatura: verificação recusada; esperado assinatura OpenSSH detached válida do "
            "operador contra o allowed_signers fora do repositório "
            "(operador == identidade assinante)"
        )
    return errors


def _validate_artifact(item_key: str, artifact) -> list:
    prefix = f"items.{item_key}.artifact"
    if not isinstance(artifact, dict):
        return [f"{prefix}: ausente; esperado objeto com 'path' (arquivo real fora do repositório) e 'sha256'"]

    art_path = artifact.get("path")
    if not isinstance(art_path, str) or not art_path.strip():
        return [f"{prefix}.path: ausente; esperado caminho absoluto do arquivo de evidência fora do repositório"]
    path = Path(art_path).expanduser()
    if not path.is_absolute():
        return [f"{prefix}.path: não absoluto; esperado caminho absoluto do arquivo de evidência fora do repositório"]
    try:
        resolved = path.resolve()
    except Exception:
        return [f"{prefix}.path: ilegível; esperado arquivo de evidência existente fora do repositório"]
    if not _is_outside_repo(resolved):
        return [f"{prefix}.path: arquivo dentro do repositório; esperado arquivo de evidência fora do repositório"]
    if not resolved.is_file():
        return [f"{prefix}.path: arquivo de evidência inexistente ou ilegível; "
                "esperado arquivo real fora do repositório"]

    declared = artifact.get("sha256")
    if not isinstance(declared, str) or not SHA256_HEX_RE.match(declared):
        return [f"{prefix}.sha256: ausente ou malformado; esperado sha256 em hex minúsculo (64 caracteres)"]

    try:
        actual = _sha256_of(resolved)
    except Exception:
        return [f"{prefix}.sha256: falha ao ler o arquivo de evidência; esperado arquivo legível fora do repositório"]
    if actual != declared:
        return [f"{prefix}.sha256: não confere com o arquivo de evidência; esperado digest sha256 real do arquivo"]
    return []


def validate_evidence_manifest(manifest_path, operator, allowed_signers_path, signature_path=None) -> list:
    """Valida o pacote de evidências do corte de forma fail-closed.

    Retorna a lista de erros (vazia = aprovado). Nunca imprime conteúdo de arquivo,
    caminhos, digests ou valores recebidos: apenas nomes de campo e expectativa.
    """
    op = operator or ""
    if not isinstance(op, str) or not OPERATOR_RE.fullmatch(op):
        return ["operador: identidade inválida; esperado identificador ASCII de 1 a 64 caracteres"]

    if not (manifest_path and str(manifest_path).strip()):
        return ["manifesto: caminho não informado; esperado manifesto JSON (--evidence-manifest)"]
    manifest = Path(str(manifest_path)).expanduser()
    if not manifest.is_file():
        return ["manifesto: arquivo ausente ou ilegível; esperado manifesto JSON de evidências do corte"]

    try:
        raw = manifest.read_bytes()
        data = json.loads(raw.decode("utf-8"))
    except Exception:
        return ["manifesto: JSON inválido ou ilegível; esperado objeto JSON"]
    if not isinstance(data, dict):
        return ["manifesto: estrutura inválida; esperado objeto JSON com approved_by, approved_at e items"]

    errors = []

    approved_by = data.get("approved_by")
    if not isinstance(approved_by, str) or not approved_by.strip():
        errors.append("approved_by: ausente ou vazio; esperado identidade do operador aprovador")
    elif approved_by != op:
        errors.append("approved_by: não corresponde ao operador; esperado approved_by == identidade do operador")

    approved_at = data.get("approved_at")
    if not isinstance(approved_at, str) or not approved_at.strip():
        errors.append("approved_at: ausente ou vazio; esperado carimbo de tempo da aprovação")

    errors.extend(
        _verify_evidence_signature(manifest, raw, op, allowed_signers_path, signature_path)
    )

    items = data.get("items")
    if not isinstance(items, dict):
        errors.append("items: ausente ou inválido; esperado objeto com os 6 itens obrigatórios")
        items = {}

    for key in EVIDENCE_ITEMS:
        if key not in items:
            errors.append(f"items.{key}: ausente; esperado item obrigatório do pacote de evidências")
            continue
        item = items[key]
        if not isinstance(item, dict):
            errors.append(
                f"items.{key}: estrutura inválida; esperado objeto com status, evidence_ref, "
                "verified_by e artifact"
            )
            continue

        status = item.get("status")
        if not (isinstance(status, str) and status == "approved"):
            errors.append(f'items.{key}.status: não aprovado; esperado string exata "approved"')

        ref = item.get("evidence_ref")
        if not isinstance(ref, str) or len(ref.strip()) < MIN_EVIDENCE_REF_LEN:
            errors.append(
                f"items.{key}.evidence_ref: ausente ou curto; "
                f"esperado referência de no mínimo {MIN_EVIDENCE_REF_LEN} caracteres"
            )

        verified_by = item.get("verified_by")
        if not isinstance(verified_by, str) or not verified_by.strip():
            errors.append(f"items.{key}.verified_by: ausente; esperado identidade do verificador")

        if key == "p0_4_identity_sms" and isinstance(verified_by, str) and verified_by.strip() == op:
            errors.append(
                "items.p0_4_identity_sms.verified_by: igual ao operador aprovador; "
                "esperado revisor distinto do operador"
            )

        errors.extend(_validate_artifact(key, item.get("artifact")))

    return errors


def _run_evidence_mode(args) -> int:
    if not (args.operator and str(args.operator).strip()):
        print(
            "ERRO DE USO: --operator é obrigatório no modo --evidence-manifest "
            "(identidade do operador assinante).",
            file=sys.stderr,
        )
        sys.exit(2)

    errors = validate_evidence_manifest(
        args.evidence_manifest, args.operator, args.allowed_signers, args.signature
    )
    if errors:
        for err in errors:
            print(f"ERRO DE PACOTE DE EVIDÊNCIAS: {err}", file=sys.stderr)
        sys.exit(3)
    sys.exit(0)


def published_ports(data: dict) -> list:
    """Return all host bindings from the rendered Compose, including profile services."""
    bindings = []
    for name, service in data.get("services", {}).items():
        for port in service.get("ports", []):
            if not isinstance(port, dict):
                raise ValueError("ports: entrada inválida")
            host_ip = str(port.get("host_ip") or "0.0.0.0")
            published = str(port.get("published") or "")
            protocol = str(port.get("protocol") or "tcp").lower()
            if not published.isdecimal() or not 1 <= int(published) <= 65535:
                raise ValueError("ports: porta publicada inválida ou dinâmica")
            if protocol not in ("tcp", "udp"):
                raise ValueError("ports: protocolo não suportado")
            bindings.append((name, host_ip, int(published), protocol))
    return bindings


def check_published_ports(data: dict) -> list:
    """Bind every published address/port. Keep sockets open to detect overlapping mappings."""
    try:
        bindings = published_ports(data)
    except ValueError as exc:
        return [str(exc)]
    errors = []
    held = []
    try:
        for name, host_ip, port, protocol in bindings:
            try:
                family = socket.AF_INET6 if ":" in host_ip else socket.AF_INET
                kind = socket.SOCK_DGRAM if protocol == "udp" else socket.SOCK_STREAM
                sock = socket.socket(family, kind)
                held.append(sock)
                if family == socket.AF_INET6:
                    sock.setsockopt(socket.IPPROTO_IPV6, socket.IPV6_V6ONLY, 1)
                sock.bind((host_ip, port))
                if kind == socket.SOCK_STREAM:
                    sock.listen(1)
            except OSError:
                errors.append(f"{name}: BLOQUEIO DE PORTAS: bind público indisponível")
    finally:
        for sock in held:
            sock.close()
    return errors


def main(argv=None):
    parser = argparse.ArgumentParser(
        description="Valida a configuração renderizada do Compose (stdin) ou o pacote de "
                    "evidências do corte (--evidence-manifest)."
    )
    parser.add_argument("--evidence-manifest", default=None,
                        help="Caminho do manifesto JSON de evidências do corte")
    parser.add_argument("--operator", default=None,
                        help="Identidade do operador aprovador (deve ser a identidade assinante)")
    parser.add_argument("--allowed-signers", default=None,
                        help="Arquivo allowed_signers fora do repositório para ssh-keygen -Y verify")
    parser.add_argument("--signature", default=None,
                        help="Assinatura OpenSSH detached do manifesto (padrão: <manifesto>.sig)")
    parser.add_argument("--check-ports", action="store_true",
                        help="Verifica bind de todas as portas publicadas no Compose renderizado")
    parser.add_argument("--check-host-paths", action="store_true",
                        help="Verifica diretórios de bind obrigatórios no host antes de mutação Docker")
    parser.add_argument("--list-images", action="store_true",
                        help="Emite referências de imagens pinadas após validar o Compose")
    args = parser.parse_args(argv)

    if args.evidence_manifest:
        _run_evidence_mode(args)

    if args.operator or args.allowed_signers or args.signature:
        print(
            "ERRO DE USO: --operator/--allowed-signers/--signature exigem --evidence-manifest.",
            file=sys.stderr,
        )
        sys.exit(2)

    try:
        raw = sys.stdin.read()
        if not raw.strip():
            print("ERRO FATAL: Entrada JSON do Docker Compose vazia.", file=sys.stderr)
            sys.exit(2)
        data = json.loads(raw)
    except SystemExit:
        raise
    except Exception:
        print("ERRO FATAL: Falha ao decodificar JSON do Compose.", file=sys.stderr)
        sys.exit(2)

    errors = validate_config(data)
    if not errors and args.check_ports:
        errors.extend(check_published_ports(data))
    if not errors and args.check_host_paths and not Path("/var/lib/node_exporter/textfile_collector").is_dir():
        errors.append("node-exporter: diretório textfile do host ausente")
    if errors:
        for err in errors:
            print(f"ERRO DE CONFIGURAÇÃO FINAL: {err}", file=sys.stderr)
        sys.exit(2)

    if args.list_images:
        for name in PINNED_IMAGE_SERVICES:
            print(data["services"][name]["image"])

    sys.exit(0)


if __name__ == "__main__":
    main()
