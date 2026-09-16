package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/content"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/placement"
)

// ── Mapa de obras con el estado de cada habilidad ─────────────────────────

type mapSkill struct {
	ID      string  `json:"id"`
	NameEn  string  `json:"nameEn"`
	NameEs  string  `json:"nameEs"`
	State   string  `json:"state"`
	Mastery float32 `json:"mastery"`
}

type mapPiece struct {
	ID     string     `json:"id"`
	NameEn string     `json:"nameEn"`
	NameEs string     `json:"nameEs"`
	Skills []mapSkill `json:"skills"`
}

type mapObra struct {
	ID      int        `json:"id"`
	Slug    string     `json:"slug"`
	Name    string     `json:"name"`
	TopicEn string     `json:"topicEn"`
	TopicEs string     `json:"topicEs"`
	Pieces  []mapPiece `json:"pieces"`
}

func (h handlers) skillMap(w http.ResponseWriter, r *http.Request) {
	u, err := h.currentUser(r)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	mastery, err := h.deps.Users.ListSkillMastery(r.Context(), u.ID)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	byID := make(map[string]mapSkill, len(mastery))
	for _, m := range mastery {
		byID[m.SkillID] = mapSkill{State: m.State, Mastery: m.Mastery}
	}

	obras := make([]mapObra, 0, len(h.deps.Catalog.Obras))
	for _, o := range h.deps.Catalog.Obras {
		mo := mapObra{ID: o.ID, Slug: o.Slug, Name: o.Name, TopicEn: o.TopicEn, TopicEs: o.TopicEs, Pieces: []mapPiece{}}
		for _, p := range o.Pieces {
			mp := mapPiece{ID: p.ID, NameEn: p.NameEn, NameEs: p.NameEs}
			for _, sid := range p.Skills {
				sk, _ := h.deps.Catalog.Skill(sid)
				ms := byID[sid]
				if ms.State == "" {
					ms.State = string(placement.Plano)
				}
				ms.ID, ms.NameEn, ms.NameEs = sid, sk.NameEn, sk.NameEs
				mp.Skills = append(mp.Skills, ms)
			}
			mo.Pieces = append(mo.Pieces, mp)
		}
		obras = append(obras, mo)
	}
	writeJSON(w, http.StatusOK, map[string]any{"contentVersion": h.deps.Catalog.Version, "obras": obras})
}

// ── Test de ubicación ─────────────────────────────────────────────────────

func (h handlers) placementOverview(w http.ResponseWriter, r *http.Request) {
	u, err := h.currentUser(r)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	parts, err := h.deps.Placement.Overview(r.Context(), u.ID)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"parts": parts})
}

func (h handlers) placementStart(w http.ResponseWriter, r *http.Request) {
	u, err := h.currentUser(r)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	st, err := h.deps.Placement.Start(r.Context(), u.ID, chi.URLParam(r, "part"))
	if err != nil {
		h.placementError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (h handlers) placementGet(w http.ResponseWriter, r *http.Request) {
	u, err := h.currentUser(r)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	runID, ok := parseUUID(chi.URLParam(r, "run"))
	if !ok {
		writeError(w, http.StatusNotFound, "corrida inexistente")
		return
	}
	st, err := h.deps.Placement.Get(r.Context(), u.ID, runID)
	if err != nil {
		h.placementError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

type answerRequest struct {
	ItemID    string           `json:"itemId"`
	Response  content.Response `json:"response"`
	LatencyMs int              `json:"latencyMs"`
}

func (h handlers) placementAnswer(w http.ResponseWriter, r *http.Request) {
	u, err := h.currentUser(r)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	runID, ok := parseUUID(chi.URLParam(r, "run"))
	if !ok {
		writeError(w, http.StatusNotFound, "corrida inexistente")
		return
	}
	var req answerRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil || req.ItemID == "" {
		writeError(w, http.StatusBadRequest, "body inválido")
		return
	}
	res, err := h.deps.Placement.Answer(r.Context(), u.ID, runID, req.ItemID, req.Response, req.LatencyMs)
	if err != nil {
		h.placementError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h handlers) placementError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, placement.ErrPartNotFound), errors.Is(err, placement.ErrRunNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, placement.ErrRunDone), errors.Is(err, placement.ErrUnexpectedItem):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, content.ErrBadResponse):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		h.internalError(w, r, err)
	}
}

func parseUUID(s string) (pgtype.UUID, bool) {
	var id pgtype.UUID
	if err := id.Scan(s); err != nil {
		return id, false
	}
	return id, true
}
