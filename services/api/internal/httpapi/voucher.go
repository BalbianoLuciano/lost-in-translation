package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/achievement"
)

// Reclamar una distinción son dos pedidos, y están separados porque entre uno y
// otro pasa algo que el servidor no controla: la persona firma y paga una
// transacción.
//
//	POST /v1/achievements/{code}/voucher  → el permiso firmado
//	   … la billetera manda mint(...) a la cadena …
//	POST /v1/achievements/{code}/mint     → el txHash, para confirmarlo
//
// El servidor nunca manda la transacción. No tiene con qué: su clave firma y no
// gasta (SDD §4). Eso es lo que hace que un backend comprometido pueda
// falsificar distinciones —malo— y no robar plata —peor—.

// voucher firma el permiso para acuñar una distinción ganada.
func (h handlers) voucher(w http.ResponseWriter, r *http.Request) {
	u, err := h.currentUser(r)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	code := achievement.Code(chi.URLParam(r, "code"))
	v, err := h.deps.Wallet.Voucher(r.Context(), u.ID, code)
	if err != nil {
		h.walletError(w, r, err)
		return
	}
	h.deps.Logger.InfoContext(r.Context(), "voucher firmado",
		"user_id", u.ID.String(), "code", string(code), "pieza", v.Pieza)
	writeJSON(w, http.StatusOK, v)
}

type mintRequest struct {
	TxHash string `json:"txHash"`
}

// mint anota la transacción y le pregunta al nodo cómo salió.
//
// Nunca devuelve error porque el RPC no contestó: el hash ya quedó guardado y
// el estado vuelve en `pending`. Perder el hash de una transacción que la
// persona ya pagó, por un nodo caído, sería el único error grave posible acá.
func (h handlers) mint(w http.ResponseWriter, r *http.Request) {
	u, err := h.currentUser(r)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	var req mintRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxCuerpoWallet)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "body inválido")
		return
	}
	code := achievement.Code(chi.URLParam(r, "code"))
	m, err := h.deps.Wallet.Confirmar(r.Context(), u.ID, code, req.TxHash)
	if err != nil {
		h.walletError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}
