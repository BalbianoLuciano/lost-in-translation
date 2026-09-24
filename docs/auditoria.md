# Auditoría: de app personal a app con usuarios (y con cadena)

> Fecha: 2026-09-24. Estado auditado: `bbbcdc5`, F0–F4 en producción.
>
> Dos preguntas: **qué se rompe cuando entra el segundo usuario**, y **qué hace
> falta para que un logro viva en una blockchain**. La segunda depende de la
> primera más de lo que parece.

---

## 1. Lo que ya escala, y conviene no tocar

Esto no es un elogio de oficio: son cuatro decisiones que ya están tomadas y que
te ahorran la refactorización que suele doler.

| Qué | Por qué aguanta |
|---|---|
| **Multiusuario desde la primera migración** | Todas las tablas arrancan con `user_id uuid REFERENCES users ON DELETE CASCADE`. No hay una sola consulta que asuma "el usuario". Borrar una cuenta es un `DELETE` y la base hace el resto |
| **Identidad de verdad** | Firebase firma, la API verifica contra las claves públicas de Google. No hay secreto compartido ni sesión propia que mantener |
| **La API no guarda estado** | El contenido va embebido en el binario y el progreso en Postgres: podés correr N réplicas sin coordinar nada. Las migraciones toman un advisory lock al arrancar, así que arrancar tres a la vez no rompe |
| **La caché de IA es global** | `ai_answers` se indexa por hash del prompt, no por usuario: con más gente **baja** el costo por persona, porque las preguntas se repiten |
| **El modo dev se niega a arrancar en producción** | `config.validate()` falla si `AUTH_MODE=dev` con `APP_ENV=production` |

Traducción: no hay que reescribir la arquitectura. Hay que ponerle **techo, puerta
y salida de emergencia**.

---

## 2. Los agujeros, en orden de lo que duele primero

### 2.1 ✅ Cualquiera con una cuenta de Google entra, y te gasta la plata

> **Cerrado el 2026-09-24.** Lo que se hizo está al final del punto.

Era el hallazgo más serio y el único que podía pasar mañana.

- No hay **ninguna** lista de permitidos. La URL es pública, `/v1/me` crea el
  usuario en el primer llamado. Quien encuentre la app, tiene cuenta.
- No hay **rate limit**. El router tiene `RequestID`, `RealIP`, logger,
  `Recoverer` y CORS. Ningún límite por IP ni por usuario
  (`services/api/internal/httpapi/router.go`).
- El único tope que existe es el del tutor: `DailyLimit = 60` **por usuario y por
  día**. Con 100 usuarios eso son 6.000 preguntas diarias contra **una sola
  clave** de Groq en free tier. El tope está del lado equivocado: falta el global.
- **La voz no tiene tope de ninguna clase.** `POST /v1/speaking/{drill}/answers`
  acepta 10 MB de audio por request y los manda a Whisper. Cien requests de 10 MB
  son 1 GB de audio transcrito con tu clave, y no hay nada que lo frene.

**Qué hace falta**

1. Una tabla de uso unificada — hoy `ai_usage` sólo cuenta preguntas al tutor;
   la voz no se cuenta. `usage(user_id, day, kind, n)` con topes por `kind`.
2. Un **presupuesto global diario** por variable de entorno. Cuando se agota, la
   IA se apaga sola y la app sigue andando (ya sabe funcionar sin clave: el
   healthz reporta `ai:false` y el chat desaparece). Esa degradación ya está
   escrita, sólo hay que poder dispararla.
3. `httprate` de chi por IP y por uid en `/v1/ask` y `/v1/speaking/*`.
4. **Allowlist de emails o invitaciones.** Es una tabla y un middleware.

> Orden de magnitud: transcribir audio no es gratis ni siquiera en free tier —
> es cuota. Un usuario aburrido con un script te deja sin la funcionalidad que
> más te costó construir.

**Lo que quedó hecho**

| Qué | Dónde |
|---|---|
| Una sola tabla de gasto diario, por usuario y por tipo (`ask`, `speaking`). La de la voz no existía | `migrations/00008_usage.sql` |
| Tope **por usuario** y tope **global** para cada tipo, configurables por entorno. Al llegar al global, la IA se apaga sola y el resto de la app sigue | `internal/budget` |
| El tope de la voz se chequea **antes** de mandar el audio: lo que no se manda, no se paga. Hay un test que lo prueba contando las llamadas al proveedor | `internal/speaking/service.go` |
| Límite de ritmo: general por IP sobre todo `/v1`, y uno más ajustado **por cuenta** en `/v1/ask` y en la respuesta oral. Balde de fichas en memoria, con barrido de los que no se usan | `internal/httpapi/ratelimit.go` |
| Lista de permitidos. Cierra la puerta sin echar a nadie: sólo se consulta cuando el usuario **todavía no existe**, así que activarla en una app en uso no deja afuera a quien ya entró | `internal/httpapi/access.go` |
| En producción sin `ALLOWED_EMAILS` no se aceptan altas nuevas. La app avisa en el log al arrancar | `internal/config/config.go` |
| Si alguien entra con Google y no está invitado, ve una pantalla que lo explica, no un error | `apps/web/src/routes/+page.svelte` |

Falta, del mismo punto: el tope de ritmo vive en memoria, así que con más de una
instancia cada una deja pasar su parte. El techo real sigue siendo el diario, que
sí está en la base.

### 2.2 🔴 No hay ciclo de vida de la cuenta

No existe `DELETE /v1/me` ni exportación. Hoy guardás, de cada persona: email,
nombre, todo lo que respondió, cuándo, cuánto tardó y las transcripciones de lo
que dijo en voz alta.

Con vos como único usuario eso es un archivo tuyo. Con terceros es **dato
personal**: hace falta poder borrar (el `ON DELETE CASCADE` ya hace el trabajo
sucio: sólo falta el endpoint), poder exportar, y tener política de privacidad y
términos escritos, aunque sean dos pantallas.

Y es **prerrequisito del bloque blockchain**: en cuanto atás una dirección de
billetera a una cuenta, tenés un identificador público asociado a una persona, y
lo que se escribe en la cadena **no se puede borrar**. Eso hay que decirlo antes
de que alguien firme, no después.

### 2.3 🟠 El contenido asume que el usuario sos vos

No es un bug, es una decisión de producto que todavía no tomaste:

- Las explicaciones son en **castellano rioplatense** (`explain_es`, con "fijate",
  "chuleta", "calzar").
- Los ejemplos son de **standup, PR, incidente y call con cliente**.
- El nivel arranca en **B1–B2**: alguien en A2 no tiene por dónde entrar.

Las dos salidas son legítimas, pero hay que elegir:

| Camino | Qué implica |
|---|---|
| **Quedarte en el nicho** — "inglés para devs hispanohablantes" | No tocás casi nada. Es tu ventaja real: ningún curso genérico compite con ejemplos de incidentes y PRs. Es, además, lo que hace bueno al contenido |
| **Generalizar** | El schema tiene que cambiar **ahora**: `explain_es` pasa a `explain: {lang, rule, analogy, why}`, el nivel se vuelve un eje del currículum y el banco se multiplica por cada L1 |

**Recomiendo el nicho**, y un cambio chico de higiene: sacar el `_es` del nombre
del campo para no quedar pintado en un rincón. Cambiar un nombre de campo hoy son
veinte minutos; en dos años son 250 archivos y una migración.

### 2.4 🟠 El muro de la primera semana

21 de 34 temas no tienen lección y tienen **un solo** ítem de práctica. Vos no lo
sentís porque el diagnóstico te dio 15 temas calzados y la app te manda a los
otros. Un usuario nuevo con menos nivel llega al muro en días.

Traer usuarios a una obra sin material es la forma más rápida de que no vuelvan.
Esto no es deuda técnica, es **falta de producto**, y es lo único de la lista que
no se resuelve programando.

### 2.5 🟠 No ves nada

Hay logs estructurados por request y se acabó. No hay métricas, ni errores
agregados, ni costo por usuario, ni idea de quién dejó de entrar. Con un usuario
alcanzaba con mirar la base. Con diez ya no sabés si la app anda.

Lo mínimo: contador de errores 5xx, latencia por ruta, gasto de IA del día, y una
pantalla de administración propia (podés reusar la autenticación y una allowlist
de admins).

### 2.6 🟡 El contenido y el progreso se pueden desincronizar

- `placement_runs.content_version` **se guarda y nunca se lee**. Si cambia el
  banco, no hay forma de saber si un diagnóstico viejo sigue siendo comparable.
- Las `cards` de FSRS apuntan a `item_id` de texto. Si borrás o renombrás un ítem
  en `content/`, quedan tarjetas huérfanas que apuntan a nada.

Con un usuario se arregla a mano. Con mil hace falta: **ítems con ciclo de vida**
(se marcan `deprecated`, no se borran nunca), una limpieza de huérfanas al
arrancar y una decisión explícita sobre cuándo un diagnóstico caduca.

### 2.7 🟡 Los detalles que sólo importan con volumen

- `ai_answers` crece para siempre: no hay poda ni tope de tamaño.
- `/healthz` es público y cuenta la versión del contenido y qué features están
  activas. No es grave; es información que no hace falta regalar.
- Una sola instancia en Railway: cada deploy corta la sesión de quien esté
  estudiando.
- Faltan los **backups** de Postgres (quedó pendiente tuyo).
- Los tipos están escritos dos veces, en Go y en TypeScript. Con más superficie,
  más lugares donde se desincronizan.

---

## 3. El bloque blockchain

### 3.1 La regla que lo hace defendible

> **La cadena es el recibo, no la verdad.**

El progreso vive en Postgres. El token es una **emisión** de ese progreso. Si la
cadena se cae, se congestiona o cambia de precio, la app no se entera: el usuario
sigue estudiando y el recibo se emite después.

Es exactamente la misma regla que ya aplicás con el modelo de IA ("si se puede
chequear con un assert, no lo decide un modelo"). Acá: **si el logro lo sabe la
base, no lo decide la cadena.** Esa frase es la que hace que el proyecto no
parezca blockchain pegado con cinta.

### 3.2 Lo que falta antes de tocar una cadena: los logros no existen

Decís que el sistema de logros ya está armado. Lo que hay, en realidad, es la
**materia prima**: dominio por habilidad, estados (plano/suspendida/calzada/
oxidada), colada y jornal. No hay ninguna entidad que diga "este usuario ganó
esto, este día, para siempre".

Eso es lo primero, y es un paquete Go como cualquier otro:

```
internal/achievement/
    achievement.go   catálogo de logros: id, nombre, condición
    evaluate.go      Evaluate(progreso) → []Earned   (función pura)
```

Tres propiedades no negociables, y las tres son testeables:

- **Determinista**: la misma entrada da siempre los mismos logros.
- **Idempotente**: evaluar dos veces no gana dos medallas (tabla `achievements`
  con `PRIMARY KEY (user_id, code)`).
- **Server-side**: el cliente nunca dice qué ganó; sólo lo muestra.

Es el mismo ejercicio que ya hiciste con `content.Grade`, y por eso encaja: la
cadena termina siendo la capa más fina de todo el sistema.

**Qué se mintea**, con la metáfora que ya tenés:

| Logro | Cuándo | Cuántos hay |
|---|---|---|
| **Pieza calzada** | Un tema llega a dominio ≥ 0.85 | 34 |
| **Obra terminada** | Todas las piezas de una obra calzadas | 7 |

Nada más. Un logro por cada cosa que costó de verdad.

### 3.3 Las seis decisiones técnicas

| Decisión | Qué elijo | Por qué |
|---|---|---|
| **Tipo de token** | ERC-721 **soulbound** (ERC-5192, `locked() = true`) | No transferible **a propósito**: es una distinción, no un activo. Sin mercado, sin especulación, sin quedar cerca de nada financiero. Y es honesto: el inglés de otro no se compra |
| **El arte** | **SVG generado on-chain**, en Solidity, con la geometría del pilar | Es el detalle que hace que esto valga. `tokenURI` no apunta a un jpeg en IPFS: devuelve un `data:` URI con **tu** pieza, con su junta única, dibujada por el contrato. Es el puente exacto entre `pilar.ts` y la cadena |
| **Cómo se mintea** | **Voucher EIP-712**: el backend firma `{to, achievementId, nonce, deadline}` y el contrato verifica que la firma recupere al firmante autorizado | El contrato no necesita saber nada de inglés, y el servidor no necesita mandar transacciones. El `nonce` evita minteos repetidos |
| **La billetera** | **Embedded wallet** (Privy o Coinbase Smart Wallet) creada con el mismo login de Google | Pedirle MetaMask a alguien que quiere aprender inglés mata la conversión. Y te lleva justo al conocimiento que hoy vale: **cuentas abstractas (ERC-4337) y passkeys** |
| **La red** | **Base** (L2 de Coinbase, OP Stack). Primero **Base Sepolia** | Testnet gratis para aprender; en mainnet un mint cuesta fracciones de centavo. Ethereum L1 está descartada por costo |
| **Vincular dirección ↔ usuario** | **SIWE (EIP-4361)**: challenge con nonce, el usuario firma, el backend verifica | Sin esto, cualquiera reclama el logro hacia la dirección que quiera |

### 3.4 Cómo entra en el código que ya hay

```sql
-- la billetera, probada con una firma, no declarada
wallets (user_id, address, chain_id, verified_at)

-- el logro: la verdad, en tu base
achievements (user_id, code, earned_at)   -- PK (user_id, code)

-- el recibo: el intento de escribirlo en la cadena
mints (user_id, code, status, tx_hash, token_id, attempts, last_error)
       -- status: pending | sent | confirmed | failed
```

El minteo es **asíncrono, con reintentos**, y nunca bloquea una respuesta HTTP:
un *outbox* en Postgres y un worker que lo drena. Es lo que impide que la latencia
y las caídas de la cadena se contagien a la sesión de estudio. Responder un
ejercicio tiene que seguir tardando lo mismo que hoy, tenga o no billetera.

Del lado del navegador es una pantalla nueva ("tus distinciones") y un botón para
conectar la billetera. Nada del flujo actual cambia.

### 3.5 Los riesgos, dichos de frente

- **Quien pueda falsificar intentos, puede falsificar una medalla.** La cadena no
  agrega verdad: sólo hace pública la que ya tenías. Por eso los logros se
  calculan en el servidor y el firmante vive en el servidor.
- **La clave del firmante es un secreto de producción de verdad**, no una variable
  de entorno más. Si se filtra, cualquiera mintea lo que quiera. Se rota, y el
  contrato tiene que permitir cambiar el firmante autorizado.
- **Lo que se escribe, no se borra.** Choca de frente con el derecho a borrar la
  cuenta del punto 2.2. La salida honesta: en la cadena va lo mínimo (una
  dirección y un código de logro), nunca el email ni el nombre, y se avisa antes
  de firmar.
- **El contrato es inmutable.** Un bug no se arregla con un deploy. Se prueba con
  Foundry como se prueba cualquier otra cosa acá, y se empieza en testnet.

---

## 4. El orden

| Fase | Qué | Por qué va acá |
|---|---|---|
| **A · Puerta y techo** | Allowlist, presupuesto global de IA, tope de voz, rate limit, borrar y exportar cuenta, privacidad y términos | Sin esto no se puede invitar a **nadie**. Es lo único bloqueante |
| **B · Logros de verdad** | `internal/achievement`, tabla, evaluación determinista, medallas visibles en la app | La cadena sin esto no tiene qué emitir. **Si el logro no se disfruta sin cadena, con cadena tampoco** |
| **C · Cadena en testnet** | Contrato soulbound con SVG on-chain, voucher EIP-712, SIWE, worker de minteo. Todo en Base Sepolia | Acá está el 90% del aprendizaje, y cuesta cero |
| **D · Mainnet** | Base mainnet, billetera embebida con el login de Google, paymaster para el gas | Sólo cuando C ande y haya alguien a quien le importe |
| **Transversal** | Contenido (el muro de 2.4) y observabilidad (2.5) | Corren en paralelo. El contenido es lo que decide si alguien vuelve |

## 5. Contra qué perfil se construye

El track de cadena no es un capricho: hay una búsqueda concreta que pide
exactamente esto. Las fases C y D existen para poder acreditar cada línea con
código que se puede leer, no con un curso.

| Lo que pide el aviso | Con qué se acredita | Estado |
|---|---|---|
| **Solidity** | El contrato de las distinciones: ERC-721 soulbound con el SVG de la pieza generado on-chain | Fase C |
| **Foundry** (o Hardhat) | Los tests del contrato. Foundry, porque los tests se escriben **en Solidity** y trae fuzzing e invariantes: encaja con cómo ya se prueba todo acá | Fase C |
| **Entornos Web3** | El circuito completo: SIWE para probar la dirección, voucher EIP-712 firmado por el backend, billetera embebida sobre ERC-4337, deploy en Base | Fases C y D |
| **Inglés conversacional** | La app misma. Es el único ítem del aviso que ya se está trabajando todos los días | En curso |

Y un quinto que el aviso no pide pero que en una entrevista pesa igual: **saber
por qué el contrato no decide nada**. Que el progreso viva en Postgres y la
cadena sea sólo el recibo es la decisión de diseño más discutible del proyecto, y
la que mejor se defiende.

> Antes de escribir una línea de Solidity conviene tener hecho el punto 2.1. No
> es sólo higiene: quien firma el voucher es el backend, así que un backend que
> cualquiera puede usar es un contrato que cualquiera puede hacer mintear.

---

### La recomendación

**No abras el registro.** Si el objetivo es incorporar los conocimientos —y eso
fue lo que dijiste—, quedate con la allowlist y hacé todo el track de cadena con
cinco personas conocidas.

Te saca de encima el costo descontrolado, el soporte, y buena parte del problema
legal, y **no te quita nada del aprendizaje**: el contrato, el voucher, la firma,
el SVG on-chain y la billetera embebida son exactamente iguales con 5 usuarios
que con 5.000. La diferencia entre las dos cosas no es técnica: es si querés
tener una empresa más.
