package tutor

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/budget"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/content"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/db"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/store"
)

// fakeLLM cuenta las llamadas y guarda el último prompt, para ver qué contexto
// se le manda al modelo.
type fakeLLM struct {
	calls  int
	last   string
	answer string
	err    error
}

func (f *fakeLLM) Complete(_ context.Context, _, user string) (string, error) {
	f.calls++
	f.last = user
	if f.err != nil {
		return "", f.err
	}
	if f.answer == "" {
		return "Va have finished.", nil
	}
	return f.answer, nil
}

func (f *fakeLLM) Model() string { return "fake-model" }

// testTutor arma un profesor contra la base de pruebas. Los topes son los de
// producción salvo que el test pida otros.
func testTutor(t *testing.T, llm *fakeLLM, limits ...budget.Limits) (*Tutor, pgtype.UUID) {
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
		FirebaseUid: "tutor-" + t.Name(), Email: "t@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "DELETE FROM usage_daily WHERE user_id = $1", u.ID); err != nil {
		t.Fatal(err)
	}
	// La caché es global y sobrevive entre corridas: sin esto, el segundo `go
	// test` mediría la caché vieja en vez del camino nuevo.
	if _, err := pool.Exec(ctx, "DELETE FROM ai_answers"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile("../content/testdata/bundle.json")
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := content.Parse(data)
	if err != nil {
		t.Fatal(err)
	}

	lim := budget.Limits{PerUser: 60}
	if len(limits) > 0 {
		lim = limits[0]
	}
	bud := budget.New(pool, map[budget.Kind]budget.Limits{budget.Ask: lim})

	var client interface {
		Complete(context.Context, string, string) (string, error)
		Model() string
	} = llm
	if llm == nil {
		client = nil
	}
	if client == nil {
		return New(pool, catalog, nil, bud), u.ID
	}
	return New(pool, catalog, llm, bud), u.ID
}

func TestAskWithoutProviderIsOff(t *testing.T) {
	tu, user := testTutor(t, nil)
	if tu.Configured() {
		t.Fatal("sin cliente no tiene que estar configurado")
	}
	if _, err := tu.Ask(context.Background(), user, "¿por qué have?", ""); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("err = %v, want ErrNotConfigured", err)
	}
}

func TestAskValidatesTheQuestion(t *testing.T) {
	tu, user := testTutor(t, &fakeLLM{})
	ctx := context.Background()
	if _, err := tu.Ask(ctx, user, "   ", ""); !errors.Is(err, ErrEmpty) {
		t.Fatalf("err = %v, want ErrEmpty", err)
	}
	if _, err := tu.Ask(ctx, user, strings.Repeat("a", MaxQuestion+1), ""); !errors.Is(err, ErrTooLong) {
		t.Fatalf("err = %v, want ErrTooLong", err)
	}
}

func TestAskCachesTheSameQuestion(t *testing.T) {
	llm := &fakeLLM{answer: "Porque el resultado importa ahora."}
	tu, user := testTutor(t, llm)
	ctx := context.Background()
	question := "¿por qué va have finished y no finished? " + t.Name()

	first, err := tu.Ask(ctx, user, question, "")
	if err != nil {
		t.Fatal(err)
	}
	if first.Cached || first.Answer != llm.answer {
		t.Fatalf("primera respuesta inesperada: %+v", first)
	}

	second, err := tu.Ask(ctx, user, question, "")
	if err != nil {
		t.Fatal(err)
	}
	if !second.Cached || second.Answer != llm.answer {
		t.Fatalf("la segunda tenía que salir de la caché: %+v", second)
	}
	if llm.calls != 1 {
		t.Fatalf("el modelo se llamó %d veces, want 1", llm.calls)
	}
	// La caché tampoco gasta pregunta del día: el contador queda donde estaba.
	if second.Left != first.Left {
		t.Fatalf("left pasó de %d a %d: la caché no tiene que consumir", first.Left, second.Left)
	}
}

func TestAskRespectsTheDailyLimit(t *testing.T) {
	llm := &fakeLLM{}
	tu, user := testTutor(t, llm, budget.Limits{PerUser: 2})
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		if _, err := tu.Ask(ctx, user, "pregunta distinta número "+string(rune('a'+i))+t.Name(), ""); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tu.Ask(ctx, user, "una más "+t.Name(), ""); !errors.Is(err, budget.ErrUserLimit) {
		t.Fatalf("err = %v, want ErrUserLimit", err)
	}
}

func TestPromptCarriesTheExerciseAndTheGlossary(t *testing.T) {
	llm := &fakeLLM{}
	tu, user := testTutor(t, llm)
	if _, err := tu.Ask(context.Background(), user, "¿cómo es el pasado de break? "+t.Name(), "a-01"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(llm.last, "migration") {
		t.Errorf("el prompt tiene que traer el ejercicio en pantalla:\n%s", llm.last)
	}
	if !strings.Contains(llm.last, "Tema:") {
		t.Errorf("el prompt tiene que decir de qué tema es:\n%s", llm.last)
	}
}

func TestProviderErrorsBubbleUp(t *testing.T) {
	llm := &fakeLLM{err: errors.New("503 del proveedor")}
	tu, user := testTutor(t, llm)
	if _, err := tu.Ask(context.Background(), user, "algo "+t.Name(), ""); err == nil {
		t.Fatal("un error del proveedor tiene que llegar al llamador")
	}
}

func TestAnswersComeBackWithoutMarkdown(t *testing.T) {
	llm := &fakeLLM{answer: "El pasado de **want** es __wanted__.\n\n## Ejemplo\nI wanted it."}
	tu, user := testTutor(t, llm)
	res, err := tu.Ask(context.Background(), user, "¿pasado de want? "+t.Name(), "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res.Answer, "**") || strings.Contains(res.Answer, "__") || strings.Contains(res.Answer, "#") {
		t.Fatalf("la respuesta tiene que llegar en texto plano: %q", res.Answer)
	}
	if !strings.Contains(res.Answer, "Ejemplo") {
		t.Fatalf("sacar el markdown no puede comerse el texto: %q", res.Answer)
	}
}

// El modelo se equivoca solo con la -ed: "want termina en t, entonces suena /t/".
// Con el dato del glosario adelante, deja de inventarlo.
func TestPromptCarriesTheSoundOfRegularVerbs(t *testing.T) {
	llm := &fakeLLM{}
	tu, user := testTutor(t, llm)
	if _, err := tu.Ask(context.Background(), user, "¿cómo suena el pasado de push? "+t.Name(), ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(llm.last, "pushed") {
		t.Errorf("el prompt tiene que traer el verbo del glosario:\n%s", llm.last)
	}
	if !strings.Contains(llm.last, "NO suma sílaba") {
		t.Errorf("y cómo suena su -ed:\n%s", llm.last)
	}
}
