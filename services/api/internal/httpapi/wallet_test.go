package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/achievement"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/auth"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/wallet"
)

// fakeWallet devuelve el error que se le configure: acá se prueba el mapeo a
// HTTP, no la criptografía, que ya tiene sus tests en internal/wallet.
type fakeWallet struct {
	configurada bool
	err         error
	vistos      []string
}

func (f *fakeWallet) Configured() bool { return f.configurada }

func (f *fakeWallet) anotar(que string) error {
	f.vistos = append(f.vistos, que)
	return f.err
}

func (f *fakeWallet) Estado(context.Context, pgtype.UUID) (wallet.Estado, error) {
	return wallet.Estado{ChainID: 84532, Mints: []wallet.MintView{}}, f.anotar("estado")
}

func (f *fakeWallet) Desafio(context.Context, pgtype.UUID) (wallet.Desafio, error) {
	return wallet.Desafio{Message: "…", Nonce: "c0ffee", ExpiresAt: time.Now()}, f.anotar("desafio")
}

func (f *fakeWallet) Vincular(_ context.Context, _ pgtype.UUID, mensaje, firma string) (wallet.Vinculo, error) {
	return wallet.Vinculo{Address: "0x" + strings.Repeat("ab", 20)}, f.anotar("vincular:" + mensaje + "|" + firma)
}

func (f *fakeWallet) Desvincular(context.Context, pgtype.UUID) (bool, error) {
	return true, f.anotar("desvincular")
}

func (f *fakeWallet) Voucher(_ context.Context, _ pgtype.UUID, code achievement.Code) (wallet.VoucherFirmado, error) {
	return wallet.VoucherFirmado{Pieza: 38}, f.anotar("voucher:" + string(code))
}

func (f *fakeWallet) Confirmar(_ context.Context, _ pgtype.UUID, code achievement.Code, tx string) (wallet.MintView, error) {
	return wallet.MintView{Code: string(code), Status: "pending", TxHash: tx}, f.anotar("mint:" + string(code) + "|" + tx)
}

func newWalletRouter(w Wallet) http.Handler {
	return NewRouter(Deps{
		Users:       newFakeStore(),
		Wallet:      w,
		Placement:   fakePlacement{},
		Catalog:     testCatalog(),
		DB:          fakePinger{},
		Verifier:    auth.DevVerifier{},
		CORSOrigins: []string{"http://localhost:5173"},
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

// Cada ruta de la cadena, una vez, con su método. Lo que se mira es que exista
// y que le pase lo que tiene que pasarle al servicio.
func TestRutasDeLaCadena(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		quiero string
	}{
		{"estado", http.MethodGet, "/v1/wallet", "", "estado"},
		{"desafío", http.MethodGet, "/v1/wallet/challenge", "", "desafio"},
		{"vincular", http.MethodPost, "/v1/wallet", `{"message":"m","signature":"s"}`, "vincular:m|s"},
		{"desvincular", http.MethodDelete, "/v1/wallet", "", "desvincular"},
		{"voucher", http.MethodPost, "/v1/achievements/pieza:verb.make_do/voucher", "", "voucher:pieza:verb.make_do"},
		{"mint", http.MethodPost, "/v1/achievements/obra:1/mint", `{"txHash":"0xabc"}`, "mint:obra:1|0xabc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeWallet{configurada: true}
			rec := do(t, newWalletRouter(fake), tt.method, tt.path, tt.body, true)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d: %s", rec.Code, rec.Body)
			}
			if len(fake.vistos) != 1 || fake.vistos[0] != tt.quiero {
				t.Fatalf("el servicio vio %v, want [%s]", fake.vistos, tt.quiero)
			}
		})
	}
}

// Sin configuración de cadena, todo contesta 503. Es la misma decisión que la
// práctica oral sin transcriptor: la funcionalidad no existe, no se rompió.
func TestSinCadenaTodoDa503(t *testing.T) {
	rutas := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/v1/wallet"},
		{http.MethodGet, "/v1/wallet/challenge"},
		{http.MethodPost, "/v1/wallet"},
		{http.MethodDelete, "/v1/wallet"},
		{http.MethodPost, "/v1/achievements/obra:1/voucher"},
		{http.MethodPost, "/v1/achievements/obra:1/mint"},
	}

	// Dos formas de no tener cadena: el servicio armado y sin configurar, y el
	// servicio que ni siquiera se armó. Desde afuera tienen que verse iguales.
	servicios := map[string]Wallet{
		"el servicio dice que no está configurado": &fakeWallet{err: wallet.ErrSinCadena},
		"ni siquiera hay servicio":                 nil,
	}
	for nombre, svc := range servicios {
		t.Run(nombre, func(t *testing.T) {
			h := newWalletRouter(svc)
			for _, r := range rutas {
				body := `{"message":"m","signature":"s","txHash":"0xabc"}`
				rec := do(t, h, r.method, r.path, body, true)
				if rec.Code != http.StatusServiceUnavailable {
					t.Fatalf("%s %s: status = %d, want 503 (%s)", r.method, r.path, rec.Code, rec.Body)
				}
			}
		})
	}
}

// Y /healthz lo dice, igual que dice `ai` y `speaking`: es de ahí que la web
// sabe que no tiene que dibujar nada de la cadena.
func TestHealthzDiceSiHayCadena(t *testing.T) {
	tests := []struct {
		name string
		svc  Wallet
		want bool
	}{
		{"con cadena", &fakeWallet{configurada: true}, true},
		{"sin configurar", &fakeWallet{}, false},
		{"sin servicio", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, newWalletRouter(tt.svc), http.MethodGet, "/healthz", "", false)
			var body struct {
				Chain bool `json:"chain"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Chain != tt.want {
				t.Fatalf("chain = %v, want %v", body.Chain, tt.want)
			}
		})
	}
}

// El mapeo de los errores del dominio a códigos HTTP. Importa que "no ganaste
// eso" no sea un 500: es una respuesta, no una falla.
func TestErroresDeLaCadenaAHTTP(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"sin cadena", wallet.ErrSinCadena, http.StatusServiceUnavailable},
		{"sin billetera", wallet.ErrSinBilletera, http.StatusConflict},
		{"el logro no está ganado", wallet.ErrNoGanada, http.StatusForbidden},
		{"la distinción no existe", wallet.ErrDistincionDesconocida, http.StatusNotFound},
		{"la billetera es de otro", wallet.ErrBilleteraDeOtro, http.StatusConflict},
		{"el desafío no vale", wallet.ErrDesafioInvalido, http.StatusBadRequest},
		{"la firma es de otra dirección", wallet.ErrFirmaAjena, http.StatusBadRequest},
		{"el mensaje es de otro dominio", wallet.ErrDominioAjeno, http.StatusBadRequest},
		{"el mensaje venció", wallet.ErrMensajeVencido, http.StatusBadRequest},
		{"cualquier otra cosa", errors.New("boom"), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeWallet{configurada: true, err: tt.err}
			rec := do(t, newWalletRouter(fake), http.MethodPost,
				"/v1/achievements/obra:1/voucher", "", true)
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tt.want, rec.Body)
			}
		})
	}
}

// Desvincular tiene que decir que en la cadena no borra nada. Está en el §8 y
// es lo único honesto que se puede decir.
func TestDesvincularAvisaQueLaCadenaNoSeBorra(t *testing.T) {
	fake := &fakeWallet{configurada: true}
	rec := do(t, newWalletRouter(fake), http.MethodDelete, "/v1/wallet", "", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body struct {
		Unlinked bool   `json:"unlinked"`
		Note     string `json:"note"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Unlinked {
		t.Fatal("había una billetera y tenía que soltarla")
	}
	for _, frase := range []string{"Nothing was removed from the blockchain", "cannot be deleted"} {
		if !strings.Contains(body.Note, frase) {
			t.Fatalf("el aviso no dice %q: %q", frase, body.Note)
		}
	}
}

func TestLaCadenaNecesitaAuth(t *testing.T) {
	fake := &fakeWallet{configurada: true}
	h := newWalletRouter(fake)
	for _, path := range []string{"/v1/wallet", "/v1/wallet/challenge", "/v1/achievements/obra:1/voucher"} {
		if rec := do(t, h, http.MethodGet, path, "", false); rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s: status = %d, want 401", path, rec.Code)
		}
	}
	if len(fake.vistos) != 0 {
		t.Fatalf("sin token no tendría que haber llegado nada al servicio: %v", fake.vistos)
	}
}

func TestVincularExigeMensajeYFirma(t *testing.T) {
	tests := []struct{ name, body string }{
		{"body que no es JSON", "{"},
		{"sin firma", `{"message":"m"}`},
		{"sin mensaje", `{"signature":"s"}`},
		{"vacío", `{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeWallet{configurada: true}
			rec := do(t, newWalletRouter(fake), http.MethodPost, "/v1/wallet", tt.body, true)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			if len(fake.vistos) != 0 {
				t.Fatal("no tendría que haber llamado al servicio")
			}
		})
	}
}
