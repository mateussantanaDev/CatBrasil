# Runbook P0.5 — Prova de Integração no Sandbox Oficial SuitPay

**Data:** 17/10/2026  
**Status da Etapa P0.5:** **PREPARAÇÃO TÉCNICA E VALIDADOR LOCAL REVISADOS E ACEITOS LOCALMENTE (129 TESTES PYTHON PASS) / ENSAIO OFICIAL E AUTENTICAÇÃO DAS EVIDÊNCIAS BLOQUEADOS (SEM CREDENCIAIS/IP/WEBHOOK) / P0.5 NÃO CONCLUÍDA**  
**Gate de Produção:** `SUITPAY_LIVE_ENABLED: "false"` literal mantido em `compose.prod.yml` e `compose.suitpay-sandbox.yml`.

---

## 1. Contexto e Delimitação do Ambiente

### 1.1 Diferença Crítica: Stub Local de Staging vs. Sandbox Oficial
O ecossistema CatBrasil possui duas fronteiras distintas de homologação que **não se confundem**:

1. **Staging Local com Stub (`compose.staging.yml`):**
   - Utiliza o serviço interno simulado `pix-sandbox` (`http://pix-sandbox:8080/api/v1/gateway`).
   - Simula payloads síncronos e assíncronos em rede Docker interna fechada.
   - Não atesta conectividade externa, não valida certificados TLS públicos, não consome quota ou regras de negócio reais do provedor e não utiliza o segredo criptográfico oficial da SuitPay.
   - Etapas P0.1, P0.2 e P0.3 foram aprovadas localmente sob este modelo e testes transacionais descartáveis.

2. **Sandbox Oficial SuitPay (`compose.suitpay-sandbox.yml`):**
   - **Ausência total de stub:** O serviço `pix-sandbox` é expressamente omitido.
   - Comunicação real de saída contra os servidores do provedor: `https://sandbox.ws.suitpay.app/api/v1/gateway` (acionada quando `PIX_SANDBOX_URL=""`, `APP_ENV="staging"` e `PIX_MODE="suitpay"`).
   - Exige resolução DNS externa, certificado TLS válido emitido publicamente e entrega de webhooks HTTP POST via internet pública pelo gateway da SuitPay.

### 1.2 Alerta Operacional: Conflito de Portas no Host Atual
- No host atual de staging/legado, o container `catbrasil_caddy` **já ocupa as portas 80 e 443 do host** (`0.0.0.0:80` e `0.0.0.0:443`).
- O overlay `compose.suitpay-sandbox.yml` publica obrigatoriamente as portas 80 e 443 para terminação TLS automática do Caddy e recepção dos webhooks do provedor.
- **PROIBIÇÃO:** **NUNCA execute `docker compose up` com o overlay de sandbox neste host compartilhado sobre a stack legada.** Haverá colisão imediata de bind de socket e risco de indisponibilidade de serviços existentes.
- **DIRETRIZ DE DEPLOY:** O ensaio oficial do sandbox exige **host dedicado (VPS limpa)** ou **roteamento reverso externo deliberado** com IP público exclusivo. O ambiente de staging existente não deve ser tocado nem alterado.

---

## 2. Pré-requisitos Obrigatórios e Bloqueios Vigentes

Em **17/10/2026**, o operador confirmou a **ausência** dos pré-requisitos externos essenciais. O ensaio oficial está formalmente **BLOQUEADO** até o atendimento integral dos seguintes itens:

| Pré-requisito | Situação em 17/10/2026 | Impacto Técnico |
| --- | --- | --- |
| **Credenciais de Sandbox (`ci` e `cs`)** | **NÃO DISPONÍVEIS** | Impossibilita autenticação em qualquer chamada à API SuitPay. |
| **IP de Saída Liberado no Painel** | **NÃO CADASTRADO** | A SuitPay bloqueia endpoints de Cash-Out (`/pix-payment`) se o IP público de origem não constar na whitelist cadastrada. |
| **FQDN e Webhook HTTPS Público** | **NÃO CONFIGURADOS** | A SuitPay exige webhook público acessível. O endpoint obrigatório é `https://{API_DOMAIN}/api/v1/webhooks/suitpay`. |
| **DNS e TLS Públicos** | **NÃO CONFIGURADOS** | Caddy não emite certificado ACME para domínios sem apontamento DNS do tipo A/AAAA válido na porta 80/443. |
| **Etapa P0.4 (Identidade Civil e SMS)** | **PENDENTE** | O envio de Cash-Out exige titularidade validada (`documentValidation`). Sem a esteira civil e entrega SMS de P0.4, testes E2E reais com usuários finais estão bloqueados. Testes de sandbox devem usar estritamente fixtures sintéticas segregadas. |
| **Operador e Revisor Distintos** | **A DEFINIR** | Segregação obrigatória de funções: quem dispara ou solicita um saque de teste não pode ser o mesmo que aprova no painel administrativo. |

### 2.1 Higiene Estrita de Segredos e Dados
1. **Zero vazamento:** É terminantemente proibido digitar, colar ou versionar Client ID (`ci`), Client Secret (`cs`), senhas de banco ou chaves JWT em mensagens de chat, commits de Git, logs ou documentação pública.
2. **Dados Pessoais:** Jamais utilize CPFs reais de terceiros ou dados de produção. Utilize geradores de dados sintéticos válidos para testes de desenvolvimento.
3. **Extratos Brutos:** Não faça upload nem cole dumps de extratos brutos com dados bancários reais.
4. **Proteção de Segredos no ERP:**
   - O endpoint `GET /api/v1/admin/config` omite segredos por design, retornando apenas flags como `suitpayClientSecretConfigured: true`.
   - O endpoint `PUT /api/v1/admin/gateway` implementa preservação atômica (`COALESCE(NULLIF(..., ''), suitpay_client_secret)`), permitindo atualizar outros parâmetros enviando o segredo vazio sem sobrescrever a chave existente.
   - **Mandatório:** Antes de inserir as credenciais oficiais no banco, validar em execução de teste que o comportamento de ocultação e preservação está ativo.

---

## 3. Especificação do Contrato Oficial SuitPay

Referência oficial da API: `https://pix.suitpay.app/` e `https://sandbox.ws.suitpay.app/api/v1/gateway`.

### 3.1 Autenticação
- Toda requisição HTTP da CatBrasil para a SuitPay utiliza os headers HTTP:
  - `ci`: Client ID fornecido pelo painel do sandbox.
  - `cs`: Client Secret fornecido pelo painel do sandbox.
  - `Content-Type`: `application/json`.

### 3.2 Cash-In (Depósito PIX)
- **Endpoint:** `POST /api/v1/gateway/request-qrcode`
- **Payload enviado:**
  ```json
  {
    "requestNumber": "DEP-UUID-LOCAL",
    "dueDate": "AAAA-MM-DD",
    "amount": 10.00,
    "callbackUrl": "https://{API_DOMAIN}/api/v1/webhooks/suitpay",
    "client": {
      "name": "Nome Sintetico Teste",
      "document": "CPF_SINTETICO_VALIDO",
      "email": "teste@sandbox.local",
      "phoneNumber": "11999999999"
    }
  }
  ```
- **Resposta da API (síncrona):**
  - Campos: `response` ("OK"), `idTransaction`, `paymentCode` (PIX copia e cola), `paymentCodeBase64` (QR code).
- **Notificação Webhook (assíncrona - recebida em `/api/v1/webhooks/suitpay`):**
  - Campos: `idTransaction`, `typeTransaction` (`"PIX"`), `statusTransaction`, `value`, `requestNumber`, `hash`.
  - Status possíveis para Cash-In:
    - `PAID_OUT`: Pagamento confirmado. Credita saldo no ledger local de forma idempotente (`outcome: applied` ou `duplicate`).
    - `CHARGEBACK`: Contestação/estorno. Se o depósito já foi pago, executa estorno contábil auditado; se o webhook chegar antecipado, cancela a transação em aberto.

### 3.3 Cash-Out (Transferência PIX / Saque)
- **Endpoint:** `POST /api/v1/gateway/pix-payment`
- **Payload enviado:**
  ```json
  {
    "value": 50.00,
    "key": "CHAVE_PIX_DESTINO",
    "typeKey": "document",
    "documentValidation": "CPF_NORMALIZADO_TITULAR",
    "externalId": "WDR-UUID-LOCAL",
    "callbackUrl": "https://{API_DOMAIN}/api/v1/webhooks/suitpay"
  }
  ```
- **Validação de Titularidade (`documentValidation`):** O backend CatBrasil envia obrigatoriamente o CPF civil normalizado do titular da conta. A SuitPay rejeita a transferência caso a chave PIX informada pertença a titular com documento diferente.
- **Resposta da API (síncrona):**
  - `response`: `"OK"` ou `"DOCUMENT_VALIDATE"`.
  - `idTransaction`: Identificador da transação no provedor (presente em respostas `"OK"`; respostas `"DOCUMENT_VALIDATE"` podem não conter `idTransaction`).
  - `status`: Status inicial retornado pela API.
  - **DIFERENÇA ESSENCIAL:** `DOCUMENT_VALIDATE` é uma resposta síncrona de erro/divergência documental emitida pela API da SuitPay no envio, **NÃO é um status de webhook assíncrono**. Respostas `DOCUMENT_VALIDATE` podem não conter `idTransaction` (ex.: `{"response":"DOCUMENT_VALIDATE","message":"Chave PIX não pertence ao documento informado"}`). Conforme validado em `payment_usecase_test.go`, a transação local **não sofre cancelamento nem estorno automático**; o status permanece como `processando` sob conciliação para tratamento pelo operador, prevenindo cancelamentos indevidos sem prova transacional inequívoca.
- **Notificação Webhook de Saque (assíncrona):**
  - Campos: `idTransaction`, `typeTransaction` (`"PIX_CASHOUT"` ou `"CASHOUT"`), `statusTransaction`, `value`, `hash`.
  - Status possíveis para Cash-Out:
    - `PAID_OUT`: Transferência PIX efetivada no banco de destino. Finaliza o saque com status `sucesso` (`outcome: applied` ou `duplicate`).
    - `CANCELED`: Transferência recusada/cancelada pela SuitPay ou sistema bancário. O campo `value` do webhook reporta o valor **líquido** (`NetAmount`). No cancelamento do saque, a conciliação realiza o estorno contábil do valor **bruto** originalmente reservado (recompondo valor líquido + taxa cobrada, ou seja, `-withdrawal.Valor`), de forma atômica e idempotente no razão (`outcome: applied`).

### 3.4 Inexistência de Consulta por externalId
- A documentação oficial da SuitPay **NÃO comprova** a existência de endpoint público para consulta de status por `externalId`.
- A reconciliação técnica da CatBrasil baseia-se na trilha auditável:
  1. Identificador local estável (`transactions.id` / `payout_submissions.internal_ref`).
  2. Identificador externo recebido na resposta síncrona (`payout_submissions.external_id` / `idTransaction`).
  3. Fila durável de callbacks e eventos (`payout_gateway_events` e `payout_event_queue`).

### 3.5 Verificação Criptográfica de Assinatura do Webhook
- **Algoritmo:** **SHA-256 simples** (`sha256(cs + valores)`). **NÃO É HMAC**.
- **Regra de Cálculo:**
  1. Concatena o `Client Secret` (`cs`) oficial com os valores literais de todos os campos escalares presentes no JSON, **na ordem exata em que aparecem no corpo recebido**, exceto a chave `"hash"`.
  2. Calcula o hash SHA-256 e compara em tempo constante (`subtle.ConstantTimeCompare`) com o valor hexadecimal do campo `hash`.
  3. Rejeita imediatamente (HTTP 401) qualquer webhook cuja assinatura não confira.

---

## 4. Matriz de Cenários de Teste

> **Aviso:** Todos os cenários estão em estado `not_executed` em 17/10/2026. Nenhum teste contra a API oficial foi realizado. Os valores monetários de teste devem respeitar o valor mínimo permitido pelas regras do sandbox da SuitPay (não presumir R$ 1,00 universal; verificar limites cadastrados no painel).

| ID | Cenário | Ação / Gatilho | Resposta Esperada da API / Webhook | Efeito no Banco de Dados e Razão | Evidência Exigida | Status em 17/10/2026 |
| --- | --- | --- | --- | --- | --- | --- |
| **SC-01** | **Cash-In: Criação de Cobrança** | `POST /request-qrcode` com cliente e valor mínimo válido. | HTTP 200, `response: "OK"`, `idTransaction` retornado, QR Code emitido. | Transação criada com status `pendente`, `gateway_provider: "suitpay"`. | Payload de requisição e resposta com `idTransaction` e horário. | `not_executed` |
| **SC-02** | **Cash-In: Liquidação Normal** | Simulação de pagamento pelo sandbox -> Webhook `type: PIX`, `status: PAID_OUT`. | HTTP 200, `outcome: "applied"`. | Transação atualizada para `sucesso`. Saldo do usuário creditado (+Valor). Log de segurança registrado. | Log do webhook, extrato do sandbox e balanço do usuário. | `not_executed` |
| **SC-03** | **Cash-In: Replay / Duplicidade** | Envio duplicado do webhook de SC-02 com mesmo `idTransaction` e `hash`. | HTTP 200, `outcome: "duplicate"`. | Nenhuma alteração de saldo. Transação permanece em `sucesso`. Idempotência estrita. | Resposta HTTP do webhook indicando desfecho idempotente sem crédito duplicado. | `not_executed` |
| **SC-04** | **Cash-In: Chargeback após Pagamento** | Webhook `type: PIX`, `status: CHARGEBACK` para transação liquidada. | HTTP 200, `outcome: "applied"`, `kind: "chargeback"`. | Transação revertida para `cancelado`. Débito correspondente no razão ou lançamento de estorno auditado. | Evento em `deposit_gateway_events` e registro contábil de reversão. | `not_executed` |
| **SC-05** | **Cash-In: Payload Inválido / Falha** | Envio de requisição com dados truncados ou `amount: 0`. | HTTP 4xx retornado pela SuitPay. | Nenhuma transação pendente órfã no banco local. | Mensagem de erro documentada retornada pela SuitPay. | `not_executed` |
| **SC-06** | **Cash-Out: Saque Automático Direto** | Solicitação de saque pelo usuário com chave PIX válida do titular. | API SuitPay aceita (`response: "OK"`). Saldo reservado. | Saque sai de `pendente` -> submissão registrada com `idTransaction` da SuitPay. | Registro na tabela `payout_submissions` com `external_id` preenchido. | `not_executed` |
| **SC-07** | **Cash-Out: Liquidação de Saque** | Webhook `type: PIX_CASHOUT`, `status: PAID_OUT`. | HTTP 200, `outcome: "applied"`. | Transação de saque transiciona para `sucesso`. Reserva de saldo consolidada definitivamente. | Notificação WebSocket entregue ao frontend e extrato bancário do sandbox. | `not_executed` |
| **SC-08** | **Cash-Out: Aprovação Manual por Admin** | Saque em revisão aprovado manualmente pelo administrador via ERP. | Admin aprova -> trigger de `dispatchPayout` -> `POST /pix-payment`. | Reserva mantida; envio ao gateway ocorre exatamente uma vez sem duplicar requisições. | Log de auditoria administrativa com ID do operador e resposta da API. | `not_executed` |
| **SC-09** | **Cash-Out: Mismatch de Titularidade** | Chave PIX informada associada a CPF diferente do `documentValidation`. | API recusa ou responde com erro documental (`DOCUMENT_VALIDATE`, que pode não conter `idTransaction`). | Saque não sofre cancelamento nem estorno automático; status permanece `processando` sob conciliação (conforme `payment_usecase_test.go`). | Resposta exata da API SuitPay evidenciando recusa e transação mantida em processamento para conciliação. | `not_executed` |
| **SC-10** | **Cash-Out: Cancelamento pelo Provedor** | Webhook `type: PIX_CASHOUT`, `status: CANCELED` (campo `value` reporta montante líquido). | HTTP 200, `outcome: "applied"`. | Transação transiciona para `cancelado`. Estorno contábil bruto da reserva (líquido + taxa cobrada) é creditado ao saldo do usuário. | Evento em `payout_gateway_events` e recomposição exata do saldo bruto reservado. | `not_executed` |
| **SC-11** | **Concorrência: Webhook Fora de Ordem** | Webhook de liquidação chega antes do término da gravação do envio no banco. | Lock transacional (`pg_advisory_xact_lock`) aguarda ou enfileira (`outcome: queued`). | Sem inconsistência: evento é persistido na fila durável sem perda de estado. | Fila `payout_event_queue` ou resolução posterior sem estado inconsistente. | `not_executed` |
| **SC-12** | **Timeout Ambíguo na Chamada de Saque** | Timeout de rede durante o `POST /pix-payment`. | Backend trata como incerto; retém o saque em processamento (`claimed`). | **NÃO cancela, NÃO estorna e NÃO reenvia.** Aguarda reconciliação humana ou webhook assíncrono. | Saque mantido em processamento seguro na fila de conciliação. | `not_executed` |
| **SC-13** | **Conciliação Contábil E2E (Batimento Triplo)** | Comparação de lote com todas as transações executadas nos testes. | 100% dos registros conferem entre: (1) Banco CatBrasil, (2) Extrato SuitPay, (3) Razão Contábil. | Soma(Créditos) - Soma(Débitos) bate centavo a centavo. Nenhum fluxo indefinido. | Relatório consolidado de batimento assinado pelo tech lead e operador. | `not_executed` |

---

## 5. Procedimento de Validação Read-Only (Preflight Local)

Antes de qualquer chamada externa ou subida de contêineres, o operador deve validar a integridade semântica dos arquivos Compose de forma **estritamente read-only**.

### 5.1 Criação do Arquivo de Variáveis Seguro
Crie localmente o arquivo `.suitpay-sandbox.env` com permissões restritas (`chmod 600`) em ambiente isolado:
```bash
touch .suitpay-sandbox.env
chmod 600 .suitpay-sandbox.env
```
Variáveis obrigatórias que devem constar no arquivo (com valores sintéticos/seguros de homologação):
- `DB_NAME=catbrasil_suitpay_sandbox`
- `DB_USER=...`
- `DB_PASSWORD=...`
- `DATABASE_URL=postgres://...:5432/catbrasil_suitpay_sandbox?sslmode=disable`
- `JWT_SECRET=...` (mínimo 32 caracteres)
- `PIX_WEBHOOK_SECRET=...` (mínimo 32 caracteres)
- `ADMIN_SECRET_PIN=...` (mínimo 6 caracteres)
- `IDENTITY_SECRET=...` (mínimo 32 caracteres)
- `CLIENT_DOMAIN=app.sandbox.seu-dominio.com`
- `ADMIN_DOMAIN=admin.sandbox.seu-dominio.com`
- `API_DOMAIN=api.sandbox.seu-dominio.com`
- `ACME_EMAIL=operacao@seu-dominio.com`
- `SUITPAY_CALLBACK_URL=https://api.sandbox.seu-dominio.com/api/v1/webhooks/suitpay`

### 5.2 Execução do Validador Semântico
Execute o script validador desenvolvido especificamente para o sandbox oficial:
```bash
python3 scripts/release/validate_suitpay_sandbox_compose.py --env-file .suitpay-sandbox.env
```
- **O que o validador verifica:**
  - Projeto isolado `catbrasil-suitpay-sandbox`.
  - Volumes dedicados (`catbrasil_suitpay_sandbox_pgdata`, etc.) sem tocar em staging ou produção.
  - Ausência total do stub `pix-sandbox`.
  - Configuração do runtime: `APP_ENV=staging`, `PIX_MODE=suitpay`, `PIX_SANDBOX_URL=""`, `SUITPAY_LIVE_ENABLED="false"`.
  - Fechamento total de portas de banco e API no host.
  - Publicação das portas 80/443 exclusivamente pelo Caddy.
  - FQDN público válido (não localhost) e rota estrita `/api/v1/webhooks/suitpay`.

### 5.3 Validação de Sintaxe sem Vazamento de Segredos
**NUNCA execute `docker compose config` sem filtros**, pois a interpolação exibirá senhas e segredos no stdout do terminal.
Para checagem segura de sintaxe, utilize o flag silencioso:
```bash
docker compose -p catbrasil-suitpay-sandbox --env-file .suitpay-sandbox.env \
  -f docker-compose.yml -f compose.suitpay-sandbox.yml config --quiet
```
*(Exit code 0 confirma sintaxe válida sem produzir saída no console).*

### 5.4 Limitações da Validação Estática
A aprovação do validador comprova **apenas a conformidade da infraestrutura Docker**. Ela **NÃO atesta**:
1. Existência ou validade das credenciais junto aos servidores da SuitPay.
2. Liberação do IP da máquina no firewall/painel da SuitPay.
3. Roteamento DNS ativo ou emissão efetiva de certificados TLS.
4. Liquidação financeira real de depósitos ou saques.

---

## 6. Automação de Evidências e Limites de Auditoria

O repositório contém o script utilitário `scripts/release/suitpay_official_sandbox.py` (modos `preflight` e `evidence`), cuja implementação e testes unitários locais foram revisados e aceitos tecnicamente pelo tech lead (29 testes do validador de evidências e preflight integrados na suíte de 129 testes Python locais com resultado `PASS`). Todavia, o ensaio oficial e a autenticação das evidências permanecem bloqueados pela ausência dos pré-requisitos externos, e a etapa P0.5 NÃO está concluída.

**Delimitação Estrita do Script:**
- O comando `preflight` realiza apenas validação estática das configurações locais do arquivo `.env` (sintaxe de chaves, URLs e flags de ambiente).
- O comando `evidence` valida unicamente a estrutura e consistência interna do manifesto JSON local (`--manifest`), checando tipos de dados, preenchimento de campos obrigatórios, correspondência de sinal contábil entre `amount_cents` e `ledger_delta_cents`, e unicidade de identificadores declarados no próprio JSON.
- **O script NÃO compara hashes, timestamps, arquivos CSV/JSON exportados do provedor nem registros de banco de dados.** Ele apenas valida o manifesto local.
- **O script NÃO atesta nem comprova autenticidade jurídica, bancária ou operacional das evidências.**
- A comprovação de autenticidade exige **conferência manual obrigatória por operador financeiro credenciado**, logado diretamente no painel web oficial da SuitPay, confrontando os saldos e as movimentações de teste registradas.

---

## 7. Critérios Objetivos de Saída (Aceite de P0.5)

A etapa P0.5 só poderá ser declarada **CONCLUÍDA** quando todos os critérios abaixo forem atendidos e assinados pelo tech lead:

1. [ ] Host dedicado ou infraestrutura de rede externa provisionada sem conflito de portas com serviços existentes.
2. [ ] Credenciais de sandbox oficiais ativas e IP de saída autorizado pela SuitPay.
3. [ ] DNS público apontado e certificado HTTPS ativo em `https://{API_DOMAIN}/api/v1/webhooks/suitpay`.
4. [ ] Todos os 13 cenários da Matriz de Testes (SC-01 a SC-13) executados com resultado `PASS`.
5. [ ] Batimento triplo (Banco Local, Extrato do Provedor e Razão Contábil) com divergência zero em centavos.
6. [ ] Relatório de evidências preenchido com todos os identificadores reais gerados no sandbox (`idTransaction`, `requestNumber`, `externalId`).
7. [ ] Conferência humana de autenticidade do extrato no dashboard oficial da SuitPay realizada por operador distinto do executor dos testes.
8. [ ] Manutenção estrita do portão de produção `SUITPAY_LIVE_ENABLED="false"`.
