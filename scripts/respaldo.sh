#!/usr/bin/env bash
#
# Copia de seguridad de la base, a mano.
#
# Los backups automáticos de Railway son de un plan pago. Hasta que haga falta
# pagarlo, la copia la hace este script: un pg_dump comprimido, con la fecha en
# el nombre, guardado donde vos digas.
#
# Uso:
#   DATABASE_URL="postgres://…" ./scripts/respaldo.sh [carpeta]
#
# pg_dump se niega a hablar con un servidor más nuevo que él, y el de Homebrew
# suele ir una versión atrás del de Railway. Para no depender de eso, corre
# adentro de un contenedor con la versión del servidor. Si Railway te da otra,
# PG_MAJOR=16 ./scripts/respaldo.sh
#
# El DATABASE_URL público sale de las variables del servicio de Postgres en
# Railway (el que dice "Public Network", no el interno: desde tu máquina el
# interno no resuelve).
#
# Restaurar:
#   docker run --rm -i postgres:17-alpine pg_restore --clean --if-exists \
#       -d "$DATABASE_URL" < lit-2026-09-25.dump
#
# Y la parte que casi nadie hace: probar la restauración. Un respaldo que nunca
# se restauró no es un respaldo, es una intención. Levantá el Postgres local
# (docker compose up -d) y restaurá ahí para verlo funcionar sin arriesgar nada.

set -euo pipefail

: "${DATABASE_URL:?falta DATABASE_URL (el público del servicio de Postgres en Railway)}"

destino="${1:-$HOME/respaldos/lost-in-translation}"
mkdir -p "$destino"

archivo="$destino/lit-$(date +%F-%H%M).dump"

imagen="postgres:${PG_MAJOR:-17}-alpine"

# host.docker.internal para que "localhost" adentro del contenedor signifique tu
# máquina y no el contenedor mismo. Contra Railway es indistinto.
url="${DATABASE_URL//localhost/host.docker.internal}"
url="${url//127.0.0.1/host.docker.internal}"

# -Fc: formato comprimido de Postgres, que pg_restore puede leer parcialmente y
# ordenar solo las dependencias. Un .sql plano pesa varias veces más.
docker run --rm -i "$imagen" \
    pg_dump "$url" --format=custom --no-owner --no-privileges > "$archivo"

# Un dump que no se puede leer no sirve, y eso se nota acá o no se nota nunca.
if ! docker run --rm -i "$imagen" pg_restore --list < "$archivo" >/dev/null 2>&1; then
    echo "el dump quedó ilegible: $archivo" >&2
    exit 1
fi

tablas=$(docker run --rm -i "$imagen" pg_restore --list < "$archivo" | grep -c 'TABLE DATA' || true)
echo "✓ $archivo ($(du -h "$archivo" | cut -f1), $tablas tablas con datos)"

# Se conservan las últimas 14 copias: con una por día es quincena, y la base
# pesa unos pocos megas.
ls -1t "$destino"/lit-*.dump 2>/dev/null | tail -n +15 | while read -r viejo; do
    echo "  borrando copia vieja: $(basename "$viejo")"
    rm -f "$viejo"
done
