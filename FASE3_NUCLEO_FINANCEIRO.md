# Fase 3 — núcleo financeiro

## Fluxos implementados

- Depósito: callback autenticado, provedor/ID externo/valor conferidos e crédito, rollover, comissão de primeiro depósito, recibo e evento financeiro gravados em uma transação PostgreSQL. Repetir o callback retorna o resultado sem repetir o crédito.
- Saque: `Idempotency-Key` por intenção; validação de saldo, PIN, giro, 2FA e trava global dentro da reserva transacional. `pendente` reserva o valor; `processando` representa uma única tentativa de envio; `sucesso` exige confirmação; `cancelado` devolve a reserva após callback autenticado; `rejeitado` devolve apenas uma reserva ainda não enviada.
- Aprovação manual e automática usam o mesmo envio. O ID interno do saque é enviado à SuitPay como `externalId`, conforme a [documentação oficial](https://pix.suitpay.app/). Uma resposta de rede incerta mantém o saque em `processando` para investigação, sem novo envio cego.
- Callbacks de saída que chegam antes da resposta do envio ficam em `payout_gateway_events`; a gravação do ID externo os concilia na mesma transação.
- Aluguel, coleta, roleta, Vale Seguro, montagem com peças e ajuste manual de saldo agora usam operações atômicas. O ajuste manual registra operador e motivo. O estoque e a carteira são protegidos por locks do PostgreSQL.
- O PIN financeiro é hash bcrypt; a migração converte PINs numéricos legados. Após cinco erros de PIN ou 2FA, novas tentativas ficam bloqueadas por 15 minutos no banco. Respostas JSON de usuário não expõem o hash.
- A fila administrativa de saques filtra somente `Saque PIX` e recebe a lista no formato retornado pela API; depósitos pendentes não aparecem como saque aprovável.
- `users.version` impede que uma atualização de perfil baseada em dados antigos sobrescreva um crédito ou débito recente.

## Migração e operação

1. Fazer backup e executar a migração `000005_financial_integrity.up.sql` pelo migrador da aplicação antes de iniciar a API.
2. Verificar `/ready` e `GET /api/v1/admin/finance/reconciliation` com credenciais administrativas. A resposta aponta saques em processamento há mais de cinco minutos, callbacks sem saque associado e diferença entre o saldo atual e o último evento financeiro do usuário.
3. Para saques em `processando` sem ID externo, consultar a SuitPay pelo `externalId` interno antes de qualquer ação operacional. Nunca reenviar com outro ID sem confirmar que a primeira transferência não ocorreu.
4. Clientes HTTP devem enviar `Idempotency-Key` em saque, aluguel e ajuste manual. Os dois frontends preservam a chave na sessão quando uma resposta se perde e a reutilizam na tentativa seguinte da mesma intenção.

## Validação feita

- `go test ./...` no backend.
- Testes PostgreSQL com `CATBRASIL_TEST_DATABASE_URL=... go test -race ./internal/repository/postgres -run 'TestConcurrent|TestPayoutClaim|TestEarlyCallback|TestClaimAndReject|TestStaleUser|TestRentAndPayout|TestAdminAdjustment|TestPayoutPIN|TestLegacyPIN' -count=1` em PostgreSQL 16 descartável. Cobrem callback duplicado, comissão única, saques e aluguel concorrentes, aprovação versus rejeição, callback antecipado, coleta/roleta/seguro/montagem concorrentes, ajuste idempotente, desvio de saldo e bloqueio de PIN.
- `npm run build:all` nos frontends; os avisos Svelte de acessibilidade e propriedades não usadas já existentes permanecem.

## Pendências antes de liberar dinheiro real

- O depósito real continua bloqueado em produção porque o cadastro ainda não possui CPF/CNPJ verificado do pagador. Não usar documento fictício.
- A integração de saque real precisa de teste fim a fim com credenciais SuitPay, IP autorizado, callback público e conferência do extrato do provedor. Os testes locais validam estado e concorrência, mas não executam transferência real.
- O diário `financial_events` começa nesta migração. Saldos e transações anteriores exigem conciliação de abertura e revisão de saques antigos pendentes antes do corte de produção.
- Um saque com resultado de rede incerto exige investigação humana. O endpoint administrativo apenas apresenta a fila; não confirma nem reenvia valores por suposição.
