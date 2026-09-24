package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/budget"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/speaking"
)

// maxAudio: 30 segundos de voz pesan menos de 1 MB; 10 da margen de sobra.
const maxAudio = 10 << 20

func (h handlers) speakingNext(w http.ResponseWriter, r *http.Request) {
	u, err := h.currentUser(r)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	next, err := h.deps.Speaking.Next(r.Context(), u.ID)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	// Sin drill no es error: no queda nada por hoy.
	writeJSON(w, http.StatusOK, map[string]any{"next": next, "enabled": h.deps.Speaking.Configured()})
}

func (h handlers) speakingAnswer(w http.ResponseWriter, r *http.Request) {
	u, err := h.currentUser(r)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAudio)
	if err := r.ParseMultipartForm(maxAudio); err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "el audio es demasiado largo")
		return
	}
	file, header, err := r.FormFile("audio")
	if err != nil {
		writeError(w, http.StatusBadRequest, "falta el audio")
		return
	}
	defer file.Close()

	seconds, _ := strconv.Atoi(r.FormValue("seconds"))
	res, err := h.deps.Speaking.Answer(r.Context(), u.ID, chi.URLParam(r, "drill"), file, header.Filename, seconds)
	switch {
	case errors.Is(err, speaking.ErrNoDrill):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, speaking.ErrNotConfigured):
		writeError(w, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, speaking.ErrEmptyAudio):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, budget.ErrUserLimit), errors.Is(err, budget.ErrGlobalLimit):
		writeError(w, http.StatusTooManyRequests, err.Error())
	case err != nil:
		h.internalError(w, r, err)
	default:
		writeJSON(w, http.StatusOK, res)
	}
}
