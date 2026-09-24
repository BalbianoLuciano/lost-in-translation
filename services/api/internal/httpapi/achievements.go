package httpapi

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/achievement"
)

// Achievements son las distinciones (achievement.Service).
type Achievements interface {
	List(ctx context.Context, userID pgtype.UUID) ([]achievement.View, error)
}

// achievements devuelve las distinciones posibles, todas, con cuáles están
// ganadas y desde cuándo.
//
// La lista completa se manda siempre: lo que falta es parte de la pantalla, y
// el cliente no tiene que saber armarla.
func (h handlers) achievements(w http.ResponseWriter, r *http.Request) {
	u, err := h.currentUser(r)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	list, err := h.deps.Achievements.List(r.Context(), u.ID)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	earned := 0
	for _, d := range list {
		if d.Earned {
			earned++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"achievements": list,
		"earned":       earned,
		"total":        len(list),
	})
}
