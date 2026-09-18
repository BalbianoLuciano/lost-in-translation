"""Esquema del banco de contenido.

El contenido vive en YAML en `content/` y es la fuente de verdad. Este módulo lo
valida y `build` lo compila a un único JSON que la API en Go embebe. Todo lo que
se pueda chequear acá, se chequea acá: un ítem mal escrito mide mal.

Las reglas de normalización y de tokenización tienen que ser idénticas a las de
`services/api/internal/content` (Go). Si cambian acá, cambian allá.
"""

from __future__ import annotations

import re
from typing import Annotated, Literal

from pydantic import BaseModel, ConfigDict, Field, field_validator, model_validator

PUNCT = ".,!?;:\"()"
SKILL_ID = r"^[a-z][a-z0-9_]*(\.[a-z0-9_]+)+$"
ITEM_ID = r"^[a-z0-9]+(-[a-z0-9]+)*$"


def normalize(s: str) -> str:
    """Normaliza una respuesta escrita: minúsculas, apóstrofos rectos, espacios
    colapsados, sin punto final."""
    s = s.replace("’", "'").replace("‘", "'").lower()
    s = re.sub(r"\s+", " ", s).strip()
    return s.rstrip(".").strip()


def token_core(token: str) -> str:
    """El token sin la puntuación pegada a los bordes (conserva apóstrofos)."""
    return token.strip(PUNCT)


def tokens(text: str) -> list[str]:
    return text.split(" ")


class Strict(BaseModel):
    model_config = ConfigDict(extra="forbid", str_strip_whitespace=True)


NonEmpty = Annotated[str, Field(min_length=1)]


# ── Mapa: obras, piezas, habilidades ──────────────────────────────────────


class Skill(Strict):
    id: Annotated[str, Field(pattern=SKILL_ID)]
    name_en: NonEmpty
    name_es: NonEmpty


class Piece(Strict):
    id: Annotated[str, Field(pattern=r"^\d+\.\d+$")]
    name_en: NonEmpty
    name_es: NonEmpty
    skills: Annotated[list[Skill], Field(min_length=1)]


class Obra(Strict):
    id: Annotated[int, Field(ge=0, le=6)]
    slug: Annotated[str, Field(pattern=r"^[a-z-]+$")]
    name: NonEmpty
    topic_en: NonEmpty
    topic_es: NonEmpty
    pieces: list[Piece] = []

    @model_validator(mode="after")
    def pieces_belong(self) -> Obra:
        for p in self.pieces:
            if not p.id.startswith(f"{self.id}."):
                raise ValueError(f"la pieza {p.id} no pertenece a la obra {self.id}")
        return self


class SkillMap(Strict):
    obras: Annotated[list[Obra], Field(min_length=1)]


class PlacementPart(Strict):
    id: Annotated[str, Field(pattern=r"^[a-z-]+$")]
    name_en: NonEmpty
    name_es: NonEmpty
    description_en: NonEmpty
    skills: Annotated[list[str], Field(min_length=1)]


class Placement(Strict):
    items_per_skill: Annotated[int, Field(ge=2, le=5)] = 3
    parts: Annotated[list[PlacementPart], Field(min_length=1)]


# ── Ítems ─────────────────────────────────────────────────────────────────


class ExplainEs(Strict):
    """Lo que muestra el botón «Explicámelo en castellano» (design.md §8)."""

    rule: NonEmpty
    analogy: NonEmpty
    why: NonEmpty


class ItemBase(Strict):
    id: Annotated[str, Field(pattern=ITEM_ID)]
    difficulty: Annotated[int, Field(ge=1, le=3)]
    placement: bool = False
    context: str = ""
    rule: Annotated[str, Field(min_length=1, max_length=180)]
    explain_es: ExplainEs

    @field_validator("context")
    @classmethod
    def short_context(cls, v: str) -> str:
        if len(v) > 140:
            raise ValueError("context: máximo 140 caracteres")
        return v


def _single_spaced(text: str) -> str:
    if "  " in text or text != text.strip():
        raise ValueError("text: sin espacios dobles ni en los bordes")
    return text


class Cloze(ItemBase):
    type: Literal["cloze"]
    text: NonEmpty
    answers: Annotated[list[NonEmpty], Field(min_length=1)]

    @model_validator(mode="after")
    def one_blank(self) -> Cloze:
        _single_spaced(self.text)
        if self.text.count("___") != 1:
            raise ValueError("cloze: el texto tiene que tener exactamente un ___")
        normalized = [normalize(a) for a in self.answers]
        if len(set(normalized)) != len(normalized):
            raise ValueError("cloze: respuestas duplicadas después de normalizar")
        return self


class Choice(ItemBase):
    type: Literal["choice"]
    text: NonEmpty
    options: Annotated[list[NonEmpty], Field(min_length=2, max_length=4)]
    answer: Annotated[int, Field(ge=0)]

    @model_validator(mode="after")
    def valid_answer(self) -> Choice:
        _single_spaced(self.text)
        if self.answer >= len(self.options):
            raise ValueError("choice: answer fuera de rango")
        if len({normalize(o) for o in self.options}) != len(self.options):
            raise ValueError("choice: opciones repetidas")
        return self


class ExplainWhy(ItemBase):
    type: Literal["explain_why"]
    text: NonEmpty
    focus: NonEmpty
    question: NonEmpty
    options: Annotated[list[NonEmpty], Field(min_length=3, max_length=4)]
    answer: Annotated[int, Field(ge=0)]

    @model_validator(mode="after")
    def valid(self) -> ExplainWhy:
        _single_spaced(self.text)
        if self.text.count(self.focus) != 1:
            raise ValueError("explain_why: focus tiene que aparecer exactamente una vez en text")
        if self.answer >= len(self.options):
            raise ValueError("explain_why: answer fuera de rango")
        if len({normalize(o) for o in self.options}) != len(self.options):
            raise ValueError("explain_why: opciones repetidas")
        return self


class FixError(ItemBase):
    """Una oración con UNA palabra mal. Se toca la palabra y se escribe la correcta."""

    type: Literal["fix_error"]
    text: NonEmpty
    wrong: NonEmpty
    corrections: Annotated[list[NonEmpty], Field(min_length=1)]

    @model_validator(mode="after")
    def valid(self) -> FixError:
        _single_spaced(self.text)
        cores = [normalize(token_core(t)) for t in tokens(self.text)]
        if " " in self.wrong:
            raise ValueError("fix_error: wrong tiene que ser una sola palabra")
        if cores.count(normalize(self.wrong)) != 1:
            raise ValueError("fix_error: wrong tiene que aparecer exactamente una vez como palabra")
        if normalize(self.wrong) in {normalize(c) for c in self.corrections}:
            raise ValueError("fix_error: una corrección es igual a la palabra mal")
        return self


Item = Annotated[Cloze | Choice | ExplainWhy | FixError, Field(discriminator="type")]


class ItemFile(Strict):
    skill: Annotated[str, Field(pattern=SKILL_ID)]
    items: Annotated[list[Item], Field(min_length=1)]


# ── Glosario: lo que se consulta, no lo que se practica ───────────────────
#
# Vive en content/glossary/*.yaml. Se embebe junto con los ítems para poder
# buscarlo al instante y sin internet.


class VerbEntry(Strict):
    base: NonEmpty
    past: NonEmpty
    participle: NonEmpty
    es: NonEmpty
    example: NonEmpty
    note_es: str = ""

    @model_validator(mode="after")
    def example_uses_the_verb(self) -> VerbEntry:
        forms = {normalize(self.base), normalize(self.past), normalize(self.participle)}
        words = {normalize(token_core(t)) for t in tokens(self.example)}
        if not forms & words:
            raise ValueError("el ejemplo tiene que usar alguna forma del verbo")
        return self


class RuleEntry(Strict):
    id: Annotated[str, Field(pattern=ITEM_ID)]
    title_en: NonEmpty
    when_es: NonEmpty
    examples: Annotated[list[NonEmpty], Field(min_length=2)]
    note_es: str = ""

    @field_validator("examples")
    @classmethod
    def arrow_format(cls, v: list[str]) -> list[str]:
        for e in v:
            if "→" not in e:
                raise ValueError(f"el ejemplo {e!r} tiene que ser 'base → forma'")
        return v


class TermEntry(Strict):
    term: NonEmpty
    type: Literal["term", "chunk", "phrasal", "false_friend"]
    es: NonEmpty
    example: NonEmpty
    note_es: str = ""


class CheatRow(Strict):
    name: NonEmpty
    form: NonEmpty
    use_es: NonEmpty
    example: NonEmpty


class Cheatsheet(Strict):
    id: Annotated[str, Field(pattern=ITEM_ID)]
    title_en: NonEmpty
    title_es: NonEmpty
    summary_es: NonEmpty
    rows: Annotated[list[CheatRow], Field(min_length=2)]
    notes_es: list[NonEmpty] = []
    skills: list[Annotated[str, Field(pattern=SKILL_ID)]] = []


class VerbsFile(Strict):
    kind: Literal["verbs"]
    entries: Annotated[list[VerbEntry], Field(min_length=1)]


class RulesFile(Strict):
    kind: Literal["rules"]
    entries: Annotated[list[RuleEntry], Field(min_length=1)]


class TermsFile(Strict):
    kind: Literal["terms"]
    entries: Annotated[list[TermEntry], Field(min_length=1)]


class CheatsheetFile(Strict):
    kind: Literal["cheatsheet"]
    sheet: Cheatsheet


GlossaryFile = Annotated[
    VerbsFile | RulesFile | TermsFile | CheatsheetFile, Field(discriminator="kind")
]


# ── Lecciones: lo que se estudia antes de practicar ───────────────────────
#
# Vive en content/lessons/<habilidad>.yaml. Una lección se lee en 5 a 8
# minutos y termina en práctica, así que cada bloque tiene que ganarse el lugar.


class LessonBlock(Strict):
    """Un bloque de la lección. `en` es lo que se lee; `es` es la red de abajo."""

    kind: Literal["idea", "form", "contrast", "trap", "chunks"]
    title_en: NonEmpty
    body_en: NonEmpty
    body_es: str = ""
    # Ejemplos de trabajo; en los contrastes van de a pares, con su por qué.
    examples: list[NonEmpty] = []
    pairs: list["LessonPair"] = []

    @model_validator(mode="after")
    def shape_matches_kind(self) -> LessonBlock:
        if self.kind == "contrast" and len(self.pairs) < 2:
            raise ValueError("un bloque de contraste necesita al menos 2 pares")
        if self.kind in {"form", "chunks"} and not self.examples:
            raise ValueError(f"un bloque {self.kind} necesita ejemplos")
        if self.kind != "contrast" and self.pairs:
            raise ValueError("sólo los bloques de contraste llevan pares")
        return self


class LessonPair(Strict):
    """Dos oraciones casi iguales: la diferencia es la lección."""

    a: NonEmpty
    b: NonEmpty
    difference_es: NonEmpty


class Lesson(Strict):
    skill: Annotated[str, Field(pattern=SKILL_ID)]
    title_en: NonEmpty
    # Qué vas a poder hacer al terminar, en una línea y en primera persona.
    goal_en: NonEmpty
    minutes: Annotated[int, Field(ge=3, le=15)]
    blocks: Annotated[list[LessonBlock], Field(min_length=2)]
    # Chuletas del glosario que amplían el tema.
    cheatsheets: list[Annotated[str, Field(pattern=ITEM_ID)]] = []

    @model_validator(mode="after")
    def starts_with_the_idea(self) -> Lesson:
        if self.blocks[0].kind != "idea":
            raise ValueError("la lección tiene que abrir con un bloque 'idea': primero para qué sirve")
        if not any(b.kind == "contrast" for b in self.blocks):
            raise ValueError("toda lección necesita al menos un bloque de contraste")
        return self


# ── Drills orales: lo único que mide hablar ───────────────────────────────
#
# Viven en content/drills/<habilidad>.yaml. La clave del diseño: la app SABE qué
# pronombre corresponde, así que la corrección no depende de que la
# transcripción sea perfecta (Whisper a veces "corrige" la gramática).

PRONOUNS = {
    "he", "she", "they", "him", "her", "them",
    "his", "hers", "their", "theirs", "its",
}


class Drill(Strict):
    id: Annotated[str, Field(pattern=ITEM_ID)]
    # pronouns: se chequea qué pronombres usaste. free: sólo transcripción y feedback.
    kind: Literal["pronouns", "free"]
    seconds: Annotated[int, Field(ge=15, le=90)]
    context: NonEmpty
    prompt_en: NonEmpty
    hint_es: str = ""
    # Los que tenés que usar y los que delatan el error.
    expect: list[NonEmpty] = []
    avoid: list[NonEmpty] = []

    @model_validator(mode="after")
    def coherent(self) -> Drill:
        if self.kind == "pronouns":
            if not self.expect:
                raise ValueError("un drill de pronombres necesita `expect`")
            for word in [*self.expect, *self.avoid]:
                if normalize(word) not in PRONOUNS:
                    raise ValueError(f"{word!r} no es un pronombre de los que se chequean")
            if set(map(normalize, self.expect)) & set(map(normalize, self.avoid)):
                raise ValueError("un pronombre no puede estar en expect y en avoid a la vez")
        elif self.expect or self.avoid:
            raise ValueError("un drill libre no lleva expect ni avoid")
        return self


class DrillFile(Strict):
    skill: Annotated[str, Field(pattern=SKILL_ID)]
    drills: Annotated[list[Drill], Field(min_length=1)]
