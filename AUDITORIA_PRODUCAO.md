# Auditoria de prontidão para produção — CatBrasil

> **Registro histórico:** os achados abaixo descrevem a auditoria inicial. As fases posteriores foram implementadas no código e testadas no staging isolado. Consulte [P0_ETAPAS.md](./P0_ETAPAS.md) para os bloqueios atuais e [FASE5_RELEASE.md](./FASE5_RELEASE.md) para as evidências e o checklist de corte.

**Data:** 16/10/2026. **Escopo:** código local, builds, testes e inspeção somente leitura da stack Docker existente. Nenhuma transação financeira, migração ou alteração no banco em execução foi realizada.

## Veredito

**Não está pronto para produção.** A estrutura funcional existe, mas o fluxo de dinheiro, a autenticação de eventos e o deploy contêm falhas críticas. A instância local confirma indisponibilidade prática: `/health` responde 200 enquanto catálogo, comunicados e configuração da roleta respondem 500.

## O que está implementado e foi verificado

| Área | Evidência | Estado |
| --- | --- | --- |
| Backend Go | `go test ./...` e `go vet ./...` passaram | Código compila e testes existentes passam; não prova integração real |
| Frontends cliente e ERP | `npm run build:all` passou | Bundles gerados, com avisos de acessibilidade e propriedades não usadas |
| Docker Compose | `docker compose config --quiet` passou | Arquivo é sintaticamente válido |
| PostgreSQL local | 16 tabelas; catálogo, comunicados e roleta têm registros | Banco existe; API atual não consegue usá-lo |
| Rotas e regras | Há implementação de cadastro/login, garagem, loja, roleta, ranking, PIX, saques, ERP, níveis e cron | Funcionalidade implementada em código, mas fluxos de ponta a ponta ainda não foram demonstrados nesta auditoria |
| Proteção de rotas ERP | `RequireAdmin` exige claim administrativa | A API restringe essas rotas; a interface ERP tem autenticação inconsistente |

## Bloqueios críticos (P0)

1. **Crédito de PIX pode ser simulado publicamente.** `POST /api/v1/dev/simulate-webhook` está fora do grupo autenticado em `backend/internal/delivery/http/router.go:88` e chama o mesmo processamento que credita o saldo em `backend/internal/delivery/http/handler/financial_handler.go:132`. Remover a rota do build de produção e manter simulação apenas em ambiente de teste isolado.
2. **Rotas de teste permitem fabricar recompensas.** Um operador autenticado pode chamar `/roulette/reset` repetidamente (`backend/internal/delivery/http/router.go:145`; `backend/internal/delivery/http/handler/roulette_handler.go:73`), tornar máquinas disponíveis para coleta por `/garage/simulate` (`backend/internal/delivery/http/handler/garage_handler.go:68`; `backend/internal/usecase/garage/garage_usecase.go:550`) e ativar indicação por `/referrals/simulate` (`backend/internal/delivery/http/handler/referral_handler.go:56`). Essas rotas alteram dados reais. Removê-las da API de produção e manter testes isolados.
3. **Webhook aceita chamada sem assinatura e estado ambíguo.** A verificação HMAC ocorre somente se o segredo **e** a assinatura vierem preenchidos (`backend/internal/usecase/payment/payment_usecase.go:229`). O callback SuitPay é recebido sem autenticação própria e repassado ao processamento genérico (`backend/internal/delivery/http/handler/financial_handler.go:170`, `backend/internal/usecase/payment/payment_usecase.go:200`). A checagem por substring também trata `UNPAID` como pago (`backend/internal/usecase/payment/payment_usecase.go:256`). Exigir autenticação verificável do provedor, comparar estados exatos e vincular valor, transação e tipo ao registro esperado antes de creditar.
4. **WebSocket permite escutar eventos de outro usuário.** `HandleWS` aceita conexão sem token válido e `join_room` troca o `userID` por qualquer valor enviado pelo cliente (`backend/internal/delivery/ws/hub.go:229`, `:170`). Eventos incluem valores e estados financeiros. Exigir JWT antes do upgrade e eliminar escolha de sala pelo cliente; restringir origem também (`:25`).
5. **Operações financeiras não são atômicas.** `GetByIDForUpdate` usa `pool.QueryRow`, sem transação que mantenha o lock após a leitura (`backend/internal/repository/postgres/user_repository.go:70`; `transaction_repository.go:97`). Depósito atualiza usuário, transação e comissão separadamente, ignorando erros (`backend/internal/usecase/payment/payment_usecase.go:280`). Saque debita antes de criar o registro e pode chamar o gateway antes de persistir (`:393`, `:415`, `:445`). Aluguel e roleta têm o mesmo padrão (`backend/internal/usecase/shop/shop_usecase.go:104`; `backend/internal/usecase/roulette/roulette_usecase.go:131`). Implementar uma unidade transacional por operação, chave de idempotência e testes concorrentes no PostgreSQL.
6. **Aprovação manual de saque não executa nem concilia PIX.** O admin altera qualquer transação para `sucesso`, sem filtrar tipo/status e sem chamada ao gateway (`backend/internal/repository/postgres/transaction_repository.go:221`). A rejeição provavelmente sempre falha: a consulta retorna 17 colunas, mas `Scan` recebe 16 destinos (`:245-252`). Corrigir fila e máquina de estados, transferir/reconciliar no provedor e testar aprovação, rejeição, retry e estorno.
7. **Migração nova não cria o schema exigido pela API.** O CLI embute e executa somente `000001_init_schema.up.sql` (`backend/migrations/migrations.go:5`, `backend/cmd/migrate/main.go:69`), enquanto os repositórios usam colunas/tabelas adicionadas em `000002` a `000004`. O Compose não executa migrações antes da API. Criar migrações versionadas, aplicar todas em ordem, registrar versão e validar um banco vazio.
8. **API sobe sem banco e anuncia saúde falsa.** Falha de conexão deixa repositórios nulos (`backend/cmd/api/main.go:80-118`) e `/health` responde 200 sem checar dependências (`backend/internal/delivery/http/router.go:58`). Nos logs da instância atual, o nome `postgres` não foi resolvido na partida; depois, endpoints com banco retornaram 500 por nil pointer. Falhar na inicialização ou reconectar com backoff e separar liveness de readiness.
9. **Bundles de produção apontam para `localhost`.** As duas APIs frontend usam fallback `http://localhost:8080/api/v1` (`frontend/apps/client/src/services/api.js:1`, `frontend/apps/admin/src/services/api.js:1`), e WebSocket usa `ws://localhost:8080/...` (`frontend/apps/client/src/services/websocketService.js:14`). Essas strings estão nos bundles gerados. `.env.example` define `VITE_API_BASE_URL`, mas o código lê `VITE_API_URL`. Usar URL relativa `/api/v1` e origem atual para WS, ou injetar URLs corretas no build.
10. **Credenciais e dados de demonstração entram no caminho de produção.** Compose/API têm senhas e JWT de fallback (`docker-compose.yml:13`, `:50-51`; `backend/cmd/api/main.go:53-72`). A migração inicial insere usuário gestor e saldos/saques fictícios (`backend/migrations/000001_init_schema.up.sql:315-364`). Remover defaults e seeds de demonstração da trilha de produção; exigir segredos externos e bootstrap administrativo seguro.

## Correções importantes (P1)

1. **ERP autentica a interface sem provar papel administrativo.** O login no browser usa `/auth/login` comum e permite fallback com credenciais fixas (`frontend/apps/admin/src/components/gestao/ErpLoginScreen.svelte:27-40`; `frontend/apps/admin/src/services/gestaoService.js:17-34`). Isso libera a *tela* mesmo sem permissão na API. `ERPLogin` no backend ignora o PIN recebido (`backend/internal/usecase/auth/auth_usecase.go:240-269`). Usar `/admin/auth/login`, validar papel/PIN/2FA no servidor e remover credenciais demo.
2. **Senha financeira em texto simples e exposta na API.** `SecurityPassword` tem JSON público (`backend/internal/domain/user.go:39`), é persistida sem hash (`backend/internal/usecase/auth/auth_usecase.go:271-284`) e comparada diretamente no saque (`backend/internal/usecase/payment/payment_usecase.go:366-372`). Devolver apenas indicador `hasSecurityPassword`, guardar hash e exigir reautenticação para troca.
3. **SMS não verifica posse do número.** `/auth/send-sms` devolve o código no JSON (`backend/internal/delivery/http/handler/auth_handler.go:34-38`), não há envio externo em `SendSMSCode`, e `Register` não exige código (`backend/internal/usecase/auth/auth_usecase.go:78-95`). Criar integração, validação no cadastro e limite de tentativas.
4. **Depósito real cai para QR simulado após erro do provedor.** `CreatePixDeposit` gera EMV com chave sintética quando a SuitPay falha (`backend/internal/usecase/payment/payment_usecase.go:122-146`), mas apresenta sucesso ao usuário. Em ambiente real, falhar fechado e sinalizar indisponibilidade; separar explicitamente sandbox.
5. **Cliente WebSocket não interpreta o formato do servidor.** Backend envia `{event,payload}` (`backend/internal/delivery/ws/hub.go:31-35`); frontend procura `data.type` e `data.paidAmount` (`frontend/apps/client/src/services/websocketService.js:26-35`). Unificar contrato e testar notificações de saldo/PIX/saque.
6. **Rate limit e identificação de IP dependem de cabeçalhos do cliente.** O limitador lê `X-Forwarded-For` diretamente (`backend/internal/delivery/http/middleware/rate_limiter.go:54`) e cadastro/login também (`backend/internal/delivery/http/handler/auth_handler.go:48`). Restringir proxies confiáveis; impedir acesso direto à API fora do proxy; usar limite compartilhado se houver múltiplas réplicas.
7. **Container de frontend aparece como unhealthy.** O healthcheck usa `localhost`, resolvido como `::1` no container, mas Nginx escuta IPv4 (`frontend/Dockerfile.client:36`, `frontend/Dockerfile.admin:36`). Na stack atual, ambos falham repetidamente embora o endpoint pelo host responda 200. Usar `127.0.0.1` e validar no container.
8. **Cron não é resiliente a atraso/restart.** Reset depende de processo vivo entre 00:00 e 00:02 e marca o dia como concluído antes de checar sucesso (`backend/internal/delivery/cron/worker.go:63-81`). Ranking semanal também usa estado só em memória e ID de recompensa fixo por usuário (`:84-119`). Persistir execução por período, idempotência, retry e horário definido.
9. **Segredos aparecem em logs e API de configuração.** API e cron imprimem a `DATABASE_URL` completa (`backend/cmd/api/main.go:79`, `backend/cmd/cron/main.go:55`). `GET /admin/config` serializa credenciais do gateway (`backend/internal/delivery/http/handler/admin_handler.go:399-409`; `backend/internal/domain/config.go:15-23`). Mascarar na leitura e nunca registrar segredos.

## Criar e melhorar antes do lançamento (P2)

- **Testes reais de integração:** banco efêmero com migrações 1–4; concorrência em débito/crédito; webhooks inválidos, repetidos e fora de ordem; contrato API/front; smoke test do build servido por Docker. Hoje os testes de repositório verificam texto SQL ou mocks, não executam as operações financeiras reais (`backend/internal/repository/postgres/schema_validation_test.go`, `db_test.go`).
- **Observabilidade e operação:** readiness com consulta ao banco, logs estruturados sem dados sensíveis, métricas e alertas de erro de gateway/webhook/cron, conciliação diária de saldos e transferências, backup com teste de restauração.
- **Deploy reproduzível:** migração obrigatória antes do start; imagens com versões fixas; variáveis por ambiente; portas PostgreSQL/API restritas à rede necessária; pipeline CI para testes, build e verificação de configuração. O código novo está majoritariamente não rastreado no Git no momento da auditoria.
- **Contratos e UI:** remover código Node/SQLite e `frontend/src` legados ou declarar migração concluída; atualizar `README.md` (ainda descreve Node/SQLite); padronizar DTOs e nomes de campos; remover mocks/fallbacks de produção; corrigir avisos de acessibilidade do build.
- **Modelo financeiro:** usar centavos inteiros ou tipo decimal consistente, limites de valor, IDs únicos robustos, estados explícitos de depósito/saque, chave do provedor real persistida e lançamentos auditáveis.

## Ordem sugerida de implementação

1. **Conter risco imediatamente:** desligar simulação pública, exigir autenticação de webhooks/WS, remover credenciais demo e defaults de produção, separar modo simulado do real.
2. **Consertar implantação e acesso:** migrações completas, API falhar/reconectar com readiness, URLs de frontend relativas, healthchecks corretos.
3. **Reescrever o núcleo financeiro:** transações ACID, idempotência, conciliação e estados; saque manual/automático; testes no PostgreSQL com concorrência.
4. **Fechar autenticação e contrato frontend:** ERP, PIN/2FA, senha financeira, SMS e eventos WS.
5. **Validar release:** fluxo completo em staging com gateway sandbox, restore de backup, observabilidade, testes de carga e checklist de corte.

## Limites desta auditoria

- Testes e builds foram executados sobre o código local. A stack Docker já existente foi apenas inspecionada; sua imagem pode não corresponder exatamente ao código atual.
- Não foi feito depósito, saque, migração ou teste destrutivo no banco existente.
- Integrações reais SuitPay/Paradise e comportamento em um servidor público não foram validados. Portanto, itens "implementados" não significam "aprovados para produção".
