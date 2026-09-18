package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/content"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/session"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/tutor"
)

func (h handlers) sessionToday(w http.ResponseWriter, r *http.Request) {
	u, err := h.currentUser(r)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	st, err := h.deps.Session.Today(r.Context(), u.ID)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func parseBlock(s string) (session.Block, bool) {
	switch session.Block(s) {
	case session.Review:
		return session.Review, true
	case session.Practice:
		return session.Practice, true
	}
	return "", false
}

func (h handlers) sessionNext(w http.ResponseWriter, r *http.Request) {
	u, err := h.currentUser(r)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	block, ok := parseBlock(r.URL.Query().Get("block"))
	if !ok {
		writeError(w, http.StatusBadRequest, "block debe ser review o practice")
		return
	}
	next, err := h.deps.Session.Next(r.Context(), u.ID, block)
	if err != nil {
		h.sessionError(w, r, err)
		return
	}
	// Sin ítem no es un error: el bloque terminó por hoy.
	writeJSON(w, http.StatusOK, map[string]any{"next": next})
}

type sessionAnswerRequest struct {
	Block     string           `json:"block"`
	ItemID    string           `json:"itemId"`
	Response  content.Response `json:"response"`
	LatencyMs int              `json:"latencyMs"`
	Consulted bool             `json:"consulted"`
}

func (h handlers) sessionAnswer(w http.ResponseWriter, r *http.Request) {
	u, err := h.currentUser(r)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	var req sessionAnswerRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil || req.ItemID == "" {
		writeError(w, http.StatusBadRequest, "body inválido")
		return
	}
	block, ok := parseBlock(req.Block)
	if !ok {
		writeError(w, http.StatusBadRequest, "block debe ser review o practice")
		return
	}
	res, err := h.deps.Session.Answer(r.Context(), u.ID, block, req.ItemID, req.Response, req.LatencyMs, req.Consulted)
	if err != nil {
		h.sessionError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h handlers) lesson(w http.ResponseWriter, r *http.Request) {
	u, err := h.currentUser(r)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	view, err := h.deps.Session.Lesson(r.Context(), u.ID, chi.URLParam(r, "skill"))
	if err != nil {
		h.sessionError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (h handlers) completeLesson(w http.ResponseWriter, r *http.Request) {
	u, err := h.currentUser(r)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	st, err := h.deps.Session.CompleteLesson(r.Context(), u.ID, chi.URLParam(r, "skill"))
	if err != nil {
		h.sessionError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (h handlers) sessionError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, session.ErrNoLesson):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, session.ErrUnexpectedItem), errors.Is(err, session.ErrLessonNotReady):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, content.ErrBadResponse):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		h.internalError(w, r, err)
	}
}

// ── El profesor de IA ─────────────────────────────────────────────────────

type askRequest struct {
	Question string `json:"question"`
	// ItemID: el ejercicio que está en pantalla, para que responda en contexto.
	ItemID string `json:"itemId"`
}

func (h handlers) ask(w http.ResponseWriter, r *http.Request) {
	u, err := h.currentUser(r)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	var req askRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "body inválido")
		return
	}
	res, err := h.deps.Tutor.Ask(r.Context(), u.ID, req.Question, req.ItemID)
	switch {
	case errors.Is(err, tutor.ErrNotConfigured):
		writeError(w, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, tutor.ErrEmpty), errors.Is(err, tutor.ErrTooLong):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, tutor.ErrDailyLimit):
		writeError(w, http.StatusTooManyRequests, err.Error())
	case err != nil:
		h.internalError(w, r, err)
	default:
		writeJSON(w, http.StatusOK, res)
	}
}
