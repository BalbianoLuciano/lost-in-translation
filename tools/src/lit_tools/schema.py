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
