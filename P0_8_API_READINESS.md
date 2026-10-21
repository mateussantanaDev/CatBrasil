# P0.8 — Inicialização da API e prontidão do banco

**Estado em 18/10/2026: corrigido no código atual e verificado localmente.** O item 8 da auditoria descrevia a imagem antiga da stack `catbrasil`: conexão inicial falhava, repositórios eram criados como `nil` e `/health` anunciava 200 enquanto rotas com banco quebravam. A stack legada ainda está em execução com essa imagem antiga; não foi substituída neste trabalho.

## Comportamento do código atual

- A API encerra o processo se `postgres.NewDB` não conectar ou retornar pool vazio. A criação dos repositórios ocorre somente depois da conexão e da validação do schema por `migrations.CheckCurrent`.
- `/health` é **liveness**: responde 200 enquanto o processo HTTP está vivo, mesmo durante uma queda transitória do banco.
- `/ready` é **readiness**: faz `Ping` no PostgreSQL e confere o schema; responde 503 se a dependência estiver indisponível. O healthcheck Docker da API usa `/ready`.
- Após uma queda transitória, o pool `pgxpool` recupera a conexão sem recriar repositórios, e `/ready` volta a 200.

## Prova reproduzível

```sh
python3 scripts/release/prove_api_readiness.py
```

O script compila a API atual, prova que ela **não abre HTTP com um hostname PostgreSQL não resolvível**, cria PostgreSQL 16 descartável em porta de loopback fixa durante o teste, aplica as migrações atuais e inicia a API com dados sintéticos. Em seguida confirma `/health=200`, `/ready=200` e catálogo 200; para o PostgreSQL, confirma `/health=200` e `/ready=503`; reinicia o banco e confirma `/ready=200` e catálogo 200. Ele encerra a API e remove apenas o contêiner de prova.

Resultado local: **PASS em três execuções consecutivas**, com `go test -count=1 ./...` também aprovado. A prova não executa o corte nem altera a stack legada. A atualização da instância antiga depende do plano de implantação e dos portões P0.4–P0.6; até lá, `/health=200` nela não indica prontidão.
