#!/usr/bin/env python3
"""
tests/test_backup_restore.py

Testes automatizados e isolados para scripts/release/backup.sh e scripts/release/restore-verify.sh.
Verifica:
1. Rejeição fail-closed quando argumentos obrigatórios ou dependências (age, docker, sha256) estão ausentes.
2. Rejeição fail-closed quando a chave pública/privada age está ausente.
3. Proibição estrita de opções de plaintext (--allow-unencrypted) tanto no backup quanto no restore.
4. Validação e conferência estrita do alvo Docker Compose (projeto, labels, serviço, volume catbrasil_prod_pgdata e nome do banco de dados).
5. Exigência obrigatória de checksum SHA-256 e validação contra divergência.
6. Limpeza completa de artefatos parciais e finais em caso de falha no checksum ou no dump.
7. Invocação segura de psql e pg_isready com -U "$POSTGRES_USER" e -d "$POSTGRES_DB" explícitos.
8. Execução ponta a ponta com mocks/fixtures:
   - Streaming direto para o age sem persistência de plaintext no host.
   - Restauração isolada com streaming direto para pg_restore.
   - Validação da query de schema exigindo todas as 28 tabelas e sequência exata de migrações 1..14 (sem interpolação de min-schema).
   - Destruição segura e limpeza do contêiner descartável efêmero em sucesso ou falha.
"""

import hashlib
import os
from pathlib import Path
import stat
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
BACKUP_SCRIPT = ROOT / "scripts" / "release" / "backup.sh"
RESTORE_SCRIPT = ROOT / "scripts" / "release" / "restore-verify.sh"


class TestBackupRestoreScripts(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.dir_path = Path(self.temp_dir.name)
        self.bin_dir = self.dir_path / "bin"
        self.bin_dir.mkdir()

        # Cria binários mock default
        self._create_mock_binary("age", """#!/usr/bin/env bash
input_file=""
for arg in "$@"; do
  if [[ -f "$arg" ]]; then
    input_file="$arg"
  fi
done

if [[ "$*" == *"-e"* ]]; then
  echo "age-encryption.org/v1"
  if [[ -n "$input_file" ]]; then
    cat "$input_file"
  else
    cat
  fi
elif [[ "$*" == *"-d"* ]]; then
  if [[ -n "$input_file" ]]; then
    sed '1{/^age-encryption.org/d;}' "$input_file"
  else
    sed '1{/^age-encryption.org/d;}'
  fi
fi
""")

    def tearDown(self):
        self.temp_dir.cleanup()

    def _create_mock_binary(self, name: str, content: str):
        path = self.bin_dir / name
        with open(path, "w", encoding="utf-8") as f:
            f.write(content)
        path.chmod(path.stat().st_mode | stat.S_IEXEC | stat.S_IXGRP | stat.S_IXOTH)
        return path

    def _run_script(self, script_path, args, env=None, stdin=None):
        base_env = os.environ.copy()
        # Coloca bin_dir no início do PATH para interceptar ferramentas se desejado
        base_env["PATH"] = f"{self.bin_dir}:{base_env.get('PATH', '')}"
        if env:
            base_env.update(env)
        return subprocess.run(
            [str(script_path)] + args,
            cwd=str(ROOT),
            env=base_env,
            input=stdin,
            text=True,
            capture_output=True,
            timeout=15
        )

    # -------------------------------------------------------------------------
    # Testes do backup.sh
    # -------------------------------------------------------------------------

    def test_backup_missing_arguments(self):
        res = self._run_script(BACKUP_SCRIPT, [])
        self.assertEqual(res.returncode, 2)
        self.assertIn("Caminho de saída do backup é obrigatório", res.stderr)

    def test_backup_missing_project_blocks(self):
        dummy_env = self.dir_path / "dummy.env"
        dummy_env.write_text("DB_NAME=catbrasil_prod\n")
        target = self.dir_path / "out.dump.age"
        # Rejeita mesmo que COMPOSE_PROJECT_NAME esteja no ambiente
        res = self._run_script(
            BACKUP_SCRIPT,
            ["-e", str(dummy_env), str(target)],
            env={"COMPOSE_PROJECT_NAME": "catbrasil-prod"}
        )
        self.assertEqual(res.returncode, 2)
        self.assertIn("Projeto Compose deve ser informado explicitamente via -p ou --project", res.stderr)

    def test_backup_missing_env_file_option_blocks(self):
        target = self.dir_path / "out.dump.age"
        # Rejeita mesmo que COMPOSE_ENV_FILES esteja no ambiente
        res = self._run_script(
            BACKUP_SCRIPT,
            ["-p", "catbrasil-prod", str(target)],
            env={"COMPOSE_ENV_FILES": ".prod.env"}
        )
        self.assertEqual(res.returncode, 2)
        self.assertIn("Arquivo de ambiente deve ser informado explicitamente via -e ou --env-file", res.stderr)

    def test_backup_compose_file_defaults_by_project(self):
        dummy_env = self.dir_path / "dummy.env"
        dummy_env.write_text("DB_NAME=catbrasil_prod\n")
        log_file = self.dir_path / "docker_cf_calls.log"

        self._create_mock_binary("docker", f"""#!/usr/bin/env bash
echo "$*" >> "{log_file}"
if [[ "$*" == *"compose"*"ps -q"* ]]; then
  echo "container_123"
  exit 0
elif [[ "$*" == *"inspect"* ]]; then
  if [[ "$*" == *"com.docker.compose.project"* ]]; then
    echo "catbrasil-prod"
  elif [[ "$*" == *"com.docker.compose.service"* ]]; then
    echo "postgres"
  elif [[ "$*" == *"Mounts"* ]]; then
    echo "volume:catbrasil_prod_pgdata"
  fi
  exit 0
elif [[ "$*" == *"pg_isready"* ]]; then
  exit 0
elif [[ "$*" == *"current_database()"* ]]; then
  echo "catbrasil_prod"
  exit 0
elif [[ "$*" == *"exec"*"pg_dump"* ]]; then
  echo "PAYLOAD"
  exit 0
fi
exit 0
""")
        # 1. Default para catbrasil-prod
        target_prod = self.dir_path / "prod.dump.age"
        res_prod = self._run_script(
            BACKUP_SCRIPT,
            ["-p", "catbrasil-prod", "-e", str(dummy_env), "-r", "age1test", str(target_prod)]
        )
        self.assertEqual(res_prod.returncode, 0, f"Stderr: {res_prod.stderr}")
        log_text = log_file.read_text()
        self.assertIn("-f docker-compose.yml -f compose.prod.yml", log_text)

        # 2. Quando informado -f explícito, não injeta o default
        log_file.unlink()
        custom_compose = self.dir_path / "custom.yml"
        custom_compose.write_text("services: {}\n")
        target_custom = self.dir_path / "custom.dump.age"
        res_custom = self._run_script(
            BACKUP_SCRIPT,
            ["-p", "catbrasil-prod", "-e", str(dummy_env), "-f", str(custom_compose), "-r", "age1test", str(target_custom)]
        )
        self.assertEqual(res_custom.returncode, 0, f"Stderr: {res_custom.stderr}")
        log_text_custom = log_file.read_text()
        self.assertIn(f"-f {custom_compose}", log_text_custom)
        self.assertNotIn("-f compose.prod.yml", log_text_custom)

    def test_backup_destination_already_exists(self):
        dummy_env = self.dir_path / "dummy.env"
        dummy_env.write_text("DB_NAME=catbrasil_prod\n")
        target = self.dir_path / "existing.dump.age"
        target.write_text("data")
        res = self._run_script(BACKUP_SCRIPT, ["-p", "catbrasil-prod", "-e", str(dummy_env), str(target)])
        self.assertEqual(res.returncode, 2)
        self.assertIn("Arquivo de destino já existe", res.stderr)

    def test_backup_destination_sha256_already_exists(self):
        dummy_env = self.dir_path / "dummy.env"
        dummy_env.write_text("DB_NAME=catbrasil_prod\n")
        target = self.dir_path / "file.dump.age"
        sha_file = self.dir_path / "file.dump.age.sha256"
        sha_file.write_text("hash")
        res = self._run_script(BACKUP_SCRIPT, ["-p", "catbrasil-prod", "-e", str(dummy_env), str(target)])
        self.assertEqual(res.returncode, 2)
        self.assertIn("Arquivo de checksum de destino já existe", res.stderr)

    def test_backup_missing_age_extension(self):
        dummy_env = self.dir_path / "dummy.env"
        dummy_env.write_text("DB_NAME=catbrasil_prod\n")
        target = self.dir_path / "plaintext.dump"
        res = self._run_script(BACKUP_SCRIPT, ["-p", "catbrasil-prod", "-e", str(dummy_env), str(target)])
        self.assertEqual(res.returncode, 2)
        self.assertIn("Destino de backup cifrado deve ter extensão .age", res.stderr)

    def test_backup_missing_age_dependency_blocks(self):
        dummy_env = self.dir_path / "dummy.env"
        dummy_env.write_text("DB_NAME=catbrasil_prod\n")
        # Remove age do bin_dir e do PATH simulado
        age_mock = self.bin_dir / "age"
        if age_mock.exists():
            age_mock.unlink()
        
        target = self.dir_path / "out.dump.age"
        res = self._run_script(
            BACKUP_SCRIPT,
            ["-p", "catbrasil-prod", "-e", str(dummy_env), "--recipient", "age1dummy", str(target)],
            env={"PATH": "/usr/bin:/bin"}
        )
        self.assertEqual(res.returncode, 2)
        self.assertIn("Dependência obrigatória 'age' não encontrada", res.stderr)

    def test_backup_missing_recipient_blocks(self):
        dummy_env = self.dir_path / "dummy.env"
        dummy_env.write_text("DB_NAME=catbrasil_prod\n")
        target = self.dir_path / "out.dump.age"
        res = self._run_script(
            BACKUP_SCRIPT,
            ["-p", "catbrasil-prod", "-e", str(dummy_env), str(target)],
            env={"AGE_RECIPIENT": "", "BACKUP_AGE_RECIPIENT": "", "AGE_RECIPIENT_FILE": ""}
        )
        self.assertEqual(res.returncode, 2)
        self.assertIn("Chave pública de cifragem age ausente", res.stderr)

    def test_backup_no_plaintext_option_rejected(self):
        dummy_env = self.dir_path / "dummy.env"
        dummy_env.write_text("DB_NAME=catbrasil_prod\n")
        target = self.dir_path / "out.dump.age"
        # Garante que --allow-unencrypted não existe e é rejeitado em prod e staging
        for proj in ["catbrasil-prod", "catbrasil-staging"]:
            res = self._run_script(
                BACKUP_SCRIPT,
                ["--allow-unencrypted", "--project", proj, "-e", str(dummy_env), str(target)]
            )
            self.assertEqual(res.returncode, 2)
            self.assertIn("Opção desconhecida: --allow-unencrypted", res.stderr)

    def test_backup_missing_env_file(self):
        target = self.dir_path / "out.dump.age"
        res = self._run_script(
            BACKUP_SCRIPT,
            ["-p", "catbrasil-prod", "-r", "age1test", "-e", str(self.dir_path / "missing.env"), str(target)]
        )
        self.assertEqual(res.returncode, 2)
        self.assertIn("Arquivo de ambiente", res.stderr)
        self.assertIn("não encontrado", res.stderr)

    def test_backup_target_container_not_running(self):
        dummy_env = self.dir_path / "dummy.env"
        dummy_env.write_text("DB_NAME=catbrasil_prod\n")
        self._create_mock_binary("docker", """#!/usr/bin/env bash
if [[ "$*" == *"compose"*"ps -q"* ]]; then
  exit 0
fi
exit 0
""")
        target = self.dir_path / "out.dump.age"
        res = self._run_script(
            BACKUP_SCRIPT,
            ["-p", "catbrasil-prod", "-r", "age1test", "-e", str(dummy_env), str(target)]
        )
        self.assertEqual(res.returncode, 2)
        self.assertIn("Serviço 'postgres' não encontrado ou inativo no projeto", res.stderr)

    def test_backup_target_container_label_mismatch(self):
        dummy_env = self.dir_path / "dummy.env"
        dummy_env.write_text("DB_NAME=catbrasil_prod\n")
        self._create_mock_binary("docker", """#!/usr/bin/env bash
if [[ "$*" == *"compose"*"ps -q"* ]]; then
  echo "container_123"
  exit 0
elif [[ "$*" == *"inspect"* ]]; then
  echo "wrong-project"
  exit 0
fi
exit 0
""")
        target = self.dir_path / "out.dump.age"
        res = self._run_script(
            BACKUP_SCRIPT,
            ["-p", "catbrasil-prod", "-r", "age1test", "-e", str(dummy_env), str(target)]
        )
        self.assertEqual(res.returncode, 2)
        self.assertIn("divergindo do projeto esperado", res.stderr)

    def test_backup_target_volume_placement_rejected_wrong_volume_name(self):
        dummy_env = self.dir_path / "dummy.env"
        dummy_env.write_text("DB_NAME=catbrasil_prod\n")
        self._create_mock_binary("docker", """#!/usr/bin/env bash
if [[ "$*" == *"compose"*"ps -q"* ]]; then
  echo "container_123"
  exit 0
elif [[ "$*" == *"inspect"* ]]; then
  if [[ "$*" == *"com.docker.compose.project"* ]]; then
    echo "catbrasil-prod"
  elif [[ "$*" == *"com.docker.compose.service"* ]]; then
    echo "postgres"
  elif [[ "$*" == *"Mounts"* ]]; then
    # Retorna volume incorreto em /var/lib/postgresql/data
    echo "volume:catbrasil_pgdata"
  fi
  exit 0
fi
exit 0
""")
        target = self.dir_path / "out.dump.age"
        res = self._run_script(
            BACKUP_SCRIPT,
            ["-p", "catbrasil-prod", "-r", "age1test", "-e", str(dummy_env), str(target)]
        )
        self.assertEqual(res.returncode, 2)
        self.assertIn("deve montar estritamente o volume 'catbrasil_prod_pgdata' (Type=volume) em Destination=/var/lib/postgresql/data", res.stderr)

    def test_backup_target_volume_placement_rejected_bind_mount(self):
        dummy_env = self.dir_path / "dummy.env"
        dummy_env.write_text("DB_NAME=catbrasil_prod\n")
        self._create_mock_binary("docker", """#!/usr/bin/env bash
if [[ "$*" == *"compose"*"ps -q"* ]]; then
  echo "container_123"
  exit 0
elif [[ "$*" == *"inspect"* ]]; then
  if [[ "$*" == *"com.docker.compose.project"* ]]; then
    echo "catbrasil-prod"
  elif [[ "$*" == *"com.docker.compose.service"* ]]; then
    echo "postgres"
  elif [[ "$*" == *"Mounts"* ]]; then
    # Retorna bind mount em /var/lib/postgresql/data
    echo "bind:/host/pgdata"
  fi
  exit 0
fi
exit 0
""")
        target = self.dir_path / "out.dump.age"
        res = self._run_script(
            BACKUP_SCRIPT,
            ["-p", "catbrasil-prod", "-r", "age1test", "-e", str(dummy_env), str(target)]
        )
        self.assertEqual(res.returncode, 2)
        self.assertIn("deve montar estritamente o volume 'catbrasil_prod_pgdata' (Type=volume) em Destination=/var/lib/postgresql/data", res.stderr)

    def test_backup_target_volume_placement_rejected_wrong_destination(self):
        dummy_env = self.dir_path / "dummy.env"
        dummy_env.write_text("DB_NAME=catbrasil_prod\n")
        self._create_mock_binary("docker", """#!/usr/bin/env bash
if [[ "$*" == *"compose"*"ps -q"* ]]; then
  echo "container_123"
  exit 0
elif [[ "$*" == *"inspect"* ]]; then
  if [[ "$*" == *"com.docker.compose.project"* ]]; then
    echo "catbrasil-prod"
  elif [[ "$*" == *"com.docker.compose.service"* ]]; then
    echo "postgres"
  elif [[ "$*" == *"Mounts"* ]]; then
    # Retorna vazio para Destination=/var/lib/postgresql/data (montado em outro path)
    echo ""
  fi
  exit 0
fi
exit 0
""")
        target = self.dir_path / "out.dump.age"
        res = self._run_script(
            BACKUP_SCRIPT,
            ["-p", "catbrasil-prod", "-r", "age1test", "-e", str(dummy_env), str(target)]
        )
        self.assertEqual(res.returncode, 2)
        self.assertIn("deve montar estritamente o volume 'catbrasil_prod_pgdata' (Type=volume) em Destination=/var/lib/postgresql/data", res.stderr)

    def test_backup_target_database_mismatch(self):
        dummy_env = self.dir_path / "dummy.env"
        dummy_env.write_text("DB_NAME=catbrasil_prod\n")
        self._create_mock_binary("docker", """#!/usr/bin/env bash
if [[ "$*" == *"compose"*"ps -q"* ]]; then
  echo "container_123"
  exit 0
elif [[ "$*" == *"inspect"* ]]; then
  if [[ "$*" == *"com.docker.compose.project"* ]]; then
    echo "catbrasil-prod"
  elif [[ "$*" == *"com.docker.compose.service"* ]]; then
    echo "postgres"
  elif [[ "$*" == *"Mounts"* ]]; then
    echo "volume:catbrasil_prod_pgdata"
  fi
  exit 0
elif [[ "$*" == *"pg_isready"* ]]; then
  exit 0
elif [[ "$*" == *"current_database()"* ]]; then
  echo "catbrasil_staging" # Errado para catbrasil-prod
  exit 0
fi
exit 0
""")
        target = self.dir_path / "out.dump.age"
        res = self._run_script(
            BACKUP_SCRIPT,
            ["-p", "catbrasil-prod", "-r", "age1test", "-e", str(dummy_env), str(target)]
        )
        self.assertEqual(res.returncode, 2)
        self.assertIn("diverge do banco de produção esperado 'catbrasil_prod'", res.stderr)

    def test_backup_psql_explicit_user_database(self):
        dummy_env = self.dir_path / "dummy.env"
        dummy_env.write_text("DB_NAME=catbrasil_prod\n")
        log_file = self.dir_path / "docker_backup_calls.log"

        self._create_mock_binary("docker", f"""#!/usr/bin/env bash
echo "$*" >> "{log_file}"
if [[ "$*" == *"compose"*"ps -q"* ]]; then
  echo "container_123"
  exit 0
elif [[ "$*" == *"inspect"* ]]; then
  if [[ "$*" == *"com.docker.compose.project"* ]]; then
    echo "catbrasil-prod"
  elif [[ "$*" == *"com.docker.compose.service"* ]]; then
    echo "postgres"
  elif [[ "$*" == *"Mounts"* ]]; then
    echo "volume:catbrasil_prod_pgdata"
  fi
  exit 0
elif [[ "$*" == *"pg_isready"* ]]; then
  exit 0
elif [[ "$*" == *"current_database()"* ]]; then
  echo "catbrasil_prod"
  exit 0
elif [[ "$*" == *"exec"*"pg_dump"* ]]; then
  echo "FAKE_PG_DUMP_PAYLOAD"
  exit 0
fi
exit 0
""")
        target = self.dir_path / "backup.dump.age"
        res = self._run_script(
            BACKUP_SCRIPT,
            ["-p", "catbrasil-prod", "-r", "age1testrecipientkey", "-e", str(dummy_env), str(target)]
        )
        self.assertEqual(res.returncode, 0, f"Stderr: {res.stderr}")

        logged_calls = log_file.read_text()
        # Verifica que psql usa explicitamente POSTGRES_USER e POSTGRES_DB
        self.assertIn('-U "$POSTGRES_USER" -d "$POSTGRES_DB"', logged_calls)
        self.assertIn('SELECT current_database();', logged_calls)
        # Verifica que pg_isready usa explicitamente POSTGRES_USER e POSTGRES_DB
        self.assertIn('pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB"', logged_calls)

    def test_backup_successful_encrypted_streaming(self):
        dummy_env = self.dir_path / "dummy.env"
        dummy_env.write_text("DB_NAME=catbrasil_prod\n")
        
        self._create_mock_binary("docker", """#!/usr/bin/env bash
if [[ "$*" == *"compose"*"ps -q"* ]]; then
  echo "container_123"
  exit 0
elif [[ "$*" == *"inspect"* ]]; then
  if [[ "$*" == *"com.docker.compose.project"* ]]; then
    echo "catbrasil-prod"
  elif [[ "$*" == *"com.docker.compose.service"* ]]; then
    echo "postgres"
  elif [[ "$*" == *"Mounts"* ]]; then
    echo "volume:catbrasil_prod_pgdata"
  fi
  exit 0
elif [[ "$*" == *"pg_isready"* ]]; then
  exit 0
elif [[ "$*" == *"current_database()"* ]]; then
  echo "catbrasil_prod"
  exit 0
elif [[ "$*" == *"exec"*"pg_dump"* ]]; then
  echo "FAKE_PG_DUMP_PAYLOAD_TEST_DATA_123456"
  exit 0
fi
exit 0
""")
        target = self.dir_path / "backup.dump.age"
        res = self._run_script(
            BACKUP_SCRIPT,
            ["-p", "catbrasil-prod", "-r", "age1testrecipientkey", "-e", str(dummy_env), str(target)]
        )
        self.assertEqual(res.returncode, 0, f"Stderr: {res.stderr}")
        self.assertTrue(target.exists())
        self.assertTrue(target.with_suffix(".age.sha256").exists())
        
        # Confere que o conteúdo foi processado pelo age mock
        content = target.read_text()
        self.assertTrue(content.startswith("age-encryption.org/v1\n"))
        self.assertIn("FAKE_PG_DUMP_PAYLOAD_TEST_DATA_123456", content)

        # Confere permissões 0600
        mode = target.stat().st_mode & 0o777
        self.assertEqual(mode, 0o600)
        sha_mode = target.with_suffix(".age.sha256").stat().st_mode & 0o777
        self.assertEqual(sha_mode, 0o600)

        # Confere checksum
        with open(target, "rb") as f:
            expected_hash = hashlib.sha256(f.read()).hexdigest()
        sha_content = target.with_suffix(".age.sha256").read_text()
        self.assertIn(expected_hash, sha_content)

    def test_backup_checksum_failure_cleans_output(self):
        dummy_env = self.dir_path / "dummy.env"
        dummy_env.write_text("DB_NAME=catbrasil_prod\n")

        self._create_mock_binary("docker", """#!/usr/bin/env bash
if [[ "$*" == *"compose"*"ps -q"* ]]; then
  echo "container_123"
  exit 0
elif [[ "$*" == *"inspect"* ]]; then
  if [[ "$*" == *"com.docker.compose.project"* ]]; then
    echo "catbrasil-prod"
  elif [[ "$*" == *"com.docker.compose.service"* ]]; then
    echo "postgres"
  elif [[ "$*" == *"Mounts"* ]]; then
    echo "volume:catbrasil_prod_pgdata"
  fi
  exit 0
elif [[ "$*" == *"pg_isready"* ]]; then
  exit 0
elif [[ "$*" == *"current_database()"* ]]; then
  echo "catbrasil_prod"
  exit 0
elif [[ "$*" == *"exec"*"pg_dump"* ]]; then
  echo "VALID_DATA_DUMP"
  exit 0
fi
exit 0
""")
        # Mock do utilitário sha256sum para falhar no teste de verificação pré-publicação
        self._create_mock_binary("sha256sum", """#!/usr/bin/env bash
if [[ "$*" == *"-c"* ]]; then
  echo "CHECKSUM_VERIFY_FAILED" >&2
  exit 1
fi
# Se for cálculo normal de hash, retorna hash válido
echo "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef  $1"
exit 0
""")

        target = self.dir_path / "corrupted_checksum.dump.age"
        res = self._run_script(
            BACKUP_SCRIPT,
            ["-p", "catbrasil-prod", "-r", "age1testrecipientkey", "-e", str(dummy_env), str(target)]
        )
        self.assertNotEqual(res.returncode, 0)
        self.assertIn("Falha na conferência de integridade do checksum SHA-256", res.stderr)

        # Confere que nem o arquivo final nem o .sha256 foram publicados
        self.assertFalse(target.exists())
        self.assertFalse(target.with_suffix(".age.sha256").exists())

        # Confere que nenhum arquivo .partial ficou no diretório
        partials = list(self.dir_path.glob("*.partial.*"))
        self.assertEqual(len(partials), 0)

    def test_backup_dump_failure_cleans_up_partial_file(self):
        dummy_env = self.dir_path / "dummy.env"
        dummy_env.write_text("DB_NAME=catbrasil_prod\n")

        self._create_mock_binary("docker", """#!/usr/bin/env bash
if [[ "$*" == *"compose"*"ps -q"* ]]; then
  echo "container_123"
  exit 0
elif [[ "$*" == *"inspect"* ]]; then
  if [[ "$*" == *"com.docker.compose.project"* ]]; then
    echo "catbrasil-prod"
  elif [[ "$*" == *"com.docker.compose.service"* ]]; then
    echo "postgres"
  elif [[ "$*" == *"Mounts"* ]]; then
    echo "volume:catbrasil_prod_pgdata"
  fi
  exit 0
elif [[ "$*" == *"pg_isready"* ]]; then
  exit 0
elif [[ "$*" == *"current_database()"* ]]; then
  echo "catbrasil_prod"
  exit 0
elif [[ "$*" == *"exec"*"pg_dump"* ]]; then
  echo "pg_dump: error: connection to database failed" >&2
  exit 1
fi
exit 0
""")
        target = self.dir_path / "failed.dump.age"
        res = self._run_script(
            BACKUP_SCRIPT,
            ["-p", "catbrasil-prod", "-r", "age1test", "-e", str(dummy_env), str(target)]
        )
        self.assertNotEqual(res.returncode, 0)
        # Confere que o arquivo final NÃO foi criado
        self.assertFalse(target.exists())
        # Confere que nenhum arquivo .partial ficou no diretório
        partials = list(self.dir_path.glob("*.partial.*"))
        self.assertEqual(len(partials), 0)

    def test_backup_trap_cleans_final_and_checksum_on_mv_failure(self):
        dummy_env = self.dir_path / "dummy.env"
        dummy_env.write_text("DB_NAME=catbrasil_prod\n")

        self._create_mock_binary("docker", """#!/usr/bin/env bash
if [[ "$*" == *"compose"*"ps -q"* ]]; then
  echo "container_123"
  exit 0
elif [[ "$*" == *"inspect"* ]]; then
  if [[ "$*" == *"com.docker.compose.project"* ]]; then
    echo "catbrasil-prod"
  elif [[ "$*" == *"com.docker.compose.service"* ]]; then
    echo "postgres"
  elif [[ "$*" == *"Mounts"* ]]; then
    echo "volume:catbrasil_prod_pgdata"
  fi
  exit 0
elif [[ "$*" == *"pg_isready"* ]]; then
  exit 0
elif [[ "$*" == *"current_database()"* ]]; then
  echo "catbrasil_prod"
  exit 0
elif [[ "$*" == *"exec"*"pg_dump"* ]]; then
  echo "VALID_DATA_FOR_MV_FAILURE"
  exit 0
fi
exit 0
""")
        # Mock do mv: move o dump com sucesso mas falha intencionalmente ao mover o .sha256
        self._create_mock_binary("mv", """#!/usr/bin/env bash
if [[ "$*" == *".sha256"* ]]; then
  echo "MOCK_MV_SHA_FAILURE" >&2
  exit 1
fi
/bin/mv "$@"
""")
        target = self.dir_path / "mv_fail.dump.age"
        res = self._run_script(
            BACKUP_SCRIPT,
            ["-p", "catbrasil-prod", "-r", "age1test", "-e", str(dummy_env), str(target)]
        )
        self.assertNotEqual(res.returncode, 0)
        # O trap deve remover o target final que tinha sido movido pelo primeiro mv
        self.assertFalse(target.exists(), "O artefato final não deve ser deixado órfão quando mv falha")
        self.assertFalse(target.with_suffix(".age.sha256").exists())
        partials = list(self.dir_path.glob("*.partial.*"))
        self.assertEqual(len(partials), 0)

    def test_backup_trap_cleans_final_and_checksum_on_post_validation_failure(self):
        dummy_env = self.dir_path / "dummy.env"
        dummy_env.write_text("DB_NAME=catbrasil_prod\n")

        self._create_mock_binary("docker", """#!/usr/bin/env bash
if [[ "$*" == *"compose"*"ps -q"* ]]; then
  echo "container_123"
  exit 0
elif [[ "$*" == *"inspect"* ]]; then
  if [[ "$*" == *"com.docker.compose.project"* ]]; then
    echo "catbrasil-prod"
  elif [[ "$*" == *"com.docker.compose.service"* ]]; then
    echo "postgres"
  elif [[ "$*" == *"Mounts"* ]]; then
    echo "volume:catbrasil_prod_pgdata"
  fi
  exit 0
elif [[ "$*" == *"pg_isready"* ]]; then
  exit 0
elif [[ "$*" == *"current_database()"* ]]; then
  echo "catbrasil_prod"
  exit 0
elif [[ "$*" == *"exec"*"pg_dump"* ]]; then
  echo "VALID_DATA_FOR_POST_CHECK_FAILURE"
  exit 0
fi
exit 0
""")
        # Mock sha256sum: passa na pré-validação (em stdin com '-') mas falha na pós-validação (com filename)
        self._create_mock_binary("sha256sum", """#!/usr/bin/env bash
if [[ "$*" == *"-c -"* ]]; then
  # Pré-validação com pipe stdin
  exit 0
elif [[ "$*" == *"-c"* ]]; then
  # Pós-validação com nome de arquivo
  echo "MOCK_POST_SHA_FAILED" >&2
  exit 1
fi
echo "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef  $1"
exit 0
""")
        target = self.dir_path / "post_sha_fail.dump.age"
        res = self._run_script(
            BACKUP_SCRIPT,
            ["-p", "catbrasil-prod", "-r", "age1test", "-e", str(dummy_env), str(target)]
        )
        self.assertNotEqual(res.returncode, 0)
        self.assertIn("Validação pós-publicação do checksum SHA-256 falhou", res.stderr)
        # Trap deve ter removido ambos os arquivos finais publicados
        self.assertFalse(target.exists(), "O artefato final deve ser limpo pelo trap em falha de pós-validação")
        self.assertFalse(target.with_suffix(".age.sha256").exists(), "O arquivo sha256 deve ser limpo pelo trap")
        partials = list(self.dir_path.glob("*.partial.*"))
        self.assertEqual(len(partials), 0)

    def test_backup_trap_cleans_final_and_checksum_on_chmod_failure(self):
        dummy_env = self.dir_path / "dummy.env"
        dummy_env.write_text("DB_NAME=catbrasil_prod\n")

        self._create_mock_binary("docker", """#!/usr/bin/env bash
if [[ "$*" == *"compose"*"ps -q"* ]]; then
  echo "container_123"
  exit 0
elif [[ "$*" == *"inspect"* ]]; then
  if [[ "$*" == *"com.docker.compose.project"* ]]; then
    echo "catbrasil-prod"
  elif [[ "$*" == *"com.docker.compose.service"* ]]; then
    echo "postgres"
  elif [[ "$*" == *"Mounts"* ]]; then
    echo "volume:catbrasil_prod_pgdata"
  fi
  exit 0
elif [[ "$*" == *"pg_isready"* ]]; then
  exit 0
elif [[ "$*" == *"current_database()"* ]]; then
  echo "catbrasil_prod"
  exit 0
elif [[ "$*" == *"exec"*"pg_dump"* ]]; then
  echo "VALID_DATA_FOR_CHMOD_FAILURE"
  exit 0
fi
exit 0
""")
        # Mock do chmod: falha intencionalmente ao aplicar 600
        self._create_mock_binary("chmod", """#!/usr/bin/env bash
if [[ "$*" == *"600"* ]]; then
  echo "MOCK_CHMOD_PERMISSION_FAILURE" >&2
  exit 1
fi
/bin/chmod "$@"
""")
        target = self.dir_path / "chmod_fail.dump.age"
        res = self._run_script(
            BACKUP_SCRIPT,
            ["-p", "catbrasil-prod", "-r", "age1test", "-e", str(dummy_env), str(target)]
        )
        self.assertNotEqual(res.returncode, 0)
        # O trap deve remover os artefatos finais publicados
        self.assertFalse(target.exists(), "O artefato final deve ser limpo pelo trap se chmod falhar")
        self.assertFalse(target.with_suffix(".age.sha256").exists())
        partials = list(self.dir_path.glob("*.partial.*"))
        self.assertEqual(len(partials), 0)

    def test_backup_trap_cleans_final_and_checksum_on_interrupt(self):
        dummy_env = self.dir_path / "dummy.env"
        dummy_env.write_text("DB_NAME=catbrasil_prod\n")

        self._create_mock_binary("docker", """#!/usr/bin/env bash
if [[ "$*" == *"compose"*"ps -q"* ]]; then
  echo "container_123"
  exit 0
elif [[ "$*" == *"inspect"* ]]; then
  if [[ "$*" == *"com.docker.compose.project"* ]]; then
    echo "catbrasil-prod"
  elif [[ "$*" == *"com.docker.compose.service"* ]]; then
    echo "postgres"
  elif [[ "$*" == *"Mounts"* ]]; then
    echo "volume:catbrasil_prod_pgdata"
  fi
  exit 0
elif [[ "$*" == *"pg_isready"* ]]; then
  exit 0
elif [[ "$*" == *"current_database()"* ]]; then
  echo "catbrasil_prod"
  exit 0
elif [[ "$*" == *"exec"*"pg_dump"* ]]; then
  echo "VALID_DATA_FOR_INTERRUPT"
  exit 0
fi
exit 0
""")
        # Mock do chmod: simula sinal SIGINT ou SIGTERM disparado antes do script desarmar o trap
        self._create_mock_binary("chmod", """#!/usr/bin/env bash
if [[ "$*" == *"600"* ]]; then
  kill -INT "$PPID"
  exit 1
fi
/bin/chmod "$@"
""")
        target = self.dir_path / "interrupt.dump.age"
        res = self._run_script(
            BACKUP_SCRIPT,
            ["-p", "catbrasil-prod", "-r", "age1test", "-e", str(dummy_env), str(target)]
        )
        self.assertNotEqual(res.returncode, 0)
        # O trap deve remover os artefatos finais publicados mesmo sob sinal de interrupção
        self.assertFalse(target.exists(), "O artefato final deve ser limpo pelo trap sob interrupção")
        self.assertFalse(target.with_suffix(".age.sha256").exists())
        partials = list(self.dir_path.glob("*.partial.*"))
        self.assertEqual(len(partials), 0)

    # -------------------------------------------------------------------------
    # Testes do restore-verify.sh
    # -------------------------------------------------------------------------

    def test_restore_missing_arguments(self):
        res = self._run_script(RESTORE_SCRIPT, [])
        self.assertEqual(res.returncode, 2)
        self.assertIn("Caminho do arquivo de backup não informado", res.stderr)

    def test_restore_missing_archive_file(self):
        res = self._run_script(RESTORE_SCRIPT, [str(self.dir_path / "nonexistent.dump.age")])
        self.assertEqual(res.returncode, 2)
        self.assertIn("Arquivo de backup não encontrado", res.stderr)

    def test_restore_missing_checksum_file(self):
        archive = self.dir_path / "test.dump.age"
        archive.write_text("data")
        res = self._run_script(RESTORE_SCRIPT, [str(archive)])
        self.assertEqual(res.returncode, 2)
        self.assertIn("Checksum obrigatório não encontrado", res.stderr)

    def test_restore_corrupted_checksum(self):
        archive = self.dir_path / "test.dump.age"
        archive.write_text("valid data")
        sha = archive.with_suffix(".age.sha256")
        sha.write_text("0000000000000000000000000000000000000000000000000000000000000000  test.dump.age\n")
        res = self._run_script(RESTORE_SCRIPT, [str(archive)])
        self.assertEqual(res.returncode, 2)
        self.assertTrue("diverge" in res.stderr or "FAILED" in res.stdout or "FAILED" in res.stderr)

    def test_restore_missing_age_dependency_blocks(self):
        archive = self.dir_path / "test.dump.age"
        archive.write_text("dummy")
        sha = archive.with_suffix(".age.sha256")
        with open(archive, "rb") as f:
            h = hashlib.sha256(f.read()).hexdigest()
        sha.write_text(f"{h}  {archive.name}\n")

        # Sem age no PATH
        age_mock = self.bin_dir / "age"
        if age_mock.exists():
            age_mock.unlink()

        res = self._run_script(
            RESTORE_SCRIPT,
            ["-i", "dummy.key", str(archive)],
            env={"PATH": "/usr/bin:/bin"}
        )
        self.assertEqual(res.returncode, 2)
        self.assertIn("Ferramenta 'age' não encontrada", res.stderr)

    def test_restore_missing_identity_file_blocks(self):
        archive = self.dir_path / "test.dump.age"
        archive.write_text("dummy")
        sha = archive.with_suffix(".age.sha256")
        with open(archive, "rb") as f:
            h = hashlib.sha256(f.read()).hexdigest()
        sha.write_text(f"{h}  {archive.name}\n")

        self._create_mock_binary("docker", "exit 0\n")

        res = self._run_script(
            RESTORE_SCRIPT,
            [str(archive)],
            env={"AGE_IDENTITY_FILE": "", "RESTORE_AGE_IDENTITY_FILE": ""}
        )
        self.assertEqual(res.returncode, 2)
        self.assertIn("Arquivo de identidade age (chave privada) obrigatório", res.stderr)

    def test_restore_no_plaintext_option_rejected(self):
        archive = self.dir_path / "test.dump.age"
        archive.write_text("dummy")
        sha = archive.with_suffix(".age.sha256")
        with open(archive, "rb") as f:
            h = hashlib.sha256(f.read()).hexdigest()
        sha.write_text(f"{h}  {archive.name}\n")

        res = self._run_script(RESTORE_SCRIPT, ["--allow-unencrypted", str(archive)])
        self.assertEqual(res.returncode, 2)
        self.assertIn("Opção desconhecida: --allow-unencrypted", res.stderr)

    def test_restore_no_min_schema_version_option(self):
        archive = self.dir_path / "test.dump.age"
        archive.write_text("dummy")
        sha = archive.with_suffix(".age.sha256")
        with open(archive, "rb") as f:
            h = hashlib.sha256(f.read()).hexdigest()
        sha.write_text(f"{h}  {archive.name}\n")

        res = self._run_script(RESTORE_SCRIPT, ["--min-schema-version", "10", str(archive)])
        self.assertEqual(res.returncode, 2)
        self.assertIn("Opção desconhecida: --min-schema-version", res.stderr)

    def test_restore_unencrypted_archive_rejected(self):
        archive = self.dir_path / "legacy.dump"
        archive.write_text("plaintext pg_dump")
        sha = archive.with_suffix(".dump.sha256")
        with open(archive, "rb") as f:
            h = hashlib.sha256(f.read()).hexdigest()
        sha.write_text(f"{h}  {archive.name}\n")

        self._create_mock_binary("docker", "exit 0\n")

        res = self._run_script(RESTORE_SCRIPT, [str(archive)])
        self.assertEqual(res.returncode, 2)
        self.assertIn("não está cifrado com age", res.stderr)
        self.assertIn("Backups sem cifragem são estritamente proibidos", res.stderr)

    def test_restore_successful_mocked_execution_and_schema_validation(self):
        archive = self.dir_path / "backup.dump.age"
        archive.write_text("age-encryption.org/v1\nFAKE_DUMP_STREAM")
        sha = archive.with_suffix(".age.sha256")
        with open(archive, "rb") as f:
            h = hashlib.sha256(f.read()).hexdigest()
        sha.write_text(f"{h}  {archive.name}\n")

        key_file = self.dir_path / "id.key"
        key_file.write_text("AGE-SECRET-KEY-1...")

        log_file = self.dir_path / "docker_calls.log"

        self._create_mock_binary("docker", f"""#!/usr/bin/env bash
echo "$*" >> "{log_file}"
if [[ "$*" == *"run --rm --network none"* ]]; then
  exit 0
elif [[ "$*" == *"SELECT 1"* ]]; then
  echo "1"
  exit 0
elif [[ "$*" == *"pg_restore"* ]]; then
  # Confere que recebeu stdin decifrado
  read -r first_line
  echo "RESTORE_STDIN: $first_line" >> "{log_file}"
  exit 0
elif [[ "$*" == *"psql"* ]]; then
  # Captura o script SQL enviado
  cat >> "{log_file}.sql"
  exit 0
elif [[ "$*" == *"rm -f"* ]]; then
  exit 0
fi
exit 0
""")

        res = self._run_script(
            RESTORE_SCRIPT,
            ["-i", str(key_file), str(archive)]
        )
        self.assertEqual(res.returncode, 0, f"Stderr: {res.stderr}\nStdout: {res.stdout}")
        self.assertIn("RESTAURAÇÃO ISOLADA VALIDADA COM SUCESSO", res.stdout)
        self.assertIn("28 tabelas e sequência de migrações 1..14 confirmadas", res.stdout)

        # Confere que o streaming direto ocorreu sem header age no pg_restore
        calls_log = log_file.read_text()
        self.assertIn("run --rm --network none", calls_log)
        self.assertIn("RESTORE_STDIN: FAKE_DUMP_STREAM", calls_log)
        self.assertIn("rm -f catbrasil-restore-proof-", calls_log)

        # Confere que as 28 tabelas foram inspecionadas no SQL
        sql_log = Path(f"{log_file}.sql").read_text()
        expected_tables = [
            "schema_migrations", "system_config", "fleet_catalog", "level_configs",
            "roulette_slices", "announcements", "users", "transactions",
            "financial_events", "payout_gateway_events", "sms_verifications",
            "user_identities", "identity_events", "machines", "referrals",
            "user_promos", "battle_pass_season_history", "ranking_season_history",
            "mining_race_weekly", "usage_logs", "security_logs", "deposit_gateway_events",
            "cashin_deposit_intents", "cashin_event_queue", "payout_submissions",
            "payout_event_queue", "http_rate_limits", "cron_executions"
        ]
        self.assertEqual(len(expected_tables), 28)
        for tbl in expected_tables:
            self.assertIn(f"'{tbl}'", sql_log, f"Tabela {tbl} não encontrada na validação de schema")

        # Confere a exigência da sequência exata 1..14 em array
        self.assertIn("ARRAY[1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14]", sql_log)
        self.assertIn("v_applied_versions != v_expected_versions", sql_log)
        self.assertNotIn("MIN_SCHEMA_VERSION", sql_log)

    def test_restore_pg_restore_failure_cleans_up_container(self):
        archive = self.dir_path / "bad.dump.age"
        archive.write_text("age-encryption.org/v1\nCORRUPTED")
        sha = archive.with_suffix(".age.sha256")
        with open(archive, "rb") as f:
            h = hashlib.sha256(f.read()).hexdigest()
        sha.write_text(f"{h}  {archive.name}\n")

        key_file = self.dir_path / "id.key"
        key_file.write_text("AGE-SECRET-KEY-1...")

        log_file = self.dir_path / "docker_fail.log"

        self._create_mock_binary("docker", f"""#!/usr/bin/env bash
echo "$*" >> "{log_file}"
if [[ "$*" == *"run --rm --network none"* ]]; then
  exit 0
elif [[ "$*" == *"SELECT 1"* ]]; then
  echo "1"
  exit 0
elif [[ "$*" == *"pg_restore"* ]]; then
  echo "pg_restore: error: input file appears to be corrupted" >&2
  exit 1
elif [[ "$*" == *"rm -f"* ]]; then
  echo "CONTAINER_CLEANED" >> "{log_file}"
  exit 0
fi
exit 0
""")

        res = self._run_script(
            RESTORE_SCRIPT,
            ["-i", str(key_file), str(archive)]
        )
        self.assertNotEqual(res.returncode, 0)
        self.assertTrue(log_file.exists())
        self.assertIn("CONTAINER_CLEANED", log_file.read_text())


if __name__ == "__main__":
    unittest.main()
