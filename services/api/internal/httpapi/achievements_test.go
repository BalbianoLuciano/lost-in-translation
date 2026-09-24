package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/achievement"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/auth"
)

type fakeAchievements struct {
	views []achievement.View
	err   error
}

func (f fakeAchievements) List(context.Context, pgtype.UUID) ([]achievement.View, error) {
	return f.views, f.err
}

func newAchievementsRouter(a Achievements) http.Handler {
	return NewRouter(Deps{
		Users:        newFakeStore(),
		Achievements: a,
		Placement:    fakePlacement{},
		Catalog:      testCatalog(),
		DB:           fakePinger{},
		Verifier:     auth.DevVerifier{},
		CORSOrigins:  []string{"http://localhost:5173"},
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

func TestAchievements(t *testing.T) {
	ganada := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	fake := fakeAchievements{views: []achievement.View{
		{Code: "pieza:a.uno", Kind: achievement.Pieza, NameEn: "One", Obra: 1, Earned: true, EarnedAt: &ganada},
		{Code: "pieza:a.dos", Kind: achievement.Pieza, NameEn: "Two", Obra: 1},
		{Code: "obra:1", Kind: achievement.Obra, NameEn: "Verb tenses", Obra: 1},
	}}

	rec := do(t, newAchievementsRouter(fake), http.MethodGet, "/v1/achievements", "", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	var body struct {
		Achievements []achievement.View `json:"achievements"`
		Earned       int                `json:"earned"`
		Total        int                `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Total != 3 || body.Earned != 1 {
		t.Fatalf("earned = %d de %d; want 1 de 3", body.Earned, body.Total)
	}
	// Las que faltan viajan igual: la pantalla muestra el camino entero.
	if len(body.Achievements) != 3 || body.Achievements[1].Earned || body.Achievements[1].EarnedAt != nil {
		t.Fatalf("respuesta inesperada: %+v", body.Achievements)
	}
	if !body.Achievements[0].EarnedAt.Equal(ganada) {
		t.Fatalf("earnedAt = %v, want %v", body.Achievements[0].EarnedAt, ganada)
	}
}

func TestAchievementsNeedsAuth(t *testing.T) {
	rec := do(t, newAchievementsRouter(fakeAchievements{}), http.MethodGet, "/v1/achievements", "", false)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestAchievementsError(t *testing.T) {
	fake := fakeAchievements{err: errors.New("boom")}
	rec := do(t, newAchievementsRouter(fake), http.MethodGet, "/v1/achievements", "", true)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
