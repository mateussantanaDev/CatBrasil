# 🚜 CATBRASIL ENTERPRISE — MANUAL DE IMPLANTAÇÃO EM PRODUÇÃO & GUIA OPERACIONAL

> **Documento histórico, ainda não aprovado para deploy.** Os exemplos abaixo usam variáveis e pressupostos anteriores à contenção de segurança. Siga o checklist atualizado em [FASE5_RELEASE.md](./FASE5_RELEASE.md). `SUITPAY_LIVE_ENABLED=false` bloqueia novas operações financeiras reais em produção até a validação oficial e a autorização do corte.

Este documento fornece as instruções detalhadas e passos consolidados para implantação em servidores de produção (VPS/Cloud), configuração de domínio, certificados SSL/TLS, credenciais da SuitPay, rotinas de backup e monitoramento contínuo da plataforma **CatBrasil**.

---

## 1. 🏗️ Arquitetura da Solução

A plataforma é composta por microserviços e frontends otimizados conteinerizados em Docker:

| Componente | Porta Interna | Porta Externa (Host) | Descrição |
| :--- | :---: | :---: | :--- |
| **`catbrasil_postgres`** | `5432` | `5432` (Privada) | PostgreSQL 16 com schema relacional atômico e transacional |
| **`catbrasil_api`** | `8080` | `8080` | Backend Go (Chi Router + pgxpool + WebSockets Hub + JWT + SuitPay) |
| **`catbrasil_cron`** | — | — | Cron Worker em Go para reset diário às 00:00 UTC e expiração de rendimentos |
| **`catbrasil_client_app`** | `80` | `3000` | Frontend SPA Svelte 5 para Operadores (Mobile-First Caterpillar Design) |
| **`catbrasil_admin_app`** | `80` | `3002` | Frontend ERP Svelte 5 de Gestão & Diretoria |

---

## 2. 🔐 Variáveis de ambiente

Use `.env.example` como lista de variáveis obrigatórias e defina valores exclusivos fora do repositório. A API exige `APP_ENV`, `PIX_MODE`, `DATABASE_URL`, `JWT_SECRET`, `PIX_WEBHOOK_SECRET`, `ADMIN_SECRET_PIN`, `IDENTITY_SECRET` e, em produção, `CORS_ALLOWED_ORIGINS`. O Compose também exige `DB_PASSWORD` e os domínios/contato do Caddy. Guarde `IDENTITY_SECRET` fora do host e junto ao plano de recuperação: sem ela, dados pessoais restaurados não podem ser lidos. Mantenha `SUITPAY_LIVE_ENABLED=false` até cumprir o checklist de corte. Nunca reutilize os exemplos históricos deste manual como segredos.

A fase 1 bloqueia operações financeiras e autocadastro em produção até as integrações restantes estarem prontas. Consulte `FASE1_CONTENCAO.md` antes de considerar o deploy.

---

## 3. 💳 Configuração do Gateway SuitPay (Cash-In & Cash-Out)

A plataforma opera em **Dual-Mode** (Modo Simulado local ou API ao Vivo da SuitPay).

### 📋 Passo a Passo para Ativação:
1. Acesse o **ERP de Gestão** (`http://seu-dominio:3002` ou `https://admin.seu-dominio.com`).
2. Faça login com o perfil da Diretoria e informe o **PIN de Segurança de 6 dígitos**.
3. Navegue até a aba **`8 · CONFIGURAÇÕES & TRAVAS`**.
4. No painel **`INTEGRAÇÃO GATEWAY SUITPAY (PIX)`**:
   - **Client ID (`ci`):** Insira o ID fornecido no painel da SuitPay.
   - **Client Secret (`cs`):** Insira o Secret Token gerado na SuitPay.
   - **Ambiente Sandbox:** Desmarque para operar em **Produção ao Vivo** (`https://ws.suitpay.app`).
   - **URL de Webhook Callback:** `https://api.seu-dominio.com/api/v1/webhook/suitpay`
   - **Status do Gateway:** Marque como **Ativo / Habilitado**.
5. Clique em **`SALVAR CREDENCIAIS SUITPAY`**.
6. Cadastre a URL de webhook no painel da SuitPay para receber os callbacks automáticos de liquidação `PAID` e `PAID_OUT`.

---

## 4. 🌐 Configuração de Reverse Proxy Nginx & SSL (Let's Encrypt)

Exemplo de configuração para o arquivo `/etc/nginx/sites-available/catbrasil.conf`:

```nginx
# 1. API Backend & WebSockets
server {
    server_name api.seudominio.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

# 2. App Cliente (Operadores)
server {
    server_name seudominio.com app.seudominio.com;

    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}

# 3. ERP Gestão (Diretoria)
server {
    server_name admin.seudominio.com;

    location / {
        proxy_pass http://127.0.0.1:3002;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

Para emissão automática dos certificados SSL gratuitos com Certbot:
```bash
sudo certbot --nginx -d seudominio.com -d app.seudominio.com -d admin.seudominio.com -d api.seudominio.com
```

---

## 5. 🚀 Deploy com Docker Compose em Produção

Em ambiente de produção, nunca execute `docker compose up -d --build` genérico. A subida oficial exige a especificação do arquivo de variáveis de ambiente protegido (fora do repositório) e a composição dos arquivos base e de produção (`compose.prod.yml` com digests de imagem imutáveis e isolamento de rede):

```bash
# 1. Entrar no diretório do repositório
cd /opt/catbrasil

# 2. Subir os serviços de produção com o overlay oficial e env protegido
docker compose --env-file /etc/catbrasil/secrets/.prod.env -p catbrasil-prod \
  -f docker-compose.yml -f compose.prod.yml up -d

# 3. Verificar integridade dos serviços
docker compose --env-file /etc/catbrasil/secrets/.prod.env -p catbrasil-prod \
  -f docker-compose.yml -f compose.prod.yml ps

# 4. Acompanhar logs em tempo real
docker compose --env-file /etc/catbrasil/secrets/.prod.env -p catbrasil-prod \
  -f docker-compose.yml -f compose.prod.yml logs -f backend-api
```

---

## 6. 💾 Rotina de Backup Criptografado e Prova de Restauração

Backups sem cifragem ou sem validação automatizada de restauração são **estritamente proibidos**. A rotina oficial utiliza `scripts/release/backup.sh` (cifragem ponta-a-ponta via `age` sem persistir plaintext em disco e geração de hash SHA-256) e validação em contêiner descartável isolado via `scripts/release/restore-verify.sh`.

### Pré-requisitos no Host de Produção
1. Utilitário `age` instalado (`apt-get install -y age` ou `brew install age`).
2. Par de chaves `age` gerado fora do repositório (ex.: `/etc/catbrasil/keys/`):
   ```bash
   sudo mkdir -p /etc/catbrasil/keys /var/backups/catbrasil
   sudo chmod 700 /etc/catbrasil/keys /var/backups/catbrasil
   age-keygen -o /etc/catbrasil/keys/backup_identity.key
   chmod 600 /etc/catbrasil/keys/backup_identity.key
   age-keygen -y /etc/catbrasil/keys/backup_identity.key > /etc/catbrasil/keys/backup_recipient.pub
   ```

### Script de Execução e Agendamento (`/opt/catbrasil/run_backup_routine.sh`)

Crie o executor do host sem embutir senhas ou segredos no repositório. O script garante diretório de trabalho absoluto e especifica explicitamente os arquivos Compose de produção para execução correta sob qualquer `cwd` do cron:

```bash
#!/usr/bin/env bash
set -euo pipefail

BACKUP_DIR="/var/backups/catbrasil"
REPO_DIR="/opt/catbrasil"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
ARCHIVE="$BACKUP_DIR/catbrasil_prod_$TIMESTAMP.dump.age"
KEY_PUB="/etc/catbrasil/keys/backup_recipient.pub"
KEY_PRIV="/etc/catbrasil/keys/backup_identity.key"
ENV_FILE="/etc/catbrasil/secrets/.prod.env"

# 1. Execução do backup lógico com streaming cifrado age e arquivos Compose explícitos
cd "$REPO_DIR"
"$REPO_DIR/scripts/release/backup.sh" \
  --project catbrasil-prod \
  --env-file "$ENV_FILE" \
  --compose-file "$REPO_DIR/docker-compose.yml" \
  --compose-file "$REPO_DIR/compose.prod.yml" \
  --recipient-file "$KEY_PUB" \
  "$ARCHIVE"

# 2. Prova de restauração e integridade de schema (28 tabelas, migrações 1..14) em contêiner isolado
"$REPO_DIR/scripts/release/restore-verify.sh" \
  --identity "$KEY_PRIV" \
  "$ARCHIVE"

# 3. Retenção: manter os últimos 30 dias de backups cifrados e seus checksums
find "$BACKUP_DIR" -type f \( -name "*.dump.age" -o -name "*.age.sha256" \) -mtime +30 -delete
```

Garanta permissões restritas no script:
```bash
chmod 700 /opt/catbrasil/run_backup_routine.sh
```

### Agendamento no Cron (`crontab -e` do root)
```bash
0 3 * * * /opt/catbrasil/run_backup_routine.sh >> /var/log/catbrasil_backup.log 2>&1
```

---

## 7. 🛡️ Recursos de Segurança Ativos

1. **Kill Switch Global:** Bloqueio emergencial de 100% dos saques via ERP em 1 clique.
2. **Rate Limiting Inteligente:** Proteção contra força bruta em rotas de login (15 req/min) e chamadas gerais (300 req/min).
3. **Trilha de Auditoria Imutável (`audit_logs`):** Gravação permanente no PostgreSQL de todas as alterações feitas pela diretoria.
4. **Proteção Anti-Multi-Contas:** Identificação por Fingerprint e IP com trava automática de bônus de indicação em fraudes de auto-afiliação.
5. **Transações Atômicas:** Bloqueio de linha (`SELECT FOR UPDATE`) para garantir consistência total de saldo em requisições concorrentes.
