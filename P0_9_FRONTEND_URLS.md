# P0.9 — URLs de frontend nos bundles

**Estado em 18/10/2026: corrigido no código e implantado no staging isolado; imagens legadas em execução ainda antigas.**

Cliente e ERP usam `/api/v1` na mesma origem. Os serviços WebSocket montam `/api/v1/ws` com o host atual e `ws:` ou `wss:` conforme o protocolo da página. Os proxies Caddy, Nginx e Vite de desenvolvimento encaminham `/api/*` para a API. O build não depende de `VITE_API_URL`, `VITE_API_BASE_URL` ou `VITE_WS_URL`.

`frontend/scripts/check-bundle-urls.mjs` inspeciona os arquivos `.js`, `.css` e `.html` gerados, exige a rota relativa `/api/v1` e rejeita URL HTTP/WS de loopback e nomes das antigas variáveis Vite. `npm run build:all` verifica cliente e ERP; os dois Dockerfiles verificam os respectivos bundles antes de copiar os arquivos para Nginx.

## Evidência local

- Build local dos dois apps e verificação dos bundles: PASS.
- Build com `VITE_API_URL=http://localhost:8080/api/v1` e `VITE_WS_URL=ws://localhost:8080/ws` definidos no ambiente: PASS; os valores não entraram nos bundles.
- Teste negativo: bundle sintético com `http://localhost:8080/api/v1` foi rejeitado (exit 1).
- O `.env` local deixou de conter as antigas variáveis `VITE_API_BASE_URL` e `VITE_WS_URL` com valores de loopback.
- Imagens de staging `catbrasil-staging-app-client-1` e `catbrasil-staging-app-admin-1`: nenhum bundle com `localhost:8080` ou `ws://localhost`.
- Atualização aplicada somente aos dois frontends do staging em 18/10/2026. Imagens novas: cliente `sha256:adc73c944b6573d211349f946f62de91534ace85f1fd9d345cba78086e047dcf`; ERP `sha256:4334c6bfb6e0440837b185ab3635e9040780df3ea8bca437998f44be821e3c27`. Ambos ficaram `healthy` e passaram em `nginx -t`. Pelo Caddy de staging, as duas páginas responderam 200, `/ready` retornou 200, `/api/v1/shop/catalog` retornou JSON 200 e `/api/v1/ws` sem token retornou 401. O handshake WebSocket com JWT de teste de 60 segundos retornou `101 Switching Protocols` nos dois hosts; o segredo não foi impresso nem salvo.
- Imagens anteriores de staging preservadas para reversão: cliente `sha256:b06ebfd81497b118fb9fcc186c9db3620577654dceb79a3b88f6a5ee09a07734`; ERP `sha256:c872d57bc4e222d3ec341b89b7011ac370b408c006406e8d986c9e547a90aaea`.
- Imagens legadas `catbrasil_client_app` e `catbrasil_admin_app`: cada uma ainda contém um bundle com essas URLs. Não houve troca dessas imagens; a implantação depende do corte P0.6.

Essa implantação comprova o caminho de frontend no staging com gateway simulado. A publicação em produção continua sujeita ao corte P0.6 e não foi executada.
