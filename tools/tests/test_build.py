"""Dos filtraciones que un alumno detecta sin saber inglés: que la correcta esté
casi siempre primera y que sea siempre la más larga. El build arregla la primera
y el validador rechaza la segunda."""

from lit_tools.build import Report, check_option_balance, check_position_references, shuffle_options


def item(id, answer, options, kind="explain_why"):
    return {"id": id, "type": kind, "answer": answer, "options": list(options)}


def test_shuffle_keeps_the_right_answer():
    it = item("x-01", 0, ["correcta", "a", "b", "c"])
    shuffle_options(it)
    assert it["options"][it["answer"]] == "correcta"
    assert sorted(it["options"]) == sorted(["correcta", "a", "b", "c"])


def test_shuffle_is_deterministic():
    a, b = item("x-01", 1, ["p", "q", "r"]), item("x-01", 1, ["p", "q", "r"])
    shuffle_options(a)
    shuffle_options(b)
    assert a == b


def test_shuffle_spreads_the_answer_across_positions():
    items = [item(f"x-{i:02d}", 0, ["correcta", "a", "b", "c"]) for i in range(40)]
    for it in items:
        shuffle_options(it)
    positions = {it["answer"] for it in items}
    assert len(positions) == 4, "la correcta tiene que caer en las cuatro posiciones"
    report = Report()
    check_option_balance(report, items)
    assert report.errors == []


def test_shuffle_ignores_items_without_options():
    it = {"id": "x-01", "type": "cloze", "answers": ["did"]}
    shuffle_options(it)
    assert it == {"id": "x-01", "type": "cloze", "answers": ["did"]}


def test_flags_a_correct_option_that_is_much_longer():
    report = Report()
    check_option_balance(report, [item("x-01", 0, ["una explicación completa y larga de la regla", "corto", "corto2"])])
    assert any("por el largo" in e for e in report.errors)


def test_allows_a_small_difference():
    report = Report()
    check_option_balance(report, [item("x-01", 0, ["una explicación de la regla", "otra explicación igual"])])
    assert report.errors == []


def test_ignores_short_options_where_length_says_nothing():
    report = Report()
    check_option_balance(report, [item("x-01", 0, ["actually", "in fact", "really"], kind="choice")])
    assert report.errors == []


def test_flags_the_answer_being_first_too_often():
    report = Report()
    check_option_balance(report, [item(f"x-{i}", 0, ["aaaa", "bbbb", "cccc"]) for i in range(20)])
    assert any("primera" in e for e in report.errors)


def test_two_option_items_may_be_first_about_half_the_time():
    """Con dos opciones, el azar ya da 50%: el tope no puede ser fijo."""
    items = [item(f"x-{i}", i % 2, ["aaaa", "bbbb"], kind="choice") for i in range(20)]
    report = Report()
    check_option_balance(report, items)
    assert report.errors == []


def explain(id, why, options=("a", "b")):
    return {"id": id, "type": "choice", "answer": 0, "options": list(options),
            "rule": "r", "explain_es": {"rule": "r", "analogy": "a", "why": why}}


def test_flags_explanations_that_cite_option_positions():
    for why in [
        "La primera es la correcta.",
        "La segunda opción es un calco.",
        "Ver opción 2.",
        "La tercera suena a otra cosa.",
    ]:
        report = Report()
        check_position_references(report, [explain("x-01", why)])
        assert report.errors, why


def test_allows_talking_about_parts_of_the_sentence():
    for why in [
        "La segunda parte de la oración va en afirmación.",
        "La primera vez algo aparece con a/an.",
        "La tercera línea del log lo explica.",
        # Un sustantivo cualquiera después del ordinal no es una referencia a opciones
        "La segunda alerta llegó cinco minutos después.",
        "La primera migración tardó una hora.",
    ]:
        report = Report()
        check_position_references(report, [explain("x-01", why)])
        assert report.errors == [], why
