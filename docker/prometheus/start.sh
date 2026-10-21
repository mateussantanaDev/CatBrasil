#!/bin/sh
set -eu

if [ "${#OBSERVABILITY_TOKEN}" -lt 32 ]; then
  echo 'OBSERVABILITY_TOKEN deve ter pelo menos 32 caracteres' >&2
  exit 1
fi

umask 077
printf '%s' "$OBSERVABILITY_TOKEN" > /tmp/metrics-token
unset OBSERVABILITY_TOKEN

# The recovery host must publish node_exporter only on a private network.
# An unset target leaves the restore-proof metric absent and its alert firing.
if [ -n "${RECOVERY_NODE_EXPORTER_TARGET:-}" ]; then
  if ! printf '%s' "$RECOVERY_NODE_EXPORTER_TARGET" | grep -Eq '^([A-Za-z0-9-]+\.)*[A-Za-z0-9-]+:[0-9]{2,5}$'; then
    echo 'RECOVERY_NODE_EXPORTER_TARGET deve ser host privado:porta' >&2
    exit 1
  fi
  case "$RECOVERY_NODE_EXPORTER_TARGET" in
    localhost:*|127.*|0.0.0.0:*)
      echo 'RECOVERY_NODE_EXPORTER_TARGET não pode apontar para loopback/wildcard' >&2
      exit 1
      ;;
  esac
  printf -- '- targets: ["%s"]\n' "$RECOVERY_NODE_EXPORTER_TARGET" > /tmp/recovery-targets.yml
else
  printf '[]\n' > /tmp/recovery-targets.yml
fi
unset RECOVERY_NODE_EXPORTER_TARGET

exec /bin/prometheus \
  --config.file=/etc/prometheus/prometheus.yml \
  --storage.tsdb.path=/prometheus \
  --storage.tsdb.retention.time=7d \
  --web.listen-address=0.0.0.0:9090
