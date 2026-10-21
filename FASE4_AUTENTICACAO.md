# Fase 4 — autenticação e contrato de interface

## Gateway SMS próprio

Configure `SMS_GATEWAY_URL` (HTTPS), `SMS_GATEWAY_TOKEN` e `SMS_CODE_SECRET` (32 caracteres ou mais). Em produção, a API recusa inicializar sem esses três valores. O gateway recebe `POST` com `Authorization: Bearer <token>`, `Content-Type: application/json` e corpo:

```json
{"phone":"+5511999999999","message":"Seu código de verificação CatBrasil é 123456. Válido por 10 minutos."}
```

Qualquer HTTP 2xx confirma o aceite do envio. O token e o código não são registrados em log pelo adaptador. O número é normalizado para `+55` seguido de DDD e celular de 9 dígitos. O segredo `SMS_CODE_SECRET` deve ser gerado aleatoriamente e guardado fora do repositório.

`POST /api/v1/auth/send-sms` recebe `{"phone":"11999999999"}` e retorna apenas `success` e `message`, sem código em qualquer ambiente. `POST /api/v1/auth/register` recebe `phone` e `smsCode` além dos campos de cadastro. O código é válido por 10 minutos, tem intervalo mínimo de reenvio de 60 segundos, cinco tentativas e uso único. O banco guarda HMAC-SHA256, sem texto puro. O envio tem limite adicional de 20 solicitações por hora por IP na instância da API. Sem gateway configurado, as rotas públicas de envio e cadastro não são abertas e o caso de uso de cadastro falha fechado. Envio recusado pelo gateway invalida o desafio e permite nova tentativa. O sandbox isolado de staging recebe o SMS para testes, sem provar entrega real a aparelho. Para produção, o gateway próprio precisa de transporte SMS real (modem/SIM ou serviço de telecomunicações), configurado fora do repositório.

A migração `000006_sms_verification.up.sql` invalida os códigos antigos de SMS em texto puro, adiciona contagem de tentativas e reserva um número normalizado para apenas uma conta.

## Autenticação e PIN

`POST /api/v1/auth/login` aceita `twoFactorCode` quando o operador tem 2FA ativo. O ERP usa `POST /api/v1/admin/auth/login` e exige papel administrativo, senha, `ADMIN_SECRET_PIN` externo e TOTP ativo em toda conta de gestão. Sem código, retorna HTTP 401 e `errorCode: TWO_FACTOR_REQUIRED`; código incorreto retorna `INVALID_2FA_CODE`; conta administrativa sem 2FA configurado recebe HTTP 403 e `TWO_FACTOR_NOT_CONFIGURED`. O JWT do ERP expira em 8 horas.

Nas rotas ERP, a API consulta o cargo e o 2FA atuais no banco a cada requisição. A remoção do cargo ou desativação do 2FA bloqueia um JWT administrativo ainda não expirado. Se o repositório de usuários estiver indisponível, a rota responde 503 e não confia apenas no cargo do token. Ao recarregar o ERP, a interface só abre após `/admin/profile` confirmar o papel e o 2FA no servidor; um marcador local ou token de operador não autoriza a tela.

O logout ainda limpa o token do navegador sem revogá-lo no servidor antes do vencimento de oito horas. Revogação de sessão por identificador ou versão de credencial deve ser tratada em uma etapa própria de segurança de sessões; o P1.1 garante validação de papel e 2FA atuais em cada rota protegida.

`POST /api/v1/user/security-password` recebe `password` (novo PIN de seis dígitos), `accountPassword`, `currentPassword` na alteração e `twoFactorCode` quando o 2FA está ativo. A API nunca devolve o hash do PIN. `POST /api/v1/auth/2fa/setup` devolve `secret` e `otpauthUrl` para geração local do QR; o segredo não é enviado a geradores de QR externos. `POST /api/v1/auth/2fa/verify` ativa o 2FA após confirmar o código TOTP.

As telas de operador e ERP oferecem ativação de 2FA com QR gerado no navegador. A sessão ERP embutida no cliente usa token separado em `sessionStorage`, preservando o token do operador. O frontend administrativo também mantém seu token próprio em `sessionStorage`.

## WebSocket

Conexão em `/api/v1/ws` com subprotocolos `['bearer', token]`. Cada frame contém exatamente um JSON `{ "event": "...", "payload": ..., "userId": "..." }`; `userId` aparece em notificações individuais. O hub envia mensagens de operações financeiras apenas ao usuário do JWT e encerra a conexão quando o JWT expira. O cliente deve usar `event` e `payload`, reconectar com token vigente e atualizar os dados da API após eventos.
