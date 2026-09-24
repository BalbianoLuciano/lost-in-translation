package budget

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/store"
)

// fakeCounter cuenta en memoria, por usuario y por tipo.
type fakeCounter struct {
	perUser map[string]int
	total   map[string]int
	added   int
}

func newFake() *fakeCounter {
	return &fakeCounter{perUser: map[string]int{}, total: map[string]int{}}
}

func (f *fakeCounter) CountUsageToday(_ context.Context, arg store.CountUsageTodayParams) (int32, error) {
	return int32(f.perUser[arg.UserID.String()+"/"+arg.Kind]), nil
}

func (f *fakeCounter) CountUsageTodayAll(_ context.Context, kind string) (int32, error) {
	return int32(f.total[kind]), nil
}

func (f *fakeCounter) AddUsage(_ context.Context, arg store.AddUsageParams) error {
	f.perUser[arg.UserID.String()+"/"+arg.Kind]++
	f.total[arg.Kind]++
	f.added++
	return nil
}

func user(n byte) pgtype.UUID {
	var id pgtype.UUID
	id.Bytes[15] = n
	id.Valid = true
	return id
}

func budgetWith(f *fakeCounter, lim Limits) *Budget {
	return &Budget{q: f, limits: map[Kind]Limits{Ask: lim}}
}

func TestCheckDevuelveLoQueQueda(t *testing.T) {
	f := newFake()
	b := budgetWith(f, Limits{PerUser: 3, Global: 100})

	for want := 3; want > 0; want-- {
		left, err := b.Check(context.Background(), user(1), Ask)
		if err != nil {
			t.Fatalf("con %d usos todavía tiene margen: %v", 3-want, err)
		}
		if left != want {
			t.Fatalf("left = %d, want %d", left, want)
		}
		if err := b.Add(context.Background(), user(1), Ask); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := b.Check(context.Background(), user(1), Ask); !errors.Is(err, ErrUserLimit) {
		t.Fatalf("err = %v, want ErrUserLimit", err)
	}
}

// El tope de una persona no puede dejar sin servicio a las demás.
func TestElTopeDeUnoNoAfectaAlOtro(t *testing.T) {
	f := newFake()
	b := budgetWith(f, Limits{PerUser: 1, Global: 100})

	if err := b.Add(context.Background(), user(1), Ask); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Check(context.Background(), user(1), Ask); !errors.Is(err, ErrUserLimit) {
		t.Fatalf("el primero ya gastó lo suyo: err = %v", err)
	}
	if _, err := b.Check(context.Background(), user(2), Ask); err != nil {
		t.Fatalf("el segundo tiene su propio margen: %v", err)
	}
}

// El motivo de este paquete: la clave del proveedor es una sola, así que el
// tope por usuario no alcanza cuando los usuarios son muchos.
func TestElTopeGlobalFrenaAUnUsuarioNuevo(t *testing.T) {
	f := newFake()
	b := budgetWith(f, Limits{PerUser: 10, Global: 3})

	for i := byte(1); i <= 3; i++ {
		if _, err := b.Check(context.Background(), user(i), Ask); err != nil {
			t.Fatalf("usuario %d todavía entra: %v", i, err)
		}
		if err := b.Add(context.Background(), user(i), Ask); err != nil {
			t.Fatal(err)
		}
	}

	// El cuarto no gastó nada suyo, pero la app ya llegó a su techo.
	_, err := b.Check(context.Background(), user(4), Ask)
	if !errors.Is(err, ErrGlobalLimit) {
		t.Fatalf("err = %v, want ErrGlobalLimit", err)
	}
}

func TestSinTopeNoSeConsulta(t *testing.T) {
	f := newFake()
	b := budgetWith(f, Limits{})

	left, err := b.Check(context.Background(), user(1), Ask)
	if err != nil {
		t.Fatal(err)
	}
	if left != NoLimit {
		t.Fatalf("left = %d, want NoLimit", left)
	}
}

// Un Kind sin configurar no tiene tope: nunca frena por omisión.
func TestKindDesconocido(t *testing.T) {
	b := budgetWith(newFake(), Limits{PerUser: 1, Global: 1})
	if _, err := b.Check(context.Background(), user(1), Speaking); err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestLeftNoBajaDeCero(t *testing.T) {
	f := newFake()
	b := budgetWith(f, Limits{PerUser: 1})

	if err := b.Add(context.Background(), user(1), Ask); err != nil {
		t.Fatal(err)
	}
	if err := b.Add(context.Background(), user(1), Ask); err != nil {
		t.Fatal(err)
	}

	left, err := b.Left(context.Background(), user(1), Ask)
	if err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Fatalf("left = %d, want 0", left)
	}
}
