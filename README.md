# Lost in Translation

A personal English tutor that measures before it teaches. It diagnoses what you
actually know, runs a daily session around your weakest topics, corrects what you
say out loud, and explains the why in Spanish when the English doesn't land.

![tests](https://img.shields.io/badge/tests-524%20Go%20·%2038%20Solidity%20·%2043%20web%20·%2020%20content%20·%208%20e2e-16a34a)
![content](https://img.shields.io/badge/bank-295%20exercises%20·%2018%20lessons%20·%20293%20glossary%20entries-2f5568)
![stack](https://img.shields.io/badge/Go%20·%20SvelteKit%20·%20Postgres%20·%20Python%20·%20Solidity-a8a49b)

> **The app knows the right answer before you open your mouth.**
>
> That one decision is what makes the speaking practice work. Whisper quietly
> fixes your grammar while transcribing — say *he* about a woman and it often
> writes *she*. So the drill declares up front which pronouns belong in the
> answer, and a plain comparison catches the slip. The model does what only it
> can do; the verdict is code you can read.

---

## The problem

I'm a B2 speaker. I can read a PR, follow a call and get by, but my grammar is
intuition I absorbed from films: I pick the right tense often enough, and I can't
say why. That plateau doesn't move by watching more series, and a generic course
starts from "the cat is on the table".

So the app is built around three rules:

1. **Measure first.** A placement test maps 34 topics before a single lesson.
2. **Never give a verdict without the rule.** Every correction shows why, and one
   tap gives the same explanation in Spanish, with an analogy.
3. **Every example is work.** Standups, PRs, incidents, client calls, interviews.
   Nothing that couldn't happen on a Tuesday.

---

## What it looks like

| The board | The correction |
|---|---|
| ![The board: streak, points, the current obra drawn as a pillar, and today's session with review, lesson, speaking and practice](docs/images/tablero.png) | ![A wrong answer: the expected answer, the rule in English, and the explanation in Spanish with an analogy](docs/images/correccion.png) |

| Speaking | The glossary |
|---|---|
| ![A speaking drill: the transcript with the wrong pronoun underlined, and the hint in Spanish](docs/images/hablar.png) | ![The glossary panel: searching "pushed" returns the verb, how the -ed sounds, and the matching rule](docs/images/glosario.png) |

Your progress is the drawing itself: one piece per topic, floating with its joint
open while you don't own it, settling into place when you do.

![The obra: every topic drawn as a concrete piece, dashed when untouched, filled and floating when in progress, settled when mastered](docs/images/obra.png)

---

## Architecture

![Architecture diagram: the SvelteKit SPA on Vercel talks to the Go API on Railway, which keeps progress in Postgres, verifies Firebase ID tokens against Google's public keys, and optionally calls Groq for chat and transcription. The exercise bank is YAML compiled by a Python tool into a JSON bundle embedded in the Go binary.](docs/architecture.svg)

```
apps/web/        SvelteKit SPA (static) → Vercel
services/api/    Go + Postgres          → Railway
content/         the exercise bank, in YAML
tools/           Python: validates content and compiles the bundle
contracts/       Solidity + Foundry: the soulbound distinctions
docs/            deploy guide, case study, design docs, images
```

Four decisions worth defending:

| Decision | Why |
|---|---|
| **Go for the API** | One static binary, a 53 MB distroless image, milliseconds to boot, almost no dependencies. Explicit error handling and SQL you can read |
| **sqlc, not an ORM** | You write real SQL and get typed Go generated from it. A query that doesn't match the schema fails at build, not in production |
| **The bank is embedded, not a table** | `tools` compiles `content/` into a JSON bundle that the binary embeds. Shipping content is a push: no seed script to forget |
| **Firebase only for identity** | Google login and token refresh are somebody else's problem. The API verifies the signature; the data stays in your Postgres |

---

## A day in the app

```console
$ curl -s https://…/healthz
{"ai":true,"content":"a98441b689a23ccc","db":"up","speaking":true,"status":"ok"}
```

1. **Review** — whatever the spaced-repetition scheduler says is due, hardest
   first. Nothing due, nothing shown.
2. **Lesson** — the next weakest topic that has a lesson written, starting from
   what the placement test found. A lesson you started is a lesson you finish:
   the app doesn't hop between topics.
3. **Practice** — ten exercises on that topic, unlocked once you've read it.
4. **Speaking** — a drill against the clock. Twenty-five seconds to tell the team
   what Sofía did yesterday, without calling her *he*.

Every answer moves the topic's mastery using the last eight attempts, weighted
towards the recent ones. What was solid and slips doesn't go back to zero: it
**rusts**, and rust is repaired by review.

---

## Grading is deterministic

The same split as any system that mixes code and models: if an assert can check
it, the model doesn't get a vote.

| Code does this | The model does this |
|---|---|
| Grade the answer against the bank | Transcribe what you said |
| Decide which pronoun the drill expected | Answer the questions the glossary doesn't cover |
| Schedule the next review (FSRS) | — |
| Move mastery, streak and points | — |
| Choose the next topic and exercise | — |

The bank ships its own explanations, so a correction is never a model's
improvisation:

```yaml
- id: pp-result-01
  type: cloze
  text: I ___ (finish) the migration, so the new table is ready to use.
  answers: [have finished, "'ve finished"]
  rule: Present perfect connects a past action to a result that matters now.
  explain_es:
    rule: El present perfect cuenta algo que pasó antes pero cuyo resultado importa ahora.
    analogy: Es una foto sacada hoy de algo que ocurrió antes.
    why: '"So the new table is ready" es el resultado presente…'
```

---

## The bank is code, and it's reviewed like code

```console
$ uv run lit-tools validate
✓ content/ válido · 295 ítems · 34 habilidades
```

The validator rejects more than typos. Two of its rules exist because the person
using the app caught the app cheating:

- **An option can't give itself away by length.** The correct option may not
  exceed the longest distractor by more than ten characters. Before this rule it
  was the longest one in 94% of items: you could pass without knowing English.
- **The answer can't sit in the same place.** Options are shuffled at compile
  time with a per-item seed, and the build fails if the correct one lands first
  more often than chance allows. It used to be first 82% of the time.
- **An explanation can't cite a position** ("the first one is correct"), because
  the shuffle would make it lie.
- **A lesson needs at least eight practice exercises**, or it's something you
  read once and forget.
- **Every regular verb carries how its -ed sounds**, and the rule is enforced
  where it's not debatable: ends in t or d → /ɪd/; ends in a voiceless sound →
  /t/.

CI runs the validator, checks the compiled bundle matches `content/`, and a Go
test grades all 295 exercises with their own answers. Bad content fails the
build, not the student.

---

## Speaking, and the trap in it

```
browser            MediaRecorder → webm/opus (~50 KB for 20 s)
   │
   ▼
Go API             multipart upload, 10 MB cap
   │
   ▼
Whisper            "transcribe exactly, do not correct", temperature 0
   │
   ▼
Go API             compare against the drill's expect/avoid lists
```

A drill declares the answer's shape before you speak:

```yaml
- id: drill-diego-standup
  kind: pronouns
  seconds: 25
  context: Standup. Diego finished the migration last night and is on call today.
  prompt_en: Tell the team what Diego did last night and what he is doing today.
  expect: [he]
  avoid: [she]
```

Say *she* and the transcript comes back with the word underlined. **The audio is
never stored** — it's transcribed and dropped; only the text survives.

---

## The tutor, grounded and cheap

The chat in the glossary isn't a raw model call. The server builds the prompt
with the exercise on your screen and the glossary entries your question mentions,
which is what stops it from inventing. It used to answer that *want* takes /t/
because it ends in t; now it gets the verb's real sound handed to it.

Answers are cached by prompt hash — asking the same thing twice is free and
doesn't count against the 60-a-day cap. No key configured? The chat disappears
and everything else works.

---

## The distinction that outlives the app

Mastering a topic earns a **distinction**, and a distinction is history, not
status: the pillar shows where you stand today, the distinction records that you
once got there. That is why rust never takes one away.

They are computed on the server, inside the same transaction that writes the
answer — a pure function of your progress, so the same input always yields the
same medal:

```go
func Earned(c *Catalog, mastery map[string]placement.State) []Code
```

Each one can then be claimed as a **non-transferable ERC-721** (ERC-5192) whose
drawing is your own piece, generated by the contract itself — not a JPEG on
IPFS. Non-transferable on purpose: nobody can buy somebody else's English.

```
progress (Postgres)  →  EIP-712 voucher signed by the server  →  you send the tx
```

The rule that holds it together: **the chain is the receipt, not the truth.**
Progress lives in Postgres; the token is an emission of it. Nothing in the app
ever waits on a network, and the signing key holds no funds — if it leaked,
someone could forge a badge, not steal a cent.

```console
$ cd contracts && forge test
Ran 3 test suites: 38 tests passed, 0 failed
```

The test that matters most runs on both sides: **Go signs a voucher, Solidity
verifies it.** A Go test writes the signature as a fixture, a Foundry test reads
it and asserts the contract accepts it. Change a field's order, the domain
separator or the `V` byte, and it breaks on the right side.

Design, threat model and what had to be learned: [`docs/sdd-distinciones.md`](docs/sdd-distinciones.md).

---

## Who can use it, and what it costs

Opening the app to other people needed a ceiling before it needed features:

- **A daily budget per person and for the app as a whole.** The provider key is
  one; a per-user cap alone protects nothing once there are many users. At the
  ceiling the AI turns itself off and the rest keeps working.
- **The speaking cap is checked before the audio is sent.** What isn't sent
  isn't paid for, and a test proves it by counting calls to the provider.
- **An allowlist that closes the door without evicting anyone**: it is only
  consulted when the user doesn't exist yet.
- **You can delete your account and take your data with you.** A test seeds a
  row in each of the seven tables and counts them one by one after the delete.

---

## Running it

Requirements: Docker, Go 1.27+, Node 24 + pnpm 10, [sqlc](https://sqlc.dev),
[uv](https://docs.astral.sh/uv/).

```sh
docker compose up -d --wait          # Postgres on 54329

cd services/api && cp .env.example .env
set -a && source .env && set +a
go run ./cmd/api                     # :8080, fake login for local dev

cd apps/web && pnpm install && pnpm dev   # :5173
```

Without Firebase variables the web shows **Enter · dev mode** and the API accepts
that user. In production that mode refuses to start.

## Tests

```sh
cd services/api && TEST_DATABASE_URL="postgres://lit:lit@localhost:54329/lit?sslmode=disable" go test ./...
cd tools && uv run pytest
cd apps/web && pnpm check && pnpm test && pnpm test:e2e
cd contracts && forge test
```

- **524 Go tests**, including integration against a real Postgres, run with `-race`.
- **43 web tests**, one of which reads the real CSS tokens and fails if any text
  drops below its contrast floor.
- **20 content tests** for the validator's rules.
- **38 Solidity tests**: unit, fuzz over random addresses and pieces, and an
  invariant that no token ever changes hands.
- **8 end-to-end flows** on desktop and phone: the whole placement test, the
  daily session, the glossary, the map and a speaking drill — the microphone and
  the transcription are faked, and production refuses both.

CI runs the five suites on every push. Railway and Vercel deploy from `main`.

## Content

```sh
cd tools
uv run lit-tools validate   # checks every rule above
uv run lit-tools build      # compiles the bundle the API embeds
uv run lit-tools stats      # exercises per skill and per type
```

Exercises live in `content/items/`, lessons in `content/lessons/`, speaking
drills in `content/drills/` and the glossary in `content/glossary/`. Each folder
has a commented reference file that sets the quality bar.

## Known gaps

- 16 of 34 topics still have no lesson, and one practice exercise each.
- Writing (F5) and the milestone exams (F6) aren't built.
- Pronunciation feedback is limited to what the transcript shows: no phoneme
  scoring.
- Seven placement items offer only two options, so a coin flip clears the
  difficulty-1 item that decides whether a topic gets skipped.
- Types are written twice, in Go and TypeScript; a mismatch breaks at runtime.
- The distinctions are not deployed to any chain yet: the contract and the
  server-side signature are tested, the deploy is not done.

---

**Where the rest is written.** [`PLAN.md`](PLAN.md) has the curriculum and the
teaching decisions, [`design.md`](design.md) the visual system,
[`docs/auditoria.md`](docs/auditoria.md) what it would take to open the app to
other people, [`docs/sdd-distinciones.md`](docs/sdd-distinciones.md) the design
of the on-chain distinctions, and [`docs/caso-de-estudio.md`](docs/caso-de-estudio.md)
the story of building it. Those are in Spanish, like the app's explanations.
