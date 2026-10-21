#!/bin/sh
set -eu

network=catbrasil-staging_net
if docker network inspect "$network" >/dev/null 2>&1; then
  exit 0
fi

docker network create --driver bridge "$network" >/dev/null
printf 'Rede %s criada.\n' "$network"
