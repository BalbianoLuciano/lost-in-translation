package achievement

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/content"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/db"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/placement"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/store"
)

// List devuelve siempre las 41, con y sin progreso: lo que falta es parte de la
// pantalla.
func TestServiceList(t *testing.T) {
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
	q := store.New(pool)
	u, err := q.UpsertUser(ctx, store.UpsertUserParams{
		FirebaseUid: "achievement-" + t.Name(), Email: "a@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "DELETE FROM achievements WHERE user_id = $1", u.ID); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile("../content/bundle.json")
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := content.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	s := NewService(pool, catalog)

	list, err := s.List(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 41 {
		t.Fatalf("len = %d, want 41", len(list))
	}
	for _, v := range list {
		if v.Earned || v.EarnedAt != nil {
			t.Fatalf("sin progreso no hay nada ganado: %+v", v)
		}
	}

	// Se calza la única pieza de la torre: tiene que salir la pieza y la obra.
	torre := "reported_speech.basic"
	earned := Earned(s.catalog, map[string]placement.State{torre: placement.Calzada})
	if err := Grant(ctx, q, u.ID, s.catalog, map[string]placement.State{torre: placement.Calzada}); err != nil {
		t.Fatal(err)
	}
	if len(earned) != 2 {
		t.Fatalf("earned = %v, want la pieza y la obra 5", earned)
	}

	list, err = s.List(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	got := map[Code]bool{}
	for _, v := range list {
		if v.Earned {
			if v.EarnedAt == nil {
				t.Fatalf("una distinción ganada tiene fecha: %+v", v)
			}
			got[v.Code] = true
		}
	}
	if len(got) != 2 || !got[PiezaCode(torre)] || !got[ObraCode(5)] {
		t.Fatalf("ganadas = %v, want la pieza de la torre y la obra 5", got)
	}
}

// Volver a guardar lo mismo no cambia nada: la fecha es la de la primera vez.
func TestGrantEsIdempotente(t *testing.T) {
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
	q := store.New(pool)
	u, err := q.UpsertUser(ctx, store.UpsertUserParams{
		FirebaseUid: "achievement-" + t.Name(), Email: "a@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "DELETE FROM achievements WHERE user_id = $1", u.ID); err != nil {
		t.Fatal(err)
	}

	cat := testCatalog(t)
	calzada := map[string]placement.State{"b.uno": placement.Calzada}
	if err := Grant(ctx, q, u.ID, cat, calzada); err != nil {
		t.Fatal(err)
	}
	antes := earnedAt(t, q, u.ID)

	for i := 0; i < 3; i++ {
		if err := Grant(ctx, q, u.ID, cat, calzada); err != nil {
			t.Fatal(err)
		}
	}
	después := earnedAt(t, q, u.ID)
	if len(después) != len(antes) {
		t.Fatalf("evaluar de nuevo no puede agregar filas: %v vs %v", después, antes)
	}
	for code, when := range antes {
		if !después[code].Equal(when) {
			t.Fatalf("%s cambió de fecha: %v → %v", code, when, después[code])
		}
	}

	// Sin nada ganado no hay consulta ni fila nueva.
	if err := Grant(ctx, q, u.ID, cat, map[string]placement.State{"b.uno": "oxidada"}); err != nil {
		t.Fatal(err)
	}
	if len(earnedAt(t, q, u.ID)) != len(antes) {
		t.Fatal("oxidarse no puede borrar ni agregar nada")
	}
}

func earnedAt(t *testing.T, q *store.Queries, user pgtype.UUID) map[string]time.Time {
	t.Helper()
	rows, err := q.ListAchievements(context.Background(), user)
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string]time.Time, len(rows))
	for _, r := range rows {
		out[r.Code] = r.EarnedAt.Time
	}
	return out
}
