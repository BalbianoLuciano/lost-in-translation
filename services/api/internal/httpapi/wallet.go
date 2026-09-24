package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/achievement"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/chain"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/wallet"
)

// Wallet es la billetera y su recibo en la cadena (wallet.Service).
//
// Va separado de Achievements a propósito, y no es prolijidad: las distinciones
// existen y se ven sin cadena ninguna, y quien implemente Achievements —el fake
// de un test, sin ir más lejos— no tiene por qué saber qué es un voucher.
type Wallet interface {
	Configured() bool
	Estado(ctx context.Context, userID pgtype.UUID) (wallet.Estado, error)
	Desafio(ctx context.Context, userID pgtype.UUID) (wallet.Desafio, error)
	Vincular(ctx context.Context, userID pgtype.UUID, mensaje, firma string) (wallet.Vinculo, error)
	Desvincular(ctx context.Context, userID pgtype.UUID) (bool, error)
	Voucher(ctx context.Context, userID pgtype.UUID, code achievement.Code) (wallet.VoucherFirmado, error)
	Confirmar(ctx context.Context, userID pgtype.UUID, code achievement.Code, txHash string) (wallet.MintView, error)
}

// maxCuerpoWallet: un mensaje SIWE con su firma no llega a 2 KB. 16 da margen y
// evita que alguien mande un megabyte para que lo parseemos.
const maxCuerpoWallet = 1 << 14

// walletError traduce los errores del paquete a códigos HTTP.
//
// El 503 de ErrSinCadena es el que importa, y es el mismo que da la práctica
// oral sin transcriptor: "esto no existe acá", no "esto se rompió". La web lo
// sabe de antemano por /healthz y ni siquiera dibuja el botón; el 503 está para
// el que llama igual.
func (h handlers) walletError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, wallet.ErrSinCadena):
		writeError(w, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, wallet.ErrSinBilletera):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, wallet.ErrNoGanada):
		// 403 y no 404: la distinción existe, lo que falta es haberla ganado.
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, wallet.ErrDistincionDesconocida):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, wallet.ErrBilleteraDeOtro):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, wallet.ErrDesafioInvalido),
		errors.Is(err, wallet.ErrFirmaAjena),
		errors.Is(err, wallet.ErrMensajeInvalido),
		errors.Is(err, wallet.ErrDominioAjeno),
		errors.Is(err, wallet.ErrCadenaAjena),
		errors.Is(err, wallet.ErrMensajeVencido),
		errors.Is(err, chain.ErrFirmaInvalida),
		errors.Is(err, chain.ErrFirmaMaleable),
		errors.Is(err, chain.ErrHashInvalido):
		// Todo lo que sea "la prueba no cierra" es 400: el pedido está mal
		// armado, no hay nada del otro lado que arreglar.
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		h.internalError(w, r, err)
	}
}

// conCadena contesta 503 cuando no hay servicio de billetera armado.
//
// No es lo mismo que "la cadena no está configurada" —eso lo dice el propio
// servicio— sino el caso de un despliegue que ni siquiera lo construyó. Desde
// afuera se ven iguales y tienen que verse iguales: la funcionalidad no existe.
func (h handlers) conCadena(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.deps.Wallet == nil {
			writeError(w, http.StatusServiceUnavailable, wallet.ErrSinCadena.Error())
			return
		}
		next(w, r)
	}
}

// walletEstado: la billetera vinculada, si hay, más lo que se acuñó.
func (h handlers) walletEstado(w http.ResponseWriter, r *http.Request) {
	u, err := h.currentUser(r)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	estado, err := h.deps.Wallet.Estado(r.Context(), u.ID)
	if err != nil {
		h.walletError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, estado)
}

// walletChallenge emite el mensaje SIWE que hay que firmar.
func (h handlers) walletChallenge(w http.ResponseWriter, r *http.Request) {
	u, err := h.currentUser(r)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	d, err := h.deps.Wallet.Desafio(r.Context(), u.ID)
	if err != nil {
		h.walletError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

type vincularRequest struct {
	Message   string `json:"message"`
	Signature string `json:"signature"`
}

// walletLink verifica la firma y guarda la dirección.
func (h handlers) walletLink(w http.ResponseWriter, r *http.Request) {
	u, err := h.currentUser(r)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	var req vincularRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxCuerpoWallet)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "body inválido")
		return
	}
	if req.Message == "" || req.Signature == "" {
		writeError(w, http.StatusBadRequest, "faltan message y signature")
		return
	}
	v, err := h.deps.Wallet.Vincular(r.Context(), u.ID, req.Message, req.Signature)
	if err != nil {
		h.walletError(w, r, err)
		return
	}
	h.deps.Logger.InfoContext(r.Context(), "billetera vinculada",
		"user_id", u.ID.String(), "address", v.Address)
	writeJSON(w, http.StatusOK, v)
}

// walletUnlink suelta la billetera.
//
// Contesta 200 con un aviso y no 204 justamente por el aviso: acá desvincular
// significa una cosa mucho más chica de lo que parece, y el §8 dice que ese
// choque se cuenta de frente y no se esconde.
func (h handlers) walletUnlink(w http.ResponseWriter, r *http.Request) {
	u, err := h.currentUser(r)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	habia, err := h.deps.Wallet.Desvincular(r.Context(), u.ID)
	if err != nil {
		h.walletError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"unlinked": habia,
		"note": "Your wallet is no longer linked to this account. " +
			"Nothing was removed from the blockchain: any distinction you already minted still exists, " +
			"still belongs to that address, and cannot be deleted by anyone, including us.",
	})
}
