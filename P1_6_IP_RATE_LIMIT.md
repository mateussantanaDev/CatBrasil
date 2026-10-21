# P1.6 — IP confiável e limite compartilhado

## Fluxo de origem

O roteador não usa `chi.RealIP`. Ele obtém o IP do socket e só usa `X-Forwarded-For` quando o peer pertence a `TRUSTED_PROXY_CIDRS`. O cabeçalho deve conter um único IP válido. `X-Real-IP` e cadeias enviadas pelo cliente não determinam a identidade. Cadastro, login, logs administrativos e limites por IP usam a mesma identificação.

Nos overlays de produção, staging e sandbox SuitPay, Caddy recebe um IP fixo em uma rede `proxy_net` dedicada. A API confia somente nesse IP `/32`. Caddy substitui `X-Forwarded-For` pelo endereço do peer externo antes de encaminhar. A API não publica porta no host nesses overlays. O Compose de desenvolvimento continua com porta local para testes, sem confiar em proxy por padrão.

Se o subnet `172.30.240.0/28` (produção), `172.30.241.0/28` (staging) ou `172.30.242.0/28` (sandbox SuitPay) colidir com a rede do host, altere juntos o subnet, IP fixo do Caddy, `TRUSTED_PROXY_CIDRS` e as constantes do validador de corte antes do deploy. Não amplie a confiança para toda a rede Docker.

## Limite entre réplicas

A migração `000013` cria `http_rate_limits`. Todos os limitadores da API usam UPSERT atômico no PostgreSQL, com chave SHA-256 e janela fixa. Se o banco falhar, rotas limitadas respondem 503 (`RATE_LIMIT_UNAVAILABLE`); `/health` continua liveness e `/ready` continua verificando o banco e schema. Registros expirados são removidos periodicamente. Com múltiplas réplicas da API apontando para o mesmo PostgreSQL, o teto é global.

## Verificação

- `cd backend && go test ./...`
- Teste de concorrência real: `CATBRASIL_TEST_DATABASE_URL=<postgres_descartável> go test ./internal/repository/postgres -run '^TestRateLimitStoreSharedAndAtomic$' -count=1`
- `docker compose -f docker-compose.yml -f compose.prod.yml config --format json` deve mostrar `backend-api.ports` vazio e Caddy na rede `proxy_net` com IP `172.30.240.2`.
- Caddyfiles validados com `caddy validate` da imagem `caddy:2.8-alpine`.

Esta alteração não foi implantada na instância em execução.
