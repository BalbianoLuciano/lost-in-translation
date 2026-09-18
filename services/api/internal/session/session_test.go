package session

import (
	"os"
	"testing"
	"time"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/content"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/placement"
)

func fixtureCatalog(t *testing.T) *content.Catalog {
	t.Helper()
	data, err := os.ReadFile("../content/testdata/bundle.json")
	if err != nil {
		t.Fatal(err)
	}
	c, err := content.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func lesson(skill string) *content.Lesson { return &content.Lesson{Skill: skill} }

func viewWith(skills ...string) *CatalogView {
	v := &CatalogView{order: map[string]int{}}
	for i, s := range skills {
		v.order[s] = i
		v.lessons = append(v.lessons, lesson(s))
	}
	return v
}

func TestNextLessonTakesTheWeakestFirst(t *testing.T) {
	v := viewWith("a.uno", "b.dos", "c.tres")
	mastery := map[string]Mastery{
		"a.uno":  {State: placement.Calzada, Mastery: 0.9},
		"b.dos":  {State: placement.Suspendida, Mastery: 0.6},
		"c.tres": {State: placement.Plano, Mastery: 0.2},
	}
	got := NextLesson(v, mastery, nil, nil)
	if got == nil || got.SkillID != "c.tres" {
		t.Fatalf("got %+v, want c.tres (está en plano)", got)
	}
}

func TestNextLessonSkipsFinishedOnes(t *testing.T) {
	v := viewWith("a.uno", "b.dos")
	mastery := map[string]Mastery{"a.uno": {State: placement.Plano}, "b.dos": {State: placement.Plano}}
	got := NextLesson(v, mastery, map[string]bool{"a.uno": true}, nil)
	if got == nil || got.SkillID != "b.dos" {
		t.Fatalf("got %+v, want b.dos", got)
	}
}

func TestNextLessonFallsBackToCurriculumOrder(t *testing.T) {
	v := viewWith("a.uno", "b.dos")
	got := NextLesson(v, map[string]Mastery{}, nil, nil) // sin diagnóstico: todo en plano
	if got == nil || got.SkillID != "a.uno" {
		t.Fatalf("got %+v, want a.uno (viene antes en el currículum)", got)
	}
}

func TestNextLessonWithNothingLeft(t *testing.T) {
	v := viewWith("a.uno")
	if got := NextLesson(v, map[string]Mastery{}, map[string]bool{"a.uno": true}, nil); got != nil {
		t.Fatalf("got %+v, want nil", got)
	}
}

// Un tema empezado se termina antes de proponer otro, aunque el otro esté peor.
func TestNextLessonFinishesWhatYouStarted(t *testing.T) {
	v := viewWith("a.uno", "b.dos")
	mastery := map[string]Mastery{
		"a.uno": {State: placement.Suspendida, Mastery: 0.6},
		"b.dos": {State: placement.Plano, Mastery: 0},
	}
	got := NextLesson(v, mastery, nil, map[string]bool{"a.uno": true})
	if got == nil || got.SkillID != "a.uno" {
		t.Fatalf("got %+v, want a.uno (ya estaba empezada)", got)
	}
}

func TestCatalogViewOrdersLessonsByCurriculum(t *testing.T) {
	v := NewCatalogView(fixtureCatalog(t))
	if len(v.Lessons()) != 1 || v.Lessons()[0].Skill != "tense.fixture_a" {
		t.Fatalf("lecciones del fixture inesperadas: %+v", v.Lessons())
	}
	if v.Order("tense.fixture_a") >= v.Order("tense.fixture_b") {
		t.Error("el orden tiene que seguir el currículum")
	}
	if v.Order("no.existe") != 1<<30 {
		t.Error("una habilidad desconocida va al final")
	}
}

func TestRecomputeMastery(t *testing.T) {
	ok := Attempt{Correct: true}
	bad := Attempt{}
	consulted := Attempt{Correct: true, Consulted: true}

	tests := []struct {
		name     string
		previous placement.State
		attempts []Attempt
		want     placement.State
	}{
		{name: "pocos intentos no mueven nada", previous: placement.Plano, attempts: []Attempt{ok, ok, ok}, want: placement.Plano},
		{name: "todo bien queda calzada", previous: placement.Suspendida, attempts: []Attempt{ok, ok, ok, ok}, want: placement.Calzada},
		{name: "mitad y mitad queda suspendida", previous: placement.Plano, attempts: []Attempt{ok, ok, bad, ok, ok, bad}, want: placement.Suspendida},
		{name: "casi todo mal vuelve a plano", previous: placement.Plano, attempts: []Attempt{bad, bad, bad, ok}, want: placement.Plano},
		{name: "lo que estaba firme y se cae, se oxida", previous: placement.Calzada, attempts: []Attempt{bad, bad, bad, ok}, want: "oxidada"},
		{name: "aciertos consultados no alcanzan para calzada", previous: placement.Suspendida, attempts: []Attempt{consulted, consulted, consulted, consulted}, want: placement.Suspendida},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, score := RecomputeMastery(tt.previous, tt.attempts)
			if got != tt.want {
				t.Fatalf("estado = %s (score %.2f), want %s", got, score, tt.want)
			}
		})
	}
}

func TestRecomputeMasteryWeighsRecentAnswersMore(t *testing.T) {
	ok, bad := Attempt{Correct: true}, Attempt{}
	// Mismos intentos, orden distinto: primero los nuevos.
	mejorando, _ := RecomputeMastery(placement.Plano, []Attempt{ok, ok, bad, bad})
	empeorando, score := RecomputeMastery(placement.Plano, []Attempt{bad, bad, ok, ok})
	if mejorando == empeorando {
		t.Fatalf("mejorar y empeorar no pueden dar lo mismo: %s vs %s (%.2f)", mejorando, empeorando, score)
	}
}

func TestColada(t *testing.T) {
	if got := Colada(&content.Item{Type: content.ExplainWhy}); got != 3 {
		t.Errorf("explain_why = %d, want 3: pensar el porqué vale más", got)
	}
	if got := Colada(&content.Item{Type: content.Cloze}); got != 1 {
		t.Errorf("cloze = %d, want 1", got)
	}
}

func TestJornal(t *testing.T) {
	day := func(daysAgo, answers int) Day {
		return Day{Date: time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC).AddDate(0, 0, -daysAgo), Answers: answers}
	}
	today := time.Date(2026, 9, 17, 20, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		days []Day
		want int
	}{
		{name: "sin días", want: 0},
		{name: "hoy solo", days: []Day{day(0, 10)}, want: 1},
		{name: "tres seguidos", days: []Day{day(0, 10), day(1, 8), day(2, 6)}, want: 3},
		{name: "un día libre no corta", days: []Day{day(0, 10), day(2, 8), day(3, 7)}, want: 3},
		{name: "dos días sin obra cortan", days: []Day{day(0, 10), day(3, 8)}, want: 1},
		{name: "ayer también cuenta", days: []Day{day(1, 10), day(2, 8)}, want: 2},
		{name: "hace tres días se perdió", days: []Day{day(3, 10), day(4, 9)}, want: 0},
		{name: "un día flojo no cuenta", days: []Day{day(0, 2), day(1, 9)}, want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Jornal(tt.days, today); got != tt.want {
				t.Fatalf("Jornal = %d, want %d", got, tt.want)
			}
		})
	}
}
