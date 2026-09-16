"""CLI de contenido.

    uv run lit-tools validate      valida content/
    uv run lit-tools build         valida y escribe el bundle que embebe la API
    uv run lit-tools check         falla si el bundle no está al día (CI)
    uv run lit-tools stats         cuántos ítems hay por habilidad y tipo
"""

from __future__ import annotations

import sys
from collections import Counter

from .build import BUNDLE, load, render


def main() -> None:
    cmd = sys.argv[1] if len(sys.argv) > 1 else "validate"
    bundle, report = load()
    if report.errors:
        print(f"✗ {len(report.errors)} errores en content/:", file=sys.stderr)
        for e in report.errors:
            print(f"  - {e}", file=sys.stderr)
        sys.exit(1)
    assert bundle is not None

    if cmd == "validate":
        print(f"✓ content/ válido · {len(bundle['items'])} ítems · {len(bundle['skills'])} habilidades")
    elif cmd == "build":
        BUNDLE.parent.mkdir(parents=True, exist_ok=True)
        BUNDLE.write_text(render(bundle), encoding="utf-8")
        print(f"✓ bundle {bundle['version']} → {BUNDLE.relative_to(BUNDLE.parents[4])}")
    elif cmd == "check":
        current = BUNDLE.read_text(encoding="utf-8") if BUNDLE.exists() else ""
        if current != render(bundle):
            print("✗ el bundle no está al día: correr `uv run lit-tools build`", file=sys.stderr)
            sys.exit(1)
        print(f"✓ bundle al día ({bundle['version']})")
    elif cmd == "stats":
        by_skill = Counter(i["skill"] for i in bundle["items"])
        by_type = Counter(i["type"] for i in bundle["items"])
        for s in bundle["skills"]:
            print(f"{by_skill.get(s['id'], 0):3d}  {s['id']}")
        print("tipos:", dict(by_type))
    else:
        print(__doc__)
        sys.exit(2)
