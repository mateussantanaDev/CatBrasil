#!/usr/bin/env python3
"""
tests/test_validate_suitpay_sandbox_compose.py

Suíte completa de testes automatizados para o validador de configuração do Docker Compose
no ambiente isolado do Sandbox Oficial SuitPay (P0.5).

Cobre:
1. Aprovação com fixture sintética completa (happy path).
2. Rejeição do serviço stub 'pix-sandbox' e de dependências no backend-api.
3. Rejeição de PIX_SANDBOX_URL apontando para o stub ou não-vazio.
4. Rejeição de SUITPAY_LIVE_ENABLED='true' ou ausente.
5. Rejeição de APP_ENV diferente de 'staging' e PIX_MODE diferente de 'suitpay'.
6. Rejeição de domínios localhost, IPs privados ou formatos inválidos para o Caddy.
7. Rejeição de URLs de callback não-HTTPS, endpoints incorretos ou hosts locais.
8. Rejeição de volumes compartilhados (legado 'catbrasil_pgdata', staging, prod).
9. Rejeição de rede compartilhada (legado 'catbrasil_net', staging, prod).
10. Rejeição de portas expostas no host para banco de dados e APIs internas.
11. Rejeição de portas públicas para o Prometheus (exige 127.0.0.1).
12. Rejeição de projetos divergentes de 'catbrasil-suitpay-sandbox'.
13. Garantia de higiene estrita: segredos NUNCA vazam em mensagens de erro.
14. Integração real com 'docker compose config --quiet' e arquivo temporário sintético.
"""

import copy
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
VALIDATOR_SCRIPT = ROOT / "scripts" / "release" / "validate_suitpay_sandbox_compose.py"

import sys
sys.path.insert(0, str(ROOT))

from scripts.release.validate_suitpay_sandbox_compose import (
    validate_config,
    parse_env_file,
    EXPECTED_PROJECT,
    EXPECTED_DB,
    EXPECTED_VOLUME_PGDATA,
    EXPECTED_VOLUME_CADDY_DATA,
    EXPECTED_VOLUME_CADDY_CONFIG,
    EXPECTED_VOLUME_PROMETHEUS,
    EXPECTED_NETWORK,
)


def get_base_valid_payload():
    """Gera uma estrutura JSON sintética válida compatível com a saída de docker compose config."""
    return {
        "name": EXPECTED_PROJECT,
        "services": {
            "postgres": {
                "environment": {
                    "POSTGRES_DB": EXPECTED_DB,
                    "POSTGRES_USER": "catadmin_sandbox",
                    "POSTGRES_PASSWORD": "synthetic_safe_test_password_123",
                    "TZ": "America/Sao_Paulo",
                },
                "ports": None,
            },
            "backend-migrate": {
                "environment": {
                    "DATABASE_URL": f"postgres://catadmin_sandbox:synthetic_safe_test_password_123@postgres:5432/{EXPECTED_DB}?sslmode=disable",
                },
                "ports": None,
            },
            "backend-api": {
                "environment": {
                    "APP_ENV": "staging",
                    "PIX_MODE": "suitpay",
                    "PIX_SANDBOX_URL": "",
                    "SUITPAY_LIVE_ENABLED": "false",
                    "DATABASE_URL": f"postgres://catadmin_sandbox:synthetic_safe_test_password_123@postgres:5432/{EXPECTED_DB}?sslmode=disable",
                    "JWT_SECRET": "0123456789012345678901234567890123456789",
                    "PIX_WEBHOOK_SECRET": "0123456789012345678901234567890123456789",
                    "ADMIN_SECRET_PIN": "123456",
                    "IDENTITY_SECRET": "0123456789012345678901234567890123456789",
                    "CORS_ALLOWED_ORIGINS": "https://sandbox.catbrasil.example.com",
                },
                "ports": None,
                "depends_on": {
                    "postgres": {"condition": "service_healthy"},
                    "backend-migrate": {"condition": "service_completed_successfully"},
                },
            },
            "backend-cron": {
                "environment": {
                    "DATABASE_URL": f"postgres://catadmin_sandbox:synthetic_safe_test_password_123@postgres:5432/{EXPECTED_DB}?sslmode=disable",
                },
                "ports": None,
            },
            "app-client": {
                "ports": None,
            },
            "app-admin": {
                "ports": None,
            },
            "caddy": {
                "environment": {
                    "CLIENT_DOMAIN": "sandbox.catbrasil.example.com",
                    "ADMIN_DOMAIN": "sandbox-admin.catbrasil.example.com",
                    "API_DOMAIN": "sandbox-api.catbrasil.example.com",
                    "ACME_EMAIL": "ops@catbrasil.example.com",
                },
                "ports": [
                    {"mode": "ingress", "host_ip": "0.0.0.0", "target": 80, "published": "80", "protocol": "tcp"},
                    {"mode": "ingress", "host_ip": "0.0.0.0", "target": 443, "published": "443", "protocol": "tcp"},
                ],
            },
            "prometheus": {
                "ports": [
                    {"mode": "ingress", "host_ip": "127.0.0.1", "target": 9090, "published": "29090", "protocol": "tcp"}
                ],
            },
        },
        "volumes": {
            "pgdata": {"name": EXPECTED_VOLUME_PGDATA},
            "caddy_data": {"name": EXPECTED_VOLUME_CADDY_DATA},
            "caddy_config": {"name": EXPECTED_VOLUME_CADDY_CONFIG},
            "prometheus_data": {"name": EXPECTED_VOLUME_PROMETHEUS},
        },
        "networks": {
            "catbrasil_net": {"name": EXPECTED_NETWORK, "external": False},
        },
    }


def get_base_valid_env():
    return {
        "CLIENT_DOMAIN": "sandbox.catbrasil.example.com",
        "ADMIN_DOMAIN": "sandbox-admin.catbrasil.example.com",
        "API_DOMAIN": "sandbox-api.catbrasil.example.com",
        "ACME_EMAIL": "ops@catbrasil.example.com",
        "SUITPAY_CALLBACK_URL": "https://sandbox-api.catbrasil.example.com/api/v1/webhooks/suitpay",
        "JWT_SECRET": "0123456789012345678901234567890123456789",
        "PIX_WEBHOOK_SECRET": "0123456789012345678901234567890123456789",
        "ADMIN_SECRET_PIN": "123456",
        "IDENTITY_SECRET": "0123456789012345678901234567890123456789",
    }


class TestValidateSuitPaySandboxCompose(unittest.TestCase):
    """Testes unitários sem dependência de Docker daemon (pure fixtures)."""

    def test_valid_configuration_passes(self):
        payload = get_base_valid_payload()
        env = get_base_valid_env()
        errors = validate_config(payload, env)
        self.assertEqual(errors, [], f"Configuração válida falhou: {errors}")

    def test_reject_stub_pix_sandbox_service(self):
        payload = get_base_valid_payload()
        payload["services"]["pix-sandbox"] = {
            "environment": {"PIX_SANDBOX_CLIENT_ID": "staging-client"},
            "ports": ["127.0.0.1:18082:8080"],
        }
        errors = validate_config(payload, get_base_valid_env())
        self.assertTrue(any("pix-sandbox" in err for err in errors))

    def test_reject_stub_in_backend_api_depends_on(self):
        payload = get_base_valid_payload()
        payload["services"]["backend-api"]["depends_on"]["pix-sandbox"] = {"condition": "service_healthy"}
        errors = validate_config(payload, get_base_valid_env())
        self.assertTrue(any("dependência" in err and "pix-sandbox" in err for err in errors))

    def test_reject_pix_sandbox_url_pointing_to_stub(self):
        payload = get_base_valid_payload()
        payload["services"]["backend-api"]["environment"]["PIX_SANDBOX_URL"] = "http://pix-sandbox:8080/api/v1/gateway"
        errors = validate_config(payload, get_base_valid_env())
        self.assertTrue(any("PIX_SANDBOX_URL deve ser vazio" in err for err in errors))

    def test_reject_suitpay_live_enabled_true(self):
        payload = get_base_valid_payload()
        payload["services"]["backend-api"]["environment"]["SUITPAY_LIVE_ENABLED"] = "true"
        errors = validate_config(payload, get_base_valid_env())
        self.assertTrue(any("SUITPAY_LIVE_ENABLED" in err for err in errors))

    def test_reject_suitpay_live_enabled_missing(self):
        payload = get_base_valid_payload()
        del payload["services"]["backend-api"]["environment"]["SUITPAY_LIVE_ENABLED"]
        errors = validate_config(payload, get_base_valid_env())
        self.assertTrue(any("SUITPAY_LIVE_ENABLED" in err for err in errors))

    def test_reject_invalid_app_env(self):
        payload = get_base_valid_payload()
        payload["services"]["backend-api"]["environment"]["APP_ENV"] = "production"
        errors = validate_config(payload, get_base_valid_env())
        self.assertTrue(any("APP_ENV" in err for err in errors))

    def test_reject_invalid_pix_mode(self):
        payload = get_base_valid_payload()
        payload["services"]["backend-api"]["environment"]["PIX_MODE"] = "simulated"
        errors = validate_config(payload, get_base_valid_env())
        self.assertTrue(any("PIX_MODE" in err for err in errors))

    def test_reject_localhost_client_domain(self):
        payload = get_base_valid_payload()
        payload["services"]["caddy"]["environment"]["CLIENT_DOMAIN"] = "localhost"
        errors = validate_config(payload, get_base_valid_env())
        self.assertTrue(any("CLIENT_DOMAIN" in err and "não localhost" in err for err in errors))

    def test_reject_localhost_api_domain(self):
        payload = get_base_valid_payload()
        payload["services"]["caddy"]["environment"]["API_DOMAIN"] = "api.localhost"
        errors = validate_config(payload, get_base_valid_env())
        self.assertTrue(any("API_DOMAIN" in err and "não localhost" in err for err in errors))

    def test_reject_loopback_ip_domain(self):
        payload = get_base_valid_payload()
        payload["services"]["caddy"]["environment"]["ADMIN_DOMAIN"] = "127.0.0.1"
        errors = validate_config(payload, get_base_valid_env())
        self.assertTrue(any("ADMIN_DOMAIN" in err and "não localhost" in err for err in errors))

    def test_reject_domain_with_protocol(self):
        payload = get_base_valid_payload()
        payload["services"]["caddy"]["environment"]["CLIENT_DOMAIN"] = "https://sandbox.catbrasil.example.com"
        errors = validate_config(payload, get_base_valid_env())
        self.assertTrue(any("CLIENT_DOMAIN" in err and "formato inválido" in err for err in errors))

    def test_reject_invalid_acme_email(self):
        payload = get_base_valid_payload()
        payload["services"]["caddy"]["environment"]["ACME_EMAIL"] = "admin@localhost"
        errors = validate_config(payload, get_base_valid_env())
        self.assertTrue(any("ACME_EMAIL" in err for err in errors))

    def test_reject_callback_url_with_http_scheme(self):
        payload = get_base_valid_payload()
        env = get_base_valid_env()
        env["SUITPAY_CALLBACK_URL"] = "http://sandbox-api.catbrasil.example.com/api/v1/webhooks/suitpay"
        errors = validate_config(payload, env)
        self.assertTrue(any("SUITPAY_CALLBACK_URL deve usar obrigatoriamente o esquema público HTTPS" in err for err in errors))

    def test_reject_callback_url_with_wrong_endpoint(self):
        payload = get_base_valid_payload()
        env = get_base_valid_env()
        env["SUITPAY_CALLBACK_URL"] = "https://sandbox-api.catbrasil.example.com/api/v1/gateway"
        errors = validate_config(payload, env)
        self.assertTrue(any("esperada rota real '/api/v1/webhooks/suitpay'" in err for err in errors))

    def test_reject_callback_url_with_localhost(self):
        payload = get_base_valid_payload()
        env = get_base_valid_env()
        env["SUITPAY_CALLBACK_URL"] = "https://localhost:8080/api/v1/webhooks/suitpay"
        errors = validate_config(payload, env)
        self.assertTrue(any("não localhost ou IP privado" in err for err in errors))

    def test_reject_shared_legacy_volume(self):
        payload = get_base_valid_payload()
        payload["volumes"]["pgdata"]["name"] = "catbrasil_pgdata"
        errors = validate_config(payload, get_base_valid_env())
        self.assertTrue(any("Volume 'pgdata' colide" in err for err in errors))

    def test_reject_shared_staging_volume(self):
        payload = get_base_valid_payload()
        payload["volumes"]["pgdata"]["name"] = "catbrasil-staging_pgdata"
        errors = validate_config(payload, get_base_valid_env())
        self.assertTrue(any("Volume 'pgdata' colide" in err for err in errors))

    def test_reject_shared_prod_volume(self):
        payload = get_base_valid_payload()
        payload["volumes"]["pgdata"]["name"] = "catbrasil_prod_pgdata"
        errors = validate_config(payload, get_base_valid_env())
        self.assertTrue(any("Volume 'pgdata' colide" in err for err in errors))

    def test_reject_shared_legacy_network(self):
        payload = get_base_valid_payload()
        payload["networks"]["catbrasil_net"]["name"] = "catbrasil_net"
        errors = validate_config(payload, get_base_valid_env())
        self.assertTrue(any("Rede 'catbrasil_net' colide" in err for err in errors))

    def test_reject_shared_staging_network(self):
        payload = get_base_valid_payload()
        payload["networks"]["catbrasil_net"]["name"] = "catbrasil-staging_net"
        errors = validate_config(payload, get_base_valid_env())
        self.assertTrue(any("Rede 'catbrasil_net' colide" in err for err in errors))

    def test_reject_external_network(self):
        payload = get_base_valid_payload()
        payload["networks"]["catbrasil_net"]["external"] = True
        errors = validate_config(payload, get_base_valid_env())
        self.assertTrue(any("não deve ser external" in err for err in errors))

    def test_reject_exposed_postgres_port(self):
        payload = get_base_valid_payload()
        payload["services"]["postgres"]["ports"] = [
            {"mode": "ingress", "host_ip": "127.0.0.1", "target": 5432, "published": "5432", "protocol": "tcp"}
        ]
        errors = validate_config(payload, get_base_valid_env())
        self.assertTrue(any("postgres não deve expor portas no host" in err for err in errors))

    def test_reject_exposed_backend_api_port(self):
        payload = get_base_valid_payload()
        payload["services"]["backend-api"]["ports"] = [
            {"mode": "ingress", "host_ip": "127.0.0.1", "target": 8080, "published": "8080", "protocol": "tcp"}
        ]
        errors = validate_config(payload, get_base_valid_env())
        self.assertTrue(any("backend-api não deve expor portas no host" in err for err in errors))

    def test_reject_exposed_frontend_ports(self):
        payload = get_base_valid_payload()
        payload["services"]["app-client"]["ports"] = [
            {"mode": "ingress", "host_ip": "127.0.0.1", "target": 80, "published": "3000", "protocol": "tcp"}
        ]
        errors = validate_config(payload, get_base_valid_env())
        self.assertTrue(any("app-client não deve expor portas no host" in err for err in errors))

    def test_reject_public_prometheus_port(self):
        payload = get_base_valid_payload()
        payload["services"]["prometheus"]["ports"] = [
            {"mode": "ingress", "host_ip": "0.0.0.0", "target": 9090, "published": "29090", "protocol": "tcp"}
        ]
        errors = validate_config(payload, get_base_valid_env())
        self.assertTrue(any("prometheus deve expor portas exclusivamente na interface loopback" in err for err in errors))

    def test_reject_wrong_project_name(self):
        for bad_project in ["catbrasil", "catbrasil-staging", "catbrasil-prod", "other"]:
            payload = get_base_valid_payload()
            payload["name"] = bad_project
            errors = validate_config(payload, get_base_valid_env())
            self.assertTrue(any("Projeto Compose inválido" in err for err in errors))

    def test_reject_wrong_postgres_db(self):
        for bad_db in ["catbrasil", "catbrasil_staging", "catbrasil_prod"]:
            payload = get_base_valid_payload()
            payload["services"]["postgres"]["environment"]["POSTGRES_DB"] = bad_db
            errors = validate_config(payload, get_base_valid_env())
            self.assertTrue(any("POSTGRES_DB inválido" in err for err in errors))

    def test_reject_wrong_database_url_host(self):
        payload = get_base_valid_payload()
        payload["services"]["backend-api"]["environment"]["DATABASE_URL"] = (
            f"postgres://catadmin:secret@127.0.0.1:5432/{EXPECTED_DB}?sslmode=disable"
        )
        errors = validate_config(payload, get_base_valid_env())
        self.assertTrue(any("host interno 'postgres'" in err for err in errors))

    def test_reject_wrong_database_url_db(self):
        payload = get_base_valid_payload()
        payload["services"]["backend-api"]["environment"]["DATABASE_URL"] = (
            "postgres://catadmin:secret@postgres:5432/catbrasil_staging?sslmode=disable"
        )
        errors = validate_config(payload, get_base_valid_env())
        self.assertTrue(any(f"base de dados de DATABASE_URL inválida; esperada '{EXPECTED_DB}'" in err for err in errors))

    def test_secret_hygiene_no_leaks_in_error_messages(self):
        """Comprova que nenhum segredo sintético é vazado nas mensagens de erro."""
        secret_password = "LEAK_PROBE_SECRET_PASSWORD_VAL_9999"
        secret_jwt = "LEAK_PROBE_JWT_SECRET_SUPER_CONFIDENTIAL_VAL"
        payload = get_base_valid_payload()
        payload["services"]["backend-api"]["environment"]["DATABASE_URL"] = (
            f"postgres://catadmin:{secret_password}@wronghost:5432/wrongdb"
        )
        payload["services"]["backend-api"]["environment"]["JWT_SECRET"] = "short"  # dispara erro de tamanho

        errors = validate_config(payload, get_base_valid_env())
        self.assertTrue(len(errors) > 0)
        for err in errors:
            self.assertNotIn(secret_password, err, "Vazamento detectado da senha do banco em mensagem de erro!")
            self.assertNotIn(secret_jwt, err, "Vazamento detectado de segredo JWT em mensagem de erro!")

    def test_reject_missing_callback_url(self):
        """Verifica que SUITPAY_CALLBACK_URL ausente ou vazia é rejeitada."""
        payload = get_base_valid_payload()
        env = get_base_valid_env()
        del env["SUITPAY_CALLBACK_URL"]
        errors = validate_config(payload, env)
        self.assertTrue(any("SUITPAY_CALLBACK_URL ausente ou vazia" in err for err in errors))

        env["SUITPAY_CALLBACK_URL"] = "   "
        errors = validate_config(payload, env)
        self.assertTrue(any("SUITPAY_CALLBACK_URL ausente ou vazia" in err for err in errors))

    def test_reject_callback_url_with_userinfo(self):
        """Verifica que userinfo na callback URL é estritamente rejeitado."""
        payload = get_base_valid_payload()
        env = get_base_valid_env()
        env["SUITPAY_CALLBACK_URL"] = "https://user:pass@sandbox-api.catbrasil.example.com/api/v1/webhooks/suitpay"
        errors = validate_config(payload, env)
        self.assertTrue(any("não deve conter credenciais de autenticação (userinfo)" in err for err in errors))

    def test_reject_callback_url_with_alternative_port(self):
        """Verifica que portas explícitas/alternativas na callback URL são rejeitadas."""
        for bad_url in [
            "https://sandbox-api.catbrasil.example.com:8443/api/v1/webhooks/suitpay",
            "https://sandbox-api.catbrasil.example.com:443/api/v1/webhooks/suitpay",
        ]:
            payload = get_base_valid_payload()
            env = get_base_valid_env()
            env["SUITPAY_CALLBACK_URL"] = bad_url
            errors = validate_config(payload, env)
            self.assertTrue(any("não deve conter porta alternativa na URL" in err for err in errors))

    def test_reject_callback_url_with_query_or_fragment(self):
        """Verifica que query params e fragmentos são rejeitados na callback URL."""
        payload = get_base_valid_payload()
        env = get_base_valid_env()
        env["SUITPAY_CALLBACK_URL"] = "https://sandbox-api.catbrasil.example.com/api/v1/webhooks/suitpay?token=secret123"
        errors = validate_config(payload, env)
        self.assertTrue(any("não deve conter parâmetros de consulta (query)" in err for err in errors))

        env["SUITPAY_CALLBACK_URL"] = "https://sandbox-api.catbrasil.example.com/api/v1/webhooks/suitpay#webhook-hash"
        errors = validate_config(payload, env)
        self.assertTrue(any("não deve conter fragmento (#)" in err for err in errors))

    def test_reject_callback_url_with_extra_or_divergent_path(self):
        """Verifica que paths adicionais ou divergentes da rota exata são rejeitados."""
        for bad_path in [
            "/api/v1/webhooks/suitpay/extra",
            "/api/v1/webhooks/suitpay/",
            "/api/v1/webhooks",
            "/webhook",
        ]:
            payload = get_base_valid_payload()
            env = get_base_valid_env()
            env["SUITPAY_CALLBACK_URL"] = f"https://sandbox-api.catbrasil.example.com{bad_path}"
            errors = validate_config(payload, env)
            self.assertTrue(any("possui rota incorreta; esperada rota real '/api/v1/webhooks/suitpay'" in err for err in errors))

    def test_reject_callback_url_with_ip_address(self):
        """Verifica que endereços IP (públicos ou privados) na callback URL são rejeitados."""
        for ip in ["192.168.1.10", "203.0.113.5", "127.0.0.1", "10.0.0.1", "198.51.100.25"]:
            payload = get_base_valid_payload()
            env = get_base_valid_env()
            env["SUITPAY_CALLBACK_URL"] = f"https://{ip}/api/v1/webhooks/suitpay"
            errors = validate_config(payload, env)
            self.assertTrue(
                any("não deve utilizar endereço IP" in err or "não localhost ou IP privado" in err for err in errors),
                f"Falha ao rejeitar IP na callback URL: {ip}",
            )

    def test_reject_callback_url_with_different_public_domain(self):
        """Verifica que domínio diferente de API_DOMAIN na callback URL é rejeitado."""
        payload = get_base_valid_payload()
        env = get_base_valid_env()
        env["SUITPAY_CALLBACK_URL"] = "https://different-domain.example.com/api/v1/webhooks/suitpay"
        errors = validate_config(payload, env)
        self.assertTrue(any("deve corresponder exatamente ao domínio configurado em API_DOMAIN" in err for err in errors))

    def test_reject_missing_critical_services(self):
        """Verifica que a ausência de qualquer serviço crítico do Compose causa rejeição."""
        critical_services = [
            "postgres", "backend-migrate", "backend-api", "backend-cron",
            "app-client", "app-admin", "caddy"
        ]
        for svc in critical_services:
            payload = get_base_valid_payload()
            del payload["services"][svc]
            errors = validate_config(payload, get_base_valid_env())
            self.assertTrue(
                any(f"Serviço crítico '{svc}' ausente" in err for err in errors),
                f"Validador não rejeitou ausência do serviço crítico: {svc}"
            )

    def test_reject_caddy_missing_even_with_domains_in_env(self):
        """Garante que a ausência do Caddy é rejeitada mesmo que as variáveis de domínio existam no .env."""
        payload = get_base_valid_payload()
        del payload["services"]["caddy"]
        env = get_base_valid_env()
        errors = validate_config(payload, env)
        self.assertTrue(any("Serviço crítico 'caddy' ausente" in err for err in errors))

    def test_caddy_ports_validation(self):
        """Verifica validação estrita de publicação das portas 80 e 443 do Caddy."""
        # 1. Caddy sem portas publicadas
        payload = get_base_valid_payload()
        payload["services"]["caddy"]["ports"] = []
        errors = validate_config(payload, get_base_valid_env())
        self.assertTrue(any("Caddy deve publicar obrigatoriamente as portas 80 e 443" in err for err in errors))

        # 2. Caddy com apenas porta 80
        payload = get_base_valid_payload()
        payload["services"]["caddy"]["ports"] = [
            {"mode": "ingress", "host_ip": "0.0.0.0", "target": 80, "published": "80", "protocol": "tcp"}
        ]
        errors = validate_config(payload, get_base_valid_env())
        self.assertTrue(any("Caddy deve publicar obrigatoriamente as portas 80 e 443" in err for err in errors))

        # 3. Caddy com apenas porta 443
        payload = get_base_valid_payload()
        payload["services"]["caddy"]["ports"] = [
            {"mode": "ingress", "host_ip": "0.0.0.0", "target": 443, "published": "443", "protocol": "tcp"}
        ]
        errors = validate_config(payload, get_base_valid_env())
        self.assertTrue(any("Caddy deve publicar obrigatoriamente as portas 80 e 443" in err for err in errors))

        # 4. Caddy com portas alternativas publicadas (8080 / 8443)
        payload = get_base_valid_payload()
        payload["services"]["caddy"]["ports"] = [
            {"mode": "ingress", "host_ip": "0.0.0.0", "target": 80, "published": "8080", "protocol": "tcp"},
            {"mode": "ingress", "host_ip": "0.0.0.0", "target": 443, "published": "8443", "protocol": "tcp"},
        ]
        errors = validate_config(payload, get_base_valid_env())
        self.assertTrue(any("Caddy deve publicar obrigatoriamente as portas 80 e 443" in err for err in errors))

        # 5. Caddy com portas 80/443 vinculadas exclusivamente a loopback (127.0.0.1)
        payload = get_base_valid_payload()
        payload["services"]["caddy"]["ports"] = [
            {"mode": "ingress", "host_ip": "127.0.0.1", "target": 80, "published": "80", "protocol": "tcp"},
            {"mode": "ingress", "host_ip": "127.0.0.1", "target": 443, "published": "443", "protocol": "tcp"},
        ]
        errors = validate_config(payload, get_base_valid_env())
        self.assertTrue(any("Caddy não deve vincular portas 80/443 exclusivamente a loopback" in err for err in errors))

        # 6. Formato string válido (0.0.0.0:80:80 e 0.0.0.0:443:443)
        payload = get_base_valid_payload()
        payload["services"]["caddy"]["ports"] = ["0.0.0.0:80:80", "0.0.0.0:443:443"]
        errors = validate_config(payload, get_base_valid_env())
        self.assertFalse(any("Caddy deve publicar obrigatoriamente as portas 80 e 443" in err for err in errors))

    def test_sentinel_leak_probes_never_leak_into_error_messages(self):
        """Injeta strings sentinela simulando segredos/tokens em todas as entradas de erro

        e valida que nenhuma delas vaza em qualquer mensagem de erro retornada.
        """
        sentinel_base = "SECRET_SENTINEL_TOKEN_"

        # 1. Project name
        sentinel_project = f"{sentinel_base}PROJECT_98765"
        p = get_base_valid_payload()
        p["name"] = sentinel_project
        errs = validate_config(p, get_base_valid_env())
        self.assertTrue(len(errs) > 0)
        for e in errs:
            self.assertNotIn(sentinel_project, e, "Vazamento de project_name detectado!")

        # 2. Volumes
        for vol_key in ["pgdata", "caddy_data", "caddy_config", "prometheus_data"]:
            sentinel_vol = f"{sentinel_base}VOL_{vol_key.upper()}_87654"
            p = get_base_valid_payload()
            p["volumes"][vol_key]["name"] = sentinel_vol
            errs = validate_config(p, get_base_valid_env())
            self.assertTrue(len(errs) > 0)
            for e in errs:
                self.assertNotIn(sentinel_vol, e, f"Vazamento do volume {vol_key} detectado!")

        # 3. Network name
        sentinel_net = f"{sentinel_base}NET_76543"
        p = get_base_valid_payload()
        p["networks"]["catbrasil_net"]["name"] = sentinel_net
        errs = validate_config(p, get_base_valid_env())
        self.assertTrue(len(errs) > 0)
        for e in errs:
            self.assertNotIn(sentinel_net, e, "Vazamento do nome da rede detectado!")

        # 4. Caddy domains
        for dom_key in ["CLIENT_DOMAIN", "ADMIN_DOMAIN", "API_DOMAIN"]:
            sentinel_dom = f"{sentinel_base}DOM_{dom_key}_65432.invalid"
            p = get_base_valid_payload()
            p["services"]["caddy"]["environment"][dom_key] = sentinel_dom
            errs = validate_config(p, get_base_valid_env())
            self.assertTrue(len(errs) > 0)
            for e in errs:
                self.assertNotIn(sentinel_dom, e, f"Vazamento de domínio {dom_key} detectado!")

        # 5. ACME Email
        sentinel_bad_email = f"{sentinel_base}EMAIL_BAD_FORMAT_43210"
        p = get_base_valid_payload()
        p["services"]["caddy"]["environment"]["ACME_EMAIL"] = sentinel_bad_email
        errs = validate_config(p, get_base_valid_env())
        self.assertTrue(len(errs) > 0)
        for e in errs:
            self.assertNotIn(sentinel_bad_email, e, "Vazamento de ACME_EMAIL detectado!")

        # 6. SUITPAY_CALLBACK_URL (em host, path, query, fragment, userinfo)
        sentinel_cb_host = f"{sentinel_base}CB_HOST_32109.com"
        p = get_base_valid_payload()
        env = get_base_valid_env()
        env["SUITPAY_CALLBACK_URL"] = f"https://{sentinel_cb_host}/api/v1/webhooks/suitpay"
        errs = validate_config(p, env)
        self.assertTrue(len(errs) > 0)
        for e in errs:
            self.assertNotIn(sentinel_cb_host, e, "Vazamento de host da callback URL detectado!")

        sentinel_cb_path = f"/api/v1/webhooks/suitpay/{sentinel_base}PATH_21098"
        env = get_base_valid_env()
        env["SUITPAY_CALLBACK_URL"] = f"https://sandbox-api.catbrasil.example.com{sentinel_cb_path}"
        errs = validate_config(p, env)
        self.assertTrue(len(errs) > 0)
        for e in errs:
            self.assertNotIn(sentinel_cb_path, e, "Vazamento de path da callback URL detectado!")
            self.assertNotIn(f"{sentinel_base}PATH_21098", e, "Vazamento de token de path detectado!")

        sentinel_cb_user = f"{sentinel_base}USER_10987"
        env = get_base_valid_env()
        env["SUITPAY_CALLBACK_URL"] = f"https://{sentinel_cb_user}:secret@sandbox-api.catbrasil.example.com/api/v1/webhooks/suitpay"
        errs = validate_config(p, env)
        self.assertTrue(len(errs) > 0)
        for e in errs:
            self.assertNotIn(sentinel_cb_user, e, "Vazamento de userinfo da callback URL detectado!")

        sentinel_cb_query = f"{sentinel_base}QUERY_09876"
        env = get_base_valid_env()
        env["SUITPAY_CALLBACK_URL"] = f"https://sandbox-api.catbrasil.example.com/api/v1/webhooks/suitpay?token={sentinel_cb_query}"
        errs = validate_config(p, env)
        self.assertTrue(len(errs) > 0)
        for e in errs:
            self.assertNotIn(sentinel_cb_query, e, "Vazamento de query da callback URL detectado!")

    def test_report_uses_comprimento_minimo_without_alleging_entropy(self):
        """Verifica se o relatório de sucesso utiliza 'comprimento mínimo' e não alega 'entropia'."""
        import io
        from contextlib import redirect_stdout
        from scripts.release.validate_suitpay_sandbox_compose import print_success_report

        buf = io.StringIO()
        with redirect_stdout(buf):
            print_success_report()
        report_output = buf.getvalue()

        self.assertIn("comprimento mínimo", report_output)
        self.assertNotIn("entropia", report_output.lower())


class TestDockerComposeExecution(unittest.TestCase):
    """Testa execução real contra os arquivos de Compose caso docker compose esteja disponível."""

    @classmethod
    def setUpClass(cls):
        cls.docker_available = shutil.which("docker") is not None
        if cls.docker_available:
            try:
                res = subprocess.run(["docker", "compose", "version"], capture_output=True, text=True, timeout=5)
                cls.docker_available = (res.returncode == 0)
            except Exception:
                cls.docker_available = False

    def test_docker_compose_config_quiet_with_synthetic_env(self):
        if not self.docker_available:
            self.skipTest("Docker compose não disponível neste ambiente de teste.")

        synthetic_env_content = """
COMPOSE_PROJECT_NAME=catbrasil-suitpay-sandbox
APP_ENV=staging
PIX_MODE=suitpay
SUITPAY_LIVE_ENABLED=false
PIX_SANDBOX_URL=
GOMAXPROCS=2
DB_NAME=catbrasil_suitpay_sandbox
DB_USER=catadmin_sandbox
DB_PASSWORD=fake_test_db_password_12345678
DATABASE_URL=postgres://catadmin_sandbox:fake_test_db_password_12345678@postgres:5432/catbrasil_suitpay_sandbox?sslmode=disable
JWT_SECRET=0123456789012345678901234567890123456789
PIX_WEBHOOK_SECRET=0123456789012345678901234567890123456789
ADMIN_SECRET_PIN=123456
IDENTITY_SECRET=0123456789012345678901234567890123456789
OBSERVABILITY_TOKEN=0123456789012345678901234567890123456789
SMS_GATEWAY_URL=
SMS_GATEWAY_TOKEN=
SMS_CODE_SECRET=
CLIENT_DOMAIN=sandbox.catbrasil.example.com
ADMIN_DOMAIN=sandbox-admin.catbrasil.example.com
API_DOMAIN=sandbox-api.catbrasil.example.com
ACME_EMAIL=ops@catbrasil.example.com
CORS_ALLOWED_ORIGINS=https://sandbox.catbrasil.example.com
SUITPAY_CALLBACK_URL=https://sandbox-api.catbrasil.example.com/api/v1/webhooks/suitpay
HTTP_BIND_ADDRESS=0.0.0.0
HTTPS_BIND_ADDRESS=0.0.0.0
HTTP_PORT=80
HTTPS_PORT=443
SANDBOX_PROMETHEUS_PORT=29090
"""
        with tempfile.NamedTemporaryFile("w", delete=False) as f:
            f.write(synthetic_env_content)
            temp_env = Path(f.name)

        try:
            # 1. Executa docker compose config --quiet (sem imprimir secrets)
            cmd_quiet = [
                "docker", "compose",
                "-p", "catbrasil-suitpay-sandbox",
                "--env-file", str(temp_env),
                "-f", str(ROOT / "docker-compose.yml"),
                "-f", str(ROOT / "compose.suitpay-sandbox.yml"),
                "config", "--quiet"
            ]
            res_quiet = subprocess.run(cmd_quiet, cwd=ROOT, capture_output=True, text=True, timeout=15)
            self.assertEqual(res_quiet.returncode, 0, f"docker compose config --quiet falhou: {res_quiet.stderr}")

            # 2. Executa o validador oficial via CLI com o arquivo de ambiente
            cmd_validator = [
                sys.executable, str(VALIDATOR_SCRIPT),
                "--env-file", str(temp_env)
            ]
            res_val = subprocess.run(cmd_validator, cwd=ROOT, capture_output=True, text=True, timeout=20)
            self.assertEqual(res_val.returncode, 0, f"Validador falhou inesperadamente: {res_val.stderr}")
            self.assertIn("Configuração do sandbox SuitPay aprovada com sucesso.", res_val.stdout)
            self.assertIn("AVISO DE FRONTEIRA E LIMITES TÉCNICOS:", res_val.stdout)
        finally:
            if temp_env.exists():
                os.remove(temp_env)


if __name__ == "__main__":
    unittest.main()
