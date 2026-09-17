package placement

import (
	"os"
	"testing"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/content"
)

func fixture(t *testing.T) (*content.Catalog, *content.PlacementPart) {
	t.Helper()
	data, err := os.ReadFile("../content/testdata/bundle.json")
	if err != nil {
		t.Fatal(err)
	}
	c, err := content.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	p, ok := c.Part("tenses")
	if !ok {
		t.Fatal("falta la parte tenses en el fixture")
	}
	return c, p
}

func nextID(c *content.Catalog, p *content.PlacementPart, a []Answer) string {
	if it := Next(c, p, a); it != nil {
		return it.ID
	}
	return ""
}

func TestNextStopsEarlyWhenFirstTwoAreRight(t *testing.T) {
	c, p := fixture(t)
	a := []Answer{}
	if got := nextID(c, p, a); got != "a-01" {
		t.Fatalf("primero = %s, want a-01", got)
	}
	a = append(a, Answer{ItemID: "a-01", Correct: true})
	if got := nextID(c, p, a); got != "a-02" {
		t.Fatalf("segundo = %s, want a-02", got)
	}
	a = append(a, Answer{ItemID: "a-02", Correct: true})
	// dos de dos: se saltea a-03 y pasa a la habilidad B, de fácil a difícil
	if got := nextID(c, p, a); got != "b-01" {
		t.Fatalf("después de 2/2 = %s, want b-01", got)
	}
}

func TestNextAsksThirdWhenOneFails(t *testing.T) {
	c, p := fixture(t)
	a := []Answer{{ItemID: "a-01", Correct: true}, {ItemID: "a-02"}}
	if got := nextID(c, p, a); got != "a-03" {
		t.Fatalf("got %s, want a-03", got)
	}
}

func TestPartFinishes(t *testing.T) {
	c, p := fixture(t)
	a := []Answer{
		{ItemID: "a-01"}, {ItemID: "a-02", Correct: true}, {ItemID: "a-03", Correct: true},
		{ItemID: "b-01", Correct: true}, {ItemID: "b-02", Correct: true},
	}
	if got := nextID(c, p, a); got != "" {
		t.Fatalf("la parte debía terminar, sigue %s", got)
	}
	out := Outcomes(c, p, a)
	if out[0].State != Suspendida || out[0].Asked != 3 || out[0].Score != 2 {
		t.Errorf("A = %+v, want suspendida 2/3", out[0])
	}
	if out[1].State != Calzada || out[1].Asked != 2 {
		t.Errorf("B = %+v, want calzada 2/2", out[1])
	}
	if pr := ProgressOf(c, p, a); pr.Answered != 5 || pr.Max != 5 {
		t.Errorf("progress = %+v, want 5/5", pr)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		asked int
		score float32
		want  State
	}{
		{0, 0, Plano}, {2, 2, Calzada}, {3, 2, Suspendida}, {3, 1, Plano}, {3, 0, Plano},
		// dos aciertos consultados no alcanzan para dar la habilidad por dominada
		{2, 1, Plano}, {3, 2.5, Suspendida},
	}
	for _, tc := range cases {
		if got, _ := classify(tc.asked, tc.score); got != tc.want {
			t.Errorf("classify(%d,%v) = %s, want %s", tc.asked, tc.score, got, tc.want)
		}
	}
}

func TestDuplicateAnswersCountOnce(t *testing.T) {
	c, p := fixture(t)
	a := []Answer{{ItemID: "a-01"}, {ItemID: "a-01", Correct: true}}
	out := Outcomes(c, p, a)
	if out[0].Asked != 1 || out[0].Score != 0 {
		t.Fatalf("la primera respuesta manda: %+v", out[0])
	}
}

func TestProgressMaxShrinksOnEarlyStop(t *testing.T) {
	c, p := fixture(t)
	if pr := ProgressOf(c, p, nil); pr.Max != 6 {
		t.Fatalf("max inicial = %d, want 6", pr.Max)
	}
	pr := ProgressOf(c, p, []Answer{{ItemID: "a-01", Correct: true}, {ItemID: "a-02", Correct: true}})
	if pr.Answered != 2 || pr.Max != 5 {
		t.Fatalf("progress = %+v, want 2/5", pr)
	}
}

func TestConsultedAnswerDoesNotStopTheSkillEarly(t *testing.T) {
	c, p := fixture(t)
	// Dos aciertos, pero el segundo con el glosario abierto: se pregunta el tercero.
	a := []Answer{
		{ItemID: "a-01", Correct: true},
		{ItemID: "a-02", Correct: true, Consulted: true},
	}
	if got := nextID(c, p, a); got != "a-03" {
		t.Fatalf("got %s, want a-03", got)
	}
	a = append(a, Answer{ItemID: "a-03", Correct: true})
	out := Outcomes(c, p, a)
	if out[0].Score != 2.5 || out[0].State != Suspendida {
		t.Fatalf("una habilidad consultada no puede quedar calzada: %+v", out[0])
	}
}
