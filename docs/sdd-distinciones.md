# SDD · Distinciones: los logros, y su recibo en la cadena

> Documento de diseño. Se escribe **antes** de programar y se corrige cuando la
> realidad lo contradice. Lo que está acá se puede discutir; lo que está en el
> código, ya no.
>
> Estado: **borrador para ejecutar**. Fecha: 2026-09-24.
> Contexto previo: [`docs/auditoria.md`](auditoria.md) §3 y §5.

---

## 1. Objetivo

Que cada cosa que costó aprender deje **dos rastros**: uno adentro de la app,
que se ve y se disfruta, y otro afuera, que sobrevive a la app.

Y, con la misma obra, incorporar de verdad Solidity, Foundry y el circuito Web3
—no leerlos—, porque hay una búsqueda concreta que pide exactamente eso.

### Objetivos

1. Los logros existen como **entidad de dominio**, calculados en el servidor de
   forma determinista, y se ven en la app sin necesidad de ninguna cadena.
2. Quien quiera puede vincular una billetera y **reclamar** cada logro como un
   token **no transferible**, cuyo dibujo es **su propia pieza**, generada por el
   contrato.
3. Todo el circuito se prueba: el contrato con Foundry, el servidor con Go, y la
   frontera entre los dos con un test que corre en CI.

### No-objetivos, dichos de frente

- **No hay token con valor.** No se compra, no se vende, no se transfiere. Si
  alguna decisión lo acerca a un activo financiero, está mal tomada.
- **La app no se descentraliza.** El progreso sigue viviendo en Postgres y la
  app sigue siendo un servidor con una base. La cadena no arregla nada de eso y
  este documento no va a pretender que sí.
- **No hay DAO, ni gobernanza, ni tokenomics, ni puentes, ni Solana.** El aviso
  la nombra, pero lo que pide es EVM.
- **No se toca la enseñanza.** Si el inglés anda peor después de esto, salió mal.

---

## 2. La decisión rectora

> **La cadena es el recibo, no la verdad.**

El progreso vive en Postgres. El token es una **emisión** de ese progreso. La app
funciona completa sin cadena, sin billetera y sin red: quien no quiera saber nada
de esto, no se entera de que existe.

Es la misma regla que ya gobierna el proyecto con la IA —*si se puede chequear
con un assert, no lo decide un modelo*— aplicada un escalón más afuera: **si lo
sabe la base, no lo decide la cadena.**

De ahí sale una consecuencia práctica que ordena todo el diseño: **ningún camino
de la app puede quedar esperando a la red.** Responder un ejercicio tarda lo
mismo con billetera que sin ella.

### Una distinción es histórica, no un estado

El dominio de un tema **baja**: una pieza calzada se oxida si la abandonás. El
token, no. Registra que **llegaste**, no que **seguís ahí**.

- El pilar de la app muestra **cómo estás hoy**.
- La distinción muestra **qué lograste alguna vez**.

Son dos cosas distintas a propósito, y es lo que hace que el token pueda ser
inmutable sin mentir.

---

## 3. Modelo de dominio: qué es un logro

Hoy **no existe**. Lo que hay es la materia prima: `skill_mastery` con su estado
(plano / suspendida / calzada / oxidada), la colada y el jornal. Ninguna fila
dice "esta persona logró esto, este día, para siempre".

### Los logros

| Código | Cuándo se gana | Cuántos |
|---|---|---|
| `pieza:<skill_id>` | el dominio del tema llega a `calzada` | 34 |
| `obra:<n>` | todas las piezas de la obra están calzadas | 7 |

41 en total, fijos, derivados de `content/skills.yaml`. No hay logros por racha
ni por colada: se premia lo que costó, no lo que se acumuló.

### Las tres propiedades no negociables

1. **Determinista.** `Earned(progreso) → []Código`, función pura. La misma
   entrada da siempre lo mismo. Se prueba con una tabla, como `content.Grade`.
2. **Idempotente.** `PRIMARY KEY (user_id, code)`. Evaluar mil veces gana lo
   mismo que evaluar una.
3. **Del lado del servidor.** El cliente nunca dice qué ganó; sólo lo muestra.

### Dónde se evalúa

Dentro de la **misma transacción** que ya escribe el intento, la tarjeta FSRS y
el dominio (`session.Service.Answer` y `speaking.Service.Answer`). El logro se
guarda junto con el progreso que lo causó, o no se guarda ninguno de los dos.

```
Answer(tx):
    insert attempt
    upsert card (FSRS)
    recompute mastery          ← ya existe
    earned := achievement.Earned(catálogo, mastery)
    insert achievements ON CONFLICT DO NOTHING   ← nuevo
    commit
```

Cuesta una consulta más por respuesta y evita cualquier proceso de fondo que
"revise logros". No hay nada que reconciliar.

```go
// internal/achievement
type Code string

// Earned devuelve, para un progreso dado, todos los logros que corresponden.
// No sabe qué había antes: eso lo resuelve el ON CONFLICT.
func Earned(c *Catalog, mastery map[string]placement.State) []Code
```

---

## 4. Arquitectura

Cinco piezas. Las tres primeras ya existen.

```
┌───────────────────────┐   ┌──────────────────────────┐   ┌─────────────┐
│ apps/web (Vercel)     │   │ services/api (Railway)   │   │  Postgres   │
│ SvelteKit             │──▶│ Go                       │──▶│  el progreso│
│ + billetera + viem    │   │ + achievement            │   │  y el logro │
└───────────┬───────────┘   │ + wallet (SIWE)          │   └─────────────┘
            │               │ + voucher (EIP-712)      │
            │               └────────────┬─────────────┘
            │                            │ firma (no gasta)
            │ manda la transacción       ▼
            │                  ┌────────────────────┐
            └─────────────────▶│ Distinciones.sol   │
                               │ Base · ERC-721     │
                               │ soulbound + SVG    │
                               └────────────────────┘
```

### El flujo completo, en orden

```
1. Vincular la billetera (una sola vez)
   web  → GET  /v1/wallet/challenge        → { nonce, mensaje SIWE }
   web  → el usuario firma el mensaje con su billetera
   web  → POST /v1/wallet {mensaje, firma} → el server recupera la dirección
                                              y la guarda si coincide

2. Reclamar una distinción
   web  → POST /v1/achievements/{code}/voucher
   API  → ¿el logro está ganado? ¿la billetera está verificada?
   API  → firma un voucher EIP-712 {to, pieza, deadline}
   web  → llama a mint(to, pieza, deadline, firma) con su billetera
   red  → el contrato verifica la firma y mintea

3. Confirmar
   web  → POST /v1/achievements/{code}/mint { txHash }
   API  → consulta el recibo por RPC y guarda tokenId y estado
```

### La decisión que cambió respecto de la auditoría

En la auditoría dije que haría falta un *outbox* con un worker que mandara las
transacciones. **Revisado: no hace falta, y es mejor que no lo haya.**

Si la transacción la manda **el usuario**, el backend nunca necesita una
billetera con fondos. Sólo tiene una clave de **firma**, que no puede gastar
nada: si se filtra, se pueden falsificar distinciones —malo— pero no se puede
robar un peso. Un backend sin fondos es un backend con una superficie de ataque
mucho más chica, y de paso me ahorra un proceso de fondo, una cola y sus
reintentos.

El relayer vuelve a aparecer recién en la etapa D, cuando el objetivo sea que el
usuario no tenga que pensar en el gas.

---

## 5. El contrato

### Estándares

| Estándar | Para qué |
|---|---|
| **ERC-721** | El token. Es lo que las billeteras y los exploradores ya saben mostrar |
| **ERC-5192** | *Soulbound*: `locked(tokenId)` devuelve `true` y cualquier transferencia revierte. Es el estándar mínimo, de una sola función, para decir "esto no se mueve" |
| **ERC-712** | La firma tipada del voucher |
| **ERC-165** | Introspección, gratis al heredar de OpenZeppelin |

Base: **OpenZeppelin**, que es lo que se usa en la industria y lo que el aviso
espera ver.

### La idea que simplifica todo: el id se calcula

```solidity
tokenId = (uint256(uint160(titular)) << 16) | pieza;
```

De ahí salen tres cosas gratis:

1. **No hace falta un contador** ni storage para el par (token → pieza): la
   pieza se lee del propio id con `uint16(tokenId)`.
2. **No hace falta una tabla de "ya reclamada"**: alcanza con mirar si el id ya
   tiene dueño.

   > **Corregido al implementarlo.** Acá el documento decía que bastaba con el
   > `_mint` de ERC-721, que revierte solo si el id existe. Es falso una vez que
   > está el candado soulbound: `_mint` pasa por `_update` **antes** de chequear
   > el id repetido, así que reclamar dos veces revertía con `NoSeTransfiere`,
   > que a quien reclama no le explica nada. Va un `if (_ownerOf(id) != 0) revert
   > YaReclamada(id)` al principio de `mint`. Sigue sin agregar storage: es un
   > `SLOAD` que `_mint` iba a hacer igual.
3. **El front puede calcular el id antes de mintear**, así que puede mostrar el
   token antes de que exista.

### Esqueleto

Dos cosas que este esqueleto se olvida y aparecieron al escribirlo: hay que
heredar también de `EIP712` (de OpenZeppelin), y la interfaz `IERC5192` **no
viene** en OpenZeppelin, así que se escribe a mano.

```solidity
contract Distinciones is ERC721, IERC5192, EIP712, Ownable {
    address public firmante;          // rotable: la clave del server se cambia

    struct Pieza {                    // 41, cargadas al desplegar
        string  nombre;
        uint8   juntaArriba;          // índice del perfil de junta
        uint8   juntaAbajo;
        uint8   obra;
    }
    Pieza[] public piezas;

    error FirmaInvalida();
    error Vencido();
    error NoSeTransfiere();

    function mint(address to, uint16 pieza, uint64 deadline, bytes calldata firma) external;
    function tokenURI(uint256 id) public view override returns (string memory);
    function locked(uint256) external pure returns (bool) { return true; }
    function setFirmante(address nuevo) external onlyOwner;
}
```

### El voucher (EIP-712)

```solidity
bytes32 constant TIPO = keccak256("Distincion(address to,uint16 pieza,uint64 deadline)");
```

Dominio: `name: "Lost in Translation"`, `version: "1"`, `chainId`,
`verifyingContract`. El dominio es lo que impide que una firma hecha para la
testnet sirva en mainnet, o que una firma de otro contrato sirva acá.

**Por qué EIP-712 y no una firma cruda:** lo que el usuario ve en la billetera
antes de firmar son los campos con sus nombres, no un hexadecimal. Acá el que
firma es el servidor, pero la propiedad que importa es la otra: el hash está
atado al tipo, al dominio y al contrato, así que **una firma no se puede reusar
en otro contexto**.

**Sobre el front-running**: cualquiera puede ver el voucher en el mempool y
mandarlo antes. No importa: el voucher dice `to`, así que el token se acuña
igual para su dueño y el atacante le pagó el gas. Es el tipo de análisis que hay
que saber hacer, y acá la respuesta es "no es un problema".

### El dibujo, on-chain

Esta es la parte que hace que el proyecto valga y no sea *blockchain* pegada con
cinta: **`tokenURI` no devuelve un link a un jpeg**. Devuelve un
`data:application/json;base64,...` con el SVG de **esa** pieza adentro, dibujado
por el contrato con la misma geometría que ya usa la app.

- Una distinción de **pieza** dibuja la pieza sola, con sus dos juntas, calzada.
- Una distinción de **obra** dibuja el pilar entero armado, con todas las juntas
  cerradas. La obra terminada es, literalmente, la obra.

Y acá aparece la primera restricción real de la plataforma: **Solidity no tiene
punto flotante**. La geometría de `pilar.ts` está en coordenadas normalizadas
(`x` de 0 a 1, `y` con decimales), así que hay que reescribirla en enteros —
milésimos de ancho— y portar el `Q` de las curvas de Bézier a la misma escala.
Los 10 perfiles de junta se guardan como arrays de `int16` cargados al desplegar.

Costo: generar el SVG es una función `view`, así que **leerlo es gratis**. El
gas se paga una sola vez, al cargar las piezas en el despliegue.

### Lo que decidí NO hacer

- **Sin proxy actualizable.** Un badge no justifica la complejidad ni el riesgo
  de un proxy. Si aparece un bug, se despliega una v2 y se apaga el minteo de la
  v1: los tokens viejos siguen existiendo, que es justamente la promesa.
- **Sin `royalties`, sin `mintPrice`, sin allowlist on-chain.** Todo lo que
  huela a mercado está fuera.
- **Sin ERC-721Enumerable.** Cuesta gas en cada mint para resolver algo que el
  front ya sabe: las 41 piezas posibles son fijas y el id se calcula.

---

## 6. El servidor

### Tablas nuevas

```sql
-- la billetera, probada con una firma, no declarada
CREATE TABLE wallets (
    user_id     uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    address     text NOT NULL,                  -- en minúsculas, 0x…
    chain_id    integer NOT NULL,
    verified_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (address)                            -- una billetera, una persona
);

-- el desafío SIWE, de vida corta
CREATE TABLE wallet_challenges (
    user_id    uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    nonce      text NOT NULL,
    expires_at timestamptz NOT NULL
);

-- el logro: la verdad
CREATE TABLE achievements (
    user_id   uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    code      text NOT NULL,
    earned_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, code)
);

-- el recibo: lo que se vio en la cadena
CREATE TABLE mints (
    user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    code         text NOT NULL,
    address      text NOT NULL,
    chain_id     integer NOT NULL,
    token_id     numeric,
    tx_hash      text,
    status       text NOT NULL CHECK (status IN ('pending', 'confirmed', 'failed')),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, code)
);
```

### Endpoints

| Método y ruta | Qué hace |
|---|---|
| `GET /v1/achievements` | Los 41, con `earnedAt` y estado de minteo |
| `GET /v1/wallet/challenge` | Devuelve el mensaje SIWE con su nonce |
| `POST /v1/wallet` | Verifica la firma y guarda la dirección |
| `DELETE /v1/wallet` | Desvincula (no borra nada de la cadena, y hay que decirlo) |
| `POST /v1/achievements/{code}/voucher` | Firma el voucher, si el logro está ganado |
| `POST /v1/achievements/{code}/mint` | Recibe el `txHash`, lo confirma por RPC |

Todos los caros van detrás del limitador por cuenta que ya existe.

### La criptografía, con las manos

Hacen falta exactamente dos primitivas: **keccak-256** y **ECDSA sobre
secp256k1** (firmar, y recuperar la dirección de una firma).

La tentación es traer `go-ethereum` entero. **No**: son cientos de megas de
dependencias para un binario que hoy entra en una imagen de 53 MB, y además
esconde justo lo que quiero entender. La alternativa es chica y explícita:

- `golang.org/x/crypto/sha3` → `NewLegacyKeccak256` (ojo: keccak **no** es
  SHA-3 estándar; es una de las trampas clásicas).
- `github.com/decred/dcrd/dcrec/secp256k1/v4` → firmar y recuperar.

Y queda por escribir a mano lo que enseña de verdad: armar el `digest` de
EIP-712 (`0x1901 ‖ domainSeparator ‖ hashStruct`), y acomodar la firma al formato
que espera Ethereum (`R ‖ S ‖ V`, con `V` en 27/28).

> **Corregido al implementarlo.** Acá el documento decía "unas 150 líneas".
> Fueron unas 400 sin tests, y la diferencia no es relleno: falta en este
> documento el cálculo de la **dirección de CREATE** (`keccak256(rlp([deployer,
> nonce]))[12:]`), sin el cual el test cruzado no cierra de forma reproducible,
> porque el separador de dominio incluye `verifyingContract` y Go tiene que
> saber dónde va a estar el contrato antes de que exista.
>
> Y una trampa que sólo apareció escribiéndolo: dar vuelta el bit de `V` con
> `v ^ 1` **está mal**, porque 27 es `0b11011` y `27 ^ 1` da 26, que no existe.
> Va `27 + ((v - 27) ^ 1)`. Es un camino que en el caso feliz no se recorre
> nunca; lo agarró el test que arma a mano una firma maleable.

**Criterio de aceptación de este punto:** que el tamaño de la imagen no suba más
de 5 MB. Se mide, no se estima.

### La clave de firma

Es el secreto más serio del proyecto. Vive como variable de entorno en Railway,
**nunca** en el repo ni en un `.env` commiteado, y el contrato tiene
`setFirmante` para rotarla sin redeploy. No tiene fondos: no puede gastar.

---

## 7. El frontend

- **viem** en lugar de ethers: es TypeScript primero, los tipos salen del ABI, y
  es lo que se usa hoy.
- Una pantalla nueva, `/distinciones`: las 41 piezas, las ganadas en hormigón y
  las que faltan en línea punteada, igual que el mapa.
- Etapa C: conexión con una billetera de navegador. Etapa D: billetera embebida
  creada con el mismo login de Google.
- **Sin billetera conectada la pantalla se ve igual**, sólo que sin el botón de
  reclamar. La cadena es opcional en la interfaz, no sólo en el discurso.

---

## 8. Modelo de amenazas

| Amenaza | Qué pasa | Mitigación |
|---|---|---|
| Se filtra la clave de firma | Cualquiera acuña distinciones falsas | No tiene fondos; `deadline` corto; `setFirmante` para rotar; la clave vive sólo en el entorno de producción |
| Alguien reclama un logro que no ganó | — | El voucher lo firma el servidor **sólo** si la fila existe en `achievements`, que se escribe en la misma transacción que el progreso |
| Alguien falsifica el progreso | Gana un logro real por un camino falso | La cadena no agrega verdad, sólo la publica. La defensa es la misma de siempre: la corrección es determinista y del lado del servidor |
| Reusar un voucher en otra red o contrato | Minteo indebido | Dominio EIP-712 con `chainId` y `verifyingContract` |
| Reusar un voucher dos veces | Doble minteo | El id determinista hace que el segundo `_mint` revierta |
| Reclamar con la billetera de otro | Robo de distinción | SIWE: la dirección se prueba firmando, no se declara. Y el voucher ata `to` |
| Vincular una billetera ya usada | Dos cuentas, una billetera | `UNIQUE (address)` |
| Front-running del voucher | Ninguno | El voucher dice `to`: el atacante paga el gas ajeno |
| La dirección queda ligada a la persona, para siempre | Privacidad | On-chain va **sólo** dirección y número de pieza: ni mail, ni nombre, ni nada del progreso. Se avisa **antes** de firmar, no después |
| Se cae el RPC | La confirmación queda pendiente | El logro ya está en la base; la confirmación se reintenta cuando el usuario vuelve a entrar |

### El choque que no tiene solución limpia

El derecho a borrar la cuenta (auditoría §2.2) y la inmutabilidad de la cadena
**no se llevan bien**. La postura, escrita antes de que alguien pregunte:

- En la cadena no hay ningún dato personal: una dirección y un número.
- Borrar la cuenta borra todo en Postgres, incluido el vínculo dirección↔persona.
- El token queda. Se avisa con todas las letras antes de la primera firma.

---

## 9. Cómo se prueba

| Capa | Herramienta | Qué se prueba |
|---|---|---|
| Logros | `go test` | `Earned` como tabla: cada logro, y que oxidarse **no** lo quita |
| Contrato · unidad | **Foundry** | Mint feliz, firma inválida, vencido, duplicado, transferencia revierte, `locked()` |
| Contrato · fuzz | Foundry | `mint` con direcciones y piezas al azar: nunca acuña sin firma válida |
| Contrato · invariantes | Foundry | Ningún token cambia nunca de dueño; el suministro es igual a la cantidad de minteos exitosos |
| **La frontera** | Go + Foundry | 👇 |
| API | `go test` | Endpoints, permisos, rechazo cuando el logro no está ganado |
| Web | Playwright | La pantalla sin billetera, y el reclamo con una billetera simulada |

### El test que más vale

**Go firma, Solidity verifica.** Un test de Go produce, con una clave fija, la
firma de un voucher conocido y la escribe como fixture JSON. Un test de Foundry
lee ese archivo con `vm.parseJson` y comprueba que el contrato la acepta.

Si alguien cambia el orden de un campo del struct, el separador de dominio o el
formato de `V`, **el test rompe del lado correcto**. Es determinista, no necesita
red ni nodo, y corre en los dos CI. Es exactamente el mismo espíritu del test que
ya corrige los 250 ejercicios con sus propias respuestas.

Extra, si sobra tiempo: lo mismo contra `anvil` levantado en CI, de punta a punta.

### CI

Un job nuevo, `contracts`: `forge fmt --check`, `forge build`, `forge test -vvv`,
`forge coverage`. Y el fixture del test cruzado se genera en el job de Go, así
que si se desincroniza, rompe.

---

## 10. Entrega por etapas

Cada etapa termina en algo que se puede mostrar. Si una se cae, la anterior
sigue siendo útil por sí sola.

| # | Etapa | Entregable | Criterio de aceptación |
|---|---|---|---|
| **A** ✅ | Puerta y techo | *(2026-09-24)* | Ver auditoría §2.1 |
| **A.2** ✅ | Datos | *(2026-09-24)* | Ver auditoría §2.2 |
| **B** ✅ | Logros sin cadena | `internal/achievement`, tabla, `GET /v1/achievements`, pantalla `/distinciones` | Los 41 se calculan; oxidarse no quita ninguno; cuesta un INSERT y ninguna consulta extra |
| **C.1** ✅ | Solidity | `contracts/Distinciones.sol` con mint, soulbound y voucher | 31 tests en verde, con fuzz e invariantes, 100 % de cobertura |
| **C.2** | El dibujo | `tokenURI` con el SVG on-chain | El SVG que devuelve el contrato se parece al de la app |
| **C.3** ✅ | La frontera | `internal/chain`: keccak, secp256k1, EIP-712 y la dirección de CREATE, sin go-ethereum | Un voucher firmado en Go lo acepta el contrato. +1,4 MB de imagen, contra los 5 de techo |
| **C.4** | Testnet | Desplegado en Base Sepolia, reclamo desde el navegador | Una distinción real, visible en el explorador |
| **D** | Mainnet | Base, billetera embebida con Google, paymaster | Reclamar sin saber qué es el gas |

Entre B y C.1 no hay dependencia técnica fuerte: **el contrato se puede empezar
en paralelo**, porque no depende de nada del backend salvo el formato del
voucher. Si el objetivo es llegar rápido a Solidity, ése es el atajo legítimo.

---

## 11. Qué hay que aprender

La parte honesta: hoy no sé Solidity, y el plan sólo sirve si el aprendizaje es
explícito. Ordenado por cuándo hace falta, con el pedazo del proyecto que lo
obliga a bajar a tierra.

### 1 · El modelo mental de la EVM *(antes de escribir nada)*

Cuentas externas contra contratos; `storage` / `memory` / `calldata` y por qué
cuesta distinto cada uno; gas; y la diferencia entre **transacción** y **llamada
de lectura**.

> Por qué importa acá: todo el diseño del `tokenURI` se apoya en que leer es
> gratis. Sin este modelo, esa decisión parece magia.

### 2 · Solidity, el lenguaje

Tipos y enteros de ancho fijo, `mapping`, `struct`, visibilidad, `modifier`,
eventos, **errores personalizados** (y por qué salen más baratos que un
`require` con texto), herencia e interfaces.

> Por qué importa acá: la ausencia de punto flotante es lo que obliga a reescribir
> la geometría del pilar en milésimos.

### 3 · Estándares de token

ERC-165, ERC-721 (y qué hace `_update` por dentro), ERC-5192, el esquema del
JSON de metadatos, `data:` URIs y Base64 on-chain.

> Por qué importa acá: el token es un ERC-721 que miente lo menos posible: dice
> que está trabado y lo cumple.

### 4 · Firmas, que es el corazón

`keccak256`; ECDSA y `ecrecover`; EIP-191 contra **EIP-712**; el separador de
dominio; maleabilidad y por qué existe `ECDSA.tryRecover` de OpenZeppelin;
protección contra repetición.

> Por qué importa acá: es la única parte que se escribe **dos veces**, en Go y en
> Solidity, y tienen que coincidir byte a byte. Es también lo que más se pregunta
> en una entrevista.

### 5 · Foundry

`forge` / `cast` / `anvil`; `vm.prank`, `vm.expectRevert`, `vm.sign`,
`vm.parseJson`; pruebas **fuzz**; pruebas de **invariantes**; `forge coverage` y
las instantáneas de gas.

> Por qué importa acá: el aviso lo pide por nombre, y las invariantes son la
> forma natural de decir "esto nunca se transfiere".

### 6 · Seguridad

Reentrada (aunque este contrato no la sufra, hay que saber por qué);
control de acceso; repetición de firmas; front-running; desbordes; y el costo
real de la inmutabilidad.

> Por qué importa acá: la tabla del §8 es este punto, y ya está escrita.

### 7 · L2 y abstracción de cuentas *(etapa D)*

Qué es un rollup y por qué el gas se va casi todo en calldata; OP Stack y Base;
ERC-4337 (`UserOperation`, bundler, paymaster, entrypoint); billeteras con
passkeys.

> Por qué importa acá: es lo que permite que alguien que aprende inglés reclame
> una distinción sin saber qué es una billetera.

### 8 · SIWE y el frontend

EIP-4361 y por qué una firma **no** es una sesión; viem; leer eventos.

### Lo que decido no estudiar ahora

DeFi, AMMs, préstamos, tokenomics, puentes, MEV y Solana. Son otro oficio, y
mezclarlos acá alarga el camino sin acercar a nada de lo que el aviso pide.

### Cómo estudiar, no sólo qué

1. Primero el tutorial oficial de Foundry hasta poder desplegar y testear un
   contrato de juguete. **Sin esto, lo demás es leer.**
2. Después, CryptoZombies o Speed Run Ethereum para agarrar el lenguaje con las
   manos.
3. Ethernaut para seguridad: son puzzles de contratos rotos y es la forma más
   rápida de que la intuición se acomode.
4. Recién ahí, este contrato. El proyecto es el examen, no el curso.
5. Leer entero el ERC-721 de OpenZeppelin. Es corto y es de los mejores códigos
   que se pueden leer.

---

## 12. Decisiones abiertas

- [ ] **¿El token se puede quemar?** A favor: es el gesto más cercano al derecho
      a borrar. En contra: con id determinista, quemar y volver a reclamar es
      posible. Propongo permitirlo y aceptarlo: es el registro de su dueño.
- [ ] **¿Las distinciones de obra también, o sólo las 34 piezas?** Propongo las
      41: la obra armada es el mejor dibujo de todos.
- [ ] **Billetera de la etapa D**: Privy contra Coinbase Smart Wallet. Se decide
      cuando C.4 funcione, no antes.
- [ ] **¿Se despliega alguna vez en mainnet?** Se puede quedar en testnet para
      siempre y el aprendizaje es idéntico. Mainnet sólo si hay alguien que lo
      quiera de verdad.

## 13. Riesgos

| Riesgo | Mitigación |
|---|---|
| **Construir la cadena en vez de estudiar inglés.** Es el riesgo real | La etapa B se puede disfrutar sola. La hora diaria de inglés no se toca |
| El contrato sale con un bug y es inmutable | Testnet primero, superficie chica, `setFirmante`, y una v2 si hace falta |
| La imagen del binario se infla con dependencias de cripto | Criterio de aceptación explícito: +5 MB como techo |
| El aviso se cierra antes de terminar | Las etapas B y C.1 ya son material de entrevista por separado |
| Aprender de memoria sin entender | El test cruzado del §9 no se puede aprobar copiando y pegando |

---

**Dónde sigue.** [`docs/auditoria.md`](auditoria.md) tiene por qué esto va
después de cerrar la puerta, y [`PLAN.md`](../PLAN.md) §11 el lugar de las
etapas F8 y F9 en el plan general.
