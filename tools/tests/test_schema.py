import pytest
from pydantic import TypeAdapter, ValidationError

from lit_tools.schema import Item, normalize, token_core

item = TypeAdapter(Item)

EXPLAIN = {"rule": "r", "analogy": "a", "why": "w"}


def base(**kw):
    return {"id": "x-01", "difficulty": 1, "rule": "A rule.", "explain_es": EXPLAIN, **kw}


def test_normalize():
    assert normalize("  I’ve   Finished. ") == "i've finished"
    assert normalize("Have finished") == normalize("have finished.")


def test_token_core_keeps_apostrophes():
    assert token_core("don't,") == "don't"
    assert token_core('"fix".') == "fix"


def test_cloze_needs_exactly_one_blank():
    item.validate_python(base(type="cloze", text="I ___ it.", answers=["did"]))
    with pytest.raises(ValidationError):
        item.validate_python(base(type="cloze", text="I did it.", answers=["did"]))
    with pytest.raises(ValidationError):
        item.validate_python(base(type="cloze", text="I ___ it ___.", answers=["did"]))


def test_cloze_rejects_duplicate_answers_after_normalizing():
    with pytest.raises(ValidationError):
        item.validate_python(base(type="cloze", text="I ___ it.", answers=["Did", "did."]))


def test_choice_answer_in_range():
    with pytest.raises(ValidationError):
        item.validate_python(base(type="choice", text="Pick ___.", options=["a", "b"], answer=2))


def test_explain_why_focus_must_appear_once():
    with pytest.raises(ValidationError):
        item.validate_python(base(type="explain_why", text="I did it.", focus="done",
                                  question="Why?", options=["a", "b", "c"], answer=0))


def test_fix_error_wrong_word_appears_once():
    item.validate_python(base(type="fix_error", text="She have a PR open.", wrong="have", corrections=["has"]))
    with pytest.raises(ValidationError):
        item.validate_python(base(type="fix_error", text="She have have a PR.", wrong="have", corrections=["has"]))
    with pytest.raises(ValidationError):
        item.validate_python(base(type="fix_error", text="She has a PR.", wrong="has", corrections=["has"]))


def test_rejects_double_spaces_and_unknown_fields():
    with pytest.raises(ValidationError):
        item.validate_python(base(type="cloze", text="I  ___ it.", answers=["did"]))
    with pytest.raises(ValidationError):
        item.validate_python(base(type="cloze", text="I ___ it.", answers=["did"], extra=True))


def test_real_content_is_valid():
    """El banco real de content/ valida completo: es lo que embebe la API."""
    from lit_tools.build import load

    bundle, report = load()
    assert report.errors == []
    assert bundle is not None and len(bundle["items"]) > 0
