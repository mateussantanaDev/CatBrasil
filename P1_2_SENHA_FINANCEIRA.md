# P1.2 — Senha financeira

## Contrato

- `GET /api/v1/user` e `GET /api/v1/user/profile` devolvem `hasSecurityPassword: boolean`.
- Respostas que serializam `User` (login/cadastro) também devolvem esse indicador. PIN e hash não são serializados.
- `POST /api/v1/user/security-password` recebe `password` (novo PIN de seis dígitos), `accountPassword`, `currentPassword` (obrigatório na troca) e `twoFactorCode` (obrigatório se o 2FA estiver ativo).
- Senha da conta incorreta retorna `403 REAUTH_FAILED`, sem encerrar a sessão; a rota tem limite de cinco tentativas por minuto por IP e por conta na instância da API.

## Persistência

- O Go armazena e compara o PIN com bcrypt. A migração `000005` converte PINs legados de seis dígitos; `000012` descarta valores inválidos restantes e impõe uma restrição que rejeita texto simples.
- O servidor Node legado, habilitável apenas em desenvolvimento, migra credenciais SQLite antigas para bcrypt antes de servir requisições e usa bcrypt na troca e no saque.
- O `db.json` de demonstração deixou de conter um PIN preenchido.

## Verificação

- `go test ./...`
- `npm run build:client`
- `python3 scripts/release/prove_migrations_empty_db.py` em PostgreSQL 16 descartável, incluindo rejeição de PIN em texto simples.

Os contêineres em execução não foram atualizados nesta etapa. Aplicar a migração antes de publicar a nova API e o novo bundle do cliente.
