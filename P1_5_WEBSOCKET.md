# P1.5 — Contrato WebSocket

O servidor e os dois clientes web usam o envelope `{ "event": "...", "payload": { ... }, "userId": "..." }`. O `userId` é opcional para comunicados globais. Clientes rejeitam mensagens sem `event` e `payload` válidos; os campos de negócio são lidos de `payload`.

Eventos financeiros consumidos:

| Evento | Campos do payload | Efeito no cliente |
| --- | --- | --- |
| `balance_updated` | `newBalance` | Atualiza saldo e mostra valor |
| `pix_confirmed` | `paidAmount`, `outcome: "applied"` | Mostra confirmação e notificação local |
| `withdraw_paid` | `paidAmount`, `outcome: "applied"` | Mostra liquidação |
| `withdraw_rejected` | Transação ou resultado do webhook | Mostra cancelamento/estorno |

`pix_duplicate` e eventos `queued` não anunciam novo crédito. O webhook simulado agora marca `applied` e `duplicate` para distinguir primeiro processamento de replay.

Validação: `cd backend && go test ./...`; `cd frontend && npm --workspace=@catbrasil/client run test:realtime && npm run build:client`.
