# P2 — estado da preparação para lançamento

Atualizado em 20/10/2026. Este documento registra o que foi **executado** no staging isolado; não autoriza um corte de produção.

## Evidência concluída neste ambiente

| Frente | Resultado |
| --- | --- |
| Banco e concorrência | `scripts/test/integration.sh`: PostgreSQL efêmero, migrações 1–14, suíte Go e teste de concorrência com `-race`. |
| Migração de staging | `validate_staging.py` confirmou serviços saudáveis e migrações 1–14 no banco isolado após rebuild. |
| Fluxo de negócio | `smoke.py --full-sandbox --pix-sandbox --bootstrap-erp` concluiu cadastro SMS local, login, PIN, 2FA, autorização ERP, revisão sintética por outra conta, bloqueio de JWT após remoção de cargo e cobrança no stub PIX. Não comprova KYC nem liquidação externa. |
| Carga limitada | 30 requisições a `/ready`, 2 RPS, 0% de erro e p95 de 20,06 ms neste Mac. |
| Backup e restauração | Dump age/SHA-256 do staging já migrado restaurado em contêiner sem rede; 28 tabelas e versões 1–14 confirmadas. A prova foi local, não em host independente. |
| Alertas | Alertmanager entregou um alerta sintético ao relay e ao receptor descartável de staging. `promtool` aceitou 17 regras e a configuração do contêiner. |
| Precisão monetária | Entradas JSON monetárias são validadas pelo lexema original e cálculos financeiros usam centavos inteiros dentro do limite `NUMERIC(15,2)`; testes de borda, SQL e concorrência passaram. Alguns DTOs e adaptadores ainda transportam `float64` depois da validação. |
| Rotinas operacionais | Backup age em mount remoto verificado, restauração periódica em host separado e conciliação interna diária têm scripts, timers, métricas e testes. Nenhuma instalação de systemd foi feita neste Mac. |
| CI remoto | O PR [#1](https://github.com/mateussantanaDev/CatBrasil/pull/1) aprovou os três jobs de backend, frontend e validação de release no commit `e5a2a57` (run `39531855839`). |

## Portões ainda pendentes no ambiente real

1. **SuitPay oficial:** homologação no sandbox, webhooks reais, extrato oficial e evidência de conciliação externa. O usuário pediu para pular esta etapa por ora; `SUITPAY_LIVE_ENABLED=false` permanece obrigatório.
2. **Infraestrutura externa:** provisionar host de produção, mount remoto de backup, host independente de recuperação, rede privada/VPN para suas métricas e destinatário HTTPS real de alertas; executar restauração e medir RPO/RTO nesses hosts.
3. **SMS e identidade:** configurar o gateway HTTP próprio com entrega real, provar posse do número e executar revisão de identidade humana antes de qualquer movimentação real.
4. **Corte real:** aprovar e integrar o PR após revisão, construir/publicar imagens imutáveis com digests verificados, assinar as evidências de corte e executar o orquestrador apenas quando os portões anteriores passarem.

Os testes locais com dados fictícios não demonstram custódia de dinheiro real nem prontidão regulatória/operacional do ambiente final.
