# P0.7 — Migrações completas em banco vazio

**Estado em 18/10/2026: resolvido e verificado localmente.** O item 7 da auditoria descrevia um estado anterior: `backend/migrations/migrations.go` embutia apenas `000001_init_schema.up.sql` e o CLI não aplicava as alterações exigidas pelos repositórios. Esse arquivo hoje contém somente fixtures legadas para um teste da migração inicial. O caminho de execução é `backend/migrations/runner.go`.

## Contrato atual

- `Available()` embute todos os arquivos `*.up.sql`, exige versões contínuas e calcula o SHA-256 de cada arquivo.
- `Apply()` executa as migrações pendentes em ordem, sob transação e lock, e registra versão, nome e checksum em `schema_migrations`. Histórico com lacuna ou checksum divergente é rejeitado.
- O CLI `backend/cmd/migrate` chama `Apply()`; a API e o cron chamam `CheckCurrent()` antes de iniciar. `/ready` também confere o schema.
- No Compose base, `backend-api` e `backend-cron` dependem de `backend-migrate` com `condition: service_completed_successfully`. O serviço de migração executa `/app/migrate up`.
- O healthcheck do PostgreSQL no Compose base e no overlay de produção abre uma conexão SQL ao banco configurado e executa `SELECT 1`. Um `pg_isready` isolado pode indicar que o servidor aceita conexões antes de a inicialização terminar de criar o banco; isso gerou uma falha intermitente na primeira versão da prova descartável.

## Prova executada

Execute novamente com:

```sh
python3 scripts/release/prove_migrations_empty_db.py
```

O script cria um PostgreSQL 16 descartável com porta aleatória **somente em 127.0.0.1**, verifica que o schema começa vazio, chama o CLI real duas vezes, confere o histórico e remove o contêiner ao final. Ele também renderiza o Compose sem subir serviços e verifica a dependência da API e do cron.

Resultado local em 18/10/2026: **PASS em cinco repetições consecutivas após a correção de prontidão**; migrações `1,2,3,4,5,6,7,8,9,10`, **26 tabelas**, quatro colunas representativas introduzidas nas versões 2–4 e segunda execução sem alteração do histórico. O Compose renderizado confirmou as dependências do migrador; um contêiner descartável iniciado via Compose confirmou que o healthcheck SQL chega ao Docker e fica `healthy`. `go test -count=1 ./...` em `backend/` também passou. Nenhuma stack existente foi alterada. Os resultados usam banco e credenciais sintéticos; não representam preflight do banco de produção nem homologação SuitPay.

Atualização P0.10: a migração versionada `000011_remove_demo_economy.up.sql` acrescentou saneamento de dados fictícios para banco greenfield. O contrato atual exige versões `1..11`; a prova atualizada em PostgreSQL descartável está descrita em `P0_10_BOOTSTRAP.md`.

O corte real permanece sujeito aos portões P0.4, P0.5 e P0.6 descritos em `P0_ETAPAS.md`.
