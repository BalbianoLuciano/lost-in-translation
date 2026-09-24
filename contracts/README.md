# contracts · Distinciones

El recibo, en la cadena, de algo que costó aprender.

`Distinciones.sol` es un ERC-721 **que no se transfiere**. Cada token dice que
alguien, alguna vez, calzó una pieza o terminó una obra en Lost in Translation.
El diseño entero está en [`docs/sdd-distinciones.md`](../docs/sdd-distinciones.md);
acá va lo justo para compilar, probar y entender qué hace.

> La cadena es el recibo, no la verdad. El progreso vive en Postgres, el
> servidor decide quién se ganó qué, y este contrato sólo verifica que lo haya
> firmado. Si la cadena se cae, la app no se entera.

## Cómo se usa

Hace falta [Foundry](https://getfoundry.sh). Las dependencias
(`openzeppelin-contracts` v5.7.0 y `forge-std` v1.11.0) están versionadas en
`lib/`, así que no hay nada que instalar.

```sh
forge build          # compila
forge test -vvv      # unidad, fuzz e invariantes
forge fmt --check    # formato
forge coverage       # cobertura
```

## Qué hace el contrato

**Acuñar.** `mint(to, pieza, deadline, firma)` verifica un voucher EIP-712 que
firmó el servidor:

```
Distincion(address to,uint16 pieza,uint64 deadline)
dominio: name "Lost in Translation", version "1", chainId, verifyingContract
```

El dominio es lo que impide que una firma hecha para la testnet sirva en
mainnet, o que la de otro contrato sirva acá. La transacción la manda el
usuario, no el servidor: la clave de firma **no tiene fondos y no puede
gastar**, y `setFirmante` la rota si se filtra, sin volver a desplegar.

Cualquiera puede mandar la transacción, incluso quien no sea `to`. Adelantarse
en el mempool sólo consigue pagarle el gas a otro: el voucher ata el
destinatario.

**El id se calcula.**

```solidity
tokenId = (uint256(uint160(titular)) << 16) | pieza;
```

De ahí salen tres cosas gratis: no hace falta un contador, la pieza se lee del
propio id con `uint16(tokenId)`, y reclamar dos veces la misma distinción choca
contra un id que ya existe. El front puede calcular el id antes de mintear.

**No se mueve.** Implementa ERC-5192: `locked()` devuelve siempre `true`, se
emite `Locked(tokenId)` al acuñar, y toda transferencia revierte con
`NoSeTransfiere()` — el candado vive en `_update`, que es por donde pasan
`transferFrom`, `safeTransferFrom` y el quemado.

**El catálogo.** Las 41 distinciones (34 piezas + 7 obras, derivadas de
`content/skills.yaml`) se cargan enteras en el constructor. No se agregan ni se
sacan: el catálogo es contenido, no código, pero tampoco cambia.

**Los metadatos.** `tokenURI` devuelve un `data:application/json;base64,…` con
`name` y `description`, armado adentro del contrato. El dibujo SVG de la pieza
es la etapa C.2 y entra en esa misma función sin tocar nada de lo demás.

## Lo que a propósito no hay

Sin proxy actualizable, sin royalties, sin precio, sin allowlist on-chain y sin
ERC-721Enumerable. Nada que huela a mercado, y nada que cueste gas para resolver
algo que el front ya sabe.

## Las pruebas

| Archivo | Qué cubre |
|---|---|
| `test/Base.t.sol` | El catálogo de prueba y la firma del voucher, armada desde cero |
| `test/Distinciones.t.sol` | Unidad y fuzz: mint feliz, firma inválida, otro firmante, otro contrato, vencido, duplicado, transferencias, `locked`, `setFirmante`, `supportsInterface` |
| `test/Distinciones.invariantes.t.sol` | Ningún token cambia jamás de dueño, y el suministro es el de los minteos |

La firma del voucher se construye en las pruebas **a mano**, sin usar nada del
contrato salvo el dominio que él mismo publica por ERC-5267. Si cambia el orden
de un campo del struct o el separador de dominio, rompe del lado correcto. Es la
mitad de Solidity del test cruzado con Go de la etapa C.3.
