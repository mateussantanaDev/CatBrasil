# P0.1 — Inventário, Conciliação e Decisão de Abertura (Greenfield Cutover Gate)

**Data de Auditoria / Deliberação:** 16/10/2026  
**Responsável Técnico:** Implementador Gemini sob revisão técnica Codex  
**Alvo Auditado em Staging:** `catbrasil_staging` (ambiente isolado staging)  
**Decisão de Abertura:** Banco Novo e Limpo (`catbrasil_prod`) com saldo inicial zero e descarte contábil de migração legada  
**Atestado Operacional:** Dados anteriores confirmados como 100% fictícios pelo usuário (sem passivo prévio a honrar)  
**Garantia Técnica:** Portão de corte fail-closed (`preflight_cutover.py`) com prova prática efêmera validada  
**Status P0.1:** **DECISÃO DE ABERTURA CONCLUÍDA** (Produção retida até cumprimento de P0.2 a P0.6)  

---

## 1. Contexto e Escopo

Esta documentação consolida a execução da etapa **P0.1 — Inventário e conciliação de abertura** prevista em `P0_ETAPAS.md`, realizada exclusivamente no banco ativo do ambiente de staging (`catbrasil_staging`).

### 1.1. Propósito e Limite Estrito da Ferramenta
A ferramenta [`scripts/release/reconcile.py`](./scripts/release/reconcile.py) foi projetada para auditar o banco ativo de staging em execução. Ela valida a aderência estrutural do schema relacional, inspeciona índices de integridade, quantifica pendências e afere discrepâncias entre o saldo operacional (`users.balance`) e o diário de eventos de saldo (`financial_events`).
- **A ferramenta NÃO foi projetada para auditar diretamente dumps legados de produção fora do ambiente configurado.**
- Como o banco de staging atual possui zero lançamentos na tabela `financial_events`, **a continuidade do encadeamento contábil permanece NÃO TESTADA neste banco**.
- **Nenhuma aprovação de corte de produção ou recomendação definitiva de arquitetura (como adoção de banco limpo vs migração legada) pode ser derivada unicamente dos dados deste staging.**

---

## 2. Modelo de Segurança e Imutabilidade

A ferramenta opera sob um modelo de segurança em camadas:

### 2.1. Garantia Técnica de Imutabilidade
A proteção contra escritas, alterações de dados ou modificações de schema é garantida no nível do motor relacional:
```sql
BEGIN TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY;
SET TRANSACTION READ ONLY;
-- [Consultas de leitura e agregações]
ROLLBACK;
```
Qualquer tentativa de `INSERT`, `UPDATE`, `DELETE`, `CREATE`, `ALTER` ou `DROP` é abortada com erro fatal pelo PostgreSQL (`ERROR: cannot execute ... in a read-only transaction`).

### 2.2. Verificação Auxiliar de Estabilidade Volumétrica de Linhas
A contagem de linhas de todas as 21 tabelas do schema `public` antes e depois da transação é realizada como **checagem auxiliar de estabilidade volumétrica**.
> **Esclarecimento Técnico Obrigatório:** A igualdade das contagens de linhas antes e depois **NÃO constitui prova de ausência de mutações**. Ela apenas atesta a ausência de variação líquida no volume de registros; não detecta comandos `UPDATE` in-place em colunas existentes nem alterações concorrentes compensadas (ex.: um INSERT e um DELETE simultâneos). A garantia de não-modificação decorre exclusivamente da transação `READ ONLY`.

### 2.3. Validação do Alvo e Contêiner
O script não depende de IDs de contêiner pré-fixados como garantia primária. Em vez disso:
1. Valida se o projeto Compose ativo é `catbrasil-staging`.
2. Valida se o serviço inspecionado é `postgres`.
3. Valida se o arquivo `.staging.env` está presente.
4. Valida se `current_database() == 'catbrasil_staging'`. Caso divirja, a execução é abortada imediatamente.

### 2.4. Higienização, Preservação Numérica e Pseudonimização
- **Pseudonimização:** IDs de usuários, transações e códigos externos associados a pendências ou anomalias não são publicados em texto plano nem em formato parcial. São pseudonimizados deterministicamente por execução usando HMAC-SHA256 com sal efêmero gerado em memória (`secrets.token_hex(16)`), no formato `anon_<tipo>_<hex>`.
- **Exclusão de Dados Pessoais:** O script não descarrega listas completas de usuários ou transações para JSON/Markdown; todas as quantificações e somatórios são agregados diretamente via SQL. Senhas, hashes bcrypt, PINs financeiros, códigos SMS e segredos TOTP/2FA são sumariamente excluídos.
- **Precisão Decimal:** Valores monetários utilizam rigorosamente `parse_float=decimal.Decimal` e são serializados como strings (`DecimalEncoder`) ou formatados via SQL, evitando qualquer conversão ou distorção por ponto flutuante IEEE-754.
- **Permissões de Arquivo:** Relatórios locais são gerados com permissão estrita `0600` (`-rw-------`) em diretório `0700` (`scripts/release/reports/`), incluído no `.gitignore`.

---

## 3. Separação de Evidências: Fatos, Suspeitas e Dependências

A análise contábil separa rigorosamente evidências auditadas no banco de hipóteses operacionais e fatores que dependem do extrato bancário.

### 3.1. Fatos Auditados no Staging (Evidência Relacional Direta)
1. **Ambiente e Banco:** Banco ativo `catbrasil_staging`, transação executada sob `REPEATABLE READ READ ONLY (ROLLBACK)`.
2. **Schema e Migrações:** O histórico `schema_migrations` registra 8 migrações aplicadas. Identificadas 21 tabelas base no schema `public`.
3. **Índices de Integridade:** Presentes e ativos os índices únicos de idempotência (`transactions_user_idempotency`), integridade de gateway (`transactions_gateway_external_id`), unicidade de evento (`financial_events_transaction_id_user_id_kind_key`) e eventos de payout (`payout_gateway_events_pkey`).
4. **Contas Cadastradas:** Registradas 10 contas de usuários no banco.
5. **Saldos:** A soma de `users.balance` é de exatamente `R$ 0,00`. A soma de `yield_balance_available` é de `R$ 0,00`. Todos os 10 usuários possuem saldo `0,00`.
6. **Transações:** Existem 7 transações registradas, todas do tipo `Depósito PIX`, status `pendente`, provedor `suitpay`, valor de `R$ 1,00` e com identificador de gateway preenchido.
7. **Diário Financeiro (`financial_events`):** Existem 0 registros na tabela `financial_events`.
8. **Divergências de Saldo Atuais:** 0 divergências detectadas entre o saldo das contas e o último evento (todas as 10 contas possuem saldo 0,00 e zero eventos).
9. **Saques:** Zero saques pendentes, em processamento ou rejeitados. Zero callbacks órfãos em `payout_gateway_events`.
10. **Estabilidade Volumétrica:** Todas as 21 tabelas apresentaram variação líquida zero de linhas antes e depois da auditoria.

### 3.2. Suspeitas e Hipóteses Operacionais
1. **Origem dos 7 Depósitos Pendentes:** É provável que sejam resultantes das rodadas de testes do script `smoke.py --full-sandbox` executadas durante a validação da Fase 5 (emissão de cobrança no stub `pix-sandbox`).
2. **Ausência de Liquidação:** A ausência de registros em `financial_events` é consistente com o comportamento esperado de depósitos pendentes que nunca receberam callback de pagamento `PAID`, pois a API só credita e lança eventos após callback autenticado.

### 3.3. Papel do Extrato Bancário: Operação Futura vs Abertura Greenfield
1. **Dispensa de Conciliação Retroativa:** O atestado operacional do usuário confirmou formalmente que nenhuma operação oficial pretérita foi realizada e que todos os dados históricos eram puramente fictícios. Portanto, não há saldos passados a honrar, créditos a migrar nem passivo financeiro com operadores anteriores. O saldo inicial de abertura no banco de produção é **estritamente ZERO (R$ 0,00)**.
2. **Verificação Técnica Independente:** A garantia de que o novo ambiente de produção está limpo decorre da prova técnica automatizada (portão de corte fail-closed atestando migrações 1..9 e ausência absoluta de registros), e não de conciliação de extrato pregresso.
3. **Necessidade do Extrato Exclusiva para Operações Futuras:** O extrato oficial e consolidado da conta SuitPay **NÃO é necessário para o saldo de abertura zero do greenfield**. Ele será indispensável apenas nas etapas subsequentes (P0.2 a P0.6) para auditar a liquidação efetiva e conciliar novas transações reais que venham a ocorrer após a homologação da plataforma.

---

## 4. Cobertura e Continuidade de `financial_events`

A tabela `financial_events` funciona como um **diário linear de eventos de saldo** (log append-only auditável), registrando atomicamente `balance_before`, `amount` e `balance_after` sob a restrição de verificação `CHECK (balance_after = balance_before + amount)`.

> **Esclarecimento Terminológico:** O `financial_events` **não é um sistema de partidas dobradas** (dupla entrada com débito em uma conta contábil e crédito em outra). Trata-se de um registro contábil linear e auditável de mutações de saldo por usuário.

### Status de Continuidade no Staging: `UNTESTED_NO_EVENTS`
Como a tabela `financial_events` no staging possui **0 registros**:
- A continuidade do encadeamento contábil ($\text{balance\_after}_{k} = \text{balance\_before}_{k+1}$) **NÃO FOI TESTADA** neste banco.
- O relatório técnico declara expressamente o status `UNTESTED_NO_EVENTS`.
- Não se pode afirmar que a cadeia contábil está "íntegra" no staging; afirma-se apenas que inexistem eventos no momento da leitura.

---

## 5. Qualificação do Histórico Anterior à Migração 5

Concluir ou arbitrar o saldo de um usuário pela soma das linhas de `transactions` ($\sum \text{Depósitos} - \sum \text{Saques}$) é uma **falácia contábil** no CatBrasil:

1. **Inexistência de Diário de Saldo Pré-v5:** O registro contábil atômico `financial_events` só foi criado na migração 5 (`000005_financial_integrity.up.sql`).
2. **Atualizações Diretas de Saldo no Código Legado:** No código anterior à migração 5, operações como rendimentos diários de máquinas (`coletas`), prêmios da roleta, bônus de indicação e custos de aluguel executavam `UPDATE users SET balance = ...` diretamente no banco sem registrar linhas correspondentes em `transactions`, ou criavam registros com tipos arbitrários e convenções de sinal inconsistentes.
3. **Injeção de Saldo por Scripts Iniciais:** Seeds ou inserções manuais injetavam saldo diretamente no registro do usuário sem qualquer contrapartida em `transactions`.
4. **Ausência de Rastreamento de Estado:** Transações anteriores à migração 5 não registravam `balance_before` nem `balance_after`.

**Regra Operacional:** Nunca tente calcular saldos somando a tabela `transactions`. A partir da migração 5, o diário `financial_events` é a única referência cronológica de saldo da plataforma.

---

## 6. Inventário Agregado de Transações e Pendências (Staging)

### 6.1. Agregação de Transações por Grupo
| Tipo | Status | Provedor | Quantidade | Total (R$) | Data Inicial | Data Final |
| :--- | :--- | :--- | :---: | :---: | :--- | :--- |
| Depósito PIX | `pendente` | `suitpay` | 7 | R$ 7.00 | 2026-10-16 16:34:19 | 2026-10-16 17:00:33 |

### 6.2. Pendências Pseudonimizadas para Investigação
A tabela abaixo lista as 7 transações pendentes utilizando identificadores pseudonimizados com HMAC efêmero, sem expor dados originais:

| Transação (Pseudônimo) | Usuário (Pseudônimo) | Valor (R$) | Provedor | External ID | Idade Aprox. |
| :--- | :--- | :---: | :--- | :--- | :---: |
| `anon_tx_73b95a2622f3` | `anon_user_bf4fec45d4f5` | R$ 1.00 | `suitpay` | `anon_ext_ef1774eedd6a` | ~65 min |
| `anon_tx_323338a240f3` | `anon_user_c138896c2b86` | R$ 1.00 | `suitpay` | `anon_ext_8c90d7f54936` | ~65 min |
| `anon_tx_4b65754fb455` | `anon_user_0191aa846796` | R$ 1.00 | `suitpay` | `anon_ext_1c5b606fffc2` | ~64 min |
| `anon_tx_55503e2ddae1` | `anon_user_f786f0275df1` | R$ 1.00 | `suitpay` | `anon_ext_4cc22a15cfc1` | ~52 min |
| `anon_tx_69fbe34bad24` | `anon_user_7da382e39cad` | R$ 1.00 | `suitpay` | `anon_ext_0abd072a253e` | ~49 min |
| `anon_tx_bfeb70bc4ce4` | `anon_user_c2523434cdb5` | R$ 1.00 | `suitpay` | `anon_ext_c7fe87f01e30` | ~43 min |
| `anon_tx_166307c6540a` | `anon_user_ffd2c6d10787` | R$ 1.00 | `suitpay` | `anon_ext_d3bca4c0617b` | ~39 min |

---

---

## 7. Decisão de Abertura: Adoção de Banco Novo (Greenfield) e Atestado Operacional

### 7.1. Atestado Operacional do Usuário vs Prova Técnica
Em deliberação de release para a etapa P0.1, o usuário confirmou textualmente que:
> *"Nada oficial foi colocado em operação e todos os dados anteriores eram fictícios."*

**Distinção Fundamental de Governança:**
1. **O Atestado Operacional:** Confirma a ausência de passivo financeiro real prévio, atesta a natureza puramente fictícia/sintética dos dados pré-existentes e desobriga a realização de conciliação retroativa contra extrato bancário oficial SuitPay. Com base nisso, deliberou-se pelo **descarte contábil de qualquer migração de saldos, usuários ou transações do ambiente anterior**. O contêiner/volume legado (`catbrasil_postgres` / `catbrasil_pgdata`) permanece intocado, preservado para fins de arquivo e **estritamente não migrado**.
2. **A Prova Técnica de Abertura:** O atestado operacional não substitui a verificação técnica. A garantia técnica de que a plataforma operará sem contaminação histórica é provida pelo **Portão Técnico de Corte Fail-Closed (`scripts/release/preflight_cutover.py`)**, que atesta matematicamente no banco de dados novo:
   - Bloqueio contra o reuso acidental do volume de dados antigo (`catbrasil_pgdata`) no caminho suportado de deploy.
   - Aplicação integral e coerente das migrações relacionais (1 a 9).
   - Inexistência absoluta de registros operacionais (zero usuários, zero saldo, zero transações, zero pendências, zero callbacks e zero identidades).
   - Classificação fail-closed: qualquer inconsistência ou métrica ausente é rotulada como `UNKNOWN` e bloqueia a inicialização da API no caminho orquestrado.

---

## 8. Ferramental de Auditoria e Portão de Corte
 
### 8.1. Auditoria de Staging (`scripts/release/reconcile.py`)
```bash
# 1. Executar testes automatizados da ferramenta de conciliação
python3 -m unittest tests/test_reconcile.py -v

# 2. Executar auditoria somente leitura contra staging
python3 scripts/release/reconcile.py --target staging --confirm-staging --check-only
```
- **Exit Codes do Reconcile:** `0` (sem desvios relacionais), `1` (divergência ou quebra de continuidade), `2` (erro fatal de alvo ou conexão).

### 8.2. Portão Técnico de Corte Greenfield (`scripts/release/preflight_cutover.py`)
O alvo de produção é fixado e inegociável no código da ferramenta:
- Projeto Compose: `catbrasil-prod`
- Serviço Compose: `postgres`
- Volume de dados: `catbrasil_prod_pgdata` (tipo: `volume`)
- Rede isolada: `catbrasil_prod_net`
- Banco de dados: `catbrasil_prod`

```bash
# 1. Executar testes unitários do portão de corte
python3 -m unittest tests/test_preflight_cutover.py -v

# 2. Executar prova prática em contêiner PostgreSQL efêmero e isolado
python3 -m unittest tests/test_ephemeral_cutover_proof.py -v

# 3. Execução real do portão contra o contêiner de produção recém-migrado (antes da API)
python3 scripts/release/preflight_cutover.py --container catbrasil-prod-postgres-1
```
- **Exit Codes do Preflight Cutover:**
  - **`0` (APPROVED_CLEAN_BASELINE):** Estado zero 100% comprovado em produção. Autorizada a inicialização dos serviços financeiros no fluxo homologado.
  - **`1` (VIOLATIONS_DETECTED):** Violação de estado zero detectada (dados preexistentes, saldo != 0, pendências ou métricas `UNKNOWN`). Inicialização terminantemente bloqueada.
  - **`2` (FATAL_CONFIG_ERROR):** Rejeição de volume antigo (`catbrasil_pgdata`), labels ausentes, serviço incorreto, rede ausente ou falha de infraestrutura. Parada imediata.

### 8.3. Orquestrador de Corte Sequencial Seguro (`scripts/release/cutover_orchestrator.sh`)
```bash
# Validação de configuração, variáveis e compose sem expor segredos e sem alterar containers:
scripts/release/cutover_orchestrator.sh --validate-only
```

> **NOTA DE RISCO OPERACIONAL (LIMITAÇÃO DE ESCOPO DO GATE):**
> O portão técnico de corte e o script orquestrador protegem o **caminho suportado e automatizado de deploy**. Um operador com privilégios de host no ambiente ainda pode contornar as validações executando comandos manuais diretos do Docker Compose (como `docker compose up -d`). Portanto, **não se alega impossibilidade física absoluta contra ações manuais arbitrárias no host**, mas sim a garantia de interrupção e bloqueio fail-closed em todo o pipeline suportado e documentado de release.

---

## 9. Checklist Operacional de Corte (Cutover Gate)

Para a abertura definitiva do novo ambiente operacional, o procedimento abaixo é de cumprimento mandatório e sequencial:

1. **Registro do Atestado Operacional:** Confirmação documental de que nenhum operador possui saldo a migrar e de que a abertura opera sob regime greenfield.
2. **Validação da Configuração (.prod.env):** Executar `scripts/release/cutover_orchestrator.sh --validate-only`. O script valida em memória (sem imprimir credenciais ou segredos) que `POSTGRES_DB` é `catbrasil_prod`, `DATABASE_URL` aponta para host `postgres` e banco `catbrasil_prod`, e `SUITPAY_LIVE_ENABLED` é estritamente `"false"`.
3. **Provisionamento do Volume Novo:** Deploy executado no caminho suportado exclusivamente através do orquestrador e do overlay [`compose.prod.yml`](./compose.prod.yml), garantindo montagem no volume novo `catbrasil_prod_pgdata` e rede `catbrasil_prod_net`, rejeitando o volume legado `catbrasil_pgdata`.
4. **Execução de Migrações:** Aplicar migrações 1 a 9 no banco `catbrasil_prod` (`backend-migrate up`).
5. **Execução do Preflight Gate (Gate de P0.6):** Na janela de corte operacional de P0.6, rodar `scripts/release/preflight_cutover.py --container catbrasil-prod-postgres-1`. O portão DEVE retornar código de saída `0` (`APPROVED_CLEAN_BASELINE`) contra o banco real antes de liberar a API.
6. **Liberação da API Backend:** Somente após o exit code `0` do preflight no caminho suportado, inicializar os serviços `backend-api` e `backend-cron` com `--no-deps`.
7. **Contenção Financeira Inviolável:** A chave `SUITPAY_LIVE_ENABLED` está fixada literalmente como `"false"` em `compose.prod.yml` nesta etapa. Essa amarração impede a ativação acidental de dinheiro real via variável de ambiente, mesmo que um operador tente contornar o orquestrador executando `docker compose up` diretamente no host. A liberação só ocorrerá no corte final de P0.6 após conclusão e homologação de P0.2 a P0.5.

> **DELIBERAÇÃO E LIMITES DE P0.1:** A decisão greenfield e o ferramental de corte estão **concluídos e validados** (atestado operacional + prova técnica em contêiner efêmero). A plataforma **NÃO está liberada para dinheiro real em produção**, e o preflight no banco de dados real de produção é portão mandatório da etapa P0.6.
