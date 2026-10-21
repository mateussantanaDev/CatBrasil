# Fase 1 — contenção de risco

Estado em 16/10/2026: alterações no código local, sem rebuild ou publicação da stack em execução e sem alteração dos dados existentes.

## Controles implementados

- `APP_ENV` e `PIX_MODE` explícitos. Produção só aceita `PIX_MODE=suitpay`; segredos JWT, webhook e PIN administrativo são obrigatórios. Compose não fornece senhas de fallback.
- Rotas de simulação, autocadastro e SMS de teste só são registradas em `development` com `PIX_MODE=simulated`. A simulação de webhook também exige JWT administrativo.
- Webhook genérico exige HMAC; callback SuitPay exige hash válido conforme o provedor. Crédito requer status confirmado exato, transação pendente do tipo depósito PIX e valor igual ao cobrado.
- WebSocket exige JWT no subprotocolo `bearer`; a sala é determinada pelo token e a origem do navegador é conferida. JWT por query string foi removido.
- Login ERP usa a API administrativa e exige PIN configurado no servidor. Fallback de credenciais e preenchimento demo foram removidos. A migração inicial não cria contas ou saldos de demonstração.
- O primeiro diretor é criado pelo CLI interativo `/app/bootstrap-admin`, depois do preflight greenfield e antes de abrir serviços públicos; senha bcrypt e TOTP são obrigatórios. Consulte `P0_10_BOOTSTRAP.md`.
- API e cron falham se o banco não conectar. O servidor Node legado só inicia com `APP_ENV=development` e `ALLOW_LEGACY_SERVER=1`.
- A API de cobrança SuitPay não cai para QR simulado após erro. Em produção, depósitos reais ficam bloqueados até cadastrar CPF/CNPJ verificado, e saques e aprovações manuais ficam bloqueados até haver persistência transacional e conciliação.

## Para preparar o próximo ambiente

1. Definir todas as variáveis de `.env.example` com valores próprios. Em Compose, `DATABASE_URL` precisa apontar para `postgres`; codificar caracteres especiais da senha na URL.
2. Substituir JWT e demais segredos usados anteriormente; tokens antigos só deixam de valer após rotação de `JWT_SECRET`.
3. Auditar contas já existentes, especialmente as que vieram dos seeds antigos. A alteração da migração não remove registros de um banco já criado. Desabilitar ou substituir essas contas com um procedimento revisado que preserve histórico financeiro.
4. Executar as fases seguintes da auditoria antes de ativar produção: migrações versionadas, documento verificado do pagador, SMS real, operações financeiras ACID, idempotência, conciliação, testes integrados com o gateway e validação de deploy.

Os testes automatizados cobrem recusa de rotas de desenvolvimento, assinatura de webhook, isolamento de sala WebSocket, PIN administrativo e configuração insegura. Eles não substituem testes de integração com PostgreSQL e SuitPay.
