"""Carga `content/`, valida referencias cruzadas y compila el bundle JSON."""

from __future__ import annotations

import hashlib
import json
from dataclasses import dataclass, field
from pathlib import Path

import yaml
from pydantic import ValidationError

from .schema import ItemFile, Placement, SkillMap

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
    }
    canonical = json.dumps(body, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    body["version"] = hashlib.sha256(canonical.encode()).hexdigest()[:16]
    return body, report


def render(bundle: dict) -> str:
    return json.dumps(bundle, ensure_ascii=False, sort_keys=True, indent=1) + "\n"
