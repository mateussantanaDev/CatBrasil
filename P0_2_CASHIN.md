# P0.2 — Cash-in PIX e Eventos Difíceis (Camada de Caso de Uso e Contrato de Integração)

**Data de Atualização:** 16/10/2026  
**Status da Etapa:** CONCLUÍDA — implementação local de código/banco aceita após verificação independente do tech lead. Sem certificação oficial do sandbox SuitPay: P0.5 (prova no provedor) e P0.6 (corte em produção) permanecem pendentes.  
**Responsáveis Técnicos:** **Gemini** e **OpenCode** implementaram em rodadas separadas; **Codex** revisou; **todos os arquivos do escopo estão aceitos.**  
**Escopo:** `backend/pkg/gateway/suitpay.go`, `suitpay_test.go`, `backend/internal/usecase/payment/payment_usecase.go`, `payment_usecase_test.go`, `cashin_integration_test.go`, `backend/internal/delivery/http/handler/financial_handler.go`, `financial_handler_test.go`, migrações relacionais, arquivos `cashin_*.go` em `repository/postgres`, `financial_repository.go`, `P0_2_CASHIN.md`  
**Contenção Financeira:** `SUITPAY_LIVE_ENABLED=false` mantido; sem credenciais reais nem chamadas à rede externa. Prova oficial em P0.5 e corte em produção em P0.6 seguem pendentes; nada aqui afirma production ready.

---

## 1. Visão Geral e Arquitetura do Cash-in

A etapa **P0.2 — Cash-in e eventos difíceis** estabelece o fluxo de entrada de capital na plataforma CatBrasil via PIX (SuitPay oficial e emulador EMV nativo). Os princípios fundamentais aplicados são:

1. **Nenhum crédito por suposição:** Apenas cobranças autenticadas com tipo estrito `PIX` e status comprovado de liquidação (`PAID_OUT`) podem creditar saldo de usuário.
2. **Nenhuma confirmação sem persistência durável:** Webhooks só recebem HTTP 200 `{"response":"OK"}` se a persistência e reconciliação no banco forem concluídas com sucesso dentro de transação ACID.
3. **Isolamento de submissão ambígua:** Em timeouts de rede ou respostas incompletas do gateway, o estado ambíguo é registrado para conciliação humana/automática, **sem reenviar requisição externa automaticamente** e sem expor PIX inválido ao pagador.
4. **Remoção de fallback inseguro:** Eliminação completa de updates não transacionais que colocavam depósitos em `processando` fora do controle de concorrência. Se o tracker falhar ou estiver ausente, a operação falha imediatamente com erro seguro.
5. **Fail-closed mandatório:** Em qualquer ambiente (desenvolvimento, staging ou produção), se os contratos de persistência atômica (`DepositEventApplier` ou `DepositSubmissionTracker`) estiverem ausentes para SuitPay, a operação falha imediatamente.
6. **Identificador externo exato:** A assinatura e o decode do callback SuitPay preservam `idTransaction` numérico como `json.Number` (nunca `float64`), impedindo perda de precisão acima de 2^53 que transformaria o ID em notação científica e quebraria a deduplicação/estorno por `external_id`.

---

## 1.1. Correção de Precisão do `idTransaction` (Revisão de Tech Lead)

**Bug:** `VerifySuitPayWebhook` validava a assinatura com `json.Decoder.UseNumber`, mas depois decodificava o callback com `json.Unmarshal`, convertendo `idTransaction interface{}` em `float64`. Identificadores com tamanho UnixNano (> 2^53) perdiam dígitos e `fmt.Sprint` produzia notação científica (ex.: `1.2345678901234568e+16`), casando o evento contra o `external_id` errado — na prática, `PAID_OUT` caía em `queued` em vez de creditar.

**Correção (`backend/pkg/gateway/suitpay.go`):** o segundo decode do callback usa `json.NewDecoder(...).UseNumber()`, então números chegam como `json.Number` (dígitos exatos) e `fmt.Sprint`/`v.String()` devolvem o ID original. O usecase trata `json.Number` explicitamente na resolução do `externalID` (`payment_usecase.go`), mantendo fallback defensivo para `float64`. Cobertura: `TestVerifySuitPayWebhook_LargeNumericIDKeepsExactExternalID` (numérico > 2^53), `TestVerifySuitPayWebhook_StringIDKeepsExactExternalID` (string), mais testes de usecase, handler e integração real com repositório.

---

## 1.2. Classificação de Estorno Antecipado e Duplicatas (Revisão de Tech Lead)

**Transição de repositório suportada:** o repositório real pode aplicar `PAID_OUT` e **imediatamente estornar** um `CHARGEBACK` anterior do mesmo `external_id` (entrega fora de ordem), retornando `outcome == "applied"` com `transaction.Status == "cancelado"`.

**Classificação no usecase (`ProcessSuitPayWebhook`):**
- `applied` + `PAID_OUT` + status `cancelado` → **`Kind: "chargeback"`**, log `PIX_CHARGEBACK`, mensagem de estorno; **nunca** tratado como depósito bem-sucedido.
- `applied` + `PAID_OUT` + status liquidado → `Kind: "deposit"` (crédito legítimo).
- `duplicate` + status `cancelado` → `Kind: "chargeback"` (idempotente, sem novo efeito).
- HTTP 200 é mantido apenas para desfechos duráveis (`applied`, `duplicate`, `queued`); falhas de persistência continuam em 500 com mensagem genérica sem SQL.

**Evento realtime (`SuitPayWebhookNotificationEvent` em `financial_handler.go`):**

| Desfecho | Evento WebSocket |
|---|---|
| `applied` + `Kind=deposit` | `pix_confirmed` |
| `applied` + `Kind=chargeback` (inclui estorno antecipado) | `pix_chargeback` |
| `duplicate` (qualquer kind) | `pix_duplicate` — **nunca** re-emite `pix_confirmed` |
| `queued` | `pix_queued` |
| `Kind=payout` | `withdraw_approved` / `withdraw_rejected` |

---

## 2. Contratos com a Camada de Repositório

### 2.1. Interface `DepositSubmissionTracker` (Obrigatória em Modo SuitPay)

Definida em `backend/internal/usecase/payment/payment_usecase.go`:

```go
type DepositSubmissionTracker interface {
	MarkDepositSubmissionStarted(ctx context.Context, txID string) error
	MarkDepositSubmissionIssue(ctx context.Context, txID, reason, externalID string) error
	MarkDepositIntentSent(ctx context.Context, requestNumber, externalID string) (*domain.Transaction, error)
}
```

- **`MarkDepositSubmissionStarted(ctx, txID)`**: Executado **antes** de qualquer chamada HTTP ao gateway SuitPay. Se o tracker estiver ausente ou o método retornar erro, **o gateway recebe exatamente 0 chamadas**, impedindo a criação de cobranças órfãs.
- **`MarkDepositSubmissionIssue(ctx, txID, reason, externalID)`**: Executado após timeouts de rede, cancelamento de contexto, respostas incompletas ou falhas na vinculação de QR code.
  - **Exceção deliberada e curta de contexto:** Após qualquer chamada HTTP ao gateway, o registro de outcome ambíguo usa **SEMPRE** `context.WithTimeout(context.Background(), 5*time.Second)` com `defer cancel()`. Isso previne que cancelamentos do cliente ou timeouts na requisição HTTP abortem a gravação da ocorrência no banco de dados.
  - Erros de persistência de ocorrência são propagados para garantir visibilidade da auditoria.
- **`MarkDepositIntentSent(ctx, requestNumber, externalID)`**: Executado assim que o gateway retorna um `idTransaction` válido, antes de `AttachDepositQRCode`, garantindo a reserva transacional do identificador externo e a vinculação durável da intenção na tabela `cashin_deposit_intents`.

### 2.2. Interface `DepositEventApplier` (Processamento de Webhooks)

Definida em `backend/internal/usecase/payment/payment_usecase.go`:

```go
type DepositEventApplier interface {
	ApplyDepositEvent(ctx context.Context, requestNumber, externalID, status string, amount float64) (*domain.Transaction, string, error)
}
```

#### Regras de Roteamento, Normalização e Limites de Schema:
1. **Tipo de Transação Estrito (`TypeTransaction`):**
   - Se `TypeTransaction == "PIX_CASHOUT"`, o fluxo é roteado para `processPayoutCallback`.
   - Se `TypeTransaction == "PIX"`, o status bruto (`rawStatus`) é repassado ao `ApplyDepositEvent`.
   - Se `TypeTransaction` for desconhecido ou diferente de PIX (ex.: `BOLETO`, `CARTAO`, `UNKNOWN`), o usecase injeta status sintético `UNSUPPORTED_TYPE:<TIPO>` limitado estritamente a **no máximo 32 caracteres** (atendendo à constraint `VARCHAR(32)` de `deposit_gateway_events.status` e `cashin_event_queue.gateway_status`). Status sintéticos **nunca** passam `PAID_OUT` ou `CHARGEBACK` ao repositório; o evento é enfileirado para conciliação sem crédito.
2. **Proibição de Normalização de `PAID` para `PAID_OUT`:**
   - O status `PAID` não prova liquidação bancária concluída no gateway; portanto, não é normalizado para `PAID_OUT` por suposição. Ele transita com status `PAID` para enfileiramento durável (`queued`) pelo repositório.
   - Apenas `PAID_OUT` é elegível para liquidação e crédito ao usuário.
   - Status como `PAID`, `UNPAID`, `CANCELED`, `EXPIRED`, `UNDER_ANALYSIS` são enfileirados duravelmente sem movimentação financeira.
3. **Resolução por Identificador:**
   - Callbacks com `requestNumber` ausente mas com `externalID` presente são repassados ao `ApplyDepositEvent` com `requestNumber == ""` para resolução pelo índice de `external_id` ou enfileiramento seguro.
   - O callback só é rejeitado com HTTP 400 (`MISSING_IDENTIFIER`) quando **ambos** os identificadores estiverem ausentes.
4. **Desfechos (`outcome`):**
   - **`applied`**: Transação liquidada no banco. Se `status == "PAID_OUT"` **e** o status final não é `cancelado`, saldo creditado e evento `pix_confirmed` emitido. Se `status == "PAID_OUT"` com status final `cancelado` (repositório aplicou e imediatamente estornou um `CHARGEBACK` antecipado do mesmo `external_id`), o resultado é classificado como **`Kind: "chargeback"`** e emite `pix_chargeback`. Se `status == "CHARGEBACK"`, estorno aplicado e evento `pix_chargeback` emitido.
   - **`duplicate`**: Idempotência confirmada; nenhum crédito repetido. HTTP 200 OK retornado. O handler emite `pix_duplicate` — **nunca** `pix_confirmed` — e transações `cancelado` mantêm `Kind: "chargeback"`.
   - **`queued`**: Evento registrado na fila de conciliação (`cashin_event_queue`). HTTP 200 OK retornado; nenhum crédito concedido; evento `pix_queued`.
   - **`error != nil`**: Falha de persistência. Retorna HTTP 500 (`PERSISTENCE_FAILED`), com mensagem genérica `"falha temporária ao persistir evento de pagamento"` sem expor detalhes SQL ou segredos de infraestrutura ao gateway/cliente.

### 2.3. Resiliência de Configuração e Kill Switch

1. **Falha de DB em `GetConfig`:**
   - Retorna HTTP 503 (`CONFIG_UNAVAILABLE`), sinalizando ao gateway provedor que deve retentar a entrega posteriormente.
2. **Segredo Ausente (`SuitPayClientSecret == ""`):**
   - Retorna HTTP 500 (`CONFIG_UNAVAILABLE`).
3. **Kill Switch Off (`SuitPayEnabled == false`):**
   - **Novas cobranças** via `CreatePixDeposit` são imediatamente bloqueadas (`"gateway SuitPay não configurado para cobranças"`).
   - **Callbacks de intenções já existentes** continuam sendo verificados criptograficamente pela assinatura e aplicados normalmente, impedindo descarte inadvertido de pagamentos legítimos em trânsito.

---

## 3. Integração com Banco Relacional e Migração 9

A camada de persistência utiliza as tabelas adicionadas pela migração `000009_cashin_event_durability.up.sql`:
- **`deposit_gateway_events`**: Chave primária composta `(external_id, status)` com `CHECK (char_length(status) BETWEEN 1 AND 32)`, garantindo registro imutável e idempotente de cada evento autenticado.
- **`cashin_event_queue`**: Fila de reconciliação com `dedup_key UNIQUE`, `gateway_status VARCHAR(32)`, `queue_status CHECK (pending, resolved)` e auditoria de evidência em resolução manual.
- **`cashin_deposit_intents`**: Rastreamento do ciclo de envio ao provedor (`intent_sent_at`, `result_ambiguous`, `ambiguity_reason`).
- **`financial_events`**: Livro-razão com constraint matemática `CHECK (balance_after = balance_before + amount)` e unicidade `(transaction_id, user_id, kind)`.

---

## 4. Exposição da Fila no Painel Administrativo

Diferente do estado inicial preliminar:
- A fila de reconciliação de cash-in (`cashin_event_queue`) e pendências de depósito ambíguo foram integradas pelo OpenCode no método `ListFinancialReconciliation`.
- O endpoint administrativo `GET /api/v1/admin/finance/reconciliation` expõe de forma unificada tanto as divergências de saques quanto os eventos de depósito enfileirados ou pendentes de conferência.

---

## 5. Evidência de Testes Automatizados

Evidência resumida (16/10/2026). Logs brutos omitidos de propósito. **Nenhuma chamada real ao provedor; nenhuma liberação de dinheiro real.**

### 5.1. Verificação independente do tech lead

| Verificação | Ambiente | Resultado |
| --- | --- | --- |
| `go test ./... -count=1` | banco PostgreSQL novo `catbrasil_p02_lead02` | PASS — todos os pacotes |
| `go test -race ./internal/repository/postgres/... ./internal/usecase/payment/... -count=1` | banco PostgreSQL novo `catbrasil_p02_lead03` | PASS |
| `go vet ./...` | local | PASS |
| 50 testes unittest Python | local | PASS |
| `bash -n scripts/release/cutover_orchestrator.sh` | local | PASS |
| `git diff --check` (apenas o diff de arquivos rastreados; arquivos não rastreados foram revisados via testes e `gofmt`, quando aplicável) | repositório | PASS |
| Migração 9/9 (`000009_cashin_event_durability.up.sql`) aplicada e conferida (`migrations.CheckCurrent`) | PostgreSQL descartável | PASS |

### 5.2. Cobertura da suíte Go (resumo)

| Pacote | Suíte | Destaques |
| --- | --- | --- |
| `pkg/gateway` | PASS | assinatura de webhook; `idTransaction` numérico (> 2^53) e string preservam `external_id` exato |
| `internal/usecase/payment` | PASS (com `-race`) | timeout ambíguo com contexto desacoplado; resposta incompleta; tracker ausente/falho → gateway nunca chamado; kill switch; tipo não suportado limitado a 32 chars; classificação de chargeback; config em 503/500 |
| `internal/delivery/http/handler` | PASS | rejeição de assinatura; persistência falha em 500 genérico sem SQL; eventos realtime `pix_confirmed` / `pix_chargeback` / `pix_duplicate` / `pix_queued` (duplicata nunca re-emite confirmação) |
| `internal/repository/postgres` | PASS (com `-race`, banco real descartável) | `ApplyDepositEvent` transacional; 8 goroutines simultâneas → 1 `applied` + 7 `duplicate` com crédito único; chargeback antecipado + `PAID_OUT` tardio → `applied`+`cancelado`, saldo líquido zero |

### 5.3. Testes de Integração com Repositório PostgreSQL Real (`cashin_integration_test.go`)

O teste de integração `backend/internal/usecase/payment/cashin_integration_test.go` utiliza **exclusivamente o repositório de produção** `postgres.NewTransactionRepository(pool)` como `DepositApplier` e `DepositSubmissionTracker` do `PaymentUseCase`.

**Princípios do Teste:**
1. **Zero duplicação de regras de negócio:** Todo o processamento contábil, travas transacionais (`pg_advisory_xact_lock`), resolução de intenções, deduplicação em `deposit_gateway_events`, estorno de saldo e manipulação de fila são executados pelo método real `postgres.TransactionRepository.ApplyDepositEvent`.
2. **Ciclo de vida do banco:** `getTestPostgresPool` executa `migrations.Apply(ctx, pool)` e `migrations.CheckCurrent(ctx, pool)` contra `CATBRASIL_TEST_DATABASE_URL` (PostgreSQL 16 descartável).
3. **Cenários Cobertos:**
   - **`PAID_OUT` Assinado:** Valida crédito no saldo (`users.balance`), incremento de rollover (`users.required_rollover`), atualização de status para `sucesso`, vínculo de `external_id` e criação de linha contábil no razão `financial_events` com invariante `balance_after = balance_before + amount`.
   - **Concorrência e Idempotência:** 8 goroutines simultâneas enviando o mesmo payload assinado resultam em exatamente 1 aplicação (`applied`) e 7 duplicatas (`duplicate`), com crédito e lançamento contábil únicos.
   - **`CHARGEBACK` (mesmo `external_id` do `PAID_OUT`):** Reverte saldo e rollover através do repositório real (`applyChargeback`), atualiza transação para `cancelado` e registra entrada correspondente no razão.
   - **`CHARGEBACK` antecipado + `PAID_OUT` tardio (mesmo `external_id`):** `CHARGEBACK` antes do crédito é enfileirado (`chargeback_before_credit`, sem efeito financeiro); o `PAID_OUT` tardio é aplicado e **imediatamente estornado** pelo repositório no mesmo ciclo, retornando `applied` + `cancelado`. O usecase classifica como `Kind: "chargeback"`; saldo líquido zero, saques permanecem bloqueados, razão com `deposit` + `deposit_chargeback`, e o replay do `PAID_OUT` resulta em `duplicate` classificado como chargeback sem novo crédito.
   - **ID numérico grande (> 2^53):** `idTransaction` numérico `9007199254740993+` creditar e grava `external_id` exato no banco (regressão de precisão float64).
   - **`idTransaction` string:** Payload com `idTransaction` como JSON string resolve e credita com `external_id` exato.
   - **Status não-PIX / Não Final:** Envia cobrança com tipo não suportado (ex.: `BOLETO`), verificando que o status sintético `<= 32` caracteres é enfileirado em `cashin_event_queue` sem qualquer alteração no saldo do usuário.

**Status de Execução (16/10/2026):** suíte executada com `go test -race -count=1 -run TestPostgresCashIn_RealRepositoryWebhookFlows` contra PostgreSQL 16 descartável — **todos os 7 cenários PASS**, incluindo `Real Repo: Signed PAID_OUT webhook applies credit, rollover, and ledger entry` (o cenário que anteriormente caía em `queued` por perda de precisão do ID). `go build ./...` e `go vet` limpos para os pacotes `pkg/gateway`, `internal/usecase/payment` e `internal/delivery/http/handler`.

---

## 6. Limitações de Sandbox e Próximos Passos (P0.5 / P0.6)

1. **Emulação em Staging vs Provedor Real:**
   - Em ambiente de staging, as respostas de QR code e webhooks são providas pelo emulador `pix-sandbox` (`127.0.0.1:18082`).
   - O modo real com a SuitPay oficial (`SUITPAY_LIVE_ENABLED=true`) permanece estritamente bloqueado.
   - A prova com credenciais oficiais de sandbox, IP autorizado e webhooks HTTPS fica expressamente reservada para a etapa **P0.5 (Prova no Sandbox Oficial)**; o corte em produção (preflight no banco real) fica reservado para **P0.6**.
2. **Aceitação e Revisão Técnica:**
   - **Gemini e OpenCode implementaram em rodadas separadas; Codex revisou**; todos os arquivos do escopo estão aceitos.
   - Verificação independente do tech lead em 16/10/2026: suíte Go completa (`-count=1`) e `-race` em `repository/postgres` + `usecase/payment` em bancos PostgreSQL descartáveis, além de `go vet ./...`, 50 unittests Python, `bash -n scripts/release/cutover_orchestrator.sh` e `git diff --check` (diff de arquivos rastreados; arquivos não rastreados revisados via testes e `gofmt`, quando aplicável) — todos PASS (ver §5.1). Sem toque em produção/staging e sem liberação de dinheiro real (`SUITPAY_LIVE_ENABLED=false`).
3. **Portões pendentes:** esta etapa **não** certifica o sandbox oficial (P0.5) nem libera produção (P0.6); `SUITPAY_LIVE_ENABLED` permanece `"false"`. Próxima etapa: **P0.3 (Cash-out e destinatário)**.
