# Conciliação interna diária self-hosted

Instale as unidades em `/etc/systemd/system/` no host que executa o Compose de produção. Crie `/etc/catbrasil/internal-reconcile.env` com permissão `0600`:

```ini
RECONCILE_REPORT_DIR=/var/lib/catbrasil/internal-reconciliation
RECONCILE_STATUS_FILE=/var/lib/node_exporter/textfile_collector/catbrasil_internal_reconcile.prom
RECONCILE_RETENTION_DAYS=90
```

O host deve ter Python 3, Docker Compose e acesso local a `/opt/catbrasil/current/.prod.env`. O processo consulta exclusivamente o projeto `catbrasil-prod`, o banco `catbrasil_prod` e o volume `catbrasil_prod_pgdata`; a auditoria usa transação `REPEATABLE READ READ ONLY`. O diretório de relatórios precisa ser privado (`0700`); os relatórios, marcador diário e arquivo de métricas ficam em `0600`. Configure o node exporter para ler o diretório textfile.

Execute `systemctl daemon-reload`, `systemctl enable --now catbrasil-internal-reconcile.timer` e `systemctl start catbrasil-internal-reconcile.service`. Confira `systemctl status catbrasil-internal-reconcile.service` e as métricas. A rotina tenta a cada 30 minutos em UTC e registra a auditoria da fotografia do banco após o fechamento do dia UTC anterior; após sucesso, novas chamadas para esse dia não repetem a consulta. Falhas e divergências deixam `catbrasil_internal_reconcile_last_run_success 0` e permitem nova tentativa. A retenção de 90 dias é aplicada somente após auditoria bem-sucedida; guarde evidências externas conforme sua política de auditoria.

Alertas recomendados: sucesso `0`, ausência da série, e `time() - catbrasil_internal_reconcile_last_success_timestamp_seconds > 90000`. O arquivo `.prom` não contém saldos ou identificadores. Os relatórios incluem agregados e pseudônimos de investigação, então devem ficar em storage privado com acesso restrito.

**Limite:** a rotina confere a cadeia e os saldos internos. Ela não verifica o extrato oficial da SuitPay, liquidação bancária ou saldo de custódia; esse controle externo continua pendente antes de operar pagamentos reais.
