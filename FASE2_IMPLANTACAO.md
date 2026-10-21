# Fase 2 — implantação e acesso

Estado em 16/10/2026: alterações implementadas e testadas em contêineres descartáveis. A stack CatBrasil já em execução e seu banco de dados não foram modificados.

## Subida da stack

1. Configure `.env` conforme `.env.example` e faça backup do banco existente antes da primeira atualização.
2. Execute `make up` (`docker compose up -d --build`). O serviço `backend-migrate` aguarda o PostgreSQL, aplica as migrações pendentes e precisa terminar com sucesso antes de iniciar API e cron.
3. Consulte `docker compose logs backend-migrate backend-api backend-cron` e `docker compose ps`. Em caso de schema parcial ou checksum divergente, a migração falha e a API não inicia; investigue a divergência antes de alterar dados.
4. Verifique `GET /health` para vida do processo e `GET /ready` para disponibilidade do banco e schema. O healthcheck Docker da API usa `/ready`; os frontends usam `/healthz` via `127.0.0.1`.

O migrador registra versão, nome, checksum e data em `schema_migrations`. Em banco vazio aplica 1–4; em banco legado sem histórico reconhece a versão pela estrutura e registra o baseline sem repetir seeds. Se encontrar lacunas ou estrutura parcial, falha. O comando `down` foi retirado porque só havia rollback para a migração 1, que removeria todas as tabelas.

## Frontends

Cliente e ERP usam `/api/v1` e WebSocket na origem da página. Caddy, Nginx das portas diretas e Vite de desenvolvimento encaminham essas rotas ao backend. O Nginx consulta novamente o DNS do Docker quando a API é recriada. Os builds Vite e Docker rejeitam URLs de loopback embutidas nos bundles.

## Validação feita

- Testes Go e `go vet ./...`; builds Vite e imagens Docker da API e dos frontends.
- PostgreSQL descartável: banco vazio (1–4, execução repetida sem replay), legado v1 (aplica 2–4), legado v4 (baseline 1–4) e schema parcial (recusa).
- API em contêiner descartável: `/health=200` e `/ready=200` com banco; após queda do banco, `/health=200` e `/ready=503`.
- Nginx dos dois frontends: configuração válida, `/healthz=200`, proxy `/api/v1/*` respondendo por upstream descartável e recuperação após recriar o upstream.

Ainda faltam os bloqueios financeiros, de autenticação e de operação descritos em `AUDITORIA_PRODUCAO.md`. A fase 2 não autoriza publicar a plataforma para transações reais.
