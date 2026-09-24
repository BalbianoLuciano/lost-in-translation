// Package wallet es el cable que une el servidor con la cadena.
//
// Hace tres cosas, y ninguna de ellas es "guardar progreso": eso ya está hecho
// y vive en Postgres. Acá sólo se puede
//
//   - probar que una billetera es de quien dice (SIWE, EIP-4361);
//   - firmar un voucher para una distinción **que ya está ganada** (EIP-712);
//   - anotar lo que se vio en la cadena cuando la transacción se minó.
//
// La regla que ordena todo el paquete es la del §2 del diseño: la cadena es el
// recibo, no la verdad. De acá no sale ninguna escritura sobre el progreso, y
// ningún camino de la app espera a la red: el único lugar donde se le habla a
// un nodo es Confirmar, con cinco segundos de paciencia, y si el nodo no está
// el dato ya quedó guardado igual.
//
// Sin configuración el servicio se construye igual y contesta ErrSinCadena a
// todo. Es la misma decisión que el chat sin clave de Groq: la funcionalidad no
// existe, no falla.
package wallet

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/achievement"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/chain"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/store"
)

// VidaDelVoucher: cuánto vale un voucher firmado.
//
// Corto a propósito, y es la mitigación barata de la peor amenaza del §8: si la
// clave de firma se filtrara, lo que el atacante robe le sirve por minutos y no
// para siempre. Diez alcanzan de sobra para apretar "confirmar" en la billetera
// y esperar un bloque; en Base salen cada dos segundos.
const VidaDelVoucher = 10 * time.Minute

var (
	// ErrSinCadena: no hay configuración de cadena. No es una falla, es que la
	// funcionalidad no existe en este despliegue.
	ErrSinCadena = errors.New("la cadena no está configurada en este servidor")
	// ErrSinBilletera: esta cuenta no vinculó ninguna billetera.
	ErrSinBilletera = errors.New("no hay ninguna billetera vinculada a esta cuenta")
	// ErrNoGanada: la distinción no está en la tabla. Es la defensa del §8
	// contra reclamar un logro que no se ganó, y es la única que importa.
	ErrNoGanada = errors.New("esa distinción todavía no está ganada")
	// ErrDistincionDesconocida: ese código no existe en el catálogo.
	ErrDistincionDesconocida = errors.New("no existe esa distinción")
	// ErrDesafioInvalido: el nonce no es el que se emitió, ya se usó o venció.
	// Los tres casos dan el mismo error a propósito: distinguirlos le regalaría
	// a quien prueba nonces un oráculo que dice cuál existió.
	ErrDesafioInvalido = errors.New("el desafío no existe, ya se usó o venció: pedí uno nuevo")
	// ErrFirmaAjena: la firma no la hizo la dirección que el mensaje declara.
	ErrFirmaAjena = errors.New("la firma no corresponde a la dirección del mensaje")
	// ErrBilleteraDeOtro: esa dirección ya está vinculada a otra cuenta. Una
	// billetera, una persona (UNIQUE address).
	ErrBilleteraDeOtro = errors.New("esa billetera ya está vinculada a otra cuenta")
)

// almacen es lo que este paquete necesita de la base, y nada más. La interfaz
// la define el consumidor: así el test la cumple con un mapa y no hace falta
// una base para probar la criptografía.
type almacen interface {
	PutWalletChallenge(ctx context.Context, arg store.PutWalletChallengeParams) error
	ConsumeWalletChallenge(ctx context.Context, arg store.ConsumeWalletChallengeParams) (string, error)
	LinkWallet(ctx context.Context, arg store.LinkWalletParams) (store.Wallet, error)
	GetWallet(ctx context.Context, userID pgtype.UUID) (store.Wallet, error)
	UnlinkWallet(ctx context.Context, userID pgtype.UUID) (int64, error)
	GetAchievementEarned(ctx context.Context, arg store.GetAchievementEarnedParams) (pgtype.Timestamptz, error)
	RecordMint(ctx context.Context, arg store.RecordMintParams) (store.Mint, error)
	GetMint(ctx context.Context, arg store.GetMintParams) (store.Mint, error)
	SettleMint(ctx context.Context, arg store.SettleMintParams) (store.Mint, error)
	ListMints(ctx context.Context, userID pgtype.UUID) ([]store.Mint, error)
}

// nodo es la única pregunta que se le hace a la cadena.
type nodo interface {
	Recibo(ctx context.Context, txHash string) (chain.Recibo, error)
}

type Service struct {
	q   almacen
	cfg *Config // nil: no hay cadena
	num *Numeracion
	rpc nodo

	// ahora y nonce se inyectan para que los tests puedan fijar el tiempo y el
	// azar. En producción son time.Now y crypto/rand.
	ahora func() time.Time
	nonce func() (string, error)
}

// NewService arma el servicio. cfg en nil es el caso normal sin cadena.
func NewService(pool *pgxpool.Pool, cfg *Config, num *Numeracion) *Service {
	var n nodo
	if cfg != nil {
		n = chain.NuevoRPC(cfg.RPCURL)
	}
	return &Service{q: store.New(pool), cfg: cfg, num: num, rpc: n, ahora: time.Now, nonce: nonceAlAzar}
}

// Configured: sin las cuatro variables, la sección de distinciones en la cadena
// no aparece en ningún lado. Lo mira /healthz y lo mira la web.
func (s *Service) Configured() bool { return s.cfg != nil }

func nonceAlAzar() (string, error) {
	b := make([]byte, largoNonce/2)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("wallet: no se pudo generar el nonce: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// ── Lo que la web ve ──────────────────────────────────────────────────────

// Estado es todo lo que la pantalla necesita saber de una vez.
type Estado struct {
	// ChainID y Contrato los necesita el cliente para armar la transacción. Que
	// vengan del servidor y no de una constante del front es lo que hace que
	// mudar de red sea cambiar una variable de entorno.
	ChainID  uint64     `json:"chainId"`
	Contrato string     `json:"contract"`
	Firmante string     `json:"signer"`
	Address  *string    `json:"address"`
	Verified *time.Time `json:"verifiedAt"`
	Mints    []MintView `json:"mints"`
}

// MintView es un recibo como lo ve la app.
type MintView struct {
	Code    string `json:"code"`
	Status  string `json:"status"`
	TxHash  string `json:"txHash"`
	TokenID string `json:"tokenId"`
}

func (s *Service) Estado(ctx context.Context, userID pgtype.UUID) (Estado, error) {
	if s.cfg == nil {
		return Estado{}, ErrSinCadena
	}
	e := Estado{
		ChainID:  s.cfg.ChainID,
		Contrato: s.cfg.Contrato.Hex(),
		Firmante: s.cfg.FirmanteHex(),
		Mints:    []MintView{},
	}

	w, err := s.q.GetWallet(ctx, userID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// Sin billetera no es un error: es lo normal antes de vincular.
		return e, nil
	case err != nil:
		return Estado{}, err
	}
	addr, at := w.Address, w.VerifiedAt.Time
	e.Address, e.Verified = &addr, &at

	filas, err := s.q.ListMints(ctx, userID)
	if err != nil {
		return Estado{}, err
	}
	for _, m := range filas {
		e.Mints = append(e.Mints, vistaDeMint(m))
	}
	return e, nil
}

func vistaDeMint(m store.Mint) MintView {
	v := MintView{Code: m.Code, Status: m.Status}
	if m.TxHash.Valid {
		v.TxHash = m.TxHash.String
	}
	if m.TokenID.Valid {
		// El uint256 sale como **texto** y no como número: un id de token tiene
		// unos 49 dígitos y cualquier parser de JSON de JavaScript lo redondea
		// sin avisar. Es el bug de Web3 más aburrido que existe.
		if val, err := m.TokenID.Value(); err == nil {
			v.TokenID, _ = val.(string)
		}
	}
	return v
}

// ── SIWE: vincular la billetera ───────────────────────────────────────────

// Desafio es el mensaje que hay que firmar, con lo que dura.
type Desafio struct {
	Message   string    `json:"message"`
	Nonce     string    `json:"nonce"`
	ExpiresAt time.Time `json:"expiresAt"`
	// Statement va aparte para que la pantalla pueda mostrarlo antes de abrir
	// la billetera: el aviso de qué queda en la cadena se lee antes de firmar,
	// no en una ventana modal de MetaMask.
	Statement string `json:"statement"`
}

// Desafio emite el mensaje EIP-4361.
//
// La dirección va en cero porque el servidor todavía no sabe cuál es: la
// completa el cliente con la que le dé la billetera, y el servidor la lee de
// vuelta del texto firmado. Esa asimetría es lo que hace que la dirección se
// **pruebe** en vez de declararse.
func (s *Service) Desafio(ctx context.Context, userID pgtype.UUID) (Desafio, error) {
	if s.cfg == nil {
		return Desafio{}, ErrSinCadena
	}
	nonce, err := s.nonce()
	if err != nil {
		return Desafio{}, err
	}
	emitido := s.ahora().UTC()
	vence := emitido.Add(VidaDelDesafio)

	msg := ArmarMensaje(Mensaje{
		Dominio:    s.cfg.Dominio,
		Statement:  statement,
		URI:        s.cfg.URI,
		Version:    "1",
		ChainID:    s.cfg.ChainID,
		Nonce:      nonce,
		IssuedAt:   emitido,
		Expiration: vence,
	})

	if err := s.q.PutWalletChallenge(ctx, store.PutWalletChallengeParams{
		UserID: userID, Nonce: nonce,
		ExpiresAt: pgtype.Timestamptz{Time: vence, Valid: true},
	}); err != nil {
		return Desafio{}, err
	}
	return Desafio{Message: msg, Nonce: nonce, ExpiresAt: vence, Statement: statement}, nil
}

// Vinculo es la billetera ya probada.
type Vinculo struct {
	Address    string    `json:"address"`
	ChainID    uint64    `json:"chainId"`
	VerifiedAt time.Time `json:"verifiedAt"`
}

// Vincular verifica la firma y guarda la dirección.
//
// El orden importa y es el que sigue:
//
//  1. el texto tiene la forma que este servidor emite;
//  2. dice nuestro dominio, nuestra URI y nuestra cadena, y no venció;
//  3. el nonce es uno que emitimos, y se quema en el intento;
//  4. recién ahí, la firma.
//
// Quemar el nonce antes de verificar la firma es deliberado: un intento con
// firma mal armada también gasta el desafío, así que nadie puede probar firmas
// contra el mismo nonce. Cuesta que un error de tipeo obligue a pedir otro
// desafío, y es barato.
func (s *Service) Vincular(ctx context.Context, userID pgtype.UUID, texto, firmaHex string) (Vinculo, error) {
	if s.cfg == nil {
		return Vinculo{}, ErrSinCadena
	}

	m, err := ParseMensaje(texto)
	if err != nil {
		return Vinculo{}, err
	}
	if err := m.Coincide(s.cfg.Dominio, s.cfg.URI, s.cfg.ChainID, s.ahora()); err != nil {
		return Vinculo{}, err
	}

	if _, err := s.q.ConsumeWalletChallenge(ctx, store.ConsumeWalletChallengeParams{
		UserID: userID, Nonce: m.Nonce,
	}); errors.Is(err, pgx.ErrNoRows) {
		return Vinculo{}, ErrDesafioInvalido
	} else if err != nil {
		return Vinculo{}, err
	}

	firma, err := chain.ParseFirmaDeBilletera(firmaHex)
	if err != nil {
		return Vinculo{}, err
	}
	// Se hashea el texto **tal como llegó**, no el que reescribiríamos desde
	// los campos parseados: lo que se firmó son esos bytes y ningún otro.
	recuperada, err := chain.Recuperar(chain.HashPersonal([]byte(texto)), firma)
	if err != nil {
		return Vinculo{}, fmt.Errorf("%w: %v", ErrFirmaAjena, err)
	}
	if recuperada != m.Direccion {
		return Vinculo{}, fmt.Errorf("%w: firmó %s y el mensaje dice %s",
			ErrFirmaAjena, recuperada.Hex(), m.Direccion.Hex())
	}

	w, err := s.q.LinkWallet(ctx, store.LinkWalletParams{
		UserID: userID,
		// Siempre en minúsculas: es lo que hace que el UNIQUE signifique algo.
		Address: recuperada.Hex(),
		ChainID: int32(s.cfg.ChainID), //nolint:gosec // un chain id no pasa de 2^31 en ninguna red real
	})
	if esViolacionDeUnico(err) {
		return Vinculo{}, ErrBilleteraDeOtro
	}
	if err != nil {
		return Vinculo{}, err
	}
	return Vinculo{Address: w.Address, ChainID: uint64(w.ChainID), VerifiedAt: w.VerifiedAt.Time}, nil
}

// Desvincular suelta la billetera de esta cuenta. Devuelve si había alguna.
//
// De este lado y nada más: en la cadena no hay nada que borrar, y eso no es un
// detalle de implementación sino la promesa del token. Quien llame a esto tiene
// que decirlo con todas las letras.
func (s *Service) Desvincular(ctx context.Context, userID pgtype.UUID) (bool, error) {
	if s.cfg == nil {
		return false, ErrSinCadena
	}
	n, err := s.q.UnlinkWallet(ctx, userID)
	return n > 0, err
}

func esViolacionDeUnico(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// ── El voucher ────────────────────────────────────────────────────────────

// VoucherFirmado es el permiso que el contrato va a verificar.
type VoucherFirmado struct {
	To       string `json:"to"`
	Pieza    uint16 `json:"pieza"`
	Deadline uint64 `json:"deadline"`
	Firma    string `json:"signature"`
	Contrato string `json:"contract"`
	ChainID  uint64 `json:"chainId"`
	// TokenID se calcula acá porque el contrato lo calcula igual y es
	// determinista: (to << 16) | pieza. Sirve para que la pantalla pueda
	// mostrar el token antes de que exista.
	TokenID string `json:"tokenId"`
}

// Voucher firma el permiso para acuñar una distinción.
//
// Las dos condiciones son las del §8 y no se negocian: la fila tiene que estar
// en `achievements` —que se escribe en la misma transacción que el progreso que
// la causó— y la billetera tiene que estar probada. El `to` del voucher es la
// dirección verificada, no una que venga en el pedido: eso es lo que hace que
// el front-running no importe y que nadie pueda reclamar con la billetera de
// otro.
func (s *Service) Voucher(ctx context.Context, userID pgtype.UUID, code achievement.Code) (VoucherFirmado, error) {
	if s.cfg == nil {
		return VoucherFirmado{}, ErrSinCadena
	}

	pieza, ok := s.num.Pieza(code)
	if !ok {
		return VoucherFirmado{}, fmt.Errorf("%w: %q", ErrDistincionDesconocida, code)
	}

	w, err := s.billetera(ctx, userID)
	if err != nil {
		return VoucherFirmado{}, err
	}
	if err := s.exigirGanada(ctx, userID, code); err != nil {
		return VoucherFirmado{}, err
	}

	to, err := chain.ParseDireccion(w.Address)
	if err != nil {
		return VoucherFirmado{}, fmt.Errorf("la dirección guardada no se puede leer: %w", err)
	}

	deadline := uint64(s.ahora().Add(VidaDelVoucher).Unix()) //nolint:gosec // una fecha futura no es negativa
	v := chain.Voucher{To: to, Pieza: pieza, Deadline: deadline}
	firma, err := chain.Firmar(s.cfg.Firmante, chain.Digest(s.dominio(), v))
	if err != nil {
		return VoucherFirmado{}, fmt.Errorf("firmar el voucher: %w", err)
	}

	return VoucherFirmado{
		To:       to.Hex(),
		Pieza:    pieza,
		Deadline: deadline,
		Firma:    firma.Hex(),
		Contrato: s.cfg.Contrato.Hex(),
		ChainID:  s.cfg.ChainID,
		TokenID:  TokenID(to, pieza).String(),
	}, nil
}

func (s *Service) dominio() chain.Dominio {
	return chain.DominioDistinciones(s.cfg.ChainID, s.cfg.Contrato)
}

// TokenID reproduce el cálculo del contrato: (uint256(uint160(to)) << 16) | pieza.
//
// Que el id se calcule y no se cuente es lo que hace que no haga falta ni un
// contador ni una tabla de "ya reclamada": reclamar dos veces choca con un
// token que ya tiene dueño (SDD §5).
func TokenID(to chain.Direccion, pieza uint16) *big.Int {
	id := new(big.Int).SetBytes(to[:])
	id.Lsh(id, 16)
	return id.Or(id, big.NewInt(int64(pieza)))
}

func (s *Service) billetera(ctx context.Context, userID pgtype.UUID) (store.Wallet, error) {
	w, err := s.q.GetWallet(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Wallet{}, ErrSinBilletera
	}
	return w, err
}

func (s *Service) exigirGanada(ctx context.Context, userID pgtype.UUID, code achievement.Code) error {
	_, err := s.q.GetAchievementEarned(ctx, store.GetAchievementEarnedParams{
		UserID: userID, Code: string(code),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%w: %s", ErrNoGanada, code)
	}
	return err
}

// ── Confirmar lo que pasó en la cadena ────────────────────────────────────

// temaTransfer es keccak256("Transfer(address,address,uint256)"), el topic 0
// del evento de ERC-721. El id del token viaja en el topic 3 porque los tres
// parámetros del evento están indexados: el `data` del log viene vacío y no hay
// nada que decodificar, sólo una palabra de 32 bytes que leer.
var temaTransfer = temaDe("Transfer(address,address,uint256)")

func temaDe(firma string) string {
	h := chain.Keccak256([]byte(firma))
	return "0x" + hex.EncodeToString(h[:])
}

// Confirmar anota el txHash y le pregunta al nodo cómo salió.
//
// El orden es lo que importa acá y sale derecho del §8: **primero se guarda el
// hash**, después se pregunta. Si el RPC no contesta —porque el nodo se cayó,
// porque tardó, o simplemente porque el bloque todavía no salió— el minteo
// queda en `pending` y esto devuelve el estado sin error. Lo que nunca puede
// pasar es perder el hash de una transacción que la persona ya pagó por un nodo
// que no estaba.
func (s *Service) Confirmar(ctx context.Context, userID pgtype.UUID, code achievement.Code, txHash string) (MintView, error) {
	if s.cfg == nil {
		return MintView{}, ErrSinCadena
	}
	if _, ok := s.num.Pieza(code); !ok {
		return MintView{}, fmt.Errorf("%w: %q", ErrDistincionDesconocida, code)
	}

	hash, err := chain.ParseHashTx(txHash)
	if err != nil {
		return MintView{}, err
	}
	w, err := s.billetera(ctx, userID)
	if err != nil {
		return MintView{}, err
	}
	if err := s.exigirGanada(ctx, userID, code); err != nil {
		return MintView{}, err
	}

	m, err := s.q.RecordMint(ctx, store.RecordMintParams{
		UserID: userID, Code: string(code), Address: w.Address,
		ChainID: int32(s.cfg.ChainID), //nolint:gosec // ver arriba
		TxHash:  pgtype.Text{String: hash, Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// El WHERE de la consulta frenó la actualización: ya estaba confirmado.
		// No hay nada que hacer y tampoco es un error.
		return s.mintGuardado(ctx, userID, code)
	}
	if err != nil {
		return MintView{}, err
	}

	rec, err := s.rpc.Recibo(ctx, hash)
	if err != nil {
		// Se devuelve el pending, no el error: el dato ya está a salvo y la
		// confirmación se reintenta la próxima vez que la persona entre.
		return vistaDeMint(m), nil //nolint:nilerr // ver el comentario de arriba
	}

	estado, tokenID := "failed", ""
	if rec.Exitoso {
		estado = "confirmed"
		tokenID = idDelRecibo(rec, s.cfg.Contrato)
	}

	var num pgtype.Numeric
	if tokenID != "" {
		if err := num.Scan(tokenID); err != nil {
			return MintView{}, fmt.Errorf("el id del token no entra en numeric: %w", err)
		}
	}
	guardado, err := s.q.SettleMint(ctx, store.SettleMintParams{
		UserID: userID, Code: string(code), Status: estado, TokenID: num,
	})
	if err != nil {
		return MintView{}, err
	}
	return vistaDeMint(guardado), nil
}

// idDelRecibo saca el id del token del evento Transfer que emitió **nuestro**
// contrato.
//
// Se filtra por dirección porque una transacción puede tocar varios contratos y
// cualquiera de ellos puede emitir un Transfer: quedarse con el primero que
// aparezca sería creerle a un contrato que no elegimos. Si no está el evento,
// devuelve vacío: el minteo se marca confirmado igual —la transacción salió
// bien— y el id queda en null, que es más honesto que inventarlo.
func idDelRecibo(rec chain.Recibo, contrato chain.Direccion) string {
	for _, l := range rec.Logs {
		dir, err := chain.ParseDireccion(l.Address)
		if err != nil || dir != contrato {
			continue
		}
		if len(l.Topics) != 4 || !strings.EqualFold(l.Topics[0], temaTransfer) {
			continue
		}
		id, err := chain.ParseUint256(l.Topics[3])
		if err != nil {
			continue
		}
		return id.String()
	}
	return ""
}

func (s *Service) mintGuardado(ctx context.Context, userID pgtype.UUID, code achievement.Code) (MintView, error) {
	m, err := s.q.GetMint(ctx, store.GetMintParams{UserID: userID, Code: string(code)})
	if err != nil {
		return MintView{}, err
	}
	return vistaDeMint(m), nil
}
