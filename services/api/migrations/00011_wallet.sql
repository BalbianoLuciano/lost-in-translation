-- +goose Up
-- El cable que une el servidor con la cadena (docs/sdd-distinciones.md §6).
--
-- Tres tablas y ninguna más. La verdad sigue viviendo en `achievements`: acá
-- sólo se guarda a quién pertenece una billetera, el desafío que prueba que es
-- suya, y lo que se vio en la cadena. Si estas tres tablas se vaciaran enteras,
-- nadie perdería una distinción: perdería el recibo, que es otra cosa.

-- La billetera, probada con una firma, no declarada.
--
-- El UNIQUE de la dirección es una regla del dominio, no una precaución: una
-- billetera, una persona. Sin él, dos cuentas podrían apuntar a la misma
-- dirección y el token quedaría contando una historia que no es de nadie. Y la
-- dirección se guarda siempre en minúsculas —lo hace chain.Direccion.Hex()—
-- porque Postgres compara texto byte a byte y un EIP-55 con mayúsculas se
-- colaría por el costado del UNIQUE sin que nadie se entere.
CREATE TABLE wallets (
    user_id     uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    address     text NOT NULL,
    chain_id    integer NOT NULL,
    verified_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (address)
);

-- El desafío SIWE, de vida corta.
--
-- Uno por persona: pedir un desafío nuevo pisa el anterior, así que no se
-- pueden juntar nonces para usarlos después. Se borra al consumirlo, que es lo
-- que lo hace de un solo uso, y el `expires_at` lo mata igual si nadie lo usa.
CREATE TABLE wallet_challenges (
    user_id    uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    nonce      text NOT NULL,
    expires_at timestamptz NOT NULL
);

-- El recibo: lo que se vio en la cadena.
--
-- La clave primaria (user_id, code) dice que una distinción se acuña una sola
-- vez, que es lo mismo que garantiza el contrato con su id determinista. El
-- estado arranca en 'pending' apenas llega el txHash y ahí se queda si el RPC
-- no contesta: el dato no se pierde nunca por un nodo caído, se reintenta.
--
-- token_id es numeric y no bigint porque un id de ERC-721 es un uint256: acá
-- son 160 bits de dirección corridos 16 lugares, muy por encima de lo que entra
-- en 64 bits.
CREATE TABLE mints (
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    code       text NOT NULL,
    address    text NOT NULL,
    chain_id   integer NOT NULL,
    token_id   numeric,
    tx_hash    text,
    status     text NOT NULL CHECK (status IN ('pending', 'confirmed', 'failed')),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, code)
);

-- +goose Down
DROP TABLE mints;
DROP TABLE wallet_challenges;
DROP TABLE wallets;
