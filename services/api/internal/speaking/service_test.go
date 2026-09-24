package speaking

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/budget"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/content"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/db"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/store"
)

// fakeSTT devuelve lo que se le diga, sin llamar a nadie.
type fakeSTT struct {
	text  string
	err   error
	calls int
	got   string
}

func (f *fakeSTT) Transcribe(_ context.Context, audio io.Reader, filename string) (string, error) {
	f.calls++
	b, _ := io.ReadAll(audio)
	f.got = string(b)
	if f.err != nil {
		return "", f.err
	}
	return f.text, nil
}

// testService arma el servicio contra la base de pruebas. Los topes son los de
// producción salvo que el test pida otros.
func testService(t *testing.T, stt *fakeSTT, limits ...budget.Limits) (*Service, pgtype.UUID, *pgxpool.Pool) {
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
		FirebaseUid: "speaking-" + t.Name(), Email: "sp@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"attempts", "skill_mastery", "daily_log", "cards", "usage_daily"} {
		if _, err := pool.Exec(ctx, "DELETE FROM "+table+" WHERE user_id = $1", u.ID); err != nil {
			t.Fatal(err)
		}
	}

	// Catálogo real: los drills de pronombres son el corazón de esta parte.
	data, err := os.ReadFile("../content/bundle.json")
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := content.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	lim := budget.Limits{PerUser: 40}
	if len(limits) > 0 {
		lim = limits[0]
	}
	bud := budget.New(pool, map[budget.Kind]budget.Limits{budget.Speaking: lim})
	if stt == nil {
		return NewService(pool, catalog, nil, bud), u.ID, pool
	}
	return NewService(pool, catalog, stt, bud), u.ID, pool
}

func TestWithoutProviderSpeakingIsOff(t *testing.T) {
	s, user, _ := testService(t, nil)
	if s.Configured() {
		t.Fatal("sin transcriptor no tiene que estar configurado")
	}
	_, err := s.Answer(context.Background(), user, "drill-sofia-standup", strings.NewReader("x"), "a.webm", 10)
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("err = %v, want ErrNotConfigured", err)
	}
}

func TestNextGivesADrillAndDoesNotRepeatItToday(t *testing.T) {
	stt := &fakeSTT{text: "She found the bug yesterday and she is testing the fix today."}
	s, user, _ := testService(t, stt)
	ctx := context.Background()

	first, err := s.Next(ctx, user)
	if err != nil || first == nil {
		t.Fatalf("tendría que haber un drill: %v", err)
	}

	res, err := s.Answer(ctx, user, first.ID, strings.NewReader("audio"), "a.webm", 20)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Correct || res.Colada == 0 {
		t.Fatalf("la respuesta era correcta: %+v", res.Analysis)
	}
	if stt.calls != 1 || stt.got != "audio" {
		t.Fatalf("el audio tiene que llegar al transcriptor: %d %q", stt.calls, stt.got)
	}
	if res.Next != nil && res.Next.ID == first.ID {
		t.Fatal("el mismo drill no se repite en el día")
	}
}

func TestWrongPronounIsCaught(t *testing.T) {
	// Habla de Sofía pero dice "he": es el error que se está midiendo.
	stt := &fakeSTT{text: "He found the bug yesterday and he is testing the fix today."}
	s, user, pool := testService(t, stt)
	ctx := context.Background()

	res, err := s.Answer(ctx, user, "drill-sofia-standup", strings.NewReader("audio"), "a.webm", 20)
	if err != nil {
		t.Fatal(err)
	}
	if res.Correct || len(res.Wrong) != 1 || res.Wrong[0] != "he" {
		t.Fatalf("tenía que marcar el he: %+v", res.Analysis)
	}
	if res.Colada != 0 {
		t.Fatalf("un error no suma colada: %d", res.Colada)
	}

	var stored string
	if err := pool.QueryRow(ctx,
		"SELECT response->>'transcript' FROM attempts WHERE user_id = $1 AND context = 'speaking'", user).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stored, "He found") {
		t.Fatalf("la transcripción tiene que quedar guardada: %q", stored)
	}
}

func TestUnknownDrill(t *testing.T) {
	s, user, _ := testService(t, &fakeSTT{})
	_, err := s.Answer(context.Background(), user, "no-existe", strings.NewReader("x"), "a.webm", 5)
	if !errors.Is(err, ErrNoDrill) {
		t.Fatalf("err = %v, want ErrNoDrill", err)
	}
}

func TestTranscriptionErrorBubblesUp(t *testing.T) {
	s, user, _ := testService(t, &fakeSTT{err: errors.New("429 del proveedor")})
	_, err := s.Answer(context.Background(), user, "drill-sofia-standup", strings.NewReader("x"), "a.webm", 5)
	if err == nil {
		t.Fatal("el error del proveedor tiene que llegar al llamador")
	}
}

// Transcribir es lo más caro que hace la app. Cuando se llega al tope, el audio
// ni se manda: lo que no se manda, no se paga.
func TestElTopeDiarioFrenaAntesDeTranscribir(t *testing.T) {
	stt := &fakeSTT{text: "He finished the migration."}
	s, user, _ := testService(t, stt, budget.Limits{PerUser: 1})
	ctx := context.Background()

	drill, err := s.Next(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if drill == nil {
		t.Skip("el catálogo no tiene drills")
	}

	if _, err := s.Answer(ctx, user, drill.ID, strings.NewReader("audio"), "a.webm", 20); err != nil {
		t.Fatalf("el primero entra: %v", err)
	}
	if stt.calls != 1 {
		t.Fatalf("llamadas al proveedor = %d, want 1", stt.calls)
	}

	_, err = s.Answer(ctx, user, drill.ID, strings.NewReader("audio"), "a.webm", 20)
	if !errors.Is(err, budget.ErrUserLimit) {
		t.Fatalf("err = %v, want ErrUserLimit", err)
	}
	if stt.calls != 1 {
		t.Fatalf("el segundo audio no tenía que salir: llamadas = %d", stt.calls)
	}
}
