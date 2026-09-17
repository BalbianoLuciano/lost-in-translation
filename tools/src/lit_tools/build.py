"""Carga `content/`, valida referencias cruzadas y compila el bundle JSON."""

from __future__ import annotations

import hashlib
import json
import random
import re
from dataclasses import dataclass, field
from pathlib import Path

import yaml
from pydantic import ValidationError

from pydantic import TypeAdapter

from .schema import GlossaryFile, ItemFile, Placement, SkillMap

glossary_file = TypeAdapter(GlossaryFile)

REPO = Path(__file__).resolve().parents[3]
CONTENT = REPO / "content"
BUNDLE = REPO / "services" / "api" / "internal" / "content" / "bundle.json"


@dataclass
class Report:
    errors: list[str] = field(default_factory=list)

    def add(self, where: str, msg: str) -> None:
        self.errors.append(f"{where}: {msg}")


class YamlError(Exception):
    pass


def _load_yaml(path: Path):
    try:
        with path.open(encoding="utf-8") as f:
            return yaml.safe_load(f)
    except yaml.YAMLError as e:
        raise YamlError(str(e).replace("\n", " ")) from e


def _pydantic_errors(report: Report, where: str, err: ValidationError) -> None:
    for e in err.errors():
        loc = ".".join(str(p) for p in e["loc"])
        report.add(where, f"{loc}: {e['msg']}")


def load(content: Path = CONTENT) -> tuple[dict | None, Report]:
    report = Report()

    try:
        skill_map = SkillMap.model_validate(_load_yaml(content / "skills.yaml"))
    except ValidationError as e:
        _pydantic_errors(report, "skills.yaml", e)
        return None, report
    except YamlError as e:
        report.add("skills.yaml", f"YAML inválido: {e}")
        return None, report
    try:
        placement = Placement.model_validate(_load_yaml(content / "placement.yaml"))
    except ValidationError as e:
        _pydantic_errors(report, "placement.yaml", e)
        return None, report
    except YamlError as e:
        report.add("placement.yaml", f"YAML inválido: {e}")
        return None, report

    skills: dict[str, dict] = {}
    for obra in skill_map.obras:
        for piece in obra.pieces:
            for s in piece.skills:
                if s.id in skills:
                    report.add("skills.yaml", f"habilidad duplicada: {s.id}")
                skills[s.id] = {"id": s.id, "piece": piece.id, "obra": obra.id,
                                "name_en": s.name_en, "name_es": s.name_es}

    items: list[dict] = []
    seen_ids: dict[str, str] = {}
    for path in sorted((content / "items").rglob("*.yaml")):
        rel = str(path.relative_to(content))
        try:
            f = ItemFile.model_validate(_load_yaml(path))
        except ValidationError as e:
            _pydantic_errors(report, rel, e)
            continue
        except YamlError as e:
            report.add(rel, f"YAML inválido: {e}")
            continue
        if f.skill not in skills:
            report.add(rel, f"skill inexistente: {f.skill}")
        for it in f.items:
            if it.id in seen_ids:
                report.add(rel, f"id repetido {it.id} (ya está en {seen_ids[it.id]})")
            seen_ids[it.id] = rel
            items.append({"skill": f.skill, **it.model_dump()})

    glossary: dict[str, list] = {"verbs": [], "rules": [], "terms": [], "cheatsheets": []}
    seen_glossary: dict[str, str] = {}
    for path in sorted((content / "glossary").rglob("*.yaml")):
        rel = str(path.relative_to(content))
        try:
            f = glossary_file.validate_python(_load_yaml(path))
        except ValidationError as e:
            _pydantic_errors(report, rel, e)
            continue
        except YamlError as e:
            report.add(rel, f"YAML inválido: {e}")
            continue
        if f.kind == "cheatsheet":
            for sid in f.sheet.skills:
                if sid not in skills:
                    report.add(rel, f"skill inexistente: {sid}")
            glossary["cheatsheets"].append(f.sheet.model_dump())
            key = f.sheet.id
            if key in seen_glossary:
                report.add(rel, f"chuleta repetida: {key} (ya está en {seen_glossary[key]})")
            seen_glossary[key] = rel
            continue
        bucket = {"verbs": "verbs", "rules": "rules", "terms": "terms"}[f.kind]
        for e in f.entries:
            key = f"{f.kind}:{getattr(e, 'base', None) or getattr(e, 'term', None) or getattr(e, 'id', '')}"
            if key in seen_glossary:
                report.add(rel, f"entrada repetida: {key} (ya está en {seen_glossary[key]})")
            seen_glossary[key] = rel
            glossary[bucket].append(e.model_dump())

    glossary["verbs"].sort(key=lambda v: v["base"])
    glossary["terms"].sort(key=lambda t: t["term"])

    for it in items:
        shuffle_options(it)
    check_option_balance(report, items)
    check_position_references(report, items)

    placement_skills: set[str] = set()
    for part in placement.parts:
        for sid in part.skills:
            if sid not in skills:
                report.add("placement.yaml", f"{part.id}: skill inexistente {sid}")
            if sid in placement_skills:
                report.add("placement.yaml", f"{part.id}: {sid} está en más de una parte")
            placement_skills.add(sid)
            n = sum(1 for i in items if i["skill"] == sid and i["placement"])
            if n < placement.items_per_skill:
                report.add("placement.yaml",
                           f"{part.id}: {sid} tiene {n} ítems de ubicación, necesita {placement.items_per_skill}")

    if report.errors:
        return None, report

    body = {
        "obras": [
            {"id": o.id, "slug": o.slug, "name": o.name, "topic_en": o.topic_en, "topic_es": o.topic_es,
             "pieces": [{"id": p.id, "name_en": p.name_en, "name_es": p.name_es,
                         "skills": [s.id for s in p.skills]} for p in o.pieces]}
            for o in skill_map.obras
        ],
        "skills": list(skills.values()),
        "placement": placement.model_dump(),
        "items": sorted(items, key=lambda i: (i["skill"], i["difficulty"], i["id"])),
        "glossary": glossary,
    }
    canonical = json.dumps(body, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    body["version"] = hashlib.sha256(canonical.encode()).hexdigest()[:16]
    return body, report


# Dos filtraciones que un alumno detecta sin saber inglés: que la respuesta
# correcta esté casi siempre primera, y que sea siempre la más larga. La primera
# se arregla acá, mezclando las opciones de forma determinista por ítem. La
# segunda no se puede automatizar: la chequea el validador y se corrige
# escribiendo distractores del mismo peso.

# La correcta no puede sacarle más de esto al distractor más largo. Diez
# caracteres son menos de dos palabras: por debajo de eso, el largo no es una
# pista. Sin este tope, la correcta termina siendo siempre la explicación
# completa y las demás, frases sueltas.
MAX_LENGTH_GAP = 10
MAX_FIRST_SHARE = 0.45   # ni puede estar primera en más de esta proporción de los ítems
MIN_SAMPLE = 10          # …con suficientes ítems como para que la proporción signifique algo


def shuffle_options(item: dict) -> None:
    """Reordena las opciones con una semilla fija: siempre igual para ese ítem."""
    options = item.get("options")
    if not options:
        return
    order = list(range(len(options)))
    random.Random(f"options:{item['id']}").shuffle(order)
    item["options"] = [options[i] for i in order]
    item["answer"] = order.index(item["answer"])


# "la primera", "la segunda opción", "opción 2"… El compilador mezcla las
# opciones, así que citar posiciones deja la explicación mintiendo. Se permite
# cuando habla de otra cosa ("la segunda parte de la oración").
POSITION_REF = re.compile(
    r"\b(?:la|las)\s+(?:primera|segunda|tercera|cuarta)\s+"
    r"(?!parte|mitad|vez|oración|palabra|línea|columna|fila|persona|opinión)"
    r"|\bopci[oó]n\s*\d",
    re.IGNORECASE,
)


def check_position_references(report: Report, items: list[dict]) -> None:
    for it in items:
        if not it.get("options"):
            continue
        texts = [it.get("rule", ""), *(it.get("explain_es") or {}).values()]
        for t in texts:
            if POSITION_REF.search(t or ""):
                report.add(it["id"], "la explicación cita la posición de una opción, y el compilador las mezcla")
                break


def check_option_balance(report: Report, items: list[dict]) -> None:
    first = {}
    total = {}
    for it in items:
        options = it.get("options")
        if not options:
            continue
        correct = options[it["answer"]]
        longest_other = max(
            (len(o) for k, o in enumerate(options) if k != it["answer"]), default=0
        )
        gap = len(correct) - longest_other
        if gap > MAX_LENGTH_GAP:
            report.add(
                it["id"],
                f"la opción correcta ({len(correct)} caracteres) le saca {gap} al mayor "
                f"distractor ({longest_other}): se adivina por el largo, sin saber inglés",
            )
        total[it["type"]] = total.get(it["type"], 0) + 1
        if it["answer"] == 0:
            first[it["type"]] = first.get(it["type"], 0) + 1

    for kind, n in total.items():
        if n < MIN_SAMPLE:
            continue
        share = first.get(kind, 0) / n
        if share > MAX_FIRST_SHARE:
            report.add(kind, f"la correcta está primera en el {share:.0%} de los ítems (máximo {MAX_FIRST_SHARE:.0%})")


def render(bundle: dict) -> str:
    return json.dumps(bundle, ensure_ascii=False, sort_keys=True, indent=1) + "\n"
