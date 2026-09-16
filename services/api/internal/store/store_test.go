package store_test

import (
	"context"
	"os"
	"testing"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/db"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/store"
)

// Test de integración contra Postgres real. Corre sólo con TEST_DATABASE_URL
// (en CI la levanta un service container; en local, docker compose).
func TestUsersAgainstPostgres(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL no definida")
	}
	ctx := context.Background()

	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	// Sólo lo propio: los paquetes de test corren en paralelo contra la misma base.
	if _, err := pool.Exec(ctx, "DELETE FROM users WHERE firebase_uid = 'uid-1'"); err != nil {
		t.Fatal(err)
	}

	q := store.New(pool)

	u, err := q.UpsertUser(ctx, store.UpsertUserParams{FirebaseUid: "uid-1", Email: "a@example.com", DisplayName: "A"})
	if err != nil {
		t.Fatal(err)
	}
	if u.Theme != "system" || !u.ID.Valid {
		t.Fatalf("defaults inesperados: %+v", u)
	}

	again, err := q.UpsertUser(ctx, store.UpsertUserParams{FirebaseUid: "uid-1", Email: "b@example.com", DisplayName: "B"})
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != u.ID || again.Email != "b@example.com" {
		t.Fatalf("el upsert debía actualizar el mismo usuario: %+v", again)
	}

	themed, err := q.UpdateUserTheme(ctx, store.UpdateUserThemeParams{FirebaseUid: "uid-1", Theme: "dark"})
	if err != nil {
		t.Fatal(err)
	}
	if themed.Theme != "dark" {
		t.Fatalf("theme = %q, want dark", themed.Theme)
	}

	if _, err := q.UpdateUserTheme(ctx, store.UpdateUserThemeParams{FirebaseUid: "uid-1", Theme: "sepia"}); err == nil {
		t.Fatal("el CHECK de la tabla debía rechazar un tema inválido")
	}
}
