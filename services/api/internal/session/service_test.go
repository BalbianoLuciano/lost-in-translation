package session

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/achievement"
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
	for _, table := range []string{"attempts", "cards", "skill_mastery", "lesson_progress", "daily_log", "achievements"} {
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

func listAchievements(t *testing.T, pool *pgxpool.Pool, user pgtype.UUID) map[string]time.Time {
	t.Helper()
	rows, err := store.New(pool).ListAchievements(context.Background(), user)
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string]time.Time, len(rows))
	for _, r := range rows {
		out[r.Code] = r.EarnedAt.Time
	}
	return out
}

// La otra mitad de la decisión central del §3 del SDD: achievement.Earned deja
// de devolver la pieza cuando se oxida, y la distinción tiene que quedar igual.
// Lo que la sostiene es el ON CONFLICT DO NOTHING, y eso sólo se puede probar
// contra la base.
func TestElOxidoNoQuitaLaDistincion(t *testing.T) {
	s, c, user, pool := testService(t)
	ctx := context.Background()

	if _, err := s.CompleteLesson(ctx, user, "tense.fixture_a"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < MinAttempts; i++ {
		answer(t, s, c, user, Practice, true)
	}

	ganadas := listAchievements(t, pool, user)
	earnedAt, ok := ganadas["pieza:tense.fixture_a"]
	if !ok {
		t.Fatalf("calzar el tema tiene que dar su distinción: %v", ganadas)
	}
	// La obra necesita todas sus piezas, y la otra del fixture sigue en plano.
	if _, ok := ganadas["obra:1"]; ok {
		t.Fatalf("la obra no se gana con una sola pieza: %v", ganadas)
	}

	// Cinco errores seguidos tiran el dominio abajo y la pieza se cae.
	//
	// Cae a `plano`, no a `oxidada`: RecomputeMastery sólo oxida cuando el paso
	// anterior era `calzada`, y con el promedio pesado la caída pasa siempre por
	// `suspendida` antes de cruzar el 0.5. Para la distinción da igual —lo que
	// importa es que dejó de estar calzada— pero está anotado en el reporte.
	for i := 0; i < 5; i++ {
		answer(t, s, c, user, Practice, false)
	}
	mastery, err := store.New(pool).ListSkillMastery(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if len(mastery) != 1 || mastery[0].State == "calzada" {
		t.Fatalf("el tema tendría que haberse caído: %+v", mastery)
	}
	if got := achievement.Earned(achievement.NewCatalog(c), map[string]placement.State{
		"tense.fixture_a": placement.State(mastery[0].State),
	}); len(got) != 0 {
		t.Fatalf("hoy ese tema ya no está calzado, Earned no tiene que devolverlo: %v", got)
	}

	después := listAchievements(t, pool, user)
	if got, ok := después["pieza:tense.fixture_a"]; !ok || !got.Equal(earnedAt) {
		t.Fatalf("la distinción tiene que seguir, con su fecha original: %v (era %v)", después, earnedAt)
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
