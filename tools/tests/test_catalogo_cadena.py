"""El catálogo que se despliega tiene que ser el currículum, no una copia vieja.

`contracts/script/catalogo.json` es la lista de las 41 distinciones que recibe el
constructor del contrato. Se escribe una vez y se despliega una vez, y después
es inmutable: si sale con un tema de menos, de más o en otro orden, el número de
pieza que firma el servidor deja de apuntar a lo que dice que apunta, y no hay
forma de arreglarlo sin desplegar de nuevo.

Los tests del contrato comprueban que el catálogo sea coherente consigo mismo.
Éste comprueba lo otro: que sea el correcto.
"""

from __future__ import annotations

import json
from pathlib import Path

import yaml

RAIZ = Path(__file__).resolve().parents[2]
CATALOGO = RAIZ / "contracts" / "script" / "catalogo.json"
SKILLS = RAIZ / "content" / "skills.yaml"


def _distinciones() -> list[dict]:
    return json.loads(CATALOGO.read_text())["distinciones"]


def _habilidades_del_plan() -> list[str]:
    obras = yaml.safe_load(SKILLS.read_text())["obras"]
    return [
        s["id"]
        for o in obras
        for p in (o.get("pieces") or [])
        for s in (p.get("skills") or [])
    ]


def _obras_del_plan() -> list[int]:
    return [o["id"] for o in yaml.safe_load(SKILLS.read_text())["obras"]]


def test_estan_todas_las_habilidades_y_en_el_mismo_orden():
    # El orden importa porque el índice en esta lista ES el número de pieza que
    # viaja en el voucher: correrlo una posición cambia qué distinción se acuña.
    piezas = [
        d["codigo"].removeprefix("pieza:")
        for d in _distinciones()
        if d["codigo"].startswith("pieza:")
    ]
    assert piezas == _habilidades_del_plan()


def test_estan_todas_las_obras():
    obras = [
        int(d["codigo"].removeprefix("obra:"))
        for d in _distinciones()
        if d["codigo"].startswith("obra:")
    ]
    assert obras == _obras_del_plan()


def test_no_hay_codigos_repetidos():
    codigos = [d["codigo"] for d in _distinciones()]
    assert len(codigos) == len(set(codigos))


def test_cada_distincion_tiene_lo_que_el_constructor_necesita():
    for d in _distinciones():
        assert d["nombre"].strip(), d["codigo"]
        assert isinstance(d["obra"], int)
        assert isinstance(d["esObra"], bool)
        # Los diez perfiles de junta de pilar.ts, portados al contrato.
        assert 0 <= d["juntaArriba"] <= 9, d["codigo"]
        assert 0 <= d["juntaAbajo"] <= 9, d["codigo"]


def test_las_juntas_vecinas_encajan():
    # Dos piezas que se tocan comparten la junta: la de abajo de una es la de
    # arriba de la siguiente. Es lo que hace que el pilar cierre sin costura.
    piezas = [d for d in _distinciones() if not d["esObra"]]
    for anterior, siguiente in zip(piezas, piezas[1:]):
        if anterior["obra"] != siguiente["obra"]:
            continue
        assert anterior["juntaAbajo"] == siguiente["juntaArriba"], (
            f"{anterior['codigo']} y {siguiente['codigo']} no calzan"
        )
