package placement

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/content"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/db"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/store"
)

func testService(t *testing.T) (*Service, pgtype.UUID, *pgxpool.Pool) {
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
		FirebaseUid: "placement-" + t.Name(), Email: "p@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"placement_runs", "skill_mastery", "cards"} {
		if _, err := pool.Exec(ctx, "DELETE FROM "+table+" WHERE user_id = $1", u.ID); err != nil {
			t.Fatal(err)
		}
	}
	c, _ := fixture(t)
	return NewService(pool, c), u.ID, pool
}

func catalogOf(t *testing.T) *content.Catalog {
	t.Helper()
	c, _ := fixture(t)
	return c
}

// choiceFor arma la respuesta correcta (o una equivocada) leyendo el ítem: el
// compilador mezcla las opciones, así que el índice no se puede fijar a mano.
func choiceFor(t *testing.T, c *content.Catalog, id string, correct bool) content.Response {
	t.Helper()
	it, ok := c.Item(id)
	if !ok {
		t.Fatalf("no existe el ítem %s", id)
	}
	pick := it.Answer
	if !correct {
		pick = (it.Answer + 1) % len(it.Options)
	}
	return content.Response{Choice: &pick}
}

func TestServiceFullPart(t *testing.T) {
	s, user, pool := testService(t)
	ctx := context.Background()

	st, err := s.Start(ctx, user, "tenses")
	if err != nil {
		t.Fatal(err)
	}
	if st.Next == nil || st.Next.ID != "a-01" {
		t.Fatalf("primer ítem = %+v", st.Next)
	}

	// Retomar devuelve la misma corrida.
	again, err := s.Start(ctx, user, "tenses")
	if err != nil || again.RunID != st.RunID {
		t.Fatalf("Start no retomó la corrida abierta: %v %s != %s", err, again.RunID, st.RunID)
	}

	var runID pgtype.UUID
	if err := runID.Scan(st.RunID); err != nil {
		t.Fatal(err)
	}

	// Responder un ítem que no toca se rechaza.
	if _, err := s.Answer(ctx, user, runID, "b-01", content.Response{Text: "has broken"}, 0, false); !errors.Is(err, ErrUnexpectedItem) {
		t.Fatalf("err = %v, want ErrUnexpectedItem", err)
	}

	steps := []struct {
		item      string
		resp      content.Response
		ok        bool
		consulted bool
	}{
		{item: "a-01", resp: content.Response{Text: "I've finished"}}, // incluye el sujeto: mal
		{item: "a-02", resp: choiceFor(t, catalogOf(t), "a-02", true), ok: true},
		{item: "a-03", resp: content.Response{TokenIndex: ptr(1), Text: "has"}, ok: true},
		{item: "b-01", resp: content.Response{Text: "'s broken"}, ok: true},
		{item: "b-02", resp: choiceFor(t, catalogOf(t), "b-02", true), ok: true},
	}
	var last AnswerResult
	for _, step := range steps {
		last, err = s.Answer(ctx, user, runID, step.item, step.resp, 1200, step.consulted)
		if err != nil {
			t.Fatalf("%s: %v", step.item, err)
		}
		if last.Correct != step.ok {
			t.Fatalf("%s: correct = %v, want %v", step.item, last.Correct, step.ok)
		}
		if last.Rule == "" || last.ExplainEs.Why == "" {
			t.Fatalf("%s: falta la regla o la explicación", step.item)
		}
	}

	if last.State.Status != "done" || last.State.Next != nil || len(last.State.Summary) != 2 {
		t.Fatalf("la parte debía terminar con resumen: %+v", last.State)
	}

	if _, err := s.Answer(ctx, user, runID, "b-03", choiceFor(t, catalogOf(t), "b-03", true), 0, false); !errors.Is(err, ErrRunDone) {
		t.Fatalf("err = %v, want ErrRunDone", err)
	}

	mastery, err := store.New(pool).ListSkillMastery(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]string{}
	for _, m := range mastery {
		states[m.SkillID] = m.State
	}
	if states["tense.fixture_a"] != "suspendida" || states["tense.fixture_b"] != "calzada" {
		t.Fatalf("dominio guardado inesperado: %v", states)
	}

	// El diagnóstico no crea tarjetas de repaso: si estos ítems volvieran en el
	// repaso diario, la próxima medición sería de memoria y no de nivel.
	var cards int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM cards WHERE user_id = $1", user).Scan(&cards); err != nil {
		t.Fatal(err)
	}
	if cards != 0 {
		t.Fatalf("cards = %d, want 0", cards)
	}

	if len(last.State.Missed) != 1 {
		t.Fatalf("missed = %d, want 1 (a-01 se respondió mal)", len(last.State.Missed))
	}
	miss := last.State.Missed[0]
	if miss.ItemID != "a-01" || miss.Expected == "" || miss.Rule == "" || miss.ExplainEs.Why == "" {
		t.Fatalf("el ítem errado tiene que traer respuesta, regla y explicación: %+v", miss)
	}

	views, err := s.Overview(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if views[0].Status != "done" || views[0].Summary == nil || len(views[0].Missed) != 1 {
		t.Fatalf("overview inesperado: %+v", views[0])
	}

	// Una parte nueva arranca de cero aunque la anterior terminó.
	fresh, err := s.Start(ctx, user, "tenses")
	if err != nil || fresh.RunID == st.RunID || fresh.Next.ID != "a-01" {
		t.Fatalf("una corrida nueva debía empezar de cero: %v %+v", err, fresh)
	}
}

func TestServiceUnknownPartAndRun(t *testing.T) {
	s, user, _ := testService(t)
	ctx := context.Background()
	if _, err := s.Start(ctx, user, "nope"); !errors.Is(err, ErrPartNotFound) {
		t.Fatalf("err = %v, want ErrPartNotFound", err)
	}
	var missing pgtype.UUID
	_ = missing.Scan("00000000-0000-0000-0000-000000000000")
	if _, err := s.Get(ctx, user, missing); !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("err = %v, want ErrRunNotFound", err)
	}
}

func ptr(i int) *int { return &i }

// Un acierto consultado se guarda como tal y no alcanza para saltear el tercer ítem.
func TestConsultedIsStored(t *testing.T) {
	s, user, pool := testService(t)
	ctx := context.Background()

	st, err := s.Start(ctx, user, "tenses")
	if err != nil {
		t.Fatal(err)
	}
	var runID pgtype.UUID
	if err := runID.Scan(st.RunID); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Answer(ctx, user, runID, "a-01", content.Response{Text: "have finished"}, 0, false); err != nil {
		t.Fatal(err)
	}
	res, err := s.Answer(ctx, user, runID, "a-02", choiceFor(t, catalogOf(t), "a-02", true), 0, true)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Correct {
		t.Fatal("la respuesta era correcta")
	}
	if res.State.Next == nil || res.State.Next.ID != "a-03" {
		t.Fatalf("después de un acierto consultado tiene que seguir a-03: %+v", res.State.Next)
	}

	var consulted bool
	if err := pool.QueryRow(ctx,
		"SELECT consulted FROM attempts WHERE user_id = $1 AND item_id = 'a-02'", user).Scan(&consulted); err != nil {
		t.Fatal(err)
	}
	if !consulted {
		t.Fatal("el intento tenía que quedar marcado como consultado")
	}
}
