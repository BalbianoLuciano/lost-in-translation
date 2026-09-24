package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/auth"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/db"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/store"
)

// El usuario de este archivo. Los paquetes de test corren en paralelo contra la
// misma base, así que cada uno toca sólo lo suyo.
const (
	cuentaUID   = "uid-cuenta"
	cuentaEmail = "cuenta@example.com"
	cuentaToken = "Bearer dev:" + cuentaUID + ":" + cuentaEmail
)

// tablasConDatosPersonales es la lista contra la que se prueba el borrado: toda
// tabla que referencia users(id). Si mañana aparece una nueva y no entra acá, el
// test no la cubre, así que agregarla es parte de agregar la tabla.
var tablasConDatosPersonales = []string{
	"placement_runs",
	"attempts",
	"cards",
	"skill_mastery",
	"lesson_progress",
	"daily_log",
	"usage_daily",
}

func abrirBase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL no definida")
	}
	pool, err := db.Open(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), "DELETE FROM users WHERE firebase_uid = $1", cuentaUID); err != nil {
		t.Fatal(err)
	}
	return pool
}

// routerReal arma el router contra Postgres de verdad: lo que se prueba acá es
// el cascade de las migraciones, y eso no lo puede probar un fake.
func routerReal(pool *pgxpool.Pool) http.Handler {
	q := store.New(pool)
	return NewRouter(Deps{
		Users:       q,
		Account:     q,
		Placement:   fakePlacement{},
		Catalog:     testCatalog(),
		DB:          pool,
		Verifier:    auth.DevVerifier{},
		CORSOrigins: []string{"http://localhost:5173"},
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

// sembrar deja al usuario con una fila en cada tabla que guarda algo suyo.
func sembrar(t *testing.T, pool *pgxpool.Pool) store.User {
	t.Helper()
	ctx := context.Background()
	q := store.New(pool)

	u, err := q.UpsertUser(ctx, store.UpsertUserParams{
		FirebaseUid: cuentaUID, Email: cuentaEmail, DisplayName: "Cuenta de prueba",
	})
	if err != nil {
		t.Fatal(err)
	}

	run, err := q.CreatePlacementRun(ctx, store.CreatePlacementRunParams{
		UserID: u.ID, Part: "tenses", ContentVersion: "test",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Uno del diagnóstico y uno oral: el oral guarda la transcripción de lo que
	// la persona dijo, que es el dato más sensible de toda la base.
	if _, err := q.InsertAttempt(ctx, store.InsertAttemptParams{
		UserID: u.ID, ItemID: "a-01", SkillID: "tense.fixture_a", Context: "placement",
		PlacementRunID: run.ID, Response: []byte(`{"text":"went"}`), Correct: true,
		LatencyMs: pgtype.Int4{Int32: 1200, Valid: true},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.InsertAttempt(ctx, store.InsertAttemptParams{
		UserID: u.ID, ItemID: "d-01", SkillID: "tense.fixture_a", Context: "speaking",
		Response: []byte(`{"transcript":"yesterday I deployed the fix"}`), Correct: true,
	}); err != nil {
		t.Fatal(err)
	}

	if err := q.UpsertCard(ctx, store.UpsertCardParams{
		UserID: u.ID, ItemID: "a-01",
		Due:        pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
		Stability:  2.5,
		Difficulty: 5,
		Reps:       1,
		State:      1,
		LastReview: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}); err != nil {
		t.Fatal(err)
	}
	if err := q.UpsertSkillMastery(ctx, store.UpsertSkillMasteryParams{
		UserID: u.ID, SkillID: "tense.fixture_a", Mastery: 0.9, State: "calzada", Source: "practice",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.CompleteLesson(ctx, store.CompleteLessonParams{UserID: u.ID, SkillID: "tense.fixture_a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.AddToDailyLog(ctx, store.AddToDailyLogParams{UserID: u.ID, Colada: 3, Seconds: 90}); err != nil {
		t.Fatal(err)
	}
	if err := q.AddUsage(ctx, store.AddUsageParams{UserID: u.ID, Kind: "ask"}); err != nil {
		t.Fatal(err)
	}
	if err := q.AddUsage(ctx, store.AddUsageParams{UserID: u.ID, Kind: "speaking"}); err != nil {
		t.Fatal(err)
	}
	return u
}

// El helper `do` de router_test.go entra siempre como lucho; acá hace falta el
// usuario de este archivo.
func newAuthedRequest(t *testing.T, method, path string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", cuentaToken)
	return req
}

func serve(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func contarPorUsuario(t *testing.T, pool *pgxpool.Pool, tabla string, userID pgtype.UUID) int {
	t.Helper()
	var n int
	// La tabla sale de una lista fija de este archivo, no de un parámetro.
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM "+tabla+" WHERE user_id = $1", userID).Scan(&n); err != nil {
		t.Fatalf("contar %s: %v", tabla, err)
	}
	return n
}

// TestBorrarCuentaNoDejaNada es el test que sostiene el derecho a borrar: siembra
// una fila en cada tabla, borra la cuenta por HTTP y cuenta tabla por tabla.
func TestBorrarCuentaNoDejaNada(t *testing.T) {
	pool := abrirBase(t)
	u := sembrar(t, pool)
	h := routerReal(pool)

	// Antes: todas las tablas tienen algo. Si no, el test de después no prueba nada.
	for _, tabla := range tablasConDatosPersonales {
		if n := contarPorUsuario(t, pool, tabla, u.ID); n == 0 {
			t.Fatalf("la siembra dejó %s vacía: el borrado no probaría nada", tabla)
		}
	}

	req := newAuthedRequest(t, http.MethodDelete, "/v1/me")
	rec := serve(h, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204: %s", rec.Code, rec.Body)
	}

	for _, tabla := range tablasConDatosPersonales {
		t.Run(tabla, func(t *testing.T) {
			if n := contarPorUsuario(t, pool, tabla, u.ID); n != 0 {
				t.Fatalf("quedaron %d filas en %s", n, tabla)
			}
		})
	}

	var usuarios int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM users WHERE id = $1", u.ID).Scan(&usuarios); err != nil {
		t.Fatal(err)
	}
	if usuarios != 0 {
		t.Fatal("el usuario sigue existiendo")
	}
}

// Borrar dos veces no falla: la segunda no tiene nada que hacer y lo dice igual.
func TestBorrarDosVecesEsIdempotente(t *testing.T) {
	pool := abrirBase(t)
	sembrar(t, pool)
	h := routerReal(pool)

	for i := range 2 {
		rec := serve(h, newAuthedRequest(t, http.MethodDelete, "/v1/me"))
		if rec.Code != http.StatusNoContent {
			t.Fatalf("borrado %d: status = %d, want 204: %s", i+1, rec.Code, rec.Body)
		}
	}
}

// La caché del profesor no tiene user_id: es global y por hash del prompt, así
// que el cascade no la toca. Está decidido a propósito y escrito en la página de
// privacidad; este test es el que avisa si alguna vez deja de ser cierto.
func TestLaCacheDeIANoSeBorraConLaCuenta(t *testing.T) {
	pool := abrirBase(t)
	ctx := context.Background()
	q := store.New(pool)
	const hash = "hash-de-prueba-cuenta"

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM ai_answers WHERE prompt_hash = $1", hash)
	})
	if _, err := pool.Exec(ctx, "DELETE FROM ai_answers WHERE prompt_hash = $1", hash); err != nil {
		t.Fatal(err)
	}
	if err := q.SaveAnswer(ctx, store.SaveAnswerParams{
		PromptHash: hash, Question: "why is it 'went'?", Context: "", Answer: "porque es irregular", Model: "test",
	}); err != nil {
		t.Fatal(err)
	}

	sembrar(t, pool)
	if rec := serve(routerReal(pool), newAuthedRequest(t, http.MethodDelete, "/v1/me")); rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}

	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM ai_answers WHERE prompt_hash = $1", hash).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("filas en ai_answers = %d, want 1: la caché es global y no se borra con la cuenta", n)
	}
}

func TestExportarTraeTodo(t *testing.T) {
	pool := abrirBase(t)
	u := sembrar(t, pool)
	h := routerReal(pool)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", u.ID)
	})

	rec := serve(h, newAuthedRequest(t, http.MethodGet, "/v1/me/export"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(got, "attachment;") {
		t.Fatalf("Content-Disposition = %q, want un attachment", got)
	}

	var out accountExport
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("la exportación no es JSON válido: %v", err)
	}

	if out.Profile.Email != cuentaEmail || out.Profile.ID != u.ID.String() {
		t.Fatalf("perfil inesperado: %+v", out.Profile)
	}
	if out.ExportedAt == "" || out.Note == "" {
		t.Fatal("la exportación tiene que decir cuándo se hizo y qué es")
	}

	// Cada sección del JSON tiene que traer lo que se sembró: una exportación a
	// la que le falta una tabla es peor que no tenerla.
	secciones := []struct {
		nombre string
		n      int
	}{
		{"placementRuns", len(out.PlacementRuns)},
		{"attempts", len(out.Attempts)},
		{"cards", len(out.Cards)},
		{"skillMastery", len(out.SkillMastery)},
		{"lessons", len(out.Lessons)},
		{"dailyLog", len(out.DailyLog)},
		{"dailyUsage", len(out.Usage)},
	}
	for _, s := range secciones {
		t.Run(s.nombre, func(t *testing.T) {
			if s.n == 0 {
				t.Fatalf("la sección %s vino vacía", s.nombre)
			}
		})
	}

	// La transcripción de lo que se dijo en voz alta también se la lleva la persona.
	var oral bool
	for _, a := range out.Attempts {
		if a.Context == "speaking" && string(a.Response) != "" {
			oral = true
		}
	}
	if !oral {
		t.Fatal("falta el intento oral con su transcripción")
	}
}

func TestExportarSinCuentaDa404(t *testing.T) {
	pool := abrirBase(t)
	rec := serve(routerReal(pool), newAuthedRequest(t, http.MethodGet, "/v1/me/export"))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", rec.Code, rec.Body)
	}
}

func TestCicloDeVidaPideAuth(t *testing.T) {
	h := newTestRouter(newFakeStore(), nil)
	tests := []struct{ method, path string }{
		{http.MethodDelete, "/v1/me"},
		{http.MethodGet, "/v1/me/export"},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			if rec := do(t, h, tt.method, tt.path, "", false); rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", rec.Code)
			}
		})
	}
}
