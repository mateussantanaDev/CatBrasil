# Métricas operacionais do release

O perfil `observability` coleta três arquivos `.prom` produzidos por `systemd`:

| Origem | Arquivo | Job Prometheus | Limite de atraso |
| --- | --- | --- | --- |
| Host de produção | `catbrasil_backup.prom` | `catbrasil-host` | 70 minutos |
| Host de produção | `catbrasil_internal_reconcile.prom` | `catbrasil-host` | 25 horas |
| Host independente de recuperação | `catbrasil_restore.prom` | `catbrasil-recovery` | 8 dias |

Crie `/var/lib/node_exporter/textfile_collector` no host de produção antes de subir o Compose. O bind é obrigatório e `create_host_path: false` impede que o Docker crie um diretório vazio silenciosamente. Instale os serviços e timers descritos em `deploy/systemd/README.md` e `deploy/systemd/INTERNAL_RECONCILIATION.md`. Rode cada serviço uma vez para publicar o primeiro status. O contêiner local do node exporter monta somente essa pasta, como leitura, sem porta publicada. Ele roda como UID 0 porque os escritores criam arquivos `0600`; não tem capacidades Linux nem privilégio adicional.

O host **independente** de recuperação precisa de seu próprio node exporter com coletor textfile apontando para `/var/lib/node_exporter/textfile_collector`. Configure `RECOVERY_NODE_EXPORTER_TARGET=nome-privado:9100` no ambiente de produção. Exponha essa porta somente na rede privada/VPN e restrinja o firewall à origem do Prometheus; nunca a publique na internet. Para integridade e confidencialidade do tráfego entre hosts, use um túnel VPN. Se o destino estiver vazio, o arquivo de descoberta não terá targets, e os alertas de ausência da prova de restore continuarão ativos. Um destino configurado, mas inacessível, também gera alerta.

Os alertas separam falha da última execução, métrica ausente, atraso do último sucesso e falha de coleta. Nos primeiros minutos de um staging novo, os alertas de backup, reconciliação e restore podem ficar ativos até que suas respectivas provas rodem. Um alerta resolvido não prova custódia externa; antes do corte, verifique o relatório de restore no host independente e registre RPO/RTO reais. A conciliação interna verifica o livro do sistema; a conciliação com extrato oficial da SuitPay segue como etapa separada.

Cheque configuração e regras com `promtool check config /etc/prometheus/prometheus.yml` no contêiner Prometheus, depois confira `up{job="catbrasil-host"}`, `up{job="catbrasil-recovery"}` e as séries dos três jobs na UI local do Prometheus. Teste a entrega dos alertas ao destinatário configurado antes do corte.
