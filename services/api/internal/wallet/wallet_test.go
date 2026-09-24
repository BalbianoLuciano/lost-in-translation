package wallet

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/achievement"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/chain"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/store"
)

// ── El doble de la base ───────────────────────────────────────────────────
//
// Con un mapa alcanza para todo lo que se prueba acá, que es la criptografía y
// las reglas. Lo que un mapa **no** puede probar —que el nonce sea de un solo
// uso con dos pedidos simultáneos, que el UNIQUE de la dirección exista— se
// prueba contra Postgres en service_test.go, porque eso lo garantiza el motor y
// no este código.

type baseFalsa struct {
	desafios  map[string]store.WalletChallenge
	billetera map[string]store.Wallet
	ganadas   map[string]time.Time
	mints     map[string]store.Mint
	// direcciones es el UNIQUE (address) a mano.
	direcciones map[string]string
}

func nuevaBase() *baseFalsa {
	return &baseFalsa{
		desafios: map[string]store.WalletChallenge{}, billetera: map[string]store.Wallet{},
		ganadas: map[string]time.Time{}, mints: map[string]store.Mint{},
		direcciones: map[string]string{},
	}
}

func clave(u pgtype.UUID, extra string) string { return u.String() + "|" + extra }

func (b *baseFalsa) PutWalletChallenge(_ context.Context, p store.PutWalletChallengeParams) error {
	b.desafios[p.UserID.String()] = store.WalletChallenge{UserID: p.UserID, Nonce: p.Nonce, ExpiresAt: p.ExpiresAt}
	return nil
}

func (b *baseFalsa) ConsumeWalletChallenge(_ context.Context, p store.ConsumeWalletChallengeParams) (string, error) {
	d, ok := b.desafios[p.UserID.String()]
	if !ok || d.Nonce != p.Nonce || !time.Now().Before(d.ExpiresAt.Time) {
		return "", pgx.ErrNoRows
	}
	delete(b.desafios, p.UserID.String())
	return d.Nonce, nil
}

func (b *baseFalsa) LinkWallet(_ context.Context, p store.LinkWalletParams) (store.Wallet, error) {
	if dueño, ok := b.direcciones[p.Address]; ok && dueño != p.UserID.String() {
		return store.Wallet{}, errUnicoFalso
	}
	if antes, ok := b.billetera[p.UserID.String()]; ok {
		delete(b.direcciones, antes.Address)
	}
	w := store.Wallet{
		UserID: p.UserID, Address: p.Address, ChainID: p.ChainID,
		VerifiedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	b.billetera[p.UserID.String()], b.direcciones[p.Address] = w, p.UserID.String()
	return w, nil
}

func (b *baseFalsa) GetWallet(_ context.Context, u pgtype.UUID) (store.Wallet, error) {
	w, ok := b.billetera[u.String()]
	if !ok {
		return store.Wallet{}, pgx.ErrNoRows
	}
	return w, nil
}

func (b *baseFalsa) UnlinkWallet(_ context.Context, u pgtype.UUID) (int64, error) {
	w, ok := b.billetera[u.String()]
	if !ok {
		return 0, nil
	}
	delete(b.billetera, u.String())
	delete(b.direcciones, w.Address)
	return 1, nil
}

func (b *baseFalsa) GetAchievementEarned(_ context.Context, p store.GetAchievementEarnedParams) (pgtype.Timestamptz, error) {
	at, ok := b.ganadas[clave(p.UserID, p.Code)]
	if !ok {
		return pgtype.Timestamptz{}, pgx.ErrNoRows
	}
	return pgtype.Timestamptz{Time: at, Valid: true}, nil
}

func (b *baseFalsa) RecordMint(_ context.Context, p store.RecordMintParams) (store.Mint, error) {
	k := clave(p.UserID, p.Code)
	if m, ok := b.mints[k]; ok && m.Status == "confirmed" {
		return store.Mint{}, pgx.ErrNoRows
	}
	m := store.Mint{
		UserID: p.UserID, Code: p.Code, Address: p.Address,
		ChainID: p.ChainID, TxHash: p.TxHash, Status: "pending",
	}
	b.mints[k] = m
	return m, nil
}

func (b *baseFalsa) SettleMint(_ context.Context, p store.SettleMintParams) (store.Mint, error) {
	k := clave(p.UserID, p.Code)
	m, ok := b.mints[k]
	if !ok {
		return store.Mint{}, pgx.ErrNoRows
	}
	m.Status, m.TokenID = p.Status, p.TokenID
	b.mints[k] = m
	return m, nil
}

func (b *baseFalsa) GetMint(_ context.Context, p store.GetMintParams) (store.Mint, error) {
	m, ok := b.mints[clave(p.UserID, p.Code)]
	if !ok {
		return store.Mint{}, pgx.ErrNoRows
	}
	return m, nil
}

func (b *baseFalsa) ListMints(_ context.Context, u pgtype.UUID) ([]store.Mint, error) {
	var out []store.Mint
	for _, m := range b.mints {
		if m.UserID == u {
			out = append(out, m)
		}
	}
	return out, nil
}

// El mismo error que devuelve Postgres al chocar contra UNIQUE (address). Se
// arma con su código y no con un errors.New para que el test recorra el
// errors.As de verdad: si alguien cambia la detección, esto lo agarra.
var errUnicoFalso = &pgconn.PgError{
	Code:           "23505",
	Message:        `duplicate key value violates unique constraint "wallets_address_key"`,
	ConstraintName: "wallets_address_key",
}

// nodoFalso devuelve lo que se le diga, incluso un error.
type nodoFalso struct {
	rec chain.Recibo
	err error
}

func (n nodoFalso) Recibo(context.Context, string) (chain.Recibo, error) { return n.rec, n.err }

// ── El armado del servicio de prueba ──────────────────────────────────────

const (
	// La cuenta 1 de anvil: la billetera que vincula la persona.
	clavePersona = "0x59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d"
	direPersona  = "0x70997970c51812dc3a010c7d01b50e0d17dc79c8"
	// La cuenta 2: cualquier otra, para probar que una firma ajena se rechaza.
	claveIntrusa = "0x5de4111afa1a4b94908f83103eb1f1706367c2e68ca870fc3fb9a804cdab365a"
	// La cuenta 0 hace de clave de firma del servidor.
	claveServidor = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

	contratoDePrueba = "0x7fa04a5a7fd31c6215c1d980514db5e173d09e69"
	cadenaDePrueba   = uint64(84532)

	// Una distinción que existe en el catálogo real, con su número de pieza.
	codigoGanado = achievement.Code("pieza:reported_speech.basic")
	piezaGanada  = uint16(38)
)

var usuario = pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4}, Valid: true}

func servicioDePrueba(t *testing.T, base *baseFalsa, n nodo) *Service {
	t.Helper()
	cfg, err := Interpretar(Crudo{
		RPCURL: "http://nodo.invalido", ChainID: cadenaDePrueba,
		Contrato: contratoDePrueba, ClaveHex: claveServidor,
		Origenes: []string{"http://localhost:5173"},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := NewService(nil, cfg, NumeracionCanonica())
	s.q, s.rpc = base, n
	return s
}

// firmarComo arma el mensaje del desafío completándolo con una dirección y lo
// firma, que es exactamente lo que hace la billetera del navegador.
func firmarComo(t *testing.T, claveHex, plantilla string) (texto, firma string) {
	t.Helper()
	priv, err := chain.ClaveDesdeHex(claveHex)
	if err != nil {
		t.Fatal(err)
	}
	texto = conDireccion(plantilla, chain.DireccionDeClave(priv.PubKey()))
	return texto, firmarTexto(t, priv, texto)
}

func firmarTexto(t *testing.T, priv *secp256k1.PrivateKey, texto string) string {
	t.Helper()
	f, err := chain.Firmar(priv, chain.HashPersonal([]byte(texto)))
	if err != nil {
		t.Fatal(err)
	}
	return f.Hex()
}

// conDireccion reemplaza la segunda línea, que es la que completa el cliente.
func conDireccion(texto string, d chain.Direccion) string {
	lineas := strings.Split(texto, "\n")
	lineas[1] = d.Hex()
	return strings.Join(lineas, "\n")
}

func vincular(t *testing.T, s *Service, claveHex string) (Vinculo, error) {
	t.Helper()
	d, err := s.Desafio(context.Background(), usuario)
	if err != nil {
		t.Fatal(err)
	}
	texto, firma := firmarComo(t, claveHex, d.Message)
	return s.Vincular(context.Background(), usuario, texto, firma)
}

// ── Sin configuración, la cadena no existe ────────────────────────────────

// La regla del §2 llevada al código: sin las cuatro variables la funcionalidad
// no falla, no existe. Es la misma decisión que el chat sin clave de Groq.
func TestSinConfiguracionTodoContestaErrSinCadena(t *testing.T) {
	s := NewService(nil, nil, NumeracionCanonica())
	s.q = nuevaBase()
	ctx := context.Background()

	if s.Configured() {
		t.Fatal("sin cfg no tendría que estar configurado")
	}

	llamadas := map[string]func() error{
		"Estado":      func() error { _, err := s.Estado(ctx, usuario); return err },
		"Desafio":     func() error { _, err := s.Desafio(ctx, usuario); return err },
		"Vincular":    func() error { _, err := s.Vincular(ctx, usuario, "x", "y"); return err },
		"Desvincular": func() error { _, err := s.Desvincular(ctx, usuario); return err },
		"Voucher":     func() error { _, err := s.Voucher(ctx, usuario, codigoGanado); return err },
		"Confirmar":   func() error { _, err := s.Confirmar(ctx, usuario, codigoGanado, hashDeMentira); return err },
	}
	for nombre, llamar := range llamadas {
		t.Run(nombre, func(t *testing.T) {
			if err := llamar(); !errors.Is(err, ErrSinCadena) {
				t.Fatalf("err = %v, want ErrSinCadena", err)
			}
		})
	}
}

// Una configuración a medias es peor que ninguna: parece que anda. Rompe el
// arranque, que es el único momento en que alguien la puede arreglar.
func TestInterpretarRechazaLaConfiguracionAMedias(t *testing.T) {
	completa := Crudo{
		RPCURL: "https://sepolia.base.org", ChainID: cadenaDePrueba,
		Contrato: contratoDePrueba, ClaveHex: claveServidor,
		Origenes: []string{"https://lit.example"},
	}

	tests := []struct {
		name string
		toca func(*Crudo)
		want error
	}{
		{"completa", func(*Crudo) {}, nil},
		{"sin nada: la cadena no existe y no es un error", func(c *Crudo) { *c = Crudo{Origenes: c.Origenes} }, nil},
		{"sin RPC", func(c *Crudo) { c.RPCURL = "" }, ErrConfigIncompleta},
		{"sin chain id", func(c *Crudo) { c.ChainID = 0 }, ErrConfigIncompleta},
		{"sin contrato", func(c *Crudo) { c.Contrato = "" }, ErrConfigIncompleta},
		{"sin clave", func(c *Crudo) { c.ClaveHex = "" }, ErrConfigIncompleta},
		{"sin orígenes no hay dominio para el mensaje SIWE", func(c *Crudo) { c.Origenes = nil }, ErrConfigIncompleta},
		{"el contrato no es una dirección", func(c *Crudo) { c.Contrato = "0xnope" }, chain.ErrDireccionInvalida},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := completa
			tt.toca(&c)
			cfg, err := Interpretar(c)
			if tt.want != nil {
				if !errors.Is(err, tt.want) {
					t.Fatalf("err = %v, want %v", err, tt.want)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if c.Definido() != (cfg != nil) {
				t.Fatalf("Definido() = %v pero cfg = %v", c.Definido(), cfg)
			}
		})
	}
}

// El mensaje de error de la clave no puede llevar la clave, ni un pedazo: los
// logs de arranque se leen en cualquier lado.
func TestElErrorDeLaClaveNoFiltraLaClave(t *testing.T) {
	const secreto = "0xdeadbeef"
	_, err := Interpretar(Crudo{
		RPCURL: "http://x", ChainID: 1, Contrato: contratoDePrueba,
		ClaveHex: secreto, Origenes: []string{"http://localhost:5173"},
	})
	if err == nil {
		t.Fatal("una clave de 4 bytes no es una clave")
	}
	if strings.Contains(err.Error(), "deadbeef") {
		t.Fatalf("el error trae la clave adentro: %v", err)
	}
}

// ── SIWE ──────────────────────────────────────────────────────────────────

func TestVincularGuardaLaDireccionQueFirmo(t *testing.T) {
	base := nuevaBase()
	s := servicioDePrueba(t, base, nodoFalso{})

	v, err := vincular(t, s, clavePersona)
	if err != nil {
		t.Fatal(err)
	}
	if v.Address != direPersona {
		t.Fatalf("address = %s, want %s", v.Address, direPersona)
	}
	if v.ChainID != cadenaDePrueba {
		t.Fatalf("chainId = %d, want %d", v.ChainID, cadenaDePrueba)
	}
	// En minúsculas: es lo que hace que el UNIQUE signifique algo.
	if v.Address != strings.ToLower(v.Address) {
		t.Fatalf("la dirección tiene que guardarse en minúsculas: %s", v.Address)
	}
}

// El corazón de SIWE: la dirección se **prueba**, no se declara. Un mensaje que
// dice ser de otro, firmado con la clave propia, no vincula nada.
func TestVincularRechazaLaFirmaDeOtraDireccion(t *testing.T) {
	base := nuevaBase()
	s := servicioDePrueba(t, base, nodoFalso{})

	d, err := s.Desafio(context.Background(), usuario)
	if err != nil {
		t.Fatal(err)
	}
	// El texto dice la dirección de la persona; quien firma es la intrusa.
	texto := conDireccion(d.Message, mustDireccion(direPersona))
	intrusa, err := chain.ClaveDesdeHex(claveIntrusa)
	if err != nil {
		t.Fatal(err)
	}
	firma := firmarTexto(t, intrusa, texto)

	if _, err := s.Vincular(context.Background(), usuario, texto, firma); !errors.Is(err, ErrFirmaAjena) {
		t.Fatalf("err = %v, want ErrFirmaAjena", err)
	}
	if len(base.billetera) != 0 {
		t.Fatal("no tenía que guardar nada")
	}
}

// Un nonce se usa una vez. Repetir el mismo mensaje con la misma firma —que es
// exactamente lo que haría quien intercepte la llamada— no vuelve a vincular.
func TestUnNonceNoSirveDosVeces(t *testing.T) {
	base := nuevaBase()
	s := servicioDePrueba(t, base, nodoFalso{})

	d, err := s.Desafio(context.Background(), usuario)
	if err != nil {
		t.Fatal(err)
	}
	texto, firma := firmarComo(t, clavePersona, d.Message)

	if _, err := s.Vincular(context.Background(), usuario, texto, firma); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Vincular(context.Background(), usuario, texto, firma); !errors.Is(err, ErrDesafioInvalido) {
		t.Fatalf("la segunda vez: err = %v, want ErrDesafioInvalido", err)
	}
}

// Y una firma mal armada también quema el desafío: si no, se podrían probar
// firmas contra el mismo nonce hasta acertar.
func TestUnIntentoFallidoQuemaElDesafio(t *testing.T) {
	base := nuevaBase()
	s := servicioDePrueba(t, base, nodoFalso{})

	d, err := s.Desafio(context.Background(), usuario)
	if err != nil {
		t.Fatal(err)
	}
	texto, firma := firmarComo(t, clavePersona, d.Message)

	// Una firma de la longitud correcta pero que no es de nadie.
	basura := "0x" + strings.Repeat("11", 64) + "1b"
	if _, err := s.Vincular(context.Background(), usuario, texto, basura); err == nil {
		t.Fatal("una firma inventada no tendría que pasar")
	}
	if _, err := s.Vincular(context.Background(), usuario, texto, firma); !errors.Is(err, ErrDesafioInvalido) {
		t.Fatalf("el desafío tenía que estar quemado: err = %v", err)
	}
}

// Un nonce que nunca se emitió no sirve, aunque el mensaje esté impecable y la
// firma cierre.
func TestUnNonceInventadoNoSirve(t *testing.T) {
	base := nuevaBase()
	s := servicioDePrueba(t, base, nodoFalso{})

	d, err := s.Desafio(context.Background(), usuario)
	if err != nil {
		t.Fatal(err)
	}
	otro := strings.Replace(d.Message, "Nonce: "+d.Nonce, "Nonce: 0000000000000000", 1)
	texto, firma := firmarComo(t, clavePersona, otro)

	if _, err := s.Vincular(context.Background(), usuario, texto, firma); !errors.Is(err, ErrDesafioInvalido) {
		t.Fatalf("err = %v, want ErrDesafioInvalido", err)
	}
}

// Un mensaje armado a mano con otro dominio: la firma es perfecta y no sirve.
// Es literalmente la razón por la que EIP-4361 existe.
func TestUnMensajeConOtroDominioNoVincula(t *testing.T) {
	base := nuevaBase()
	s := servicioDePrueba(t, base, nodoFalso{})

	d, err := s.Desafio(context.Background(), usuario)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		toca func(string) string
		want error
	}{
		{"otro dominio en el encabezado",
			func(m string) string { return strings.Replace(m, "localhost:5173 wants", "evil.example wants", 1) },
			ErrDominioAjeno},
		{"otra URI",
			func(m string) string {
				return strings.Replace(m, "URI: http://localhost:5173", "URI: https://evil.example", 1)
			},
			ErrDominioAjeno},
		{"otra cadena",
			func(m string) string { return strings.Replace(m, "Chain ID: 84532", "Chain ID: 1", 1) },
			ErrCadenaAjena},
		{"otro statement",
			func(m string) string { return strings.Replace(m, statement, "Just sign, it is fine", 1) },
			ErrMensajeInvalido},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			texto, firma := firmarComo(t, clavePersona, tt.toca(d.Message))
			_, err := s.Vincular(context.Background(), usuario, texto, firma)
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			// Y el desafío sigue vivo: el pedido ni siquiera llegó a la base.
			if len(base.desafios) != 1 {
				t.Fatal("un mensaje de otro dominio no tendría que quemar el nonce")
			}
		})
	}
}

// Vencido no sirve. El reloj lo pone el servicio, así que se puede adelantar.
func TestUnDesafioVencidoNoSirve(t *testing.T) {
	base := nuevaBase()
	s := servicioDePrueba(t, base, nodoFalso{})

	d, err := s.Desafio(context.Background(), usuario)
	if err != nil {
		t.Fatal(err)
	}
	texto, firma := firmarComo(t, clavePersona, d.Message)

	// Un minuto después de que el propio mensaje dice que venció.
	s.ahora = func() time.Time { return d.ExpiresAt.Add(time.Minute) }
	if _, err := s.Vincular(context.Background(), usuario, texto, firma); !errors.Is(err, ErrMensajeVencido) {
		t.Fatalf("err = %v, want ErrMensajeVencido", err)
	}
}

// Una billetera, una persona (UNIQUE address).
func TestUnaBilleteraDeOtroNoSeVincula(t *testing.T) {
	base := nuevaBase()
	s := servicioDePrueba(t, base, nodoFalso{})
	base.direcciones[direPersona] = "otra-cuenta"

	if _, err := vincular(t, s, clavePersona); !errors.Is(err, ErrBilleteraDeOtro) {
		t.Fatalf("err = %v, want ErrBilleteraDeOtro", err)
	}
}

func TestDesvincular(t *testing.T) {
	base := nuevaBase()
	s := servicioDePrueba(t, base, nodoFalso{})
	if _, err := vincular(t, s, clavePersona); err != nil {
		t.Fatal(err)
	}

	habia, err := s.Desvincular(context.Background(), usuario)
	if err != nil || !habia {
		t.Fatalf("Desvincular = %v, %v", habia, err)
	}
	// Desvincular dos veces no es un error: el segundo no tenía nada que soltar.
	if habia, err := s.Desvincular(context.Background(), usuario); err != nil || habia {
		t.Fatalf("la segunda vez = %v, %v", habia, err)
	}
	// Y la dirección queda libre para que la use otra cuenta.
	if len(base.direcciones) != 0 {
		t.Fatal("desvincular tenía que soltar la dirección")
	}
}

// ── El voucher ────────────────────────────────────────────────────────────

// Las dos condiciones del §8, probadas por separado y en tabla.
func TestVoucherSoloConLogroGanadoYBilleteraVerificada(t *testing.T) {
	tests := []struct {
		name      string
		billetera bool
		ganada    bool
		code      achievement.Code
		want      error
	}{
		{"todo en orden", true, true, codigoGanado, nil},
		{"sin billetera verificada", false, true, codigoGanado, ErrSinBilletera},
		{"el logro no está ganado", true, false, codigoGanado, ErrNoGanada},
		{"ni billetera ni logro", false, false, codigoGanado, ErrSinBilletera},
		{"una distinción que no existe", true, true, "pieza:no.existe", ErrDistincionDesconocida},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := nuevaBase()
			s := servicioDePrueba(t, base, nodoFalso{})
			if tt.billetera {
				if _, err := vincular(t, s, clavePersona); err != nil {
					t.Fatal(err)
				}
			}
			if tt.ganada {
				base.ganadas[clave(usuario, string(tt.code))] = time.Now()
			}

			v, err := s.Voucher(context.Background(), usuario, tt.code)
			if tt.want != nil {
				if !errors.Is(err, tt.want) {
					t.Fatalf("err = %v, want %v", err, tt.want)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if v.To != direPersona || v.Pieza != piezaGanada {
				t.Fatalf("voucher = %+v", v)
			}
		})
	}
}

// El voucher que se firma tiene que ser el que el contrato verifica: mismo
// dominio EIP-712, mismos campos, y la firma tiene que recuperar al firmante
// que el contrato tiene configurado. Es la mitad de Go del test cruzado.
func TestElVoucherLoFirmaLaClaveDelServidorYNoVenceTarde(t *testing.T) {
	base := nuevaBase()
	s := servicioDePrueba(t, base, nodoFalso{})
	if _, err := vincular(t, s, clavePersona); err != nil {
		t.Fatal(err)
	}
	base.ganadas[clave(usuario, string(codigoGanado))] = time.Now()

	antes := time.Now()
	v, err := s.Voucher(context.Background(), usuario, codigoGanado)
	if err != nil {
		t.Fatal(err)
	}

	// El deadline es corto: es la mitigación barata de que se filtre la clave.
	sobra := time.Unix(int64(v.Deadline), 0).Sub(antes)
	if sobra <= 0 || sobra > 15*time.Minute {
		t.Fatalf("el deadline vence en %v: tiene que ser de minutos", sobra)
	}

	firma, err := chain.ParseFirma(v.Firma)
	if err != nil {
		t.Fatal(err)
	}
	digest := chain.Digest(
		chain.DominioDistinciones(cadenaDePrueba, mustDireccion(contratoDePrueba)),
		chain.Voucher{To: mustDireccion(direPersona), Pieza: piezaGanada, Deadline: v.Deadline},
	)
	recuperado, err := chain.Recuperar(digest, firma)
	if err != nil {
		t.Fatal(err)
	}
	if recuperado.Hex() != s.cfg.FirmanteHex() {
		t.Fatalf("firmó %s y el firmante configurado es %s", recuperado.Hex(), s.cfg.FirmanteHex())
	}

	// Y el id del token es el que el contrato va a calcular solo.
	quiero := TokenID(mustDireccion(direPersona), piezaGanada).String()
	if v.TokenID != quiero {
		t.Fatalf("tokenId = %s, want %s", v.TokenID, quiero)
	}
}

// El id se calcula, no se cuenta: (to << 16) | pieza.
func TestTokenID(t *testing.T) {
	// El mismo número que verifica el test de chain, desde el otro lado.
	const want = "42128477998799320712182006334230556568206271482167313"
	if got := TokenID(mustDireccion(direPersona), 17).String(); got != want {
		t.Fatalf("TokenID = %s, want %s", got, want)
	}
	// Dos piezas de la misma persona dan ids distintos, y la misma pieza de dos
	// personas también: es lo que hace que no haga falta una tabla de reclamos.
	a := TokenID(mustDireccion(direPersona), 1)
	b := TokenID(mustDireccion(direPersona), 2)
	c := TokenID(mustDireccion(contratoDePrueba), 1)
	if a.Cmp(b) == 0 || a.Cmp(c) == 0 {
		t.Fatal("el id tiene que distinguir pieza y dueño")
	}
}

// ── Confirmar ─────────────────────────────────────────────────────────────

const hashDeMentira = "0x2222222222222222222222222222222222222222222222222222222222222222"

func reciboConTransfer(tokenID string) chain.Recibo {
	return chain.Recibo{
		Exitoso: true, Bloque: 10,
		Logs: []chain.LogRecibo{{
			Address: contratoDePrueba,
			Topics: []string{
				temaTransfer,
				"0x" + strings.Repeat("0", 64),
				"0x000000000000000000000000" + strings.TrimPrefix(direPersona, "0x"),
				tokenID,
			},
		}},
	}
}

func TestConfirmar(t *testing.T) {
	// El id que el contrato acuña para la pieza 38 de esta dirección.
	esperado := TokenID(mustDireccion(direPersona), piezaGanada)
	palabra := "0x" + strings.Repeat("0", 64-len(esperado.Text(16))) + esperado.Text(16)

	tests := []struct {
		name       string
		nodo       nodoFalso
		wantStatus string
		wantToken  string
	}{
		{
			name:       "el recibo dice que salió bien",
			nodo:       nodoFalso{rec: reciboConTransfer(palabra)},
			wantStatus: "confirmed", wantToken: esperado.String(),
		},
		{
			// La trampa: hay recibo y la transacción revirtió igual.
			name:       "el recibo dice que revirtió",
			nodo:       nodoFalso{rec: chain.Recibo{Exitoso: false}},
			wantStatus: "failed",
		},
		{
			// La regla del §8: el logro ya está en la base, así que un nodo
			// caído deja pending y se reintenta. Nunca un error que pierda el dato.
			name:       "el nodo no contesta",
			nodo:       nodoFalso{err: errors.New("connection refused")},
			wantStatus: "pending",
		},
		{
			name:       "la transacción todavía no se minó",
			nodo:       nodoFalso{err: chain.ErrSinRecibo},
			wantStatus: "pending",
		},
		{
			// Sin el evento, la transacción salió bien igual: se confirma y el
			// id queda vacío, que es más honesto que inventarlo.
			name:       "salió bien pero sin evento Transfer nuestro",
			nodo:       nodoFalso{rec: chain.Recibo{Exitoso: true}},
			wantStatus: "confirmed",
		},
		{
			// Un Transfer de otro contrato en la misma transacción no cuenta.
			name: "el Transfer es de otro contrato",
			nodo: nodoFalso{rec: chain.Recibo{Exitoso: true, Logs: []chain.LogRecibo{{
				Address: "0x1111111111111111111111111111111111111111",
				Topics:  []string{temaTransfer, "0x0", "0x0", palabra},
			}}}},
			wantStatus: "confirmed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := nuevaBase()
			s := servicioDePrueba(t, base, tt.nodo)
			if _, err := vincular(t, s, clavePersona); err != nil {
				t.Fatal(err)
			}
			base.ganadas[clave(usuario, string(codigoGanado))] = time.Now()

			m, err := s.Confirmar(context.Background(), usuario, codigoGanado, hashDeMentira)
			if err != nil {
				t.Fatalf("Confirmar nunca puede devolver error por culpa del nodo: %v", err)
			}
			if m.Status != tt.wantStatus {
				t.Fatalf("status = %q, want %q", m.Status, tt.wantStatus)
			}
			if m.TokenID != tt.wantToken {
				t.Fatalf("tokenId = %q, want %q", m.TokenID, tt.wantToken)
			}
			// El hash siempre quedó guardado, pase lo que pase con el nodo.
			if guardado := base.mints[clave(usuario, string(codigoGanado))]; guardado.TxHash.String != hashDeMentira {
				t.Fatalf("el txHash no se guardó: %+v", guardado)
			}
		})
	}
}

// Confirmar exige lo mismo que el voucher: el hash de una transacción de otro
// no puede anotarse contra un logro que no se ganó.
func TestConfirmarExigeLoMismoQueElVoucher(t *testing.T) {
	tests := []struct {
		name      string
		billetera bool
		ganada    bool
		hash      string
		want      error
	}{
		{"sin billetera", false, true, hashDeMentira, ErrSinBilletera},
		{"sin el logro ganado", true, false, hashDeMentira, ErrNoGanada},
		{"el hash no es un hash", true, true, "no soy un hash", chain.ErrHashInvalido},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := nuevaBase()
			s := servicioDePrueba(t, base, nodoFalso{})
			if tt.billetera {
				if _, err := vincular(t, s, clavePersona); err != nil {
					t.Fatal(err)
				}
			}
			if tt.ganada {
				base.ganadas[clave(usuario, string(codigoGanado))] = time.Now()
			}
			if _, err := s.Confirmar(context.Background(), usuario, codigoGanado, tt.hash); !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

// Una distinción ya confirmada no vuelve atrás por un txHash nuevo.
func TestConfirmarNoPisaLoQueYaEstaConfirmado(t *testing.T) {
	esperado := TokenID(mustDireccion(direPersona), piezaGanada)
	palabra := "0x" + strings.Repeat("0", 64-len(esperado.Text(16))) + esperado.Text(16)

	base := nuevaBase()
	s := servicioDePrueba(t, base, nodoFalso{rec: reciboConTransfer(palabra)})
	if _, err := vincular(t, s, clavePersona); err != nil {
		t.Fatal(err)
	}
	base.ganadas[clave(usuario, string(codigoGanado))] = time.Now()

	if _, err := s.Confirmar(context.Background(), usuario, codigoGanado, hashDeMentira); err != nil {
		t.Fatal(err)
	}

	// Ahora llega otro hash, con un nodo que diría que falló.
	s.rpc = nodoFalso{rec: chain.Recibo{Exitoso: false}}
	otro := "0x" + strings.Repeat("33", 32)
	m, err := s.Confirmar(context.Background(), usuario, codigoGanado, otro)
	if err != nil {
		t.Fatal(err)
	}
	if m.Status != "confirmed" || m.TxHash != hashDeMentira {
		t.Fatalf("lo confirmado no se pisa: %+v", m)
	}
}

// Estado es lo que la pantalla lee de una sola vez.
func TestEstado(t *testing.T) {
	base := nuevaBase()
	s := servicioDePrueba(t, base, nodoFalso{})
	ctx := context.Background()

	// Sin billetera: no es un error, es lo normal antes de vincular.
	e, err := s.Estado(ctx, usuario)
	if err != nil {
		t.Fatal(err)
	}
	if e.Address != nil || e.ChainID != cadenaDePrueba || e.Contrato != contratoDePrueba {
		t.Fatalf("estado sin billetera = %+v", e)
	}
	if len(e.Mints) != 0 {
		t.Fatal("mints tiene que ser una lista vacía y nunca null")
	}

	if _, err := vincular(t, s, clavePersona); err != nil {
		t.Fatal(err)
	}
	e, err = s.Estado(ctx, usuario)
	if err != nil {
		t.Fatal(err)
	}
	if e.Address == nil || *e.Address != direPersona || e.Verified == nil {
		t.Fatalf("estado con billetera = %+v", e)
	}
}
