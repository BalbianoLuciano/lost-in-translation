package wallet

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/db"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/store"
)

// Lo que sólo Postgres puede probar.
//
// Las reglas y la criptografía se prueban con un mapa en wallet_test.go, que es
// más rápido y más claro. Lo que sigue es lo otro: las garantías que **no** las
// da este código sino el motor —el UNIQUE de la dirección, el borrado atómico
// del desafío, el numeric que aguanta un uint256—. Probar eso con un doble no
// prueba nada: probaría el doble.

func conBase(t *testing.T) (*pgxpool.Pool, *store.Queries) {
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
	return pool, store.New(pool)
}

// usuarioDePrueba da de alta una cuenta propia del test y la deja limpia. Los
// paquetes de test corren en paralelo contra la misma base: cada uno toca sólo
// lo suyo.
func usuarioDePrueba(t *testing.T, q *store.Queries, sufijo string) pgtype.UUID {
	t.Helper()
	ctx := context.Background()
	u, err := q.UpsertUser(ctx, store.UpsertUserParams{
		FirebaseUid: "wallet-" + t.Name() + "-" + sufijo, Email: "w@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	return u.ID
}

func servicioConBase(t *testing.T, pool *pgxpool.Pool) *Service {
	t.Helper()
	cfg, err := Interpretar(Crudo{
		RPCURL: "http://nodo.invalido", ChainID: cadenaDePrueba,
		Contrato: contratoDePrueba, ClaveHex: claveServidor,
		Origenes: []string{"http://localhost:5173"},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := NewService(pool, cfg, NumeracionCanonica())
	s.rpc = nodoFalso{err: errors.New("en los tests no se sale a la red")}
	return s
}

// El nonce es de un solo uso porque la sentencia compara y borra a la vez. Con
// dos consultas —leer, verificar, borrar— dos pedidos simultáneos pasarían los
// dos, y eso es exactamente lo que este test mira.
func TestElDesafioSeConsumeUnaSolaVez(t *testing.T) {
	pool, q := conBase(t)
	ctx := context.Background()
	user := usuarioDePrueba(t, q, "nonce")

	vence := pgtype.Timestamptz{Time: time.Now().Add(VidaDelDesafio), Valid: true}
	if err := q.PutWalletChallenge(ctx, store.PutWalletChallengeParams{
		UserID: user, Nonce: "abcdef0123456789", ExpiresAt: vence,
	}); err != nil {
		t.Fatal(err)
	}

	consumir := func(nonce string) error {
		_, err := q.ConsumeWalletChallenge(ctx, store.ConsumeWalletChallengeParams{UserID: user, Nonce: nonce})
		return err
	}

	if err := consumir("abcdef0123456789"); err != nil {
		t.Fatalf("la primera vez tenía que andar: %v", err)
	}
	if err := consumir("abcdef0123456789"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("la segunda vez: err = %v, want ErrNoRows", err)
	}

	// Un nonce que no es el emitido tampoco, y no borra el que sí está.
	if err := q.PutWalletChallenge(ctx, store.PutWalletChallengeParams{
		UserID: user, Nonce: "1111111111111111", ExpiresAt: vence,
	}); err != nil {
		t.Fatal(err)
	}
	if err := consumir("2222222222222222"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("un nonce ajeno: err = %v, want ErrNoRows", err)
	}
	if err := consumir("1111111111111111"); err != nil {
		t.Fatalf("el nonce bueno seguía valiendo: %v", err)
	}

	// Y uno vencido no sirve, con el reloj de Postgres y no el del proceso: si
	// hay dos réplicas, el que decide es el motor.
	if err := q.PutWalletChallenge(ctx, store.PutWalletChallengeParams{
		UserID: user, Nonce: "3333333333333333",
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(-time.Second), Valid: true},
	}); err != nil {
		t.Fatal(err)
	}
	if err := consumir("3333333333333333"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("un desafío vencido: err = %v, want ErrNoRows", err)
	}

	// Pedir uno nuevo pisa el anterior: no se pueden juntar nonces.
	s := servicioConBase(t, pool)
	primero, err := s.Desafio(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Desafio(ctx, user); err != nil {
		t.Fatal(err)
	}
	if err := consumir(primero.Nonce); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("el desafío viejo tenía que haber quedado pisado: %v", err)
	}
}

// Una billetera, una persona. Lo garantiza el UNIQUE (address) de la tabla, y
// el servicio lo traduce a un error que se entiende.
func TestUnaBilleteraNoEsDeDosCuentas(t *testing.T) {
	pool, q := conBase(t)
	ctx := context.Background()
	uno := usuarioDePrueba(t, q, "uno")
	dos := usuarioDePrueba(t, q, "dos")
	for _, u := range []pgtype.UUID{uno, dos} {
		if _, err := pool.Exec(ctx, "DELETE FROM wallets WHERE user_id = $1", u); err != nil {
			t.Fatal(err)
		}
	}
	// Y la dirección puede haber quedado de una corrida anterior.
	if _, err := pool.Exec(ctx, "DELETE FROM wallets WHERE address = $1", direPersona); err != nil {
		t.Fatal(err)
	}

	s := servicioConBase(t, pool)
	if err := vincularA(t, s, uno, clavePersona); err != nil {
		t.Fatal(err)
	}
	if err := vincularA(t, s, dos, clavePersona); !errors.Is(err, ErrBilleteraDeOtro) {
		t.Fatalf("err = %v, want ErrBilleteraDeOtro", err)
	}

	// Pero la misma cuenta puede volver a vincular la suya sin chocar consigo
	// misma, y puede cambiarla por otra.
	if err := vincularA(t, s, uno, clavePersona); err != nil {
		t.Fatalf("revincular la propia: %v", err)
	}
	if err := vincularA(t, s, uno, claveIntrusa); err != nil {
		t.Fatalf("cambiar de billetera: %v", err)
	}
	// Y ahora que la soltó, la primera queda libre para la otra cuenta.
	if err := vincularA(t, s, dos, clavePersona); err != nil {
		t.Fatalf("la dirección liberada: %v", err)
	}
}

func vincularA(t *testing.T, s *Service, user pgtype.UUID, claveHex string) error {
	t.Helper()
	ctx := context.Background()
	d, err := s.Desafio(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	texto, firma := firmarComo(t, claveHex, d.Message)
	_, err = s.Vincular(ctx, user, texto, firma)
	return err
}

// El ciclo entero contra la base: vincular, no poder firmar sin el logro,
// ganarlo, firmar, y guardar un recibo con un id de 49 dígitos que tiene que
// volver exactamente igual.
func TestElCicloCompletoContraPostgres(t *testing.T) {
	pool, q := conBase(t)
	ctx := context.Background()
	user := usuarioDePrueba(t, q, "ciclo")
	for _, sql := range []string{
		"DELETE FROM wallets WHERE user_id = $1",
		"DELETE FROM mints WHERE user_id = $1",
		"DELETE FROM achievements WHERE user_id = $1",
	} {
		if _, err := pool.Exec(ctx, sql, user); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, "DELETE FROM wallets WHERE address = $1", direPersona); err != nil {
		t.Fatal(err)
	}

	s := servicioConBase(t, pool)

	// Sin billetera no hay voucher, aunque el logro esté ganado.
	if err := q.GrantAchievements(ctx, store.GrantAchievementsParams{
		UserID: user, Codes: []string{string(codigoGanado)},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Voucher(ctx, user, codigoGanado); !errors.Is(err, ErrSinBilletera) {
		t.Fatalf("err = %v, want ErrSinBilletera", err)
	}

	if err := vincularA(t, s, user, clavePersona); err != nil {
		t.Fatal(err)
	}

	// Con billetera y sin el logro, tampoco: la distinción se gana, no se pide.
	if _, err := s.Voucher(ctx, user, "obra:1"); !errors.Is(err, ErrNoGanada) {
		t.Fatalf("err = %v, want ErrNoGanada", err)
	}

	v, err := s.Voucher(ctx, user, codigoGanado)
	if err != nil {
		t.Fatal(err)
	}
	if v.Pieza != piezaGanada || v.To != direPersona {
		t.Fatalf("voucher = %+v", v)
	}

	// El nodo no contesta: queda pending y el hash igual guardado.
	m, err := s.Confirmar(ctx, user, codigoGanado, hashDeMentira)
	if err != nil {
		t.Fatalf("un nodo caído no puede devolver error: %v", err)
	}
	if m.Status != "pending" || m.TxHash != hashDeMentira {
		t.Fatalf("mint = %+v", m)
	}

	// Ahora el nodo contesta, y el id de 49 dígitos tiene que sobrevivir a la
	// ida y vuelta por la columna numeric sin perder un dígito.
	esperado := TokenID(mustDireccion(direPersona), piezaGanada)
	palabra := "0x" + strings.Repeat("0", 64-len(esperado.Text(16))) + esperado.Text(16)
	s.rpc = nodoFalso{rec: reciboConTransfer(palabra)}

	m, err = s.Confirmar(ctx, user, codigoGanado, hashDeMentira)
	if err != nil {
		t.Fatal(err)
	}
	if m.Status != "confirmed" || m.TokenID != esperado.String() {
		t.Fatalf("mint = %+v, want confirmed con %s", m, esperado)
	}
	if len(m.TokenID) < 45 {
		t.Fatalf("el id tendría que tener unos 49 dígitos y tiene %d", len(m.TokenID))
	}

	// Lo confirmado no vuelve atrás: el WHERE de RecordMint lo frena en la base,
	// que es donde tiene que frenarse. El id es determinista, así que no hay
	// segundo minteo posible y un txHash nuevo sólo puede ser ruido.
	s.rpc = nodoFalso{rec: reciboConTransfer("0x0")}
	otro, err := s.Confirmar(ctx, user, codigoGanado, "0x"+strings.Repeat("44", 32))
	if err != nil {
		t.Fatal(err)
	}
	if otro.Status != "confirmed" || otro.TxHash != hashDeMentira || otro.TokenID != esperado.String() {
		t.Fatalf("lo confirmado se pisó: %+v", otro)
	}

	// Y desde Estado se ve lo mismo.
	e, err := s.Estado(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Mints) != 1 || e.Mints[0].TokenID != esperado.String() {
		t.Fatalf("estado = %+v", e)
	}
}
