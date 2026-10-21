#!/usr/bin/env python3
"""
tests/test_ephemeral_cutover_proof.py

Prova prática do portão técnico em contêiner PostgreSQL 16 efêmero e isolado:
1. Sobe PostgreSQL 16 sem portas publicadas e sem rede (--network none).
2. Sem uso de volumes legados (armazenamento estritamente efêmero/descartável).
3. Aplica sequencialmente todas as migrações de backend/migrations/*.up.sql.
4. Executa scripts/release/preflight_cutover.py:
   - Prova 1: No modo de prova (--proof-mode-evaluation-only), valida que o banco novo migrado
     possui zero registros operacionais e sequência completa íntegra. Comprova que gate_authorized
     permanece FALSE (o modo de prova JAMAIS emite autorização de corte de produção).
   - Prova 2: Bloqueio fail-closed com exit code 1 ao injetar dado fictício de teste APENAS no banco efêmero.
   - Prova 3: No modo real de produção (sem flag de prova), o preflight aborta com exit code 2
     rejeitando o contêiner efêmero por ausência das labels e rede fixadas de produção.
   - Prova 4: Detecção de estados de risco operacional em payout_submissions e payout_event_queue sem vazamento de dados.
   - Prova 5: Banco efêmero migrado APENAS até a versão 9 (sem migração 10 / tabelas 25 e 26):
     comprova que o portão NÃO quebra com erro de parse de relações ausentes (exit 2),
     mas sim retorna estritamente exit 1 (VIOLATIONS_DETECTED) e gate_authorized=False.
5. Destrói o contêiner efêmero ao término (sem tocar em staging ou produção).
"""

import atexit
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import time
import unittest
import uuid

ROOT = Path(__file__).resolve().parents[1]
MIGRATIONS_DIR = ROOT / "backend" / "migrations"


class TestEphemeralCutoverProof(unittest.TestCase):
    """Executa a prova do portão de corte em contêiner PostgreSQL descartável."""

    @classmethod
    def setUpClass(cls):
        try:
            subprocess.run(["docker", "info"], capture_output=True, check=True)
        except Exception:
            raise unittest.SkipTest("Docker não está disponível no host para teste efêmero.")

        cls.container_name = f"catbrasil-proof-{int(time.time())}-{uuid.uuid4().hex[:6]}"
        cls.db_name = "catbrasil_prod"

        # 1. Inicia contêiner PostgreSQL 16 isolado, sem portas, sem rede externa
        cmd_run = [
            "docker", "run", "--rm", "--network", "none",
            "--name", cls.container_name,
            "-e", "POSTGRES_HOST_AUTH_METHOD=trust",
            "-e", f"POSTGRES_DB={cls.db_name}",
            "-d", "postgres:16-alpine"
        ]
        res = subprocess.run(cmd_run, capture_output=True, text=True)
        if res.returncode != 0:
            raise RuntimeError(f"Falha ao iniciar contêiner efêmero: {res.stderr}")

        atexit.register(lambda: subprocess.run(["docker", "rm", "-f", cls.container_name], capture_output=True))

        # 2. Aguarda prontidão completa do PostgreSQL e criação do banco catbrasil_prod
        ready = False
        for _ in range(30):
            check = subprocess.run(
                ["docker", "exec", cls.container_name, "psql", "-U", "postgres", "-d", cls.db_name, "-c", "SELECT 1"],
                capture_output=True
            )
            if check.returncode == 0:
                ready = True
                break
            time.sleep(1)

        if not ready:
            subprocess.run(["docker", "rm", "-f", cls.container_name], capture_output=True)
            raise RuntimeError("PostgreSQL efêmero não respondeu a pg_isready dentro do tempo limite.")

        # 3. Cria tabela de controle de migrações e aplica todas as migrações.
        init_schema_migrations_sql = """
        CREATE TABLE IF NOT EXISTS schema_migrations (
            version INTEGER PRIMARY KEY,
            name TEXT NOT NULL,
            checksum CHAR(64) NOT NULL,
            applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
        );
        """
        cls._exec_psql(init_schema_migrations_sql)

        # Localiza arquivos .up.sql ordenados
        up_files = sorted(list(MIGRATIONS_DIR.glob("*.up.sql")))
        if len(up_files) < 14:
            raise RuntimeError(f"Esperadas ao menos 14 migrações em {MIGRATIONS_DIR}, encontradas {len(up_files)}")

        for up_path in up_files:
            sql_content = up_path.read_text(encoding="utf-8")
            version_str, _, mig_name = up_path.name.partition("_")
            version_int = int(version_str)
            checksum = hashlib.sha256(sql_content.encode("utf-8")).hexdigest()

            # Executa SQL da migração
            cls._exec_psql(sql_content)

            # Registra na tabela schema_migrations
            record_sql = f"""
            INSERT INTO schema_migrations (version, name, checksum)
            VALUES ({version_int}, '{up_path.name}', '{checksum}');
            """
            cls._exec_psql(record_sql)

    @classmethod
    def tearDownClass(cls):
        if hasattr(cls, "container_name"):
            subprocess.run(["docker", "rm", "-f", cls.container_name], capture_output=True)

    @classmethod
    def _exec_psql(cls, sql_text: str):
        cmd = [
            "docker", "exec", "-i", cls.container_name,
            "psql", "-XAt", "-v", "ON_ERROR_STOP=1", "-U", "postgres", "-d", cls.db_name
        ]
        res = subprocess.run(cmd, input=sql_text, text=True, capture_output=True)
        if res.returncode != 0:
            raise RuntimeError(f"Falha ao executar SQL no contêiner efêmero: {res.stderr}\nQuery: {sql_text[:200]}")
        return res.stdout

    def test_01_proof_mode_verifies_zero_state_without_authorizing_cutover(self):
        """
        Prova 1: O modo de prova valida o estado zero completo do banco recém-migrado,
        porém JAMAIS autoriza o corte de produção (gate_authorized=False).
        """
        preflight_script = ROOT / "scripts" / "release" / "preflight_cutover.py"
        cmd = [
            sys.executable, str(preflight_script),
            "--container", self.container_name,
            "--proof-mode-evaluation-only",
            "--json"
        ]
        proc = subprocess.run(cmd, capture_output=True, text=True)
        self.assertEqual(proc.returncode, 0, f"Modo de prova deveria sair com 0. Saída:\n{proc.stdout}\n{proc.stderr}")

        result = json.loads(proc.stdout)
        self.assertEqual(result["status"], "PROOF_MODE_ZERO_STATE_VERIFIED")
        self.assertFalse(result["gate_authorized"], "Modo de prova JAMAIS deve emitir gate_authorized=True")
        self.assertTrue(result["is_proof_mode"])
        self.assertEqual(result["exit_code"], 0)
        expected = list(range(1, len(list(MIGRATIONS_DIR.glob("*.up.sql"))) + 1))
        self.assertEqual(result["migrations"]["versions_applied"], expected)
        self.assertEqual(result["migrations"]["max_version"], expected[-1])
        self.assertEqual(result["tables_checked_count"], 28)
        self.assertEqual(len(result["violations"]), 0)

        # Checa presença e valor das métricas operacionais P0.3, P1.6, P1.5/CRON
        self.assertIn("payout_submissions_count", result["zero_state_metrics"])
        self.assertIn("unlinked_payout_submissions_count", result["zero_state_metrics"])
        self.assertIn("payout_event_queue_count", result["zero_state_metrics"])
        self.assertIn("pending_payout_events_count", result["zero_state_metrics"])
        self.assertIn("http_rate_limits_count", result["zero_state_metrics"])
        self.assertIn("cron_executions_count", result["zero_state_metrics"])

        # Checa todas as tabelas operacionais em zero absoluto
        for metric, val in result["zero_state_metrics"].items():
            if "balance_sum" in metric:
                self.assertEqual(val, "0.00", f"Saldo não zerado na métrica {metric}")
            else:
                self.assertEqual(val, 0, f"Contagem não zerada na métrica {metric}")

    def test_02_proof_mode_blocks_when_fictional_data_injected(self):
        """
        Prova 2: A injeção de dado fictício no banco efêmero bloqueia imediatamente o portão
        com exit code 1 e identifica as violações de estado zero.
        """
        fake_insert_sql = """
        INSERT INTO users (id, username, phone, password_hash, balance)
        VALUES ('proof-fake-user-1', 'fakeuser_proof', '+5511999999999', 'fake_hash', 100.00);
        """
        self._exec_psql(fake_insert_sql)

        preflight_script = ROOT / "scripts" / "release" / "preflight_cutover.py"
        cmd = [
            sys.executable, str(preflight_script),
            "--container", self.container_name,
            "--proof-mode-evaluation-only",
            "--json"
        ]
        proc = subprocess.run(cmd, capture_output=True, text=True)
        self.assertEqual(proc.returncode, 1, f"Preflight deveria falhar com código 1. Saída:\n{proc.stdout}")

        result = json.loads(proc.stdout)
        self.assertEqual(result["status"], "VIOLATIONS_DETECTED")
        self.assertFalse(result["gate_authorized"])
        self.assertEqual(result["exit_code"], 1)

        violations_str = " ".join(result["violations"])
        self.assertIn("users_count", violations_str)
        self.assertIn("users_balance_sum", violations_str)

    def test_03_production_mode_strictly_rejects_ephemeral_container_labels(self):
        """
        Prova 3: Em modo de produção real (sem a flag de prova), o contêiner efêmero é
        imediatamente rejeitado (código 2) por ausência de labels Compose e rede de produção.
        """
        preflight_script = ROOT / "scripts" / "release" / "preflight_cutover.py"
        cmd = [
            sys.executable, str(preflight_script),
            "--container", self.container_name,
            "--json"
        ]
        proc = subprocess.run(cmd, capture_output=True, text=True)
        self.assertEqual(proc.returncode, 2, f"Modo de produção real deveria abortar com código 2. Saída:\n{proc.stdout}\n{proc.stderr}")
        self.assertIn("LABEL_AUSENTE", proc.stderr)

    def test_04_proof_mode_detects_unlinked_payout_and_pending_queue_risk(self):
        """
        Prova 4: Submissões de saque sem external_id (unlinked) e eventos pendentes na fila
        são identificados como RISCO OPERACIONAL pelo preflight sem expor referências ou dados sensíveis.
        """
        insert_payout_risk_sql = """
        INSERT INTO transactions (id, user_id, tipo, valor, status)
        VALUES ('proof-fake-tx-10', 'proof-fake-user-1', 'Saque PIX', 250.00, 'processando');

        INSERT INTO payout_submissions (transaction_id, provider, internal_ref, external_id)
        VALUES ('proof-fake-tx-10', 'suitpay', 'proof-internal-ref-secret-777', NULL);

        INSERT INTO payout_event_queue (dedup_key, kind, transaction_id, external_id, reason, queue_status)
        VALUES ('proof-dedup-secret-888', 'unlinked_callback', 'proof-fake-tx-10', 'ext-secret-999', 'callback fora de ordem', 'pending');
        """
        self._exec_psql(insert_payout_risk_sql)

        preflight_script = ROOT / "scripts" / "release" / "preflight_cutover.py"
        cmd = [
            sys.executable, str(preflight_script),
            "--container", self.container_name,
            "--proof-mode-evaluation-only",
            "--json"
        ]
        proc = subprocess.run(cmd, capture_output=True, text=True)
        self.assertEqual(proc.returncode, 1, f"Preflight deveria detectar violações e sair com código 1. Saída:\n{proc.stdout}")

        result = json.loads(proc.stdout)
        self.assertEqual(result["status"], "VIOLATIONS_DETECTED")
        self.assertFalse(result["gate_authorized"])
        self.assertEqual(result["exit_code"], 1)

        violations_str = " ".join(result["violations"])
        self.assertIn("unlinked_payout_submissions_count", violations_str)
        self.assertIn("Submissões de saque sem retorno do provedor vinculado", violations_str)
        self.assertIn("pending_payout_events_count", violations_str)
        self.assertIn("Eventos de saque pendentes de reconciliação manual", violations_str)

        # Proteção estrita contra vazamento de referências e identificadores sensíveis
        self.assertNotIn("proof-internal-ref-secret-777", proc.stdout)
        self.assertNotIn("proof-dedup-secret-888", proc.stdout)
        self.assertNotIn("ext-secret-999", proc.stdout)
        self.assertNotIn("callback fora de ordem", proc.stdout)

    def test_05_proof_mode_blocks_when_database_migrated_only_to_version_9(self):
        """
        Prova 5: Em banco de dados efêmero migrado APENAS até a versão 9 (sem a migração 10
        e sem as tabelas payout_submissions e payout_event_queue):
        Comprova que o portão de corte NÃO falha com erro de parse/relação inexistente (exit 2),
        mas sim detecta com segurança a ausência da migração 10 e tabelas faltantes,
        retornando estritamente exit 1 (VIOLATIONS_DETECTED) e gate_authorized=False.
        """
        container_mig9 = f"catbrasil-proof-mig9-{int(time.time())}-{uuid.uuid4().hex[:6]}"
        db_name = "catbrasil_prod"

        cmd_run = [
            "docker", "run", "--rm", "--network", "none",
            "--name", container_mig9,
            "-e", "POSTGRES_HOST_AUTH_METHOD=trust",
            "-e", f"POSTGRES_DB={db_name}",
            "-d", "postgres:16-alpine"
        ]
        res = subprocess.run(cmd_run, capture_output=True, text=True)
        if res.returncode != 0:
            raise RuntimeError(f"Falha ao iniciar contêiner efêmero mig9: {res.stderr}")

        try:
            # Aguarda prontidão do PostgreSQL
            ready = False
            for _ in range(30):
                check = subprocess.run(
                    ["docker", "exec", container_mig9, "psql", "-U", "postgres", "-d", db_name, "-c", "SELECT 1"],
                    capture_output=True
                )
                if check.returncode == 0:
                    ready = True
                    break
                time.sleep(1)
            self.assertTrue(ready, "PostgreSQL efêmero mig9 não respondeu a tempo.")

            # Aplica schema_migrations
            init_migrations_sql = """
            CREATE TABLE IF NOT EXISTS schema_migrations (
                version INTEGER PRIMARY KEY,
                name TEXT NOT NULL,
                checksum CHAR(64) NOT NULL,
                applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
            );
            """
            cmd_init = ["docker", "exec", "-i", container_mig9, "psql", "-XAt", "-v", "ON_ERROR_STOP=1", "-U", "postgres", "-d", db_name]
            subprocess.run(cmd_init, input=init_migrations_sql, text=True, check=True, capture_output=True)

            # Aplica APENAS as migrações 1 a 9 (omitindo intencionalmente a versão 10)
            up_files = sorted(list(MIGRATIONS_DIR.glob("*.up.sql")))
            for up_path in up_files[:9]:
                sql_content = up_path.read_text(encoding="utf-8")
                version_str, _, _ = up_path.name.partition("_")
                version_int = int(version_str)
                checksum = hashlib.sha256(sql_content.encode("utf-8")).hexdigest()

                subprocess.run(cmd_init, input=sql_content, text=True, check=True, capture_output=True)
                record_sql = f"INSERT INTO schema_migrations (version, name, checksum) VALUES ({version_int}, '{up_path.name}', '{checksum}');"
                subprocess.run(cmd_init, input=record_sql, text=True, check=True, capture_output=True)

            # Executa o portão técnico de corte
            preflight_script = ROOT / "scripts" / "release" / "preflight_cutover.py"
            cmd_preflight = [
                sys.executable, str(preflight_script),
                "--container", container_mig9,
                "--proof-mode-evaluation-only",
                "--json"
            ]
            proc = subprocess.run(cmd_preflight, capture_output=True, text=True)

            # O portão DEVE sair com exit code 1 (VIOLATIONS_DETECTED) e JAMAIS com exit 2 (FATAL_ERROR)
            self.assertEqual(
                proc.returncode, 1,
                f"Banco migrado até 9 deve resultar em exit code 1 (VIOLATIONS_DETECTED), mas retornou {proc.returncode}.\nSTDOUT:\n{proc.stdout}\nSTDERR:\n{proc.stderr}"
            )

            result = json.loads(proc.stdout)
            self.assertEqual(result["status"], "VIOLATIONS_DETECTED")
            self.assertFalse(result["gate_authorized"])
            self.assertEqual(result["exit_code"], 1)
            self.assertEqual(result["migrations"]["versions_applied"], [1, 2, 3, 4, 5, 6, 7, 8, 9])
            self.assertEqual(result["migrations"]["max_version"], 9)
            self.assertEqual(result["tables_checked_count"], 28)

            violations_str = " ".join(result["violations"])
            # Comprova detecção de migrações incoerentes e tabelas ausentes
            self.assertIn("MIGRACOES_INCOERENTES", violations_str)
            self.assertIn("payout_submissions", violations_str)
            self.assertIn("payout_event_queue", violations_str)

            # Garante que não houve erro fatal de SQL nem de parse de relação
            self.assertNotIn("BLOQUEIO FATAL DE CONSULTA SQL", proc.stderr)
            self.assertNotIn('relation "payout_submissions" does not exist', proc.stderr)
            self.assertNotIn('relation "payout_event_queue" does not exist', proc.stderr)

        finally:
            subprocess.run(["docker", "rm", "-f", container_mig9], capture_output=True)


if __name__ == "__main__":
    unittest.main()
