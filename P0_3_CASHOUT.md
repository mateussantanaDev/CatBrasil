# P0.3 — Cash-out: arquitetura integrada, durabilidade de eventos e reconciliação

**Data:** 16/10/2026  
**Status:** Implementação local completa e integrada (gateway, usecase, repositório transacional, handlers HTTP, WebSocket, frontend e portão de corte preflight) concluída e aceita após verificação independente do tech lead. Prova no sandbox oficial SuitPay permanece pendente em **P0.5**; corte em produção em **P0.6**; `SUITPAY_LIVE_ENABLED=false` literal no `compose.prod.yml`.

---

## 1. Escopo integrado

Diferente da versão preliminar que cobria estritamente a camada de banco de dados, esta especificação reflete a **solução completa e integrada de ponta a ponta** no repositório:

- **Gateway HTTP (`backend/pkg/gateway/suitpay.go` e testes):**
  - Implementação do endpoint de transferência PIX (`POST /api/v1/gateway/pix-payment`).
  - Suporte ao campo `documentValidation` (CPF/CNPJ do destinatário para conferência pelo provedor) e `externalId` único para deduplicação no provedor.
  - Verificação rigorosa de assinatura no webhook (`VerifySuitPayWebhook`): SHA-256 sobre `clientSecret` concatenado aos valores escalares na ordem do JSON (sem o campo `hash`), com comparação de assinatura em tempo constante.
  - Mock de testes simulando comportamento real: respostas 200 OK, 400 Bad Request (`DOCUMENT_VALIDATE`), timeouts de rede e erros 5xx.

- **Usecase de Pagamentos (`backend/internal/usecase/payment/payment_usecase.go` e testes):**
  - Exigência mandatória de identidade civil verificada (`IdentityRepo.VerifiedPerson`) e CPF normalizado do titular antes de autorizar a reserva ou envio do saque.
  - Reserva atômica de saldo e conferência de rollover com fail-closed.
  - Retenção estrita em processamento em caso de timeout de rede, resposta sem "OK" ou retorno `DOCUMENT_VALIDATE` (código HTTP documentado retornado pelo gateway quando a chave não pertence ao documento): **nenhum cancelamento automático, nenhum estorno precipitado e nenhum reenvio automático**.
  - Resolução segura de `PayoutApplier` exclusivamente a partir do `FinancialRepository` efetivo (sem fallback cruzado ao `txRepo`), garantindo unicidade da verdade financeira.
  - Recepção de callbacks autenticados com status não reconhecido/inesperado: repasse direto a `ApplyPayoutCallbackWithOutcome`, que os enfileira duravelmente na `payout_event_queue`, devolvendo HTTP 200 com outcome `"queued"` para cessar retentativas em loop do gateway.

- **Repositório Transacional PostgreSQL (`backend/internal/repository/postgres/` e migração 10/10):**
  - Migração `000010_cashout_event_durability.up.sql`: criação de `payout_submissions` (trilha durável de intenção) e `payout_event_queue` (fila de conciliação humana).
  - Métodos `ClaimPayout`, `RecordPayoutSubmission` e `ApplyPayoutCallbackWithOutcome` sob locks transacionais rígidos: advisory lock por `external_id` (`hashtextextended(external_id, 718819)`) e row lock `FOR UPDATE`.
  - Isolamento de escrita de vínculo via `SAVEPOINT payout_link`: se ocorrer erro de unicidade 23505, a transação reverte ao savepoint, enfileira a evidência na `payout_event_queue` e conclui o commit sem perda de dados.
  - Deduplicação inteligente de incidentes (`payoutReviewDedupKey`): chave inclui o valor (`%.2f`), preservando múltiplos conflitos numéricos distintos sem sobrescrita.

- **Handlers HTTP e WebSocket (`backend/internal/delivery/http/handler/` e testes):**
  - `AdminHandler.ApproveWithdrawal`: **remoção mandatória do evento WebSocket `withdraw_approved` para o usuário**. A mera aprovação pelo admin apenas despacha a ordem para processamento; emitir evento ao cliente gerava falso sinal de saque pago. O admin recebe o estado no payload HTTP de retorno.
  - `FinancialHandler.HandleSuitPayWebhook` / `SuitPayWebhookNotificationEvent`: mapeamento exato de eventos:
    - Payout com outcome `applied` e status `sucesso` → emite `withdraw_paid` ao usuário.
    - Payout com outcome `applied` e status `cancelado` → emite `withdraw_rejected` ao usuário.
    - Payout com outcome `duplicate` ou `queued` → retorna `""` (nenhum evento WebSocket emitido, prevenindo replays e avisos incorretos).

- **Frontend Client (`frontend/apps/client/src/App.svelte`):**
  - Listener WebSocket atualizado:
    - Evento `withdraw_paid`: dispara toast visual de sucesso (`type: "win"`, verde) com o valor liquidado e recarrega os dados do usuário (`loadUserData()`).
    - Evento `withdraw_rejected`: dispara toast de erro (`type: "error"`, vermelho) notificando cancelamento do saque e estorno do saldo, seguido de `loadUserData()`.
    - O frontend ainda reconhece o evento legado `withdraw_approved` (cujo toast o descreve como "em processamento"), mas o backend não emite mais esse evento na aprovação manual.

- **Portão Técnico de Corte (`scripts/release/preflight_cutover.py` e testes Python):**
  - Atualizado para validar 10 migrações obrigatórias (1..10) e 26 tabelas no esquema público (incluindo `payout_submissions` e `payout_event_queue`).
  - Arquitetura fail-closed à prova de parse: verificação estrutural antes de queries de integridade, garantindo saída limpa exit 1 (`VIOLATIONS_DETECTED`) sem falhas de parse SQL quando executado contra bancos parcialmente migrados (provado em contêiner efêmero migrado até 9).

---

## 2. Garantias arquiteturais do ciclo de cash-out

```
[ Usuário ] -> Solicita Saque (PixKey + Senha)
      |
      v
[ Usecase ] -> Valida Identidade Civil (VerifiedPerson) & Saldo/Rollover
      |
      v
[ Postgres ] -> ReservePayout (débito de saldo + registro pendente)
      |
      +---> Se Manual: Aguarda aprovação do Admin (AdminHandler.ApproveWithdrawal)
      |         (Sem emitir WS ao cliente; transição pendente -> processando)
      |
      v
[ Usecase / Gateway ] -> TransferPix (POST /api/v1/gateway/pix-payment com documentValidation + externalId)
      |
      +---> Timeout / Erro 5xx / DOCUMENT_VALIDATE:
      |         Retém em "processando", reserva mantida, SEM reenvio automático.
      |
      +---> Resposta 200 OK (idTransaction retornado):
      |         RecordPayoutSubmission vincula ID local ao externalId do provedor.
      |
      v
[ Webhook SuitPay ] -> ProcessSuitPayWebhook (valida assinatura SHA-256 sobre clientSecret + valores do JSON)
      |
      v
[ Postgres ] -> ApplyPayoutCallbackWithOutcome (Advisory Lock + Row Lock)
      |
      +---> Outcome "applied"   (sucesso: liquidação | cancelado: estorno único no razão)
      |         --> Dispara WS withdraw_paid ou withdraw_rejected
      |         --> Toast verde ou vermelho no frontend + refresh
      |
      +---> Outcome "duplicate" (replay idempotente; sem novo efeito contábil)
      |         --> Sem emissão de WS
      |
      +---> Outcome "queued"    (órfão, status não suportado, conflito de valor)
                --> Gravado em payout_event_queue para revisão humana
                --> Responde HTTP 200 ao gateway; sem emissão de WS
```

### 2.1 Contrato de outcome transacional

| Outcome | Condição no Banco | Efeito Contábil | Notificação WS |
| --- | --- | --- | --- |
| **`applied`** | Primeira transição definitiva de estado (`sucesso` ou `cancelado`) | Liquidação definitiva OU estorno único no razão (`payout_cancel_refund`) | `withdraw_paid` (sucesso) ou `withdraw_rejected` (cancelado) |
| **`duplicate`** | Evento válido idêntico a estado já conciliado | Nenhum (idempotência estrita) | Nenhuma (`""`) |
| **`queued`** | Sem vínculo local, valor divergente, status inesperado ou `PAID_OUT` após cancelamento sem liquidação | Nenhum (saldo intocado); evidência registrada em `payout_event_queue` | Nenhuma (`""`) |

### 2.2 Reconciliação e delimitação do provedor

1. **Reconciliação por externalId:** Trilha ancorada em:
   - Identificador interno estável (`transactions.id` / `payout_submissions.internal_ref`).
   - Identificador de retorno do provedor (`payout_submissions.external_id` / `idTransaction`).
   - Fila durável de eventos e conciliação (`payout_gateway_events` e `payout_event_queue`).
   - **NÃO se afirma consulta ativa à API da SuitPay por `externalId`**, visto que a documentação pública do gateway não comprova a existência de endpoint GET para consulta de transferência por `externalId`.
2. **Validação de titularidade (`documentValidation`):** O código envia o CPF civil normalizado do titular no campo documentado `documentValidation` da SuitPay. No entanto, a confirmação de que o gateway barra transferências para terceiros é um contrato documentado pendente de prova prática no sandbox oficial (P0.5). A validação civil documental humana do CPF pertence à P0.4.

---

## 3. Evidência de testes e verificação independente

Todos os testes foram executados e validados pelo tech lead em ambientes locais isolados e bancos PostgreSQL descartáveis:

| Verificação | Ambiente / Banco | Resultado |
| --- | --- | --- |
| `go test ./... -count=1` | banco PostgreSQL novo `catbrasil_p03_lead06` | **PASS** — todos os pacotes Go |
| `go test -race ./internal/repository/postgres/... ./internal/usecase/payment/... ./internal/delivery/http/handler/... -count=1` | banco PostgreSQL novo `catbrasil_p03_lead07` | **PASS** — sem data races |
| `go vet ./...` | local | **PASS** |
| 56 testes unittest Python (`tests/test_preflight_cutover.py`, `tests/test_ephemeral_cutover_proof.py`, `tests/test_staging_reconciliation.py`) | local | **PASS** (100%) |
| `npm run build:client` | `frontend/apps/client` | **PASS** — bundle Svelte compilado sem erros |
| `bash -n scripts/release/cutover_orchestrator.sh` | local | **PASS** — sintaxe bash íntegra |
| Preflight 10/26 em PostgreSQL efêmero com banco até migração 9 | contêiner efêmero | **PASS** — fail-closed correto (`VIOLATIONS_DETECTED`, exit 1) sem erro de parse SQL |

---

## 4. Limitações declaradas (rigorosas)

1. **Sem teste sandbox oficial executado:** Nada nesta entrega foi executado contra a SuitPay real ou em sandbox oficial. Credenciais, liberação de IP, webhook HTTPS público e conciliação cruzada com o extrato da conta gráfica são o escopo exclusivo de **P0.5**.
2. **`SUITPAY_LIVE_ENABLED=false` literal no `compose.prod.yml`:** A produção permanece bloqueada contra transações reais.
3. **Validação de identidade documental:** O usecase verifica a interface `IdentityRepo.VerifiedPerson`. A implantação de uma esteira humana de revisão civil com evidência auditável e provedor real de entrega de SMS é o escopo de **P0.4**.
4. **Fila operacional `payout_event_queue`:** Eventos com valor conflitante, status divergente ou órfãos requerem resolução manual documentada por operador com razão e evidência; por projeto seguro, a fila nunca movimenta saldos de forma autônoma.
