package achievement

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/content"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/placement"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/store"
)

// Grant evalúa el progreso y guarda lo que corresponde.
//
// Pide el *store.Queries de una transacción en curso a propósito: la distinción
// se guarda junto con el progreso que la causó, o no se guarda ninguna de las
// dos cosas. Sin logros nuevos no hay consulta.
func Grant(
	ctx context.Context, q *store.Queries, userID pgtype.UUID,
	c *Catalog, mastery map[string]placement.State,
) error {
	earned := Earned(c, mastery)
	if len(earned) == 0 {
		return nil
	}
	if err := q.GrantAchievements(ctx, store.GrantAchievementsParams{
		UserID: userID, Codes: Codes(earned),
	}); err != nil {
		return fmt.Errorf("guardar distinciones: %w", err)
	}
	return nil
}

type Service struct {
	pool    *pgxpool.Pool
	catalog *Catalog
}

func NewService(pool *pgxpool.Pool, c *content.Catalog) *Service {
	return &Service{pool: pool, catalog: NewCatalog(c)}
}

// View es una distinción como la ve la app. Se devuelven siempre todas, ganadas
// o no: la pantalla muestra el camino entero, no sólo lo hecho.
type View struct {
	Code     Code       `json:"code"`
	Kind     Kind       `json:"kind"`
	NameEn   string     `json:"nameEn"`
	NameEs   string     `json:"nameEs"`
	Obra     int        `json:"obra"`
	ObraName string     `json:"obraName"`
	Earned   bool       `json:"earned"`
	EarnedAt *time.Time `json:"earnedAt"`
}

// List devuelve las distinciones en orden de currículum, marcando cuáles están
// ganadas.
//
// Lo ganado sale de la tabla, no de recalcular el progreso: si un tema se oxidó
// después, la distinción sigue estando y con su fecha original.
func (s *Service) List(ctx context.Context, userID pgtype.UUID) ([]View, error) {
	rows, err := store.New(s.pool).ListAchievements(ctx, userID)
	if err != nil {
		return nil, err
	}
	when := make(map[Code]time.Time, len(rows))
	for _, r := range rows {
		when[Code(r.Code)] = r.EarnedAt.Time
	}

	out := make([]View, 0, s.catalog.Len())
	for _, d := range s.catalog.All() {
		v := View{
			Code: d.Code, Kind: d.Kind, NameEn: d.NameEn, NameEs: d.NameEs,
			Obra: d.Obra, ObraName: d.ObraName,
		}
		if t, ok := when[d.Code]; ok {
			at := t
			v.Earned, v.EarnedAt = true, &at
		}
		out = append(out, v)
	}
	return out, nil
}
