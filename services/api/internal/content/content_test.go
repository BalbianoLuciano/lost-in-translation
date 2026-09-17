package content

import (
	"os"
	"testing"
)

func fixture(t *testing.T) *Catalog {
	t.Helper()
	data, err := os.ReadFile("testdata/bundle.json")
	if err != nil {
		t.Fatal(err)
	}
	c, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func ptr(i int) *int { return &i }

// Mismos casos que tools/tests/test_schema.py: si divergen, un ítem válido en
// Python se corrige distinto en Go.
func TestNormalizeMatchesPython(t *testing.T) {
	cases := map[string]string{
		"  I’ve   Finished. ": "i've finished",
		"Have finished":       "have finished",
		"have finished.":      "have finished",
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
	if tokenCore(`don't,`) != "don't" || tokenCore(`"fix".`) != "fix" {
		t.Error("tokenCore no respeta apóstrofos o no limpia puntuación")
	}
}

func TestPlacementItemsSortedAndTrimmed(t *testing.T) {
	c := fixture(t)
	got := c.PlacementItems("tense.fixture_b")
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	for i, want := range []string{"b-01", "b-02", "b-03"} {
		if got[i].ID != want {
			t.Errorf("posición %d = %s, want %s", i, got[i].ID, want)
		}
	}
	for _, it := range c.PlacementItems("tense.fixture_a") {
		if it.ID == "a-04" {
			t.Error("a-04 no es de ubicación y apareció")
		}
	}
}

func TestGrade(t *testing.T) {
	c := fixture(t)
	item := func(id string) *Item {
		it, ok := c.Item(id)
		if !ok {
			t.Fatalf("no existe %s", id)
		}
		return it
	}

	tests := []struct {
		name        string
		item        string
		resp        Response
		wantCorrect bool
		wantPartOk  bool
		wantErr     bool
	}{
		{name: "cloze exacta", item: "a-01", resp: Response{Text: "have finished"}, wantCorrect: true},
		{name: "cloze contracción y mayúsculas", item: "a-01", resp: Response{Text: "’VE Finished."}, wantCorrect: true},
		{name: "cloze incorrecta", item: "a-01", resp: Response{Text: "finished"}},
		{name: "cloze vacía", item: "a-01", resp: Response{Text: "  "}},
		{name: "choice correcta", item: "a-02", resp: Response{Choice: ptr(0)}, wantCorrect: true},
		{name: "choice incorrecta", item: "a-02", resp: Response{Choice: ptr(1)}},
		{name: "choice fuera de rango", item: "a-02", resp: Response{Choice: ptr(5)}, wantErr: true},
		{name: "choice sin elección", item: "a-02", resp: Response{}, wantErr: true},
		{name: "explain_why correcta", item: "b-03", resp: Response{Choice: ptr(1)}, wantCorrect: true},
		{name: "fix_error correcta", item: "a-03", resp: Response{TokenIndex: ptr(1), Text: "has"}, wantCorrect: true},
		{name: "fix_error palabra bien, corrección mal", item: "a-03", resp: Response{TokenIndex: ptr(1), Text: "had"}, wantPartOk: true},
		{name: "fix_error palabra mal", item: "a-03", resp: Response{TokenIndex: ptr(0), Text: "has"}},
		{name: "fix_error índice inválido", item: "a-03", resp: Response{TokenIndex: ptr(99), Text: "has"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := Grade(item(tt.item), tt.resp)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if res.Correct != tt.wantCorrect || res.PartOk != tt.wantPartOk {
				t.Fatalf("got correct=%v partOk=%v, want %v/%v", res.Correct, res.PartOk, tt.wantCorrect, tt.wantPartOk)
			}
			if res.Expected == "" {
				t.Fatal("Expected vacío: la corrección tiene que mostrar la respuesta")
			}
		})
	}
}

func TestPublicItemHidesAnswers(t *testing.T) {
	c := fixture(t)
	it, _ := c.Item("a-03")
	p := it.Public()
	if len(p.Tokens) != 8 || p.Tokens[1] != "have" {
		t.Fatalf("tokens inesperados: %v", p.Tokens)
	}
	// PublicItem no tiene campos de respuesta: esto es un chequeo de compilación
	// que documenta la intención.
	var _ = PublicItem{ID: p.ID}
}

func TestParseRejectsBrokenBundles(t *testing.T) {
	if _, err := Parse([]byte(`{`)); err == nil {
		t.Error("JSON roto debía fallar")
	}
	bad := `{"skills":[],"items":[{"id":"x","skill":"nope","type":"cloze"}],"placement":{"items_per_skill":3,"parts":[]}}`
	if _, err := Parse([]byte(bad)); err == nil {
		t.Error("ítem con habilidad inexistente debía fallar")
	}
}

func TestEmbeddedBundleLoads(t *testing.T) {
	if _, err := Load(); err != nil {
		t.Fatalf("el bundle embebido no carga: %v", err)
	}
}

// Recorre el banco real: la respuesta correcta de cada ítem tiene que corregirse
// como correcta, y una respuesta equivocada como incorrecta.
func TestEveryEmbeddedItemGradesItsOwnAnswer(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for i := range c.Items {
		it := &c.Items[i]
		t.Run(it.ID, func(t *testing.T) {
			var right, wrong Response
			switch it.Type {
			case Cloze:
				right = Response{Text: it.Answers[0]}
				wrong = Response{Text: "zzz"}
			case Choice, ExplainWhy:
				right = Response{Choice: ptr(it.Answer)}
				wrong = Response{Choice: ptr((it.Answer + 1) % len(it.Options))}
			case FixError:
				right = Response{TokenIndex: ptr(it.wrongIndex), Text: it.Corrections[0]}
				wrong = Response{TokenIndex: ptr(it.wrongIndex), Text: it.Wrong}
			}
			if res, err := Grade(it, right); err != nil || !res.Correct {
				t.Fatalf("la respuesta correcta no corrige como correcta: %+v %v", res, err)
			}
			if res, err := Grade(it, wrong); err != nil || res.Correct {
				t.Fatalf("una respuesta equivocada corrige como correcta: %+v %v", res, err)
			}
		})
	}
	for _, p := range c.Placement.Parts {
		for _, s := range p.Skills {
			if n := len(c.PlacementItems(s)); n != c.Placement.ItemsPerSkill {
				t.Errorf("%s: %d ítems de ubicación", s, n)
			}
		}
	}
}
