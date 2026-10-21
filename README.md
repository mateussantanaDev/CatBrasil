# 🚜 CatBrasil - Plataforma ERP & Gamificação Caterpillar

> **Estado atual e bloqueio de corte:** A arquitetura ativa do repositório é baseada em **Go (Golang 1.25+)**, **PostgreSQL 16+** e monorepo frontend em **`frontend/apps`** (Svelte 5 + Node 22). O corte real para produção segue **ESTRITAMENTE BLOQUEADO** até a conclusão dos testes em sandbox oficial SuitPay e homologação completa das evidências P0 assinadas (o sistema **NÃO** está apto para produção sem a liberação formal dos gates). Consulte [FASE5_RELEASE.md](./FASE5_RELEASE.md) para os critérios de bloqueio e liberação de corte, [docker/README.md](./docker/README.md) para infraestrutura Docker Compose e [FASE4_AUTENTICACAO.md](./FASE4_AUTENTICACAO.md) para autenticação. As referências a **Node.js + Express + SQLite** descrevem a stack legada, mantidas apenas para histórico.

CatBrasil é uma aplicação web full-stack de alta performance focada em gestão ERP gamificada e mineração virtual inspirada no universo Caterpillar. O sistema combina interfaces reativas modernas em **Svelte 5 + Vite** com um backend transacional seguro em **Go**, banco relacional **PostgreSQL**, suporte a **WebSockets** em tempo real e integração com pagamentos via **PIX**.

---

## 🛠️ Tecnologias Utilizadas

### **Stack Ativa (Go / PostgreSQL / Frontend Monorepo)**

#### **Backend**
- **Linguagem & Runtime:** Go 1.25+ (conforme `backend/go.mod`)
- **Porta padrão da API:** `8080` (configurável via variável de ambiente `PORT`)
- **Banco de Dados:** PostgreSQL 16+ (driver `pgx/v5`, pooling e transações ACID)
- **Migrações:** Motor nativo Go com controle transacional (`backend/migrations`)
- **Real-time:** WebSockets nativos com hub concorrente
- **Segurança:** Autenticação por token, hash de senhas/segredos (bcrypt), rate limiting por IP e rotas financeiras blindadas

#### **Frontend**
- **Runtime:** Node.js 22+ (conforme Dockerfiles multi-stage em `frontend/Dockerfile.*`)
- **Monorepo:** npm workspaces (`frontend/apps/client`, `frontend/apps/admin`, `frontend/packages/*`)
- **Framework:** Svelte 5 (Runes) + Vite
- **Comunicação:** WebSockets e Fetch API estruturada
- **Estilização:** CSS3 Vanilla com Design System customizado

---

### **Stack Legada (Histórico / Deprecada)**
- **Runtime:** Node.js + Express
- **Banco de Dados:** SQLite (`better-sqlite3`)
- *Nota: Mantida apenas como referência no repositório; não utilizar para deploys ou novos desenvolvimentos.*

---

## ⚙️ Configuração de Ambiente e Pré-requisitos

### **Pré-requisitos**
- [Go](https://golang.org/) 1.25.0 ou superior
- [Node.js](https://nodejs.org/) 22 ou superior e npm
- [Docker](https://www.docker.com/) e Docker Compose
- [Python](https://www.python.org/) 3.11 ou superior (para scripts de release e validação)

### **Variáveis de Ambiente Obrigatórias**
Os comandos de execução local da API e as migrações gerenciadas pelo Compose dependem da preparação prévia de um arquivo `.env` na raiz do projeto. Nenhuma migração ou serviço persistente deve ser iniciado sem configurar as variáveis mínimas:

```bash
# Copiar o exemplo de configuração antes de iniciar serviços
cp .env.example .env
```

Parâmetros essenciais exigidos pelo runtime Go e pelas migrações:
- `DATABASE_URL`: String de conexão PostgreSQL externa sem credenciais default em código (ex: `postgres://<USUARIO>:<SENHA>@<HOST>:<PORTA>/<BANCO>?sslmode=disable`)
- `APP_ENV`: `development`, `staging` ou `production`
- `PORT`: Porta HTTP da API (padrão `8080`)
- `JWT_SECRET`, `ADMIN_SECRET_PIN`, `IDENTITY_SECRET`, `SMS_CODE_SECRET`: Chaves criptográficas do backend

---

## 🚀 Como Executar o Projeto (Stack Ativa)

### **1. Inicialização do Banco Local com Compose**
Após configurar o arquivo `.env`:
```bash
# Inicia container de banco PostgreSQL de desenvolvimento
make db-up

# Aplica migrações pendentes no banco configurado no .env
make migrate
```
> **Atenção:** `make migrate` invoca o serviço `backend-migrate` do Compose e falhará se `DATABASE_URL` ou o arquivo `.env` não estiverem definidos.

### **2. Executar o Backend Go**
```bash
cd backend
go run cmd/api/main.go
```
> O servidor da API iniciará na porta `http://localhost:8080` (ou na definida em `PORT`).

### **3. Executar o Frontend (Monorepo)**
```bash
cd frontend
npm ci

# Executar aplicação Cliente (porta 3000)
npm run dev:client

# Executar aplicação Gestão/Admin (porta 3002)
npm run dev:admin
```

---

## 🧪 Testes e Portão de Integração

O repositório possui portões de validação para unidade, integração real, integridade de containers e arquitetura de release:

### **1. Testes Rápidos / Unitários (Go)**
Executa os testes que não exigem banco relacional ativo:
```bash
make test
# Ou diretamente:
cd backend && go test ./...
```

### **2. Portão de Testes de Integração Reais (PostgreSQL Efêmero Docker)**
O script `scripts/test/integration.sh` é o **gate único de integração** para o backend:
- Sobe um container isolado `postgres:16-alpine` com porta dinâmica (`127.0.0.1:0:5432`) e schema vazio.
- Não toca volumes persistentes, arquivos `.env` locais nem serviços existentes do Compose.
- Executa migrações reais e todos os testes de integração Go (concorrência, webhooks, race detector `-race`).
- Destrói e limpa o container automaticamente ao finalizar, sem exibir segredos ou DSNs:
```bash
./scripts/test/integration.sh
```

### **3. Build do Frontend e Smoke Test de Containers**
Compilação das aplicações Svelte 5:
```bash
cd frontend
npm run build:client
npm run build:admin
# ou build de todos os apps:
npm run build:all
```

Smoke test de saúde dos containers Docker frontend (imagens descartáveis com validação de `/healthz` via HTTP):
```bash
python3 scripts/release/verify_frontend_health.py --build
```

### **4. Validação de Arquitetura e Regras de Release**
Executa testes unitários Python que validam regras de Compose e observabilidade via fixtures sem depender de variáveis de produção:
```bash
python3 -m unittest tests/test_validate_suitpay_sandbox_compose.py
python3 -m unittest tests/test_observability_rules.py
python3 -m unittest tests/test_preflight_cutover.py tests/test_cutover_orchestrator.py
python3 -m unittest tests/test_ephemeral_cutover_proof.py
```

---

## 📂 Estrutura do Projeto

```text
CatBrasil/
├── .github/workflows/  # Pipelines de CI (GitHub Actions)
├── backend/            # API REST (porta 8080), WebSockets, Migrations e domínio em Go 1.25
│   ├── cmd/            # Entrypoints (api, cron, migrate, bootstrap-admin)
│   ├── internal/       # Domínio, regras financeiras, handlers e repositórios
│   ├── migrations/     # Migrações SQL transacionais (000001 a 000014)
│   └── pkg/            # Utilitários de criptografia, gateways e autenticação
├── frontend/           # Monorepo Svelte 5 + Node 22
│   ├── apps/client/    # SPA do operador/cliente (porta 3000)
│   ├── apps/admin/     # SPA administrativo e financeiro (porta 3002)
│   └── packages/       # Pacotes compartilhados
├── docker/             # Configurações de containers e Dockerfiles
├── scripts/            # Scripts de release, validação e portão de testes
│   ├── release/        # Verificadores de health, regras compose e cutover
│   └── test/           # Portão de integração PostgreSQL efêmero (integration.sh)
├── DESIGN_SYSTEM.md    # Especificações do design system da aplicação
└── README.md           # Documentação principal
```

---

## 📄 Licença

Projeto desenvolvido para fins comerciais / proprietários. Todos os direitos reservados.
