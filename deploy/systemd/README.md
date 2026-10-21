# Backup operacional self-hosted

Esta rotina usa `pg_dump` em streaming, `age`, SHA-256 e um **filesystem remoto montado**. O host de produção precisa de `python3`, `findmnt`, `docker`, `age` e o repositório instalado em `/opt/catbrasil/current` (ou ajuste `ExecStart` nas unidades). O destino deve ser um mountpoint NFS, CIFS/SMB ou SSHFS dedicado; um diretório local ou bind mount falha antes do dump. A prova de restauração é executada em **outro host** com Docker e acesso de leitura ao mesmo armazenamento.

No host de banco, monte e monitore um servidor de backup independente em `/mnt/catbrasil-backup`. Configure `/etc/catbrasil/backup.env` com permissão `0600`:

```ini
BACKUP_REMOTE_MOUNT=/mnt/catbrasil-backup
BACKUP_ENV_FILE=/etc/catbrasil/production.env
AGE_RECIPIENT_FILE=/etc/catbrasil/backup-recipient.pub
BACKUP_STATUS_FILE=/var/lib/node_exporter/textfile_collector/catbrasil_backup.prom
BACKUP_RETENTION_DAYS=30
```

A chave **privada** `age` não deve ficar nesse host. O status `.prom` deve ser lido pelo coletor textfile do node exporter; alerte quando `catbrasil_backup_last_run_success` for `0`, quando `time() - catbrasil_backup_last_success_timestamp_seconds > 4200`, ou quando não houver série. A periodicidade é 1 hora, sempre no minuto 10 UTC. A política de retenção apaga pares antigos somente após um backup novo concluído; preserve cópias independentes e imutáveis conforme sua política de recuperação.

Instale as unidades da pasta em `/etc/systemd/system/`, execute `systemctl daemon-reload`, depois `systemctl enable --now catbrasil-backup.timer`. Execute `systemctl start catbrasil-backup.service` para a primeira prova e confira `systemctl status`, `journalctl -u catbrasil-backup.service` e o `.prom`. A rotina usa `findmnt --mountpoint` para falhar se a montagem não estiver ativa, inclusive quando o diretório local subjacente existir.

No host **independente** de recuperação, instale o mesmo código e as unidades de restore. Configure `/etc/catbrasil/restore-proof.env` com permissão `0600`:

```ini
BACKUP_REMOTE_MOUNT=/mnt/catbrasil-backup
AGE_IDENTITY_FILE=/etc/catbrasil/backup-identity.key
BACKUP_STATUS_FILE=/var/lib/node_exporter/textfile_collector/catbrasil_restore.prom
BACKUP_RESTORE_MAX_SECONDS=3600
```

O arquivo de identidade deve ter acesso restrito e uma cópia de custódia separada. Execute `systemctl enable --now catbrasil-restore-proof.timer` e uma primeira prova com `systemctl start catbrasil-restore-proof.service`. A prova semanal restaura o último backup em PostgreSQL descartável, sem rede, registra RTO e idade do backup em `*.restore-proof.json` no mount, e publica métricas. Alerte para `catbrasil_restore_proof_last_run_success == 0`, prova ausente ou com mais de 8 dias e RTO acima da meta acordada.

**Limite da evidência local:** mocks e a prova com contêineres no mesmo computador validam o procedimento e o schema, mas não comprovam transferência para outro host nem resistência à perda do host de produção. Antes do corte, registre uma prova real feita no host de recuperação, o horário do último backup íntegro, o RPO observado e o RTO medido.
