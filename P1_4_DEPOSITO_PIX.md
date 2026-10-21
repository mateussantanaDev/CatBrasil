# P1.4 — Depósito PIX falha fechado

- `PIX_MODE=simulated` só gera QR de demonstração com `APP_ENV=development`. O caso de uso rejeita a combinação também quando é chamado fora da configuração normal da API.
- `APP_ENV=staging` com `PIX_MODE=suitpay` exige sandbox SuitPay ativo e `PIX_SANDBOX_URL` isolada antes de criar uma intenção ou chamar o gateway. Em produção, sandbox é rejeitado e a chave `SUITPAY_LIVE_ENABLED` continua obrigatória.
- Erro de comunicação, resposta incompleta ou falha de persistência no fluxo real nunca devolvem QR. A rota retorna HTTP 503 com `PIX_PROVIDER_UNAVAILABLE`, sem detalhes internos. Quando a submissão ao provedor pode ter ocorrido, a intenção permanece registrada para conciliação, sem inventar um PIX local.
- Callback de uma intenção anterior continua permitido mesmo quando novas cobranças estão bloqueadas.

Verificação: `cd backend && go test ./...`. Os testes cobrem bloqueio da simulação em produção, isolamento de staging, timeout da SuitPay, resposta incompleta e contrato HTTP 503 sem QR. O teste usa gateway local controlado; a validação ponta a ponta com SuitPay depende das credenciais e acesso ao sandbox oficial.
