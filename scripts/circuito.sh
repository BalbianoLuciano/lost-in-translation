#!/usr/bin/env bash
#
# El ensayo general del circuito de las distinciones, contra un nodo local.
#
# Levanta anvil, despliega el contrato con el catálogo de verdad, y corre el
# test de Go que firma un voucher, manda la transacción y lee el recibo. Es
# gratis, no toca ninguna red y no necesita ninguna clave tuya: anvil reparte
# cuentas conocidas con plata de mentira.
#
# Para qué sirve: entre "Go y Solidity se entienden" —que ya lo prueba el
# fixture del test cruzado— y "el circuito funciona" quedan cosas que sólo
# aparecen contra un nodo. Conviene correrlo antes de pelearse con un faucet.
#
#   ./scripts/circuito.sh
#
set -euo pipefail

cd "$(dirname "$0")/.."

puerto="${PUERTO:-8545}"
rpc="http://127.0.0.1:$puerto"

# Las dos primeras cuentas de anvil, que son siempre las mismas.
clave_despliegue="0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
dir_despliegue="0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"
clave_reclamante="0x59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d"

# La clave con la que firma el servidor. Es de juguete y está acá a propósito:
# anvil no tiene nada que robar. La de producción vive sólo en Railway.
clave_firmante="0x7c852118294e51e653712a81e05800f419141751be58f605c371e15141b007a6"

for cmd in anvil forge cast go; do
    command -v "$cmd" >/dev/null || { echo "falta $cmd en el PATH" >&2; exit 1; }
done

# La dirección del firmante se deriva de su clave, no se escribe al lado: una
# dirección copiada a mano se desincroniza, y el síntoma sería "firma inválida"
# en cada reclamo, que no explica nada. (Pasó la primera vez que corrió esto.)
dir_firmante=$(cast wallet address --private-key "$clave_firmante")

anvil --port "$puerto" --silent &
anvil_pid=$!
trap 'kill "$anvil_pid" 2>/dev/null || true' EXIT

# El nodo tarda un instante en atender. Se pregunta en vez de dormir a ciegas.
until cast block-number --rpc-url "$rpc" >/dev/null 2>&1; do
    sleep 0.2
done
echo "anvil en $rpc"

echo "desplegando el contrato con las 41 distinciones…"
salida=$(
    cd contracts && \
    DUENIO="$dir_despliegue" FIRMANTE="$dir_firmante" \
    forge script script/Desplegar.s.sol:Desplegar \
        --rpc-url "$rpc" --private-key "$clave_despliegue" --broadcast 2>&1
)

contrato=$(printf '%s\n' "$salida" | grep -o 'Distinciones: 0x[0-9a-fA-F]\{40\}' | tail -1 | awk '{print $2}')
if [ -z "${contrato:-}" ]; then
    printf '%s\n' "$salida" >&2
    echo "no se pudo sacar la dirección del contrato de la salida de forge" >&2
    exit 1
fi
echo "contrato en $contrato"

echo
cd services/api
CIRCUITO_RPC="$rpc" \
CIRCUITO_CONTRATO="$contrato" \
CIRCUITO_CHAIN_ID=31337 \
CIRCUITO_CLAVE_FIRMANTE="$clave_firmante" \
CIRCUITO_CLAVE_RECLAMANTE="$clave_reclamante" \
    go test ./internal/chain/ -run 'Circuito|LaCadenaRechaza' -v -count=1
