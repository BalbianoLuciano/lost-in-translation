package session

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/content"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/db"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/placement"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/store"
)

func testService(t *testing.T) (*Service, *content.Catalog, pgtype.UUID, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL no definida")
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	u, err := store.New(pool).UpsertUser(ctx, store.UpsertUserParams{
		FirebaseUid: "session-" + t.Name(), Email: "s@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"attempts", "cards", "skill_mastery", "lesson_progress", "daily_log"} {
		if _, err := pool.Exec(ctx, "DELETE FROM "+table+" WHERE user_id = $1", u.ID); err != nil {
			t.Fatal(err)
		}
	}
	c := fixtureCatalog(t)
	return NewService(pool, c), c, u.ID, pool
}

// answer responde el ítem que toca, bien o mal según `correct`.
func answer(t *testing.T, s *Service, c *content.Catalog, user pgtype.UUID, block Block, correct bool) AnswerResult {
	t.Helper()
	ctx := context.Background()
	next, err := s.Next(ctx, user, block)
	if err != nil {
		t.Fatal(err)
	}
	if next == nil {
		t.Fatalf("no hay ítem en el bloque %s", block)
	}
	it, _ := c.Item(next.Item.ID)
	var resp content.Response
	switch it.Type {
	case content.Cloze:
		text := it.Answers[0]
		if !correct {
			text = "zzz"
		}
		resp = content.Response{Text: text}
	case content.Choice, content.ExplainWhy:
		pick := it.Answer
		if !correct {
			pick = (it.Answer + 1) % len(it.Options)
		}
		resp = content.Response{Choice: &pick}
	}
	res, err := s.Answer(ctx, user, block, it.ID, resp, 4000, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Correct != correct {
		t.Fatalf("%s: correct = %v, want %v", it.ID, res.Correct, correct)
	}
	return res
}

func TestSessionFlow(t *testing.T) {
	s, c, user, pool := testService(t)
	ctx := context.Background()

	st, err := s.Today(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if st.Lesson == nil || st.Lesson.Skill != "tense.fixture_a" || st.Lesson.Status != "not_started" {
		t.Fatalf("la lección de hoy tendría que ser la del fixture: %+v", st.Lesson)
	}
	if st.Practice == nil || !st.Practice.Locked {
		t.Fatalf("la práctica arranca trabada hasta leer la lección: %+v", st.Practice)
	}
	if st.Review.Due != 0 || st.Jornal != 0 || st.ColadaTotal != 0 {
		t.Fatalf("un usuario nuevo arranca en cero: %+v", st)
	}

	// Con la práctica trabada no hay ítem.
	if next, err := s.Next(ctx, user, Practice); err != nil || next != nil {
		t.Fatalf("next = %+v, err = %v; want nil", next, err)
	}

	view, err := s.Lesson(ctx, user, "tense.fixture_a")
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != "in_progress" || len(view.Blocks) == 0 {
		t.Fatalf("la lección tendría que abrirse y quedar empezada: %+v", view.Status)
	}
	if _, err := s.Lesson(ctx, user, "no.existe"); !errors.Is(err, ErrNoLesson) {
		t.Fatalf("err = %v, want ErrNoLesson", err)
	}

	if _, err := s.CompleteLesson(ctx, user, "tense.fixture_a"); err != nil {
		t.Fatal(err)
	}
	st, err = s.Today(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if st.Practice.Locked || st.Practice.Total != 9 {
		t.Fatalf("leída la lección, la práctica se destraba: %+v", st.Practice)
	}

	// Responder un ítem que no toca se rechaza.
	if _, err := s.Answer(ctx, user, Practice, "a-01", content.Response{Text: "x"}, 0, false); !errors.Is(err, ErrUnexpectedItem) {
		t.Fatalf("err = %v, want ErrUnexpectedItem", err)
	}

	first := answer(t, s, c, user, Practice, true)
	if first.Colada < 1 || first.State.ColadaToday < 1 {
		t.Fatalf("una respuesta correcta suma colada: %+v", first)
	}
	if first.Next == nil {
		t.Fatal("tendría que quedar otro ítem de práctica")
	}
	if first.State.Practice.Done != 1 {
		t.Fatalf("done = %d, want 1", first.State.Practice.Done)
	}

	for i := 0; i < 3; i++ {
		answer(t, s, c, user, Practice, true)
	}

	mastery, err := store.New(pool).ListSkillMastery(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if len(mastery) != 1 || mastery[0].State != "calzada" || mastery[0].Source != "practice" {
		t.Fatalf("cuatro aciertos dejan la habilidad calzada: %+v", mastery)
	}

	st, err = s.Today(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if st.ColadaTotal < 4 || st.MinutesToday < 0 {
		t.Fatalf("el día tiene que acumular: %+v", st)
	}
	if st.Jornal != 0 {
		t.Fatalf("con 4 respuestas todavía no hay jornal (mínimo %d): %d", MinAnswersForStreak, st.Jornal)
	}
	answer(t, s, c, user, Practice, true)
	if st, err = s.Today(ctx, user); err != nil || st.Jornal != 1 {
		t.Fatalf("con %d respuestas arranca el jornal: %d (%v)", MinAnswersForStreak, st.Jornal, err)
	}
}

func TestReviewComesBeforePractice(t *testing.T) {
	s, c, user, pool := testService(t)
	ctx := context.Background()

	if _, err := s.CompleteLesson(ctx, user, "tense.fixture_a"); err != nil {
		t.Fatal(err)
	}
	res := answer(t, s, c, user, Practice, true)

	// Nada vencido todavía: la tarjeta recién creada queda para más adelante.
	if next, err := s.Next(ctx, user, Review); err != nil || next != nil {
		t.Fatalf("next = %+v, err = %v; want nil", next, err)
	}
	st, err := s.Today(ctx, user)
	if err != nil || st.Review.Due != 0 {
		t.Fatalf("due = %d, want 0 (%v)", st.Review.Due, err)
	}

	// Se adelanta el vencimiento: ese ítem tiene que aparecer en el repaso.
	if _, err := pool.Exec(ctx, "UPDATE cards SET due = now() - interval '1 day' WHERE user_id = $1", user); err != nil {
		t.Fatal(err)
	}
	next, err := s.Next(ctx, user, Review)
	if err != nil {
		t.Fatal(err)
	}
	if next == nil || next.Item.ID != res.ItemID || next.Block != Review {
		t.Fatalf("el repaso tiene que traer el ítem vencido: %+v", next)
	}
	if st, err = s.Today(ctx, user); err != nil || st.Review.Due != 1 {
		t.Fatalf("due = %d, want 1 (%v)", st.Review.Due, err)
	}
}

// El recorrido del óxido por la base: la pieza se calza, se cae y se recupera.
//
// Es el camino que estaba roto: como el promedio es pesado, la caída pasa
// siempre por suspendida, así que mirando sólo el estado anterior la pieza
// terminaba en plano y no se oxidaba nunca. Ahora la memoria vive en
// skill_mastery.was_calzada.
func TestPracticeRustsAPieceThatWasCalzada(t *testing.T) {
	s, c, user, pool := testService(t)
	ctx := context.Background()

	mastery := func() store.SkillMastery {
		t.Helper()
		rows, err := store.New(pool).ListSkillMastery(ctx, user)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 {
			t.Fatalf("esperaba una sola habilidad con dominio: %+v", rows)
		}
		return rows[0]
	}
	// La práctica no repite ítems dentro del mismo día. Correr los intentos hacia
	// atrás es la única forma de seguir practicando el mismo tema mañana.
	nextDay := func() {
		t.Helper()
		if _, err := pool.Exec(ctx,
			"UPDATE attempts SET created_at = created_at - interval '1 day' WHERE user_id = $1", user,
		); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := s.CompleteLesson(ctx, user, "tense.fixture_a"); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 4; i++ {
		answer(t, s, c, user, Practice, true)
	}
	if m := mastery(); m.State != string(placement.Calzada) || !m.WasCalzada {
		t.Fatalf("cuatro aciertos calzan la pieza y dejan la marca: %+v", m)
	}

	// Se afloja: todavía no es óxido.
	answer(t, s, c, user, Practice, false)
	if m := mastery(); m.State != string(placement.Suspendida) {
		t.Fatalf("el primer error afloja la pieza: %+v", m)
	}
	// Y se cae: acá es donde antes quedaba suspendida y después plano.
	answer(t, s, c, user, Practice, false)
	m := mastery()
	if m.State != string(placement.Oxidada) {
		t.Fatalf("la pieza que estuvo calzada y se cae chorrea óxido: %+v", m)
	}
	if m.Mastery >= 0.7 {
		t.Fatalf("el óxido empieza abajo de 0.7 (PLAN.md §8): %+v", m)
	}
	if !m.WasCalzada {
		t.Fatalf("la memoria de haber calzado no se borra: %+v", m)
	}

	// Oxidada manda en el mapa: el tema vuelve a ser el que toca estudiar.
	st, err := s.Today(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if st.Lesson == nil || st.Lesson.Skill != "tense.fixture_a" {
		t.Fatalf("la pieza oxidada tiene que seguir siendo la lección de hoy: %+v", st.Lesson)
	}

	// Al día siguiente se recupera: acertando vuelve a calzar y el óxido se va.
	nextDay()
	for i := 0; i < 6 && mastery().State != string(placement.Calzada); i++ {
		answer(t, s, c, user, Practice, true)
	}
	if m := mastery(); m.State != string(placement.Calzada) || !m.WasCalzada {
		t.Fatalf("acertando de nuevo la pieza vuelve a calzar y se limpia: %+v", m)
	}
}
