# P1.7 — Healthcheck dos frontends

Os Dockerfiles cliente e admin usam `wget` em `http://127.0.0.1/healthz` com instrução `HEALTHCHECK` em formato exec. O Compose também usa IPv4 explícito. O Nginx responde 200 em `/healthz` pela interface IPv4.

Diagnóstico em 19/10/2026: os containers de staging `catbrasil-staging-app-client-1` e `catbrasil-staging-app-admin-1` estavam `healthy` e usavam o healthcheck IPv4. Os containers legados `catbrasil_client_app` e `catbrasil_admin_app` estavam `unhealthy`; o Docker registrava tentativa em `[::1]:80` com conexão recusada. Em todos os quatro containers, `wget http://127.0.0.1/healthz` funcionou e `wget http://localhost/healthz` falhou.

Validação isolada das imagens construídas do código atual:

```sh
python3 scripts/release/verify_frontend_health.py --build
```

O script cria containers descartáveis sem rede, consulta o estado de saúde do Docker, chama o endpoint dentro do container e remove apenas os containers e tags temporários que criou. A stack legada em execução continua com a imagem antiga até uma recriação controlada; não alterá-la como parte deste teste.

## Resultado da verificação independente

- `client: /healthz respondeu e Docker marcou healthy`
- `admin: /healthz respondeu e Docker marcou healthy`
- Containers descartáveis (`--network none`) e tags temporárias removidos após o teste;
- Nenhuma stack ou container em execução foi alterado.

