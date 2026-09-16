# Lost in Translation · Plan

Un profesor de inglés personal: una app que diagnostica, enseña, hace practicar,
evalúa y explica en castellano cuando algo no cierra.

Diseño visual y metáfora de progreso: [`design.md`](./design.md).

---

## 1. Punto de partida y meta

### Hoy

| | |
|---|---|
| Nivel | B2 (EF SET) |
| Perfil | Fullstack / team lead, empresa propia, contacto directo con clientes |
| Uso actual del inglés | Diario: lee, escribe y entiende. **No habla en reuniones** |
| Dificultad, en orden | 1. Hablar · 2. Escribir · 3. Entender |
| Gramática | Intuitiva, aprendida por exposición. Reconoce los tiempos verbales pero no los arma con seguridad ni sabe explicar por qué eligió una forma |
| Error conocido | Confunde **he/she** al hablar, aunque conoce la regla |
| Dedicación | **1 hora por día**, sin fecha límite |

### Meta

Poder, sin traducir en la cabeza:

1. Sostener una reunión con un cliente: presentar, estimar, negociar alcance y
   decir que no.
2. Liderar un equipo en inglés: daily, feedback, 1:1.
3. Pasar una entrevista técnica y conductual para un rol de fullstack o team lead.

### Cómo se sabe que se llegó

| Criterio | Umbral |
|---|---|
| EF SET | **C1 (61+)** dos meses seguidos |
| Certificación paga | **Linguaskill Business** en C1 |
| Errores de pronombre al hablar | < 1 cada 10 minutos de habla grabada |
| Precisión en tiempos verbales (examen propio) | ≥ 90% |
| "Por qué" gramatical | Explica correctamente la regla en ≥ 80% de los ítems de tipo *explain why* |
| Mock interview final | Aprobada según la rúbrica de la obra 6 |

---

## 2. Principios de enseñanza

Cada decisión de producto sale de uno de estos principios (ver la investigación
de la primera sesión).

1. **Chunks antes que palabras.** Se aprenden bloques como *"I've been working
   on…"*, que traen la gramática adentro. Es lo que corta la traducción.
2. **De intuición a regla (noticing).** Para vos no alcanza con acertar: hay
   ejercicios donde tenés que **elegir la regla que justifica** la respuesta.
3. **Recuperar, no releer.** Todo se practica produciendo la respuesta, no
   reconociéndola.
4. **Repaso espaciado (FSRS).** Cada ítem vuelve justo cuando estás por
   olvidarlo.
5. **Output forzado.** Hablar y escribir todos los días, aunque cueste. Hablar
   tiene el bloque más largo de la sesión.
6. **Contraste con el castellano.** Los errores típicos de hispanohablantes se
   atacan de frente con pares contrastivos: *"I worked" vs "I've worked"*.
7. **Automatizar, no sólo saber.** El he/she no es un problema de conocimiento
   sino de automatización: se ataca con drills orales rápidos y repetidos.
8. **Contexto real de IT.** Los ejemplos son standups, PRs, clientes y
   entrevistas, no "the cat is on the table".
9. **El castellano es una red, no el piso.** Todo pasa en inglés; el botón
   «Explicámelo en castellano» está siempre ahí, pero es a pedido.

---

## 3. La sesión diaria (60 minutos)

La arma el motor de la app según lo que toca repasar, tus eflorescencias (errores recurrentes) y la obra
actual.

| # | Bloque | Min | Qué pasa |
|---|---|---|---|
| 1 | **Repaso** | 10 | Ítems que FSRS marca como vencidos. Primero los temas con eflorescencia |
| 2 | **Lección** | 15 | Tema nuevo de la obra actual: regla, analogía, línea de tiempo, pares contrastivos y ejercicios |
| 3 | **Hablar** | 15 | Drills orales del tema + prompt libre grabado + shadowing |
| 4 | **Escribir** | 10 | Un mensaje real: Slack, descripción de PR, mail a cliente. Corrección con rúbrica |
| 5 | **Escuchar** | 10 | Fragmento de una charla real (1 a 3 min) con preguntas, dictado de chunks y shadowing |

- **Día corto (20 min):** sólo repaso y hablar. Sostiene el jornal.
- **Día libre:** uno por semana, no rompe el jornal.
- **Domingo:** repaso de la semana + mini examen de 15 ítems.

---

## 4. Currículum: las obras

Duraciones estimadas a 1 h/día. Una obra se desbloquea cuando se aprueba el
**examen de hito** de la anterior (≥ 80%, con parte escrita y oral).

### Obra 0 · Cimiento: diagnóstico (1 semana)

Objetivo: saber exactamente dónde estás, tema por tema, para no estudiar lo que ya
dominás.

- **Test de ubicación adaptativo** (~120 ítems en 3 días), etiquetados por
  habilidad. Si acertás los 3 primeros de un tema, se salta al siguiente.
- **Grabación base:** 3 minutos hablando de tu trabajo. Se transcribe y se miden
  los errores por tipo, entre ellos he/she.
- **Escritura base:** un mail a un cliente explicando una demora.
- **EF SET inicial**, cargado a mano en la app.
- **Resultado:** el pilar de cada obra. Lo que ya dominás arranca calzado.

### Obra 1 · Pilar: tiempos verbales (7 a 8 semanas)

La columna vertebral. Cada tiempo tiene línea de tiempo visual, analogía, marcas
temporales, chunks de IT y pares contrastivos.

| Pieza | Contenido | Contraste clave para hispanohablantes |
|---|---|---|
| 1.1 Presente | Simple vs continuous; verbos de estado (*know, need, belong*) | *"I'm knowing"* ✗ |
| 1.2 Pasado | Simple vs continuous; *used to / would* | *"When I was arriving"* ✗ |
| 1.3 Present perfect | Experiencia, resultado, *just/already/yet/ever* | **"hice" ≠ *I have done*.** Lo más importante de la obra |
| 1.4 Perfect vs past simple | Tiempo terminado vs abierto; *ago* vs *for/since* | *"I have worked there in 2020"* ✗ |
| 1.5 Perfect continuous | Duración hasta ahora; *How long have you been…?* | *"I work here since 2020"* ✗ (calco de "trabajo acá desde") |
| 1.6 Past perfect | Anterioridad en relatos (postmortems, entrevistas) | |
| 1.7 Futuros | *will / going to / present continuous / present simple* (horarios) | Decisión vs plan vs agenda |
| 1.8 Futuro avanzado | *future continuous / future perfect* (estimaciones: *"we'll have finished by…"*) | |

**Hito:** examen de 60 ítems (completar, corrección de errores, *explain why*) +
narrar en voz alta un proyecto pasado usando al menos 4 tiempos.

### Obra 2 · Pequeña estructura: las piezas chicas (5 semanas)

Los errores "tontos" que más se notan al hablar.

| Pieza | Contenido |
|---|---|
| 2.1 Pronombres | **he/she/they, his/her/their**, *singular they*, posesivos. Drills orales de automatización (ver 5.3) |
| 2.2 Artículos | *a/an/the/∅*: *"the users"* vs *"users"*, *"I'm ∅ developer"* ✗ |
| 2.3 Preposiciones | Tiempo y lugar (*in/on/at*); dependientes (*depend on, responsible for, good at*) |
| 2.4 Contables | *information, feedback, advice, software, equipment* son incontables (*"a feedback"* ✗) |
| 2.5 Verbos confusos | *make/do, say/tell, speak/talk, borrow/lend, remember/remind* |
| 2.6 False friends | *actually, eventually, assist, attend, realize, sensible, library, carpet* |
| 2.7 Preguntas | Orden en preguntas, preguntas indirectas (*"Could you tell me where is it"* ✗) |

**Hito:** examen escrito + 5 minutos hablando de tu equipo y tus clientes (mucho
he/she) con cero o un error de pronombre.

### Obra 3 · Anillo: inglés de IT (6 semanas)

| Pieza | Situación | Ejemplos de chunks |
|---|---|---|
| 3.1 Daily standup | Qué hiciste, qué harás, bloqueos | *"I'm blocked on…", "I'll pick up…"* |
| 3.2 Code review | Pedir y dar cambios con tacto | *"Nit:", "Have you considered…", "LGTM once…"* |
| 3.3 Bugs e incidentes | Reportar, escalar, postmortem | *"It turned out that…", "The root cause was…"* |
| 3.4 Explicar algo técnico | Arquitectura, tradeoffs, en voz alta | *"The tradeoff here is…", "Under the hood…"* |
| 3.5 Async | Slack, tickets, documentación | Registro: claro y breve |
| 3.6 Phrasal verbs de IT | *roll out, roll back, set up, look into, figure out, come up with, run into* | |

**Hito:** simulación grabada de un daily y de la explicación de un incidente +
descripción escrita de un PR.

### Obra 4 · Galería: el cliente (6 semanas)

| Pieza | Contenido | Gramática que se cuela |
|---|---|---|
| 4.1 Cortesía y hedging | Sonar seguro sin sonar brusco | Modales: *could, would, might, should* |
| 4.2 Discovery call | Preguntar, repreguntar, confirmar | Preguntas indirectas |
| 4.3 Estimaciones | Rangos, supuestos, riesgos | Futuros, *"assuming that…"* |
| 4.4 Alcance y pushback | Decir que no, negociar | **Condicionales 0, 1, 2 y 3** |
| 4.5 Malas noticias | Demoras, bugs en producción | Voz pasiva, *"unfortunately"* |
| 4.6 Propuesta y demo | Presentar trabajo, manejar preguntas | Conectores |
| 4.7 Mails | Estructura y registro formal vs cercano | |

**Hito:** role-play con un **cliente simulado** por IA (pide más alcance con el
mismo presupuesto) + mail escrito de propuesta.

### Obra 5 · Torre: liderazgo (5 semanas)

| Pieza | Contenido | Gramática |
|---|---|---|
| 5.1 Feedback | Modelo SBI, feedback positivo y correctivo | Reported speech |
| 5.2 1:1 | Preguntas abiertas, escucha activa | |
| 5.3 Delegar | Pedir con claridad, expectativas | *have/get something done* |
| 5.4 Conflicto | Desacuerdo sin agresión | *"I see your point, but…"* |
| 5.5 Documentar decisiones | ADRs, RFCs | Pasiva, relativas, conectores |

**Hito:** role-play de un 1:1 difícil + escrito de una decisión técnica (ADR).

### Obra 6 · Refugio en la Puna: la entrevista (6 semanas)

| Pieza | Contenido |
|---|---|
| 6.1 *Tell me about yourself* | Pitch de 90 segundos, versiones fullstack y lead |
| 6.2 Behavioral (STAR) | Banco de 12 historias propias: conflicto, fracaso, liderazgo, deadline |
| 6.3 Técnica en voz alta | Pensar en voz alta resolviendo un problema |
| 6.4 System design | Vocabulario y estructura para diseñar en voz alta |
| 6.5 Tus preguntas y negociación | Preguntar al entrevistador, hablar de salario |
| 6.6 Small talk | Los primeros y últimos 5 minutos |

**Hito final:** mock interview completa de 45 minutos con IA (behavioral, técnica
y cierre), evaluada con rúbrica. Aprobarla es llegar al paisaje.

**Total estimado: 36 a 38 semanas (~9 meses).** Después sigue el **modo
mantenimiento**: repaso diario + un role-play semanal.

---

## 5. Tipos de ejercicio

Todos los ejercicios llevan **etiquetas de habilidad**
(`tense.present_perfect.vs_past_simple`, `pronoun.gender`) para medir y
personalizar.

### 5.1 Corrección automática (sin IA)

| Tipo | Ejemplo |
|---|---|
| **Completar** | *"I ___ (work) here since 2021."* |
| **Elección contrastiva** | *I worked* / *I've worked* + contexto |
| **Explain why** | *¿Por qué "have worked"?* → elegir la regla correcta entre 4 |
| **Corregir el error** | Encontrar y corregir el error en una oración |
| **Transformar** | Pasar a pasiva, a pregunta indirecta, a reported speech |
| **Ordenar** | Armar la oración con bloques |
| **Chunk cloze** | *"It ___ out that the cache was stale."* |
| **Dictado** | Escuchar un chunk y escribirlo |

### 5.2 Corrección con IA (Groq)

| Tipo | Cómo se corrige |
|---|---|
| **Escritura libre** | Rúbrica fija (gramática, registro, claridad, chunks usados). El modelo devuelve JSON con errores etiquetados |
| **Role-play** | Chat con un personaje (cliente, entrevistador, dev junior) y feedback al final |
| **Hablar libre** | Transcripción con Whisper + misma rúbrica |

### 5.3 Drills orales (para el he/she)

El he/she no se arregla estudiando: se arregla **automatizando bajo presión de
tiempo**.

- **Ficha de persona:** aparece una persona con nombre y género (*"Maria, QA
  lead"*) y tenés 20 segundos para contar qué hizo ayer. La app **sabe** qué
  pronombre corresponde, así que la corrección no depende de que Whisper
  transcriba perfecto.
- **Alternancia rápida:** dos personas de distinto género en la misma consigna
  (*"Compare what Juan and Sofia did in the sprint"*).
- **Shadowing con pronombres:** repetir fragmentos de charlas con muchos pronombres.
- Cada error de pronombre marca **eflorescencia** en la junta de la pieza 2.1, que
  vuelve en todos los repasos hasta cerrarse.

> **Ojo con Whisper:** a veces "corrige" la gramática al transcribir (dijiste
> *he*, escribe *she*). Por eso los drills tienen **respuesta esperada conocida**
> y se transcribe con un prompt que pide transcripción literal. En habla libre,
> la corrección de pronombres se toma como orientativa.

---

## 6. Personalización

### 6.1 Modelo del alumno

- **Por ítem:** estado FSRS (estabilidad, dificultad, próximo repaso).
- **Por habilidad:** dominio de 0 a 1, calculado a partir de los ítems de esa
  etiqueta. Define si la pieza está en **plano**, **suspendida**, **calzada** u **oxidada**.
- **Registro de errores:** cada error guarda la etiqueta, tu respuesta, la
  correcta y el contexto. Tres errores de la misma etiqueta en 7 días marcan
  **eflorescencia**.

### 6.2 Armado de la sesión

1. Ítems vencidos por FSRS, primero los de habilidades con eflorescencia (tope: 10 min).
2. Siguiente lección de la obra actual. Si hay una eflorescencia grave, se intercala una
   **micro-lección de refuerzo** de ese tema.
3. Drills orales del tema de la lección + uno de la peor eflorescencia.
4. Consigna de escritura que obliga a usar lo de la semana.
5. Audio del nivel y tema actuales.

### 6.3 Botón «Explicámelo en castellano»

1. Primero busca una explicación **escrita a mano** para ese ítem o esa habilidad
   (el contenido núcleo ya viene con regla, analogía y línea de tiempo).
2. Si no hay, la pide a Groq con un prompt fijo: regla, analogía, por qué tu
   respuesta y un ejemplo más. Se pasa tu respuesta y la correcta.
3. La respuesta **se guarda**: la próxima vez sale de la base, sin volver a llamar
   a la IA.
4. Botones **"otra analogía"** y **"sigo sin entender"**. El segundo marca la
   explicación como mala y baja la confianza de ese ítem.

---

## 7. Evaluación

| Frecuencia | Qué | Dónde queda |
|---|---|---|
| Cada ejercicio | Corrección inmediata | Colada, FSRS |
| Semanal (domingo) | Mini examen de 15 ítems de la semana | Dominio por habilidad |
| Fin de obra | **Examen de hito** escrito + oral (≥ 80%) | Se abre el **vano** y se desbloquea la obra siguiente |
| **Mensual** | **EF SET** (gratis, externo). Se carga el puntaje en la app | Un **tensor** marcado en la retícula de la obra; gráfico de evolución |
| Trimestral | **Examen imprimible en PDF** con clave de respuestas | Se carga el puntaje a mano |
| Meta | **Linguaskill Business** (Cambridge, online) | Cuando haya C1 en EF SET dos meses seguidos |

### Ruta de certificación

1. **EF SET mensual:** gratis, para ver la tendencia.
2. **Linguaskill Business:** examen de Cambridge enfocado en inglés de negocios,
   online, con certificado CEFR. Unos £80 las 4 habilidades en UK; el precio
   varía por país. **Antes de apuntar: confirmar centro o modalidad desde
   Argentina.**
3. *Opcional:* **Duolingo English Test** (USD 70, 1 hora, desde casa) como ensayo
   de examen proctorizado. Está más orientado a lo académico.
4. *Largo plazo:* **Cambridge C1 Advanced** (presencial, no vence).

---

## 8. Gamificación

Detalle visual en `design.md` §3 y §7.

| Mecánica | Regla |
|---|---|
| **Colada (XP)** | +1 por ítem correcto, +3 por *explain why*, +5 por drill oral, +10 por escritura o role-play |
| **Jornal (racha)** | Suma con 20 min o más. 1 día libre por semana sin cortar |
| **Calzar** | Una pieza calza (se cierra su junta) cuando su dominio es ≥ 0.85 y cada ítem tiene estabilidad FSRS ≥ 21 días |
| **Óxido** | Si el dominio de una pieza calzada baja de 0.7, chorrea óxido, se separa y sus ítems vuelven a la sesión |
| **Eflorescencia** | Aparece en la junta con 3 errores de la misma etiqueta en 7 días. Se limpia con 5 aciertos seguidos en días distintos |
| **Vano** | Examen de hito aprobado |
| **Tensor** | EF SET mensual cargado |
| **Paisaje** | Mock interview final aprobada |

Nada castiga: no se pierde colada y el óxido se limpia repasando.

---

## 9. Arquitectura

```
┌───────────────────────────┐        ┌────────────────────────────┐
│  web (Vercel)             │  HTTPS │  api (Railway)             │
│  SvelteKit + TypeScript   │ ─────▶ │  Go                        │
│  PWA, SVG de las obras    │  JSON  │  sesión, FSRS, colada      │
│  MediaRecorder (audio)    │        │  proxy a Groq              │
└───────────────────────────┘        └──────┬───────────┬─────────┘
                                            │           │
                                  ┌─────────▼───┐   ┌───▼───────────────┐
                                  │  Postgres   │   │  Groq (free tier) │
                                  │  (Railway)  │   │  LLM + Whisper    │
                                  └─────────▲───┘   └───────────────────┘
                                            │ seed
┌───────────────────────────────────────────┴────────────────────────────┐
│  content/ (YAML en git)  ◀── tools/ (Python): genera, valida, siembra, │
│                                arma PDFs de examen (Typst)             │
└────────────────────────────────────────────────────────────────────────┘
```

### 9.1 Piezas

| Pieza | Tecnología | Por qué |
|---|---|---|
| **Frontend** | **SvelteKit** + TypeScript, desplegado en **Vercel** | Liviano, rápido en el celu, fácil de volver PWA. Es algo nuevo (vos venís de React/Astro) sin ser exótico |
| **API** | **Go** (`net/http` + `chi`), en **Railway** | Un binario chico, arranque rápido, bueno para aprender Go con un dominio real |
| **Base de datos** | **Postgres** en Railway, con **sqlc** + **goose** (migraciones) | SQLite en Railway necesita un volumen montado; Postgres gestionado es más simple. sqlc genera Go tipado desde SQL |
| **Repaso espaciado** | [`go-fsrs`](https://github.com/open-spaced-repetition/go-fsrs) | Implementación oficial de FSRS |
| **IA** | **Groq**: Llama 3.3 70B (o el mejor modelo disponible) + Whisper large v3 | Free tier: ~14.400 requests/día de texto y 2.000 de audio. Sobra para uso personal |
| **Contenido** | YAML en `content/`, validado con **Pydantic** | El banco de ejercicios vive en git, revisable y versionado |
| **Herramientas** | **Python** con **uv**: generación asistida, validación, seed y PDFs | Python para tareas de datos e IA; Go para el runtime |
| **PDFs de examen** | **Typst** | Maquetado tipográfico serio, rápido y declarativo; también es algo nuevo |
| **Auth** | **Firebase Authentication** con login de Google. El front obtiene el ID token y la API en Go lo verifica con `firebase-admin-go` | Cero pantallas de login propias, gratis, y no hay contraseñas que guardar. Firebase sólo se usa para auth: los datos siguen en Postgres |

**Abstracción de proveedor de IA:** interfaz `LLM` y `STT` en Go. Si el free tier
de Groq cambia, se enchufa otro proveedor gratuito (OpenRouter free, Gemini) sin
tocar el resto.

### 9.2 Estructura del repo

```
lost-in-translation/
├── design.md
├── PLAN.md
├── apps/
│   └── web/                 # SvelteKit (Vercel)
├── services/
│   └── api/                 # Go (Railway)
│       ├── cmd/api/
│       ├── internal/
│       │   ├── session/     # armado de la sesión diaria
│       │   ├── srs/         # FSRS
│       │   ├── mastery/     # dominio, calce, óxido, eflorescencia
│       │   ├── grading/     # corrección determinista
│       │   ├── ai/          # interfaces LLM/STT + Groq
│       │   └── store/       # sqlc
│       └── migrations/
├── content/
│   ├── skills.yaml          # árbol de habilidades y etiquetas
│   └── obras/
│       ├── 0-cimiento/
│       ├── 1-pilar/
│       │   └── 1.3-present-perfect/
│       │       ├── lesson.yaml      # regla, analogía, línea de tiempo, castellano
│       │       ├── items.yaml       # ejercicios
│       │       └── drills.yaml      # orales
│       └── ...
└── tools/
    ├── pyproject.toml
    ├── content_schema/      # modelos Pydantic
    ├── generate/            # borradores de ítems con Groq (se revisan a mano)
    ├── build/               # YAML → bundle JSON que embebe la API
    ├── exams/               # Typst → PDF
    └── listening/           # fragmentos de YouTube + transcripción
```

### 9.3 Modelo de datos (primer corte)

| Tabla | Campos principales |
|---|---|
| *(contenido)* | Habilidades e ítems **no van en la base**: `tools` compila `content/` a un JSON que la API embebe en el binario. Deploy de contenido = push |
| `placement_runs` | `user_id`, `part`, `content_version`, `status` |
| `cards` | `user_id`, `item_id`, estado FSRS, `due_at` |
| `attempts` | `user_id`, `item_id`, `response`, `correct`, `error_tags[]`, `latency_ms`, `created_at` |
| `skill_mastery` | `user_id`, `skill_id`, `mastery`, `state` (plano/suspendida/calzada/oxidada) |
| `fissures` | `user_id`, `skill_id`, `opened_at`, `closed_at`, `streak` |
| `explanations` | `item_id` o `skill_id`, `user_answer_hash`, `text_es`, `source` (manual/ia), `rating` |
| `recordings` | `user_id`, `prompt_id`, `transcript`, `analysis` (jsonb), `duration_s` |
| `writings` | `user_id`, `prompt_id`, `text`, `analysis` (jsonb) |
| `exams` | `user_id`, `kind` (hito/semanal/impreso/efset/linguaskill), `score`, `details`, `taken_at` |
| `daily_log` | `user_id`, `date`, `minutes`, `colada`, `blocks_done` |

El audio **no se guarda**: se transcribe y se descarta. Queda el texto.

### 9.4 API (primer corte)

```
POST /auth/session                  # recibe el ID token de Firebase
GET  /session/today                 # bloques de hoy
POST /attempts                      # respuesta a un ítem → corrección + FSRS
POST /explain                       # «Explicámelo en castellano»
POST /speaking   (multipart audio)  # → transcripción + análisis
POST /writing                       # → análisis con rúbrica
POST /roleplay/:scenario/messages   # chat con personaje
GET  /progress                      # obras, piezas, colada, jornal, eflorescencias
POST /exams/external                # cargar EF SET / Linguaskill
GET  /exams/:id/pdf                 # examen imprimible
```

---

## 10. Contenido: cómo se produce

El contenido es lo más importante y lo más trabajoso.

1. **Núcleo escrito a mano** (con asistencia): cada pieza tiene lección, regla,
   analogía, línea de tiempo y explicación en castellano revisadas.
2. **Ítems generados como borrador** con `tools/generate` (Groq), a partir de la
   lección y de una lista de errores típicos de hispanohablantes.
3. **Validación automática** (Pydantic + chequeos): respuesta única, etiquetas
   existentes, sin duplicados, largo razonable.
4. **Revisión humana** antes de mergear: se lee el diff del YAML en un PR.
5. **Meta por pieza:** ~40 ítems, ~10 drills orales, 2 consignas de escritura.

### 10.1 Escucha: fragmentos de charlas reales

Ley 03 del diseño (*as found*): nada de audios grabados para la app.

- **Fuente:** charlas y videos públicos de YouTube de conferencias y canales de
  ingeniería (GOTO, InfoQ, NDC, CNCF/KubeCon, ByteByteGo), entrevistas
  simuladas públicas y podcasts de ingeniería con video.
- **No se descarga nada.** Se guarda el `videoId`, el inicio y el fin del
  fragmento, y se reproduce con la **YouTube IFrame Player API** (`start`, `end`
  y loop del fragmento para shadowing).
- **Transcripción:** `tools/listening` (Python) lee los subtítulos públicos del
  video (`youtube-transcript-api`) al armar el contenido y guarda sólo las líneas
  del fragmento con sus tiempos. Si el video no tiene subtítulos, se transcribe
  el fragmento con Whisper vía Groq.
- **Del fragmento salen:** 3 a 5 preguntas de comprensión, 3 chunks para dictado
  y shadowing, y marcas de la gramática de la obra actual (*"find the present
  perfect"*).
- **Nivel:** se etiqueta por velocidad (palabras por minuto), acento y densidad
  técnica. La sesión elige según la obra y tus resultados de escucha.
- **Links rotos:** un chequeo semanal (GitHub Action) marca los videos que dejaron
  de estar disponibles.

---

## 11. Roadmap de desarrollo

Cada fase deja algo **usable**: se estudia con la app desde la fase 2.

| Fase | Entregable | Incluye |
|---|---|---|
| **F0 · Obrador** ✅ (falta deploy) | Monorepo desplegado | Esqueletos de SvelteKit (Vercel) y Go (Railway), Postgres, migraciones, CI, Firebase Auth, tokens de diseño y toggle de tema |
| **F1 · Cimiento** ✅ | Diagnóstico funcionando | Esquema de contenido (Pydantic) compilado y embebido, 132 ítems en 33 habilidades, corrección determinista, FSRS, test de ubicación adaptativo en 3 partes, «Explicámelo en castellano» con explicaciones escritas |
| **F2 · Primer pilar** | **Se empieza a estudiar** | Sesión diaria (repaso + lección), contenido 1.1 a 1.4, línea de tiempo verbal, botón «Explicámelo en castellano» con Groq y caché |
| **F3 · Piezas** | La obra visible | Pilar con juntas (portado del carrusel de Gridwright), primitivas isométricas de Salvatierra, estados plano/suspendida/calzada/oxidada, colada, jornal, eflorescencias |
| **F4 · Voz** | Hablar | Grabación, Whisper, drills con respuesta esperada, drills de he/she |
| **F5 · Escritura y role-play** | Producción libre | Rúbricas, cliente simulado, análisis estructurado |
| **F6 · Exámenes** | Validación | Exámenes de hito, mini examen semanal, PDFs con Typst, carga de EF SET, gráfico de evolución |
| **F7+ · Obras 1 a 6** | Contenido | Se completan las obras en paralelo al avance real de estudio |

---

## 12. Riesgos

| Riesgo | Mitigación |
|---|---|
| Groq cambia o recorta el free tier | Interfaz de proveedor intercambiable; caché de explicaciones; lo núcleo funciona sin IA |
| Contenido generado con errores | Revisión humana obligatoria y validación automática |
| Whisper corrige la gramática al transcribir | Drills con respuesta esperada; prompt de transcripción literal |
| Construir la app en lugar de estudiar | Desde F2 se estudia todos los días; el desarrollo va aparte de la hora diaria |
| Abandono | Día corto de 20 min, día libre semanal, nada punitivo |
| Railway con costo | Plan Hobby; si molesta, la API se mueve a Fly.io sin cambios de código |

---

## 13. Decisiones pendientes

- [ ] Confirmar que se puede rendir Linguaskill Business desde Argentina, y el
      precio.
- [ ] Lista inicial de canales y charlas para la escucha (ver 10.1).
- [ ] Nombre final de las obras.

### Decididas

- **Auth:** Firebase Authentication con Google (2026-09-16).
- **Escucha:** fragmentos enlazados de charlas reales (2026-09-16).
- **Tipografía:** Inter (grotesca de la línea Helvetica, con desambiguación) + IBM Plex Mono (2026-09-16).
- **Tema:** oscuro por defecto, toggle a claro, lectura en AAA (2026-09-16).
- **Identidad visual:** manual de marca personal (*Especificación 002*) + pilar de
  Gridwright + volúmenes de Salvatierra (2026-09-16).
