# Infraestrutura self-hosted

## Staging isolado

1. Copie `.staging.env.example` para `.staging.env` e preencha os segredos exclusivos (mínimo 32 caracteres para JWT, webhook, SMS e observabilidade). A senha do banco na `DATABASE_URL` deve ser URL encoded e corresponder a `DB_PASSWORD`.
2. Crie a rede isolada de staging (idempotente) e execute:

   ```sh
   sh docker/staging/prepare-network.sh
   docker compose -p catbrasil-staging --env-file .staging.env -f docker-compose.yml -f compose.staging.yml --profile observability up -d --build
   ```

3. Os sites estão em `http://localhost:8088` (cliente), `http://admin.localhost:8088` (ERP) e `http://api.localhost:8088` (API). Somente Caddy, sandboxes e Prometheus são publicados no loopback. PostgreSQL, API e frontends não publicam portas diretamente neste projeto.

O override usa `!override` e requer Docker Compose 2.24.4 ou mais recente. O projeto separa os containers, dá nomes exclusivos aos volumes e usa uma rede externa de staging criada pelo script; nunca reutilize o volume ou a rede de produção. A rede externa evita um problema de labels de redes no Compose v5. Os nomes de volume da configuração base foram mantidos para preservar dados de instalações existentes. Staging não ativa TLS e deve permanecer acessível apenas localmente ou por túnel seguro.

## Sandboxes

O SMS sandbox aceita `POST /send` com `Authorization: Bearer $SMS_GATEWAY_TOKEN` e JSON `{"phone":"+5511999999999","message":"..."}`. A caixa de entrada está em `GET http://127.0.0.1:18081/messages?phone=%2B5511999999999` com `Authorization: Bearer $SMS_SANDBOX_INSPECT_TOKEN`. A resposta retorna mensagens mais recentes primeiro, com campos `message`, `code` e `createdAt`. O serviço não registra mensagens nem códigos em logs; a caixa de entrada é temporária e fica apenas na memória.

O PIX sandbox recebe os contratos SuitPay `POST /api/v1/gateway/request-qrcode` e `POST /api/v1/gateway/pix-payment`, autentica os headers `ci` e `cs`, e retorna identificadores `STG-*`. Configure no ERP `suitpayClientId`, `suitpayClientSecret`, `suitpaySandbox=true` e `suitpayEnabled=true` com os valores do sandbox. O código copia e cola retornado é deliberadamente inválido para pagamento real. Transações de teste ficam em `GET http://127.0.0.1:18082/transactions` com `Authorization: Bearer $PIX_SANDBOX_INSPECT_TOKEN`. O mock não envia callbacks automaticamente.

Ambos os sandboxes recusam iniciar fora de `APP_ENV=staging`. O backend deve aceitar `PIX_SANDBOX_URL` e HTTP do SMS somente para os hosts internos exatos de staging. Nunca inclua `compose.staging.yml` no comando de produção.

## Produção e observabilidade

O Compose base é somente a base de configuração. **Não execute** `docker compose up` diretamente para produção, nem reutilize o projeto/volume legado `catbrasil`. Prepare `.prod.env` a partir de `.prod.env.example` com segredos externos, domínios, TLS, gateway SMS HTTPS real e referências de imagens por digest. O projeto de corte é `catbrasil-prod` com `docker-compose.yml` **e** `compose.prod.yml`; o overlay isola o volume `catbrasil_prod_pgdata` e impede a publicação direta do banco e da API. Faça primeiro a validação sem mutação:

```sh
scripts/release/cutover_orchestrator.sh --validate-only
```

O corte, quando todos os portões de evidência estiverem aprovados, usa exclusivamente o orquestrador descrito em `FASE5_RELEASE.md`. A variável `SUITPAY_LIVE_ENABLED` permanece `false` até homologação e autorização separadas. O app bloqueia `PIX_MODE=simulated` em produção.

`--profile observability` inicia Prometheus em `127.0.0.1:9090` (staging: `19090`), Alertmanager, relay e node exporter privados. O Prometheus retém sete dias de métricas e autentica o scrape em `/internal/metrics` com `OBSERVABILITY_TOKEN`; o Caddy devolve 404 para essa rota pública. O bind de métricas do host e o destino privado do host de recuperação estão em [prometheus/README.md](prometheus/README.md). Logs Docker rotacionam automaticamente em cinco arquivos de 10 MB por serviço principal. Consulte saúde por `/health` e `/ready`, logs por `docker compose logs`, e métricas pela interface local do Prometheus.

SMS real exige um provedor ou hardware de telefonia e costuma ter custo por mensagem; o sandbox serve apenas para validação local. O receptor descartável provou a entrega apenas em staging. Configure `NTFY_URL` HTTPS e teste o recebimento pelo plantão antes do corte.

## Validação do staging atual

Após construir e iniciar a stack isolada, execute `python3 scripts/release/validate_staging.py`. O verificador lê as migrações SQL da fonte, compara com `schema_migrations` no PostgreSQL de staging e exige serviços ativos e healthchecks aprovados; ele não altera dados. Em seguida execute os smoke tests e a carga leve descritos em `FASE5_RELEASE.md`. A validação local com stubs não substitui identidade/SMS reais nem a homologação da SuitPay.
