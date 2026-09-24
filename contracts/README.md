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

```solidity
struct Pieza {
    string nombre;
    uint8  obra;
    uint8  juntaArriba;   // el perfil que comparte con la pieza de arriba
    uint8  juntaAbajo;    // el que comparte con la de abajo
    bool   esObra;        // dibuja el pilar entero en vez de una pieza sola
}
```

> **La ABI del constructor cambió en C.2.** El struct tenía dos campos y ahora
> tiene cinco. No hay nada desplegado todavía, así que no rompe nada, pero si
> alguien guardó un llamado al constructor viejo en algún lado, ya no sirve. La
> dirección del contrato **no** depende de esto: sale de
> `keccak256(rlp([deployer, nonce]))`, así que el test cruzado con Go
> (`test/FirmaDelServidor.t.sol`) siguió en verde sin tocar el fixture.

**Los metadatos, y el dibujo.** `tokenURI` devuelve un
`data:application/json;base64,…` con `name`, `description`, `attributes` y un
`image` que es un `data:image/svg+xml;base64,…`. El SVG **lo dibuja el
contrato**: no hay servidor de imágenes, no hay IPFS, no hay nada que se pueda
apagar. Los detalles están más abajo, en *El dibujo, adentro del contrato*.

## Lo que a propósito no hay

Sin proxy actualizable, sin royalties, sin precio, sin allowlist on-chain y sin
ERC-721Enumerable. Nada que huela a mercado, y nada que cueste gas para resolver
algo que el front ya sabe.

## El dibujo, adentro del contrato

`src/Pilar.sol` es la misma geometría que `apps/web/src/lib/pilar.ts`, portada a
una máquina que no tiene punto flotante.

**Las dos escalas pasan a milésimos.** La app trabaja normalizada: `x` de 0 a 1
sobre el ancho de la pieza, e `y` en unidades de amplitud de junta. Acá `x` va
de 0 a 1000 e `y` de -2000 a 2000, y las coordenadas salen de una multiplicación
y una división enteras. La división trunca hacia cero; el error es de menos de una
unidad de viewBox sobre 1260, y es determinista, que es lo único que este dibujo
tiene que prometer.

**Las curvas no se aproximan.** La tentación era subdividir cada Bézier en
segmentos rectos para no pelearse con los decimales. No hace falta: el `Q` del
SVG es una Bézier cuadrática y el renderer la resuelve. Lo único que hay que
llevar a enteros son sus tres puntos, que es la misma cuenta que para una recta.

**Los diez perfiles de junta van como constantes, no en storage.** El SDD
proponía cargarlos al desplegar. Se decidió al revés: los perfiles no son
contenido —son la misma geometría que la app tiene escrita en TypeScript—, y
guardarlos costaría unos cuarenta `SSTORE`, cerca de 800.000 de gas, para leer
después en cada `tokenURI` lo que el bytecode ya sabe. Si un perfil saliera mal,
la reparación es la misma en los dos casos: una v2. El catálogo sí va por
parámetro, porque cambia cuando cambia el contenido; estos diez dibujos, no.

**Dos clases de distinción, dos dibujos.** Una de pieza dibuja la pieza sola con
sus dos juntas. Una de obra recorre el catálogo, junta todas las piezas de esa
obra y dibuja el pilar entero con las juntas cerradas: la obra terminada es,
literalmente, la obra. Las juntas cierran porque la de abajo de una pieza y la
de arriba de la siguiente son el mismo perfil recorrido en los dos sentidos.

**Qué se pierde respecto del dibujo de la app.** La app tiene estado y el token
no: no hay huecos entre piezas (el hueco sale del dominio, que baja; una
distinción no baja nunca), no hay óxido, no hay levitación, no hay filtro de
grano —`feTurbulence` en un SVG que arma un contrato es plata tirada— y todas
las piezas miden lo mismo, porque el alto de la app sale de cuántos ejercicios
tiene el tema y eso cambia con el contenido. Lo que sí está es lo que hace a la
pieza *esa* pieza: sus dos juntas, la paleta de `design.md` y los mechinales.

### El gas

Medido con `forge test --match-test test_Gas -vv`, con el perfil de
`foundry.toml` (optimizador, 200 corridas). El test tiene los techos puestos
como assert, así que si esto se encarece, CI avisa.

| | gas |
|---|---|
| Transacción de despliegue, completa (41 distinciones) | **5.588.053** |
| — de los cuales, `CREATE`: constructor + depósito del código | 5.256.705 |
| — de los cuales, los datos del initcode | 310.348 |
| Por distinción del catálogo | 47.687 |
| `tokenURI` de una pieza | 194.513 |
| `tokenURI` de una obra de 5 piezas | 720.397 |

`tokenURI` es `view`: **leerla no cuesta un peso**. Los números están para saber
que un `eth_call` la puede ejecutar de sobra —el techo habitual anda por los
50.000.000— y no porque alguien los vaya a pagar. El gas de verdad se paga una
sola vez, al desplegar, y las tres cuartas partes son el depósito del bytecode.

## Cómo se despliega

El script es `script/Desplegar.s.sol`. No tiene adentro ninguna clave, ninguna
dirección y ninguna red: todo entra por variables de entorno.

### El catálogo

`script/catalogo.json` tiene las 41 distinciones, derivadas de
`content/skills.yaml`: una por cada habilidad (34) y una por cada obra (7). El
índice en ese arreglo **es** el número de pieza del voucher, y el campo `codigo`
es el código del logro que usa el servidor (`pieza:<skill_id>` / `obra:<n>`).

Las juntas están encadenadas: la de abajo de una pieza es la de arriba de la
siguiente de su obra, y las dos puntas del pilar van rectas.
`test/Catalogo.t.sol` lo verifica, despliega el catálogo de verdad contra la EVM
del test y comprueba que las 41 dibujen. Es lo más parecido a un ensayo del
despliegue que se puede hacer sin gastar un peso.

> Si cambia `content/skills.yaml`, hay que regenerar este JSON **antes** de
> desplegar. Después no hay forma: el catálogo entra una sola vez.
>
> **Pendiente, y dicho de frente:** hoy no hay nada que verifique que el JSON
> siga coincidiendo con `content/skills.yaml`. Los tests comprueban que el
> catálogo sea coherente consigo mismo —41 entradas, juntas que encadenan, todas
> dibujan—, no que sea el catálogo correcto. Generarlo desde el YAML es trabajo
> de `tools/`, que es de donde sale el resto del contenido compilado, y hasta que
> eso exista la regeneración es a mano y la revisa quien despliega.

### Las variables

Van en `contracts/.env`, que está gitignoreado. Nunca en el repo.

| Variable | Qué es |
|---|---|
| `DUENIO` | Quien puede rotar el firmante. Idealmente una billetera de hardware, no la que despliega |
| `FIRMANTE` | La dirección pública de la clave con la que el servidor firma los vouchers. **La privada nunca sale de Railway** |
| `CATALOGO` | Ruta del JSON. Por defecto, `script/catalogo.json` |
| `RPC_URL` | El nodo. Para Base Sepolia, `https://sepolia.base.org` sirve |
| `BASESCAN_API_KEY` | Sólo para verificar el código en el explorador |

La clave que manda la transacción **no** va en una variable: se pasa en la línea
de comandos, con `--ledger`, `--trezor` o `--interactive` (que la pide por
teclado y no la deja en el historial del shell).

### Base Sepolia, paso a paso

1. **Plata de testnet.** Base Sepolia no tiene su propio grifo grande; lo
   habitual es pedir Sepolia ETH en un grifo de Ethereum
   ([sepoliafaucet.com](https://sepoliafaucet.com), el de Alchemy, el de
   Infura) y pasarlo a Base Sepolia por el puente oficial de
   [bridge.base.org](https://bridge.base.org). El grifo de Coinbase Developer
   Platform entrega Base Sepolia ETH directo si tenés cuenta. Hace falta muy
   poco: el despliegue son ~5,6 millones de gas y en testnet el gas no vale
   nada, pero el nodo igual quiere saldo distinto de cero.

2. **Ensayar sin mandar nada.** Sin `--broadcast` el script corre contra una
   EVM local y no toca la red:

   ```sh
   cd contracts
   set -a; source .env; set +a
   forge script script/Desplegar.s.sol:Desplegar -vvvv
   ```

3. **Desplegar de verdad.**

   ```sh
   forge script script/Desplegar.s.sol:Desplegar \
       --rpc-url "$RPC_URL" \
       --interactive \
       --broadcast \
       --verify \
       --verifier-url https://api-sepolia.basescan.org/api \
       --etherscan-api-key "$BASESCAN_API_KEY" \
       -vvvv
   ```

   Chain id de Base Sepolia: **84532**. El explorador es
   [sepolia.basescan.org](https://sepolia.basescan.org).

4. **Verificar, si el paso 3 no lo hizo.** Pasa seguido: se cae la API del
   explorador justo ahí. Se puede repetir después, sin volver a desplegar:

   ```sh
   forge verify-contract <direccion> src/Distinciones.sol:Distinciones \
       --chain 84532 \
       --verifier-url https://api-sepolia.basescan.org/api \
       --etherscan-api-key "$BASESCAN_API_KEY" \
       --constructor-args "$(cast abi-encode \
           'constructor(address,address,(string,uint8,uint8,uint8,bool)[])' ...)" \
       --watch
   ```

   Los argumentos del constructor son la parte molesta: el catálogo entero
   codificado. La salida de `broadcast/Desplegar.s.sol/84532/run-latest.json` ya
   los tiene, en `transactions[0].transaction.input` después del bytecode.

5. **Anotar la dirección.** El servidor la necesita: entra en el separador de
   dominio de EIP-712, así que una firma hecha para otra dirección no sirve.

Una vez verificado, el `tokenURI` se puede leer desde el explorador y la imagen
se ve ahí mismo, sin pasar por la app. Ése es más o menos el punto.

## Las pruebas

| Archivo | Qué cubre |
|---|---|
| `test/Base.t.sol` | El catálogo de prueba y la firma del voucher, armada desde cero |
| `test/Distinciones.t.sol` | Unidad y fuzz: mint feliz, firma inválida, otro firmante, otro contrato, vencido, duplicado, transferencias, `locked`, `setFirmante`, `supportsInterface` |
| `test/Distinciones.invariantes.t.sol` | Ningún token cambia jamás de dueño, y el suministro es el de los minteos |
| `test/Dibujo.t.sol` | Que el `tokenURI` decodifique a un JSON con un SVG adentro, que el SVG abra y cierre, que dos distinciones dibujen distinto, que la misma dibuje siempre igual, que la obra dibuje más piezas, y el gas |
| `test/Catalogo.t.sol` | El catálogo de verdad: que sean 41, que las juntas encadenen, y que las 41 dibujen |
| `test/FirmaDelServidor.t.sol` | El test cruzado: Go firma, Solidity verifica |

La firma del voucher se construye en las pruebas **a mano**, sin usar nada del
contrato salvo el dominio que él mismo publica por ERC-5267. Si cambia el orden
de un campo del struct o el separador de dominio, rompe del lado correcto. Es la
mitad de Solidity del test cruzado con Go de la etapa C.3.

`forge test` deja en `test/salida/` —gitignoreado— unos SVG para mirar a ojo:
`pieza.svg` y `obra.svg` del catálogo de prueba, y `catalogo-pieza.svg` y
`catalogo-obra.svg` de las distinciones de verdad. Un dibujo generado se revisa
mirándolo; los asserts sólo dicen que no está roto.
