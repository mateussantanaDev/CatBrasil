#!/usr/bin/env python3
"""
tests/test_cutover_orchestrator.py

Testes automatizados para scripts/release/cutover_orchestrator.sh e scripts/release/validate_compose_config.py.
Verifica:
1. Rejeição com código 2 quando DB_NAME=catbrasil é exportado no shell sobrepondo arquivo válido.
2. Rejeição com código 2 quando DATABASE_URL maliciosa é exportada no shell sobrepondo arquivo válido.
3. Aprovação com código 0 no modo --validate-only quando a configuração final é limpa e coerente.
4. Nenhuma impressão de credenciais, valores recebidos ou sentinelas (SECRET_X) em stdout/stderr.
5. SUITPAY_LIVE_ENABLED exige estritamente 'false' (qualquer valor diferente, desconhecido ou ausente bloqueia).
6. Falha no comando 'docker compose ps' durante --execute aborta fail-closed com exit code 2 sem prosseguir.
7. Portão de Evidências P0.6 (somente --execute): exige CUTOVER_OPERATOR + manifesto assinado;
   status/ref autoafirmados não bastam; cada item precisa de arquivo REAL fora do repositório com
   sha256 conferido; assinatura OpenSSH detached validada com ssh-keygen -Y verify contra
   allowed_signers fora do repositório (fail-closed); operador == assinante; revisor de P0.4 distinto.
8. O portão roda ANTES de qualquer chamada Docker; as portas 80/443 são checadas antes de
   QUALQUER mutação Docker no --execute.
9. Imagens de produção precisam de digest @sha256:<64 hex> no compose renderizado.

Todos os testes usam fixtures sintéticas (chaves/assinaturas geradas em tmpdir, mocks de docker):
NENHUM teste executa o corte real nem muta containers.
"""

import hashlib
import json
import os
import shutil
import socket
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

ROOT = Path(__file__).resolve().parents[1]
ORCHESTRATOR = ROOT / "scripts" / "release" / "cutover_orchestrator.sh"
VALIDATOR = ROOT / "scripts" / "release" / "validate_compose_config.py"

sys.path.insert(0, str(ROOT / "scripts" / "release"))
import validate_compose_config

EVIDENCE_ITEMS = validate_compose_config.EVIDENCE_ITEMS
SIGN_NAMESPACE = validate_compose_config.EVIDENCE_SIGN_NAMESPACE
SSH_KEYGEN = shutil.which("ssh-keygen")

FAKE_64 = "a" * 64
DB_PASS = "p010-db-secret-0123456789abcdef"


def digest_of(seed: str) -> str:
    return hashlib.sha256(seed.encode()).hexdigest()


PROD_IMAGE_ENV = "\n".join([
    f"POSTGRES_IMAGE_DIGEST={digest_of('postgres')}",
    f"CADDY_IMAGE_DIGEST={digest_of('caddy')}",
    f"PROMETHEUS_IMAGE_DIGEST={digest_of('prometheus')}",
    f"ALERTMANAGER_IMAGE_DIGEST={digest_of('alertmanager')}",
    f"NODE_EXPORTER_IMAGE_DIGEST={digest_of('node-exporter')}",
    f"PROD_BACKEND_MIGRATE_IMAGE=registry.example.invalid/catbrasil/backend-migrate@sha256:{digest_of('migrate')}",
    f"PROD_BACKEND_API_IMAGE=registry.example.invalid/catbrasil/backend-api@sha256:{digest_of('api')}",
    f"PROD_BACKEND_CRON_IMAGE=registry.example.invalid/catbrasil/backend-cron@sha256:{digest_of('cron')}",
    f"PROD_APP_CLIENT_IMAGE=registry.example.invalid/catbrasil/app-client@sha256:{digest_of('client')}",
    f"PROD_APP_ADMIN_IMAGE=registry.example.invalid/catbrasil/app-admin@sha256:{digest_of('admin')}",
    f"PROD_ALERT_RELAY_IMAGE=registry.example.invalid/catbrasil/alert-relay@sha256:{digest_of('relay')}",
])

BASE_ENV = f"""
APP_ENV=production
PIX_MODE=suitpay
DB_NAME=catbrasil_prod
DB_USER=catadmin
DB_PASSWORD={DB_PASS}
DB_PORT=5432
API_PORT=8080
CLIENT_PORT=3000
ADMIN_PORT=3002
GOMAXPROCS=2
DATABASE_URL=postgres://catadmin:{DB_PASS}@postgres:5432/catbrasil_prod?sslmode=disable
JWT_SECRET=p010-jwt-secret-0123456789abcdef
PIX_WEBHOOK_SECRET=p010-webhook-secret-0123456789abcdef
ADMIN_SECRET_PIN=pin-p010-exclusivo
BOOTSTRAP_ADMIN_USERNAME=release_director
SMS_GATEWAY_URL=https://sms.example.invalid
SMS_GATEWAY_TOKEN=p010-sms-gateway-token
SMS_CODE_SECRET=p010-sms-code-secret-0123456789abcdef
IDENTITY_SECRET=p010-identity-secret-0123456789abcdef
SUITPAY_LIVE_ENABLED=false
OBSERVABILITY_TOKEN=dummy_obs_token
NTFY_URL=https://alerts.example.invalid/catbrasil
NTFY_TOKEN=dummy_ntfy_token
RECOVERY_NODE_EXPORTER_TARGET=100.90.1.2:9100
CORS_ALLOWED_ORIGINS=https://example.invalid
ACME_EMAIL=admin@example.invalid
CLIENT_DOMAIN=client.example.invalid
ADMIN_DOMAIN=admin.example.invalid
API_DOMAIN=api.example.invalid
{PROD_IMAGE_ENV}
"""


def write_env_file(directory, extra: str = "") -> str:
    path = Path(directory) / "prod.env"
    path.write_text(BASE_ENV + extra)
    return str(path)


def rendered_compose_json() -> dict:
    def svc(image_seed, environment=None):
        obj = {"image": f"registry.example.invalid/catbrasil/{image_seed}@sha256:{digest_of(image_seed)}"}
        if environment:
            obj["environment"] = environment
        return obj

    return {
        "services": {
            "postgres": svc("postgres", {"POSTGRES_DB": "catbrasil_prod", "POSTGRES_PASSWORD": DB_PASS}),
            "backend-migrate": svc(
                "backend-migrate",
                {"DATABASE_URL": f"postgres://user:{DB_PASS}@postgres:5432/catbrasil_prod"},
            ),
            "backend-api": {**svc(
                "backend-api",
                {
                    "DATABASE_URL": f"postgres://user:{DB_PASS}@postgres:5432/catbrasil_prod",
                    "APP_ENV": "production",
                    "PIX_MODE": "suitpay",
                    "SUITPAY_LIVE_ENABLED": "false",
                    "JWT_SECRET": "p010-jwt-secret-0123456789abcdef",
                    "PIX_WEBHOOK_SECRET": "p010-webhook-secret-0123456789abcdef",
                    "IDENTITY_SECRET": "p010-identity-secret-0123456789abcdef",
                    "SMS_CODE_SECRET": "p010-sms-code-secret-0123456789abcdef",
                    "ADMIN_SECRET_PIN": "pin-p010-exclusivo",
                    "SMS_GATEWAY_TOKEN": "p010-sms-gateway-token",
                    "SMS_GATEWAY_URL": "https://sms.example.invalid/send",
                    "CORS_ALLOWED_ORIGINS": "https://client.example.invalid",
                    "TRUSTED_PROXY_CIDRS": "172.30.240.2/32",
                },
            ), "networks": {"proxy_net": {"aliases": ["api-proxy"]}}},
            "bootstrap-admin": {
                "image": f"registry.example.invalid/catbrasil/backend-api@sha256:{digest_of('backend-api')}",
                "entrypoint": ["/app/bootstrap-admin"],
                "logging": {"driver": "none"},
                "environment": {
                    "DATABASE_URL": f"postgres://user:{DB_PASS}@postgres:5432/catbrasil_prod",
                    "APP_ENV": "production",
                    "ADMIN_SECRET_PIN": "pin-p010-exclusivo",
                },
            },
            "backend-cron": svc(
                "backend-cron",
                {"DATABASE_URL": f"postgres://user:{DB_PASS}@postgres:5432/catbrasil_prod"},
            ),
            "app-client": svc("app-client"),
            "app-admin": svc("app-admin"),
            "caddy": {**svc("caddy"), "networks": {"proxy_net": {"ipv4_address": "172.30.240.2"}}},
            "prometheus": svc("prometheus", {"RECOVERY_NODE_EXPORTER_TARGET": "100.90.1.2:9100"}),
            "node-exporter": {**svc("node-exporter"), "volumes": [{
                "type": "bind", "source": "/var/lib/node_exporter/textfile_collector",
                "target": "/textfile", "read_only": True,
                "bind": {"create_host_path": False},
            }]},
            "alertmanager": svc("alertmanager"),
            "alert-relay": svc("alert-relay", {
                "APP_ENV": "production",
                "NTFY_URL": "https://alerts.example.invalid/catbrasil",
                "NTFY_TOKEN": "dummy_ntfy_token",
            }),
        },
        "volumes": {"pgdata": {"name": "catbrasil_prod_pgdata"},
                    "alertmanager_data": {"name": "catbrasil_prod_alertmanager_data"}},
        "networks": {"catbrasil_net": {"name": "catbrasil_prod_net"}, "proxy_net": {"name": "catbrasil_prod_proxy_net", "ipam": {"config": [{"subnet": "172.30.240.0/28"}]}}},
    }


MOCK_DOCKER_SCRIPT = r'''#!/bin/sh
{
  printf 'CALL'
  for a in "$@"; do printf ' | %s' "$a"; done
  printf '\n'
} >> "$MOCK_LOG"

all=" $* "

if [ "$1" = "pull" ]; then
  [ "${MOCK_PULL_FAIL:-0}" = "1" ] && exit 1
  exit 0
fi
if [ "$1" = "image" ] && [ "$2" = "inspect" ]; then
  [ "${MOCK_INSPECT_FAIL:-0}" = "1" ] && exit 1
  ref="$5"
  repo="${ref%@*}"
  digest="${ref##*@}"
  repo="${repo%:*}"
  printf '["%s@%s"]\n' "$repo" "$digest"
  exit 0
fi

case "$all" in
  *" config "*)
    cat "$MOCK_CONFIG_JSON"
    exit 0
    ;;
esac

case "$all" in
  *" ps "*)
    if [ "${MOCK_PS_FAIL:-0}" = "1" ]; then
      echo "Simulated Docker daemon failure on ps check" >&2
      exit 1
    fi
    case "$all" in
      *" --services "*)
        if [ -n "${MOCK_PS_SERVICES:-}" ]; then printf '%s\n' $MOCK_PS_SERVICES; fi
        exit 0
        ;;
      *" -q "*)
        echo "mockpostgrescontainerid"
        exit 0
        ;;
      *)
        case "$all" in
          *" ps postgres "*) echo "healthy" ;;
          *" ps caddy "*) echo "${MOCK_CADDY_HEALTH:-healthy}" ;;
          *" ps alert-relay "*) echo "${MOCK_ALERT_RELAY_HEALTH:-healthy}" ;;
          *" ps alertmanager "*) echo "${MOCK_ALERTMANAGER_HEALTH:-healthy}" ;;
          *".State"*) echo "${MOCK_STATE:-running}" ;;
          *) echo "${MOCK_HEALTH:-healthy}" ;;
        esac
        exit 0
        ;;
    esac
    ;;
esac

case "$all" in
  *" up "*|*" run "*|*" stop "*)
    printf 'MUTATION %s\n' "$all" >> "$MOCK_LOG"
    exit "${MOCK_MUTATION_EXIT:-0}"
    ;;
esac

exit 0
'''


def free_ports(count: int = 2):
    socks = []
    for _ in range(count):
        sock = socket.socket()
        sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        sock.bind(("127.0.0.1", 0))
        sock.listen(1)
        socks.append(sock)
    ports = [s.getsockname()[1] for s in socks]
    for s in socks:
        s.close()
    return ports


def busy_ports(count: int = 2):
    socks = []
    for _ in range(count):
        sock = socket.socket()
        sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        sock.bind(("127.0.0.1", 0))
        sock.listen(1)
        socks.append(sock)
    return socks, [s.getsockname()[1] for s in socks]


@unittest.skipUnless(SSH_KEYGEN, "ssh-keygen indisponível no ambiente")
def build_evidence_package(base_dir, operator="operador-corte-p06",
                           reviewer="revisor-p04-distinto") -> dict:
    """Pacote de evidências 100% sintético em tmpdir (fora do repositório)."""
    base = Path(base_dir)
    base.mkdir(parents=True, exist_ok=True)

    key = base / "signer_ed25519"
    subprocess.run(
        ["ssh-keygen", "-t", "ed25519", "-f", str(key), "-N", "", "-q"],
        check=True, capture_output=True, text=True,
    )
    pub_parts = (base / "signer_ed25519.pub").read_text().split()[:2]
    allowed_signers = base / "allowed_signers"
    allowed_signers.write_text(f"{operator} {pub_parts[0]} {pub_parts[1]}\n")

    items = {}
    for name in EVIDENCE_ITEMS:
        artifact = base / f"{name}.artifact"
        content = f"fixture sintetica de evidencia {name}\n".encode()
        artifact.write_bytes(content)
        items[name] = {
            "status": "approved",
            "evidence_ref": f"EV06-{name}",
            "verified_by": reviewer,
            "artifact": {
                "path": str(artifact),
                "sha256": hashlib.sha256(content).hexdigest(),
            },
        }

    manifest_path = base / "p0_cutover_evidence.json"
    manifest_path.write_text(json.dumps({
        "manifest_version": 1,
        "approved_by": operator,
        "approved_at": "2026-10-17T12:00:00Z",
        "items": items,
    }, indent=2, sort_keys=True))

    package = {
        "dir": base,
        "manifest": manifest_path,
        "signature": Path(str(manifest_path) + ".sig"),
        "allowed_signers": allowed_signers,
        "key": key,
        "operator": operator,
        "reviewer": reviewer,
    }
    resign_manifest(package)
    return package


def resign_manifest(package: dict):
    if package["signature"].exists():
        package["signature"].unlink()
    subprocess.run(
        ["ssh-keygen", "-Y", "sign", "-f", str(package["key"]),
         "-n", SIGN_NAMESPACE, str(package["manifest"])],
        check=True, capture_output=True, text=True,
    )


def rewrite_manifest(package: dict, mutate, resign: bool = True) -> dict:
    data = json.loads(package["manifest"].read_text())
    mutate(data)
    package["manifest"].write_text(json.dumps(data, indent=2, sort_keys=True))
    if resign:
        resign_manifest(package)
    return package


class TestCutoverOrchestratorValidation(unittest.TestCase):
    """Testa a validação in-memory do cutover_orchestrator contra precedência de variáveis de shell (docker real)."""

    def setUp(self):
        self.tmp_dir = tempfile.mkdtemp(prefix="p06_render_")
        self.env_file = write_env_file(self.tmp_dir)

    def tearDown(self):
        shutil.rmtree(self.tmp_dir, ignore_errors=True)

    def _clean_env(self) -> dict:
        env = os.environ.copy()
        env["ENV_FILE"] = self.env_file
        for key in ("DB_NAME", "POSTGRES_DB", "DATABASE_URL", "SUITPAY_LIVE_ENABLED",
                    "POSTGRES_IMAGE_DIGEST", "CADDY_IMAGE_DIGEST", "PROMETHEUS_IMAGE_DIGEST",
                    "PROD_BACKEND_MIGRATE_IMAGE", "PROD_BACKEND_API_IMAGE",
                    "PROD_BACKEND_CRON_IMAGE", "PROD_APP_CLIENT_IMAGE", "PROD_APP_ADMIN_IMAGE"):
            env.pop(key, None)
        return env

    def test_clean_config_passes_with_exit_code_0(self):
        """Prova que a configuração válida renderiza com código 0 e sem vazar segredos."""
        proc = subprocess.run(
            ["bash", str(ORCHESTRATOR), "--validate-only"],
            cwd=ROOT, capture_output=True, text=True, env=self._clean_env(),
        )
        self.assertEqual(proc.returncode, 0, f"Deveria sair com 0. Saída:\n{proc.stdout}\n{proc.stderr}")
        self.assertIn("Configuração final renderizada aprovada", proc.stdout)
        self.assertNotIn("dummy_secret_pass_1234", proc.stdout)
        self.assertNotIn("dummy_jwt_secret", proc.stdout)

    def test_validate_only_reports_execute_requirements(self):
        """--validate-only informa que --execute exige o pacote de evidências e portas livres."""
        proc = subprocess.run(
            ["bash", str(ORCHESTRATOR), "--validate-only"],
            cwd=ROOT, capture_output=True, text=True, env=self._clean_env(),
        )
        self.assertEqual(proc.returncode, 0)
        self.assertIn("CUTOVER_OPERATOR", proc.stdout)
        self.assertIn("CUTOVER_EVIDENCE_MANIFEST", proc.stdout)
        self.assertIn("CUTOVER_ALLOWED_SIGNERS", proc.stdout)
        self.assertIn("todas as portas publicadas", proc.stdout)

    def test_reject_when_shell_overrides_db_name_to_legacy(self):
        """Prova que exportar DB_NAME=catbrasil no shell é detectado na configuração renderizada e aborta com código 2."""
        env = self._clean_env()
        env["DB_NAME"] = "catbrasil"

        proc = subprocess.run(
            ["bash", str(ORCHESTRATOR), "--validate-only"],
            cwd=ROOT, capture_output=True, text=True, env=env,
        )
        self.assertEqual(proc.returncode, 2, f"Deveria sair com 2. Saída:\n{proc.stdout}\n{proc.stderr}")
        self.assertIn("postgres.environment.POSTGRES_DB inválido; esperado 'catbrasil_prod'", proc.stderr)
        self.assertNotIn("dummy_secret_pass_1234", proc.stdout)
        self.assertNotIn("dummy_secret_pass_1234", proc.stderr)

    def test_reject_when_shell_overrides_database_url_to_legacy(self):
        """Prova que exportar DATABASE_URL maliciosa é detectado e aborta com código 2 sem expor URL recebida."""
        env = self._clean_env()
        env["DATABASE_URL"] = "postgres://catadmin:dummy_secret_pass_1234@postgres:5432/catbrasil?sslmode=disable"

        proc = subprocess.run(
            ["bash", str(ORCHESTRATOR), "--validate-only"],
            cwd=ROOT, capture_output=True, text=True, env=env,
        )
        self.assertEqual(proc.returncode, 2, f"Deveria sair com 2. Saída:\n{proc.stdout}\n{proc.stderr}")
        self.assertIn("base de dados de DATABASE_URL inválida; esperada 'catbrasil_prod'", proc.stderr)
        self.assertNotIn("dummy_secret_pass_1234", proc.stdout)
        self.assertNotIn("dummy_secret_pass_1234", proc.stderr)

    def test_shell_suitpay_live_enabled_true_is_strictly_neutralized_to_false_by_overlay(self):
        """Prova que mesmo se SUITPAY_LIVE_ENABLED=true for exportado no shell, o overlay compose.prod.yml
        mantém 'SUITPAY_LIVE_ENABLED: "false"' literal inviolável, neutralizando o shell e mantendo a contenção."""
        env = self._clean_env()
        env["SUITPAY_LIVE_ENABLED"] = "true"  # Tentativa de ativação via shell

        proc = subprocess.run(
            ["bash", str(ORCHESTRATOR), "--validate-only"],
            cwd=ROOT, capture_output=True, text=True, env=env,
        )
        self.assertEqual(proc.returncode, 0, f"Deveria sair com 0 neutralizado pelo overlay. Saída:\n{proc.stdout}\n{proc.stderr}")
        self.assertIn("SUITPAY_LIVE_ENABLED: false", proc.stdout)

    def test_validate_only_rejects_invalid_image_digest(self):
        """Digest curto/inválido de imagem é rejeitado no --validate-only com exit 2 (fail-closed)."""
        env = self._clean_env()
        env["POSTGRES_IMAGE_DIGEST"] = "deadbeef"

        proc = subprocess.run(
            ["bash", str(ORCHESTRATOR), "--validate-only"],
            cwd=ROOT, capture_output=True, text=True, env=env,
        )
        self.assertEqual(proc.returncode, 2, f"Deveria sair com 2. Saída:\n{proc.stdout}\n{proc.stderr}")
        self.assertIn(
            "postgres: imagem de produção sem digest imutável; "
            "esperado '<repo>:<tag>@sha256:<64 hex minúsculo>'",
            proc.stderr,
        )
        self.assertNotIn("deadbeef", proc.stderr)


class TestValidateComposeConfigHygienics(unittest.TestCase):
    """Testa diretamente a sanitização e rigor de scripts/release/validate_compose_config.py."""

    def _build_valid_payload(self) -> dict:
        return rendered_compose_json()

    def test_valid_payload_produces_no_errors(self):
        payload = self._build_valid_payload()
        errors = validate_compose_config.validate_config(payload)
        self.assertEqual(errors, [])

    def test_proxy_trust_and_api_port_are_required(self):
        for mutate, expected in (
            (lambda p: p["services"]["backend-api"]["environment"].update(TRUSTED_PROXY_CIDRS="0.0.0.0/0"), "TRUSTED_PROXY_CIDRS"),
            (lambda p: p["services"]["caddy"]["networks"]["proxy_net"].update(ipv4_address="172.30.240.9"), "IP do proxy dedicado"),
            (lambda p: p["services"]["backend-api"].update(ports=[{"published": "8080", "target": 8080}]), "portas publicadas são proibidas"),
        ):
            payload = self._build_valid_payload()
            mutate(payload)
            errors = validate_compose_config.validate_config(payload)
            self.assertTrue(any(expected in error for error in errors), errors)

    def test_suitpay_live_enabled_exact_false_requirement(self):
        """Exige exatamente 'false'; qualquer outro valor (desconhecido, ausente, 0, 1) é rejeitado."""
        # 1. Ausente
        p_missing = self._build_valid_payload()
        del p_missing["services"]["backend-api"]["environment"]["SUITPAY_LIVE_ENABLED"]
        errs = validate_compose_config.validate_config(p_missing)
        self.assertTrue(any("SUITPAY_LIVE_ENABLED ausente; esperado exatamente 'false'" in e for e in errs))

        # 2. Desconhecido / 'unknown'
        p_unknown = self._build_valid_payload()
        p_unknown["services"]["backend-api"]["environment"]["SUITPAY_LIVE_ENABLED"] = "unknown"
        errs = validate_compose_config.validate_config(p_unknown)
        self.assertTrue(any("SUITPAY_LIVE_ENABLED inválido; esperado exatamente 'false'" in e for e in errs))

        # 3. '0' ou 'no'
        p_zero = self._build_valid_payload()
        p_zero["services"]["backend-api"]["environment"]["SUITPAY_LIVE_ENABLED"] = "0"
        errs = validate_compose_config.validate_config(p_zero)
        self.assertTrue(any("SUITPAY_LIVE_ENABLED inválido; esperado exatamente 'false'" in e for e in errs))

    def test_malformed_database_url_sentinel_never_leaked(self):
        """Prova que URL malformada contendo sentinela SECRET_X reporta erro sem vazar SECRET_X."""
        sentinel = "SECRET_X_SUPER_SENSITIVE_TOKEN_999"
        p = self._build_valid_payload()
        # Injeta sentinela em porta inválida / URL truncada que causa erro de parse em urllib
        p["services"]["backend-api"]["environment"]["DATABASE_URL"] = f"postgres://admin:{sentinel}@postgres:bad_port/catbrasil_prod"

        errors = validate_compose_config.validate_config(p)
        self.assertTrue(len(errors) > 0, "Deveria reportar erro para porta inválida")
        for err in errors:
            self.assertNotIn(sentinel, err, f"Sentinela vazou na mensagem de erro: {err}")
        self.assertTrue(any("porta de DATABASE_URL inválida; esperada porta 5432" in err for err in errors))

    def test_database_password_must_match_postgres_without_leaking(self):
        sentinel = "SECRET_X_DATABASE_PASSWORD_999"
        p = self._build_valid_payload()
        p["services"]["backend-cron"]["environment"]["DATABASE_URL"] = (
            f"postgres://app:{sentinel}@postgres:5432/catbrasil_prod"
        )
        errors = validate_compose_config.validate_config(p)
        self.assertTrue(any("backend-cron: senha de DATABASE_URL ausente ou diferente" in err for err in errors))
        self.assertTrue(all(sentinel not in err for err in errors))

    def test_weak_api_secret_rejected_without_leaking(self):
        sentinel = "SECRET_X_SHORT"
        p = self._build_valid_payload()
        p["services"]["backend-api"]["environment"]["JWT_SECRET"] = sentinel
        errors = validate_compose_config.validate_config(p)
        self.assertTrue(any("backend-api.environment.JWT_SECRET ausente ou curto" in err for err in errors))
        self.assertTrue(all(sentinel not in err for err in errors))

    def test_bootstrap_admin_pin_must_match_api(self):
        p = self._build_valid_payload()
        p["services"]["bootstrap-admin"]["environment"]["ADMIN_SECRET_PIN"] = "other-secret-pin"
        errors = validate_compose_config.validate_config(p)
        self.assertTrue(any("bootstrap-admin.environment.ADMIN_SECRET_PIN deve ser idêntico" in err for err in errors))

    def test_broken_schema_database_url_sentinel_never_leaked(self):
        """Prova que erro de esquema com sentinela nunca imprime o sentinela."""
        sentinel = "SECRET_X_CREDENTIAL_IN_SCHEME"
        p = self._build_valid_payload()
        p["services"]["backend-migrate"]["environment"]["DATABASE_URL"] = f"{sentinel}://postgres:5432/catbrasil_prod"

        errors = validate_compose_config.validate_config(p)
        self.assertTrue(len(errors) > 0)
        for err in errors:
            self.assertNotIn(sentinel, err)
        self.assertTrue(any("esquema de DATABASE_URL inválido; esperado 'postgres' ou 'postgresql'" in err for err in errors))


class TestImmutableImagePinning(unittest.TestCase):
    """Imagens de produção renderizadas precisam de @sha256:<64 hex> (sem tags mutáveis)."""

    def _payload(self) -> dict:
        payload = rendered_compose_json()
        return payload

    def test_every_pinned_service_requires_digest_image(self):
        for svc_name in validate_compose_config.PINNED_IMAGE_SERVICES:
            with self.subTest(service=svc_name):
                payload = self._payload()
                payload["services"][svc_name]["image"] = "registry.example.invalid/app:latest"
                errors = validate_compose_config.validate_config(payload)
                self.assertTrue(
                    any(f"{svc_name}: imagem de produção sem digest imutável" in e for e in errors),
                    f"Deveria rejeitar tag mutável em {svc_name}: {errors}",
                )

    def test_short_digest_is_rejected(self):
        payload = self._payload()
        payload["services"]["backend-api"]["image"] = f"registry.example.invalid/app@sha256:{FAKE_64[:-8]}"
        errors = validate_compose_config.validate_config(payload)
        self.assertTrue(any("backend-api: imagem de produção sem digest imutável" in e for e in errors))

    def test_missing_image_is_rejected(self):
        payload = self._payload()
        del payload["services"]["caddy"]["image"]
        errors = validate_compose_config.validate_config(payload)
        self.assertTrue(any("caddy: imagem de produção sem digest imutável" in e for e in errors))

    def test_error_message_does_not_echo_received_image(self):
        payload = self._payload()
        payload["services"]["prometheus"]["image"] = "registry.example.invalid/secret-repo:latest"
        errors = validate_compose_config.validate_config(payload)
        for err in errors:
            self.assertNotIn("secret-repo", err)

    def test_newline_in_image_reference_is_rejected(self):
        payload = self._payload()
        payload["services"]["backend-api"]["image"] = (
            f"registry.example.invalid/first@sha256:{FAKE_64}\n"
            f"registry.example.invalid/second@sha256:{FAKE_64}"
        )
        errors = validate_compose_config.validate_config(payload)
        self.assertTrue(any("backend-api: imagem de produção sem digest imutável" in e for e in errors))

    def test_build_is_rejected_even_with_pinned_image(self):
        payload = self._payload()
        payload["services"]["backend-api"]["build"] = {"context": "."}
        errors = validate_compose_config.validate_config(payload)
        self.assertTrue(any("backend-api: build proibido" in e for e in errors))

    def test_alert_receiver_requires_external_https_and_no_public_ports(self):
        for url in ("", "http://alerts.example.invalid/topic", "https://user:password@alerts.example.invalid/topic", "https://alerts.example.invalid/topic#fragment", "https://localhost/topic", "https://127.0.0.1/topic", "https://alerts.example.invalid/"):
            with self.subTest(url=url):
                payload = self._payload()
                payload["services"]["alert-relay"]["environment"]["NTFY_URL"] = url
                errors = validate_compose_config.validate_config(payload)
                self.assertTrue(any("NTFY_URL deve ser HTTPS" in e for e in errors))
                self.assertTrue(all("password" not in e for e in errors))
        for service in ("alertmanager", "alert-relay"):
            payload = self._payload()
            payload["services"][service]["ports"] = [{"published": "9093"}]
            errors = validate_compose_config.validate_config(payload)
            self.assertTrue(any(f"{service}: portas publicadas são proibidas" in e for e in errors))

    def test_node_exporter_private_mount_and_recovery_target_are_required(self):
        payload = self._payload()
        payload["services"]["node-exporter"]["ports"] = [{"published": "9100"}]
        payload["services"]["node-exporter"]["volumes"][0]["read_only"] = False
        payload["services"]["prometheus"]["environment"]["RECOVERY_NODE_EXPORTER_TARGET"] = "127.0.0.1:9100"
        errors = validate_compose_config.validate_config(payload)
        self.assertTrue(any("node-exporter: portas publicadas" in e for e in errors))
        self.assertTrue(any("node-exporter: mount textfile" in e for e in errors))
        self.assertTrue(any("RECOVERY_NODE_EXPORTER_TARGET deve ser IPv4 privado:porta" in e for e in errors))

        # Some Compose releases omit the explicit false field in rendered JSON.
        clean = self._payload()
        clean["services"]["node-exporter"]["volumes"][0]["bind"] = {}
        self.assertFalse(any("node-exporter: mount textfile" in e
                             for e in validate_compose_config.validate_config(clean)))
        clean["services"]["node-exporter"]["volumes"][0]["bind"] = {"create_host_path": True}
        self.assertTrue(any("node-exporter: mount textfile" in e
                            for e in validate_compose_config.validate_config(clean)))

    def test_live_flag_requires_literal_lowercase_false(self):
        for value in (False, "FALSE", " false "):
            with self.subTest(value=value):
                payload = self._payload()
                payload["services"]["backend-api"]["environment"]["SUITPAY_LIVE_ENABLED"] = value
                errors = validate_compose_config.validate_config(payload)
                self.assertTrue(any("SUITPAY_LIVE_ENABLED inválido" in e for e in errors))

    def test_bootstrap_has_no_docker_logs_or_public_port_and_uses_api_image(self):
        for field, value, fragment in (
            ("logging", {"driver": "json-file"}, "logging Docker deve estar desativado"),
            ("ports", [{"published": "9999", "target": 8080}], "portas publicadas são proibidas"),
            ("image", f"registry.example.invalid/other@sha256:{FAKE_64}", "imagem deve ser idêntica"),
        ):
            with self.subTest(field=field):
                payload = self._payload()
                payload["services"]["bootstrap-admin"][field] = value
                errors = validate_compose_config.validate_config(payload)
                self.assertTrue(any(fragment in error for error in errors), errors)


class TestPublishedPorts(unittest.TestCase):
    def test_busy_non_loopback_bind_is_rejected(self):
        sock = socket.socket()
        sock.bind(("0.0.0.0", 0))
        sock.listen(1)
        try:
            port = sock.getsockname()[1]
            payload = rendered_compose_json()
            payload["services"]["caddy"]["ports"] = [
                {"host_ip": "0.0.0.0", "published": str(port), "protocol": "tcp"}
            ]
            errors = validate_compose_config.check_published_ports(payload)
            self.assertTrue(any("caddy: BLOQUEIO DE PORTAS" in e for e in errors))
        finally:
            sock.close()

    def test_duplicate_published_mapping_is_rejected(self):
        payload = rendered_compose_json()
        port = free_ports(1)[0]
        for name in ("backend-api", "app-client"):
            payload["services"][name]["ports"] = [
                {"host_ip": "127.0.0.1", "published": str(port), "protocol": "tcp"}
            ]
        errors = validate_compose_config.check_published_ports(payload)
        self.assertTrue(any("app-client: BLOQUEIO DE PORTAS" in e for e in errors))


@unittest.skipUnless(SSH_KEYGEN, "ssh-keygen indisponível no ambiente")
class TestValidateEvidenceManifestUnit(unittest.TestCase):
    """Testes unitários de validate_evidence_manifest (fixtures sintéticas, zero mutação docker)."""

    def setUp(self):
        self.tmp_dir = tempfile.mkdtemp(prefix="p06_unit_")

    def tearDown(self):
        shutil.rmtree(self.tmp_dir, ignore_errors=True)

    def _package(self, name="pkg", **kwargs) -> dict:
        return build_evidence_package(Path(self.tmp_dir) / name, **kwargs)

    def _validate(self, package, operator=None):
        return validate_compose_config.validate_evidence_manifest(
            str(package["manifest"]),
            operator if operator is not None else package["operator"],
            str(package["allowed_signers"]),
            str(package["signature"]),
        )

    def test_valid_package_produces_no_errors(self):
        package = self._package()
        self.assertEqual(self._validate(package), [])

    def test_missing_manifest_file_is_rejected(self):
        package = self._package("missing")
        package["manifest"].unlink()
        errors = self._validate(package)
        self.assertTrue(any("manifesto: arquivo ausente" in e for e in errors))

    def test_missing_required_item_is_rejected(self):
        package = self._package("missing_item")

        def mutate(data):
            del data["items"]["backup_restored_offhost"]

        rewrite_manifest(package, mutate)
        errors = self._validate(package)
        self.assertTrue(any("items.backup_restored_offhost: ausente" in e for e in errors))

    def test_non_dict_item_is_rejected(self):
        package = self._package("bad_shape")

        def mutate(data):
            data["items"]["monitoring_alerts"] = "approved"

        rewrite_manifest(package, mutate)
        errors = self._validate(package)
        self.assertTrue(any("items.monitoring_alerts: estrutura inválida" in e for e in errors))

    def test_boolean_self_declared_status_is_rejected(self):
        """status: true (autoafirmado, não string exata) é recusado mesmo com assinatura válida."""
        package = self._package("bool_status")

        def mutate(data):
            data["items"]["images_immutable"]["status"] = True

        rewrite_manifest(package, mutate)
        errors = self._validate(package)
        self.assertTrue(any('items.images_immutable.status: não aprovado; esperado string exata "approved"' in e for e in errors))

    def test_empty_evidence_ref_is_rejected(self):
        package = self._package("empty_ref")

        def mutate(data):
            data["items"]["release_checklist_signed"]["evidence_ref"] = "  "

        rewrite_manifest(package, mutate)
        errors = self._validate(package)
        self.assertTrue(any("items.release_checklist_signed.evidence_ref" in e for e in errors))

    def test_self_declared_ref_without_artifact_is_rejected(self):
        """Ref autoafirmada sem artefato real externo não passa no portão."""
        package = self._package("ref_only")

        def mutate(data):
            del data["items"]["p0_5_sandbox_official"]["artifact"]

        rewrite_manifest(package, mutate)
        errors = self._validate(package)
        self.assertTrue(any("items.p0_5_sandbox_official.artifact: ausente" in e for e in errors))

    def test_approved_by_must_match_operator(self):
        package = self._package("approved_by")
        errors = self._validate(package, operator="outro-operador")
        self.assertTrue(any("approved_by: não corresponde ao operador" in e for e in errors))
        self.assertTrue(any("assinatura: verificação recusada" in e for e in errors))

    def test_p04_reviewer_must_be_distinct_from_operator(self):
        package = self._package("same_reviewer", operator="assinante-unico",
                                reviewer="assinante-unico")
        errors = self._validate(package)
        self.assertTrue(
            any("items.p0_4_identity_sms.verified_by: igual ao operador aprovador" in e for e in errors)
        )

    def test_artifact_missing_file_is_rejected(self):
        package = self._package("missing_artifact")
        artifact = Path(package["dir"]) / "p0_4_identity_sms.artifact"
        artifact.unlink()
        errors = self._validate(package)
        self.assertTrue(any("items.p0_4_identity_sms.artifact.path: arquivo de evidência inexistente" in e for e in errors))

    def test_artifact_digest_mismatch_is_rejected(self):
        package = self._package("tampered_artifact")
        artifact = Path(package["dir"]) / "p0_4_identity_sms.artifact"
        artifact.write_bytes(b"conteudo adulterado depois da aprovacao\n")
        errors = self._validate(package)
        self.assertTrue(any("artifact.sha256: não confere" in e for e in errors))

    def test_artifact_inside_repository_is_rejected(self):
        package = self._package("inside_repo")
        repo_file = ROOT / "README.md"
        real_digest = hashlib.sha256(repo_file.read_bytes()).hexdigest()

        def mutate(data):
            data["items"]["backup_restored_offhost"]["artifact"] = {
                "path": str(repo_file),
                "sha256": real_digest,
            }

        rewrite_manifest(package, mutate)
        errors = self._validate(package)
        self.assertTrue(any("artifact.path: arquivo dentro do repositório" in e for e in errors))

    def test_relative_artifact_path_is_rejected(self):
        package = self._package("relative_path")

        def mutate(data):
            data["items"]["monitoring_alerts"]["artifact"]["path"] = "release_evidence/evidencia.txt"

        rewrite_manifest(package, mutate)
        errors = self._validate(package)
        self.assertTrue(any("artifact.path: não absoluto" in e for e in errors))

    def test_malformed_declared_sha256_is_rejected(self):
        package = self._package("bad_sha")

        def mutate(data):
            data["items"]["images_immutable"]["artifact"]["sha256"] = "abc123"

        rewrite_manifest(package, mutate)
        errors = self._validate(package)
        self.assertTrue(any("artifact.sha256: ausente ou malformado" in e for e in errors))

    def test_missing_signature_file_is_rejected(self):
        package = self._package("no_sig")
        package["signature"].unlink()
        errors = self._validate(package)
        self.assertTrue(any("assinatura: arquivo de assinatura OpenSSH ausente" in e for e in errors))

    def test_tampered_manifest_breaks_signature(self):
        package = self._package("tampered_manifest")

        def mutate(data):
            data["approved_at"] = "2026-01-01T00:00:00Z"
            data["items"]["p0_4_identity_sms"]["verified_by"] = "outro-revisor"

        rewrite_manifest(package, mutate, resign=False)
        errors = self._validate(package)
        self.assertTrue(any("assinatura: verificação recusada" in e for e in errors))

    def test_operator_must_be_the_signer_identity(self):
        """Operador diferente do principipal do allowed_signers é recusado (operador == assinante)."""
        package = self._package("signer_mismatch")
        errors = self._validate(package, operator="operador-sem-chave")
        self.assertTrue(any("assinatura: verificação recusada" in e for e in errors))

    def test_missing_allowed_signers_fails_closed(self):
        package = self._package("no_allowed")
        package["allowed_signers"].unlink()
        errors = validate_compose_config.validate_evidence_manifest(
            str(package["manifest"]), package["operator"],
            str(package["allowed_signers"]), str(package["signature"]),
        )
        self.assertTrue(any("allowed_signers: arquivo ausente" in e for e in errors))

    def test_allowed_signers_inside_repository_is_rejected(self):
        package = self._package("allowed_in_repo")
        errors = validate_compose_config.validate_evidence_manifest(
            str(package["manifest"]), package["operator"],
            str(ROOT / "README.md"), str(package["signature"]),
        )
        self.assertTrue(any("allowed_signers: arquivo dentro do repositório" in e for e in errors))

    def test_unavailable_ssh_keygen_fails_closed(self):
        package = self._package("no_ssh_keygen")
        with mock.patch.object(validate_compose_config.shutil, "which", return_value=None):
            errors = self._validate(package)
        self.assertTrue(any("ssh-keygen indisponível" in e for e in errors))

    def test_errors_never_leak_artifact_path_or_content(self):
        package = self._package("hygiene")
        sentinel = "SECRET_X_EVIDENCIA_SUPER_SENSIVEL"
        artifact = Path(package["dir"]) / "p0_4_identity_sms.artifact"
        artifact.write_bytes(f"{sentinel}\n".encode())  # digest agora diverge
        errors = self._validate(package)
        self.assertTrue(errors)
        for err in errors:
            self.assertNotIn(sentinel, err)
            self.assertNotIn(str(artifact), err)
            self.assertNotIn(str(package["dir"]), err)

    def test_unsafe_operator_is_rejected_without_echoing_value(self):
        package = self._package("operator_hygiene")
        sentinel = "SECRET_X_OPERATOR\nINJECTED"
        errors = self._validate(package, operator=sentinel)
        self.assertTrue(any("operador: identidade inválida" in e for e in errors))
        self.assertTrue(all(sentinel not in e and "SECRET_X" not in e for e in errors))


class ExecuteGateHarness(unittest.TestCase):
    """Harness: env de produção sintético + mock de docker que registra cada chamada."""

    def setUp(self):
        self.tmp_dir = tempfile.mkdtemp(prefix="p06_exec_")
        self.addCleanup(shutil.rmtree, self.tmp_dir, ignore_errors=True)
        self.env_file = write_env_file(self.tmp_dir)
        self.rendered_config = Path(self.tmp_dir) / "rendered.json"
        self.config_data = rendered_compose_json()
        self.port_numbers = free_ports(6)
        for service, index in (("app-client", 1),
                               ("app-admin", 2), ("prometheus", 5)):
            self.config_data["services"][service]["ports"] = [{
                "host_ip": "127.0.0.1", "published": str(self.port_numbers[index]),
                "target": 80,
                "protocol": "tcp",
            }]
        self.config_data["services"]["caddy"]["ports"] = [
            {"host_ip": "0.0.0.0", "published": str(self.port_numbers[i]),
             "target": target, "protocol": "tcp"}
            for i, target in ((3, 80), (4, 443))
        ]
        self.rendered_config.write_text(json.dumps(self.config_data))
        self.mock_dir = Path(self.tmp_dir) / "bin"
        self.mock_dir.mkdir()
        self.mock_log = self.mock_dir / "docker_calls.log"
        mock_bin = self.mock_dir / "docker"
        mock_bin.write_text(MOCK_DOCKER_SCRIPT)
        mock_bin.chmod(0o755)
        mock_python = self.mock_dir / "python3"
        mock_python.write_text(
            "#!/bin/sh\n"
            "if [ \"$1\" = \"scripts/release/preflight_cutover.py\" ] && "
            "[ -n \"${MOCK_PREFLIGHT_EXIT:-}\" ]; then exit \"$MOCK_PREFLIGHT_EXIT\"; fi\n"
            "if [ \"${2:-}\" = \"--check-host-paths\" ]; then exit \"${MOCK_HOST_PATHS_EXIT:-0}\"; fi\n"
            f"exec '{sys.executable}' \"$@\"\n"
        )
        mock_python.chmod(0o755)
        mock_sleep = self.mock_dir / "sleep"
        mock_sleep.write_text("#!/bin/sh\nexit 0\n")
        mock_sleep.chmod(0o755)

    def _base_env(self) -> dict:
        env = os.environ.copy()
        for key in ("CUTOVER_OPERATOR", "CUTOVER_EVIDENCE_MANIFEST", "CUTOVER_ALLOWED_SIGNERS",
                    "CUTOVER_EVIDENCE_SIGNATURE", "CUTOVER_HTTP_PORT", "CUTOVER_HTTPS_PORT",
                    "MOCK_PS_FAIL", "MOCK_MUTATION_EXIT", "MOCK_PS_SERVICES", "MOCK_HEALTH",
                    "MOCK_PULL_FAIL", "MOCK_INSPECT_FAIL", "MOCK_STATE", "MOCK_PREFLIGHT_EXIT",
                    "MOCK_CADDY_HEALTH", "MOCK_HOST_PATHS_EXIT"):
            env.pop(key, None)
        env["ENV_FILE"] = self.env_file
        env["PATH"] = f"{self.mock_dir}:{env.get('PATH', '')}"
        env["MOCK_LOG"] = str(self.mock_log)
        env["MOCK_CONFIG_JSON"] = str(self.rendered_config)
        return env

    def run_orchestrator(self, args, env_overrides=None):
        env = self._base_env()
        if env_overrides:
            env.update({k: v for k, v in env_overrides.items() if v is not None})
        return subprocess.run(
            ["bash", str(ORCHESTRATOR), *args],
            cwd=ROOT, capture_output=True, text=True, env=env,
        )

    def docker_calls(self) -> str:
        if not self.mock_log.exists():
            return ""
        return self.mock_log.read_text()

    def assert_no_docker_calls(self, proc):
        calls = self.docker_calls()
        self.assertEqual(calls.strip(), "", f"Docker não podia ter sido chamado. Saída:\n{proc.stdout}\n{proc.stderr}")

    def assert_no_mutation(self, calls: str):
        for line in calls.splitlines():
            self.assertNotIn("MUTATION", line, f"Nenhuma mutação Docker era permitida: {line}")

    @staticmethod
    def evidence_env(package: dict, http_port=None, https_port=None) -> dict:
        overrides = {
            "CUTOVER_OPERATOR": package["operator"],
            "CUTOVER_EVIDENCE_MANIFEST": str(package["manifest"]),
            "CUTOVER_ALLOWED_SIGNERS": str(package["allowed_signers"]),
            "CUTOVER_EVIDENCE_SIGNATURE": str(package["signature"]),
        }
        if http_port is not None:
            overrides["CUTOVER_HTTP_PORT"] = str(http_port)
        if https_port is not None:
            overrides["CUTOVER_HTTPS_PORT"] = str(https_port)
        return overrides


@unittest.skipUnless(SSH_KEYGEN, "ssh-keygen indisponível no ambiente")
class TestExecuteEvidenceGate(ExecuteGateHarness):
    """Portão de Evidências do --execute: bloqueia ANTES de qualquer chamada Docker."""

    def _package(self, name="pkg", **kwargs) -> dict:
        return build_evidence_package(Path(self.tmp_dir) / name, **kwargs)

    def test_execute_without_operator_blocks_before_any_docker_call(self):
        proc = self.run_orchestrator(["--execute"])
        self.assertEqual(proc.returncode, 3, f"Saída:\n{proc.stdout}\n{proc.stderr}")
        self.assertIn("CUTOVER_OPERATOR não definido", proc.stderr)
        self.assert_no_docker_calls(proc)

    def test_execute_operator_from_env_file_without_manifest_blocks(self):
        """CUTOVER_OPERATOR lido do .prod.env, mas manifesto padrão inexistente => exit 3."""
        env_file = Path(self.env_file)
        env_file.write_text(env_file.read_text() + "\nCUTOVER_OPERATOR=operador-do-env-file\n")
        proc = self.run_orchestrator(["--execute"])
        self.assertEqual(proc.returncode, 3, f"Saída:\n{proc.stdout}\n{proc.stderr}")
        self.assertIn("manifesto: arquivo ausente", proc.stderr)
        self.assert_no_docker_calls(proc)

    def test_execute_missing_manifest_fails_closed(self):
        overrides = {
            "CUTOVER_OPERATOR": "operador-corte-p06",
            "CUTOVER_EVIDENCE_MANIFEST": str(Path(self.tmp_dir) / "nao_existe.json"),
        }
        proc = self.run_orchestrator(["--execute"], overrides)
        self.assertEqual(proc.returncode, 3, f"Saída:\n{proc.stdout}\n{proc.stderr}")
        self.assertIn("manifesto: arquivo ausente", proc.stderr)
        self.assert_no_docker_calls(proc)

    def test_execute_self_declared_boolean_status_is_rejected(self):
        package = self._package("bool_status")

        def mutate(data):
            data["items"]["p0_4_identity_sms"]["status"] = True

        rewrite_manifest(package, mutate)
        proc = self.run_orchestrator(["--execute"], self.evidence_env(package))
        self.assertEqual(proc.returncode, 3, f"Saída:\n{proc.stdout}\n{proc.stderr}")
        self.assertIn("ERRO DE PACOTE DE EVIDÊNCIAS", proc.stderr)
        self.assertIn("status: não aprovado", proc.stderr)
        self.assert_no_docker_calls(proc)

    def test_execute_self_declared_ref_without_artifact_is_rejected(self):
        package = self._package("ref_only")

        def mutate(data):
            data["items"]["images_immutable"]["artifact"] = None

        rewrite_manifest(package, mutate)
        proc = self.run_orchestrator(["--execute"], self.evidence_env(package))
        self.assertEqual(proc.returncode, 3, f"Saída:\n{proc.stdout}\n{proc.stderr}")
        self.assertIn("artifact: ausente", proc.stderr)
        self.assert_no_docker_calls(proc)

    def test_execute_artifact_digest_mismatch_is_rejected(self):
        package = self._package("tampered")
        artifact = Path(package["dir"]) / "backup_restored_offhost.artifact"
        artifact.write_bytes(b"backup adulterado\n")
        proc = self.run_orchestrator(["--execute"], self.evidence_env(package))
        self.assertEqual(proc.returncode, 3, f"Saída:\n{proc.stdout}\n{proc.stderr}")
        self.assertIn("artifact.sha256: não confere", proc.stderr)
        self.assert_no_docker_calls(proc)

    def test_execute_artifact_inside_repository_is_rejected(self):
        package = self._package("in_repo")
        repo_file = ROOT / "README.md"

        def mutate(data):
            data["items"]["release_checklist_signed"]["artifact"] = {
                "path": str(repo_file),
                "sha256": hashlib.sha256(repo_file.read_bytes()).hexdigest(),
            }

        rewrite_manifest(package, mutate)
        proc = self.run_orchestrator(["--execute"], self.evidence_env(package))
        self.assertEqual(proc.returncode, 3, f"Saída:\n{proc.stdout}\n{proc.stderr}")
        self.assertIn("artifact.path: arquivo dentro do repositório", proc.stderr)
        self.assert_no_docker_calls(proc)

    def test_execute_operator_must_match_signer_identity(self):
        package = self._package("signer")
        overrides = self.evidence_env(package)
        overrides["CUTOVER_OPERATOR"] = "operador-divergente"
        proc = self.run_orchestrator(["--execute"], overrides)
        self.assertEqual(proc.returncode, 3, f"Saída:\n{proc.stdout}\n{proc.stderr}")
        self.assertIn("assinatura: verificação recusada", proc.stderr)
        self.assert_no_docker_calls(proc)

    def test_execute_p04_reviewer_must_be_distinct_from_operator(self):
        package = self._package("reviewer", operator="assinante-unico",
                                reviewer="assinante-unico")
        proc = self.run_orchestrator(["--execute"], self.evidence_env(package))
        self.assertEqual(proc.returncode, 3, f"Saída:\n{proc.stdout}\n{proc.stderr}")
        self.assertIn("items.p0_4_identity_sms.verified_by: igual ao operador aprovador", proc.stderr)
        self.assert_no_docker_calls(proc)

    def test_execute_missing_allowed_signers_fails_closed(self):
        package = self._package("allowed_missing")
        package["allowed_signers"].unlink()
        proc = self.run_orchestrator(["--execute"], self.evidence_env(package))
        self.assertEqual(proc.returncode, 3, f"Saída:\n{proc.stdout}\n{proc.stderr}")
        self.assertIn("allowed_signers: arquivo ausente", proc.stderr)
        self.assert_no_docker_calls(proc)

    def test_execute_valid_package_passes_gate_before_first_mutation(self):
        """Pacote válido aprova o portão; o mock falha no primeiro 'up' => prova que o gate
        roda antes de qualquer mutação e que nenhuma mutação teve sucesso."""
        package = self._package("valid")
        http_port, https_port = free_ports(2)
        overrides = self.evidence_env(package, http_port, https_port)
        overrides["MOCK_MUTATION_EXIT"] = "42"

        proc = self.run_orchestrator(["--execute"], overrides)

        self.assertEqual(proc.returncode, 42, f"Saída:\n{proc.stdout}\n{proc.stderr}")
        self.assertIn("Portão de Evidências APROVADO", proc.stdout)
        self.assertNotIn("ERRO DE PACOTE DE EVIDÊNCIAS", proc.stderr, "Não deveria ter rejeitado")
        # Ordem: gate aprovado ANTES da primeira mutação (FASE 5)
        gate_idx = proc.stdout.index("Portão de Evidências APROVADO")
        mutation_idx = proc.stdout.index("[FASE 5] Inicializando exclusivamente o serviço PostgreSQL")
        self.assertLess(gate_idx, mutation_idx)

        calls = self.docker_calls()
        self.assertIn("config", calls, "validação de compose deveria rodar")
        self.assertIn("ps", calls, "pré-checagem de serviços deveria rodar")
        self.assertIn("MUTATION", calls)
        self.assertNotIn(" | run | ", calls, "migrações jamais podiam ser executadas")
        # Nenhuma mutação bem-sucedida: o mock só sai 0 fora de up/run e nunca retornou 0 em up
        mutation_lines = [l for l in calls.splitlines() if l.startswith("MUTATION")]
        self.assertEqual(len(mutation_lines), 1, f"Esperava exatamente 1 tentativa de mutação: {mutation_lines}")
        self.assertIn("up -d postgres", mutation_lines[0])

    def test_execute_busy_public_ports_block_before_any_mutation(self):
        """Host com Caddy ocupando 80/443 (comportamento atual) => exit 3, nenhuma mutação."""
        package = self._package("busy_ports")
        socks, (http_port, https_port) = busy_ports(2)
        try:
            self.config_data["services"]["caddy"]["ports"][0]["published"] = str(http_port)
            self.config_data["services"]["caddy"]["ports"][1]["published"] = str(https_port)
            self.rendered_config.write_text(json.dumps(self.config_data))
            overrides = self.evidence_env(package, http_port, https_port)
            proc = self.run_orchestrator(["--execute"], overrides)
        finally:
            for sock in socks:
                sock.close()

        self.assertEqual(proc.returncode, 3, f"Saída:\n{proc.stdout}\n{proc.stderr}")
        self.assertIn("BLOQUEIO DE PORTAS", proc.stderr)
        self.assertIn("caddy", proc.stderr)
        calls = self.docker_calls()
        self.assertIn("config", calls)
        self.assertNotIn(" ps ", f" {calls} ", "Checagem de portas veio antes do ps")
        self.assert_no_mutation(calls)

    def test_execute_missing_bootstrap_username_blocks_before_mutation(self):
        package = self._package("bootstrap_missing")
        env_file = Path(self.env_file)
        env_file.write_text(env_file.read_text().replace(
            "BOOTSTRAP_ADMIN_USERNAME=release_director",
            "BOOTSTRAP_ADMIN_USERNAME=",
        ))
        proc = self.run_orchestrator(["--execute"], self.evidence_env(package))
        self.assertEqual(proc.returncode, 3)
        self.assertIn("BLOQUEIO DE BOOTSTRAP", proc.stderr)
        self.assert_no_mutation(self.docker_calls())

    def test_execute_rejects_any_published_api_port(self):
        package = self._package("api_port")
        socks, (api_port,) = busy_ports(1)
        try:
            self.config_data["services"]["backend-api"]["ports"] = [{
                "host_ip": "127.0.0.1", "published": str(api_port), "target": 8080, "protocol": "tcp",
            }]
            self.rendered_config.write_text(json.dumps(self.config_data))
            proc = self.run_orchestrator(["--execute"], self.evidence_env(package))
        finally:
            socks[0].close()
        self.assertEqual(proc.returncode, 2)
        self.assertIn("backend-api: portas publicadas são proibidas", proc.stderr)
        self.assert_no_mutation(self.docker_calls())

    def test_ps_failure_after_gate_aborts_fail_closed_with_exit_2(self):
        """Com 'docker compose ps' falhando, o orquestrador não pode ser fail-open; deve sair com 2."""
        package = self._package("ps_fail")
        http_port, https_port = free_ports(2)
        overrides = self.evidence_env(package, http_port, https_port)
        overrides["MOCK_PS_FAIL"] = "1"

        proc = self.run_orchestrator(["--execute"], overrides)

        self.assertEqual(proc.returncode, 2, f"Deveria abortar com 2. Saída:\n{proc.stdout}\n{proc.stderr}")
        self.assertIn("Falha ao inspecionar status dos serviços do projeto catbrasil-prod via 'docker compose ps'", proc.stderr)
        # Garante que JAMAIS tentou subir o postgres nem aplicar migrações
        self.assertNotIn("Inicializando exclusivamente o serviço PostgreSQL", proc.stdout)
        calls = self.docker_calls()
        self.assertNotIn("MUTATION", calls)
        self.assertNotIn(" | run | ", calls)

    def test_pull_failure_blocks_before_service_mutation(self):
        package = self._package("pull_failure")
        proc = self.run_orchestrator(["--execute"], {
            **self.evidence_env(package), "MOCK_PULL_FAIL": "1",
        })
        self.assertEqual(proc.returncode, 2)
        self.assertIn("Falha ao baixar imagem pinada", proc.stderr)
        self.assert_no_mutation(self.docker_calls())

    def test_digest_inspection_failure_blocks_before_service_mutation(self):
        package = self._package("inspect_failure")
        proc = self.run_orchestrator(["--execute"], {
            **self.evidence_env(package), "MOCK_INSPECT_FAIL": "1",
        })
        self.assertEqual(proc.returncode, 2)
        self.assertIn("Falha ao inspecionar digest local", proc.stderr)
        self.assert_no_mutation(self.docker_calls())

    def test_success_starts_services_only_after_preflight_and_health(self):
        package = self._package("sequence")
        proc = self.run_orchestrator(["--execute"], {
            **self.evidence_env(package), "MOCK_PREFLIGHT_EXIT": "0",
        })
        self.assertEqual(proc.returncode, 0, f"Saída:\n{proc.stdout}\n{proc.stderr}")
        calls = self.docker_calls()
        mutations = [line for line in calls.splitlines() if line.startswith("MUTATION")]
        self.assertEqual(len(mutations), 11)
        for index, phrase in enumerate((
            "up -d postgres", "run --rm backend-migrate", "run --rm --no-deps --interactive --tty bootstrap-admin --username release_director", "up -d --no-deps backend-api",
            "up -d --no-deps app-client app-admin", "up -d --no-deps backend-cron",
            "up -d --no-deps alert-relay", "up -d --no-deps alertmanager",
            "up -d --no-deps node-exporter", "up -d --no-deps prometheus", "up -d --no-deps caddy",
        )):
            self.assertIn(phrase, mutations[index])
        self.assertIn("CORTE SEQUENCIAL GREENFIELD CONCLUÍDO", proc.stdout)

    def test_unhealthy_api_prevents_frontends_and_public_services(self):
        package = self._package("unhealthy_api")
        proc = self.run_orchestrator(["--execute"], {
            **self.evidence_env(package), "MOCK_PREFLIGHT_EXIT": "0", "MOCK_HEALTH": "unhealthy",
        })
        self.assertEqual(proc.returncode, 2)
        calls = self.docker_calls()
        self.assertIn("up -d --no-deps backend-api", calls)
        self.assertNotIn("up -d --no-deps app-client", calls)
        self.assertNotIn("up -d --no-deps caddy", calls)
        self.assertIn("stop --timeout 10 backend-api backend-cron app-client app-admin caddy prometheus node-exporter alertmanager alert-relay", calls)

    def test_unhealthy_caddy_stops_all_application_services(self):
        package = self._package("unhealthy_caddy")
        proc = self.run_orchestrator(["--execute"], {
            **self.evidence_env(package), "MOCK_PREFLIGHT_EXIT": "0",
            "MOCK_CADDY_HEALTH": "unhealthy",
        })
        self.assertEqual(proc.returncode, 2)
        calls = self.docker_calls()
        self.assertIn("up -d --no-deps caddy", calls)
        self.assertIn("up -d --no-deps prometheus", calls)
        self.assertIn("stop --timeout 10 backend-api backend-cron app-client app-admin caddy prometheus node-exporter alertmanager alert-relay", calls)
        self.assertNotIn("down", calls)

    def test_unhealthy_alert_relay_prevents_public_caddy_and_stops_services(self):
        package = self._package("unhealthy_alert_relay")
        proc = self.run_orchestrator(["--execute"], {
            **self.evidence_env(package), "MOCK_PREFLIGHT_EXIT": "0",
            "MOCK_ALERT_RELAY_HEALTH": "unhealthy",
        })
        self.assertEqual(proc.returncode, 2)
        calls = self.docker_calls()
        self.assertIn("up -d --no-deps alert-relay", calls)
        self.assertNotIn("up -d --no-deps caddy", calls)
        self.assertIn("stop --timeout 10 backend-api backend-cron app-client app-admin caddy prometheus node-exporter alertmanager alert-relay", calls)

    def test_missing_host_textfile_directory_blocks_before_docker_mutation(self):
        package = self._package("missing_textfile")
        proc = self.run_orchestrator(["--execute"], {
            **self.evidence_env(package), "MOCK_HOST_PATHS_EXIT": "2",
        })
        self.assertEqual(proc.returncode, 2)
        self.assertIn("diretório textfile do host ausente", proc.stderr)
        self.assert_no_mutation(self.docker_calls())


class TestPsFailureFailClosedMocked(ExecuteGateHarness):
    """--execute com evidências aprovadas e portas livres, mas 'ps' falhando => exit 2 (regressão)."""

    def test_ps_failure_without_evidence_never_reaches_docker(self):
        """Sem evidências o exit é 3 e o docker não é chamado (gate vem antes do ps)."""
        proc = self.run_orchestrator(["--execute"], {"MOCK_PS_FAIL": "1"})
        self.assertEqual(proc.returncode, 3)
        self.assertIn("CUTOVER_OPERATOR não definido", proc.stderr)
        self.assert_no_docker_calls(proc)


if __name__ == "__main__":
    unittest.main()
