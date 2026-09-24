-- name: PutWalletChallenge :exec
-- Un desafío por persona: pedir uno nuevo pisa el anterior. Así nadie puede
-- juntar nonces válidos y usarlos más tarde.
INSERT INTO wallet_challenges (user_id, nonce, expires_at)
VALUES ($1, $2, $3)
ON CONFLICT (user_id) DO UPDATE
SET nonce = EXCLUDED.nonce, expires_at = EXCLUDED.expires_at;

-- name: ConsumeWalletChallenge :one
-- Comparar y borrar en la misma sentencia es lo que hace al nonce de un solo
-- uso de verdad. Si fueran dos consultas —leer, verificar, borrar— dos pedidos
-- simultáneos con el mismo nonce pasarían los dos. Y el vencimiento lo decide
-- el reloj de Postgres, no el del proceso: si hay dos réplicas, tienen que
-- estar de acuerdo.
DELETE FROM wallet_challenges
WHERE user_id = $1 AND nonce = $2 AND expires_at > now()
RETURNING nonce;

-- name: LinkWallet :one
-- Vincular de nuevo pisa la dirección anterior de esa persona. El UNIQUE de la
-- tabla sigue estando: si la dirección ya es de otro, esto falla y tiene que
-- fallar.
INSERT INTO wallets (user_id, address, chain_id)
VALUES ($1, $2, $3)
ON CONFLICT (user_id) DO UPDATE
SET address = EXCLUDED.address, chain_id = EXCLUDED.chain_id, verified_at = now()
RETURNING *;

-- name: GetWallet :one
SELECT * FROM wallets WHERE user_id = $1;

-- name: UnlinkWallet :execrows
-- Desvincula de este lado y nada más: en la cadena no hay nada que borrar.
DELETE FROM wallets WHERE user_id = $1;

-- name: GetAchievementEarned :one
-- La única pregunta que el voucher necesita hacerle a la verdad: ¿está ganado?
-- Se lee de la tabla, no se recalcula, porque una pieza oxidada después sigue
-- teniendo su distinción.
SELECT earned_at FROM achievements WHERE user_id = $1 AND code = $2;

-- name: RecordMint :one
-- El txHash entra como 'pending' antes de preguntarle nada al RPC. Si el nodo
-- no contesta, el dato ya está guardado y la confirmación se reintenta: lo que
-- nunca puede pasar es perder el hash de una transacción que la persona ya pagó.
INSERT INTO mints (user_id, code, address, chain_id, tx_hash, status)
VALUES ($1, $2, $3, $4, $5, 'pending')
ON CONFLICT (user_id, code) DO UPDATE
SET tx_hash = EXCLUDED.tx_hash, address = EXCLUDED.address,
    chain_id = EXCLUDED.chain_id, updated_at = now()
-- Una distinción ya confirmada no vuelve atrás por un txHash nuevo: el token
-- existe y el id es determinista, así que no hay segundo minteo posible.
WHERE mints.status <> 'confirmed'
RETURNING *;

-- name: SettleMint :one
-- Lo que dijo el recibo. token_id viaja como texto y se convierte acá porque un
-- uint256 no entra en ningún entero de Go.
UPDATE mints
SET status = $3, token_id = sqlc.narg('token_id')::numeric, updated_at = now()
WHERE user_id = $1 AND code = $2
RETURNING *;

-- name: GetMint :one
SELECT * FROM mints WHERE user_id = $1 AND code = $2;

-- name: ListMints :many
SELECT * FROM mints WHERE user_id = $1;
