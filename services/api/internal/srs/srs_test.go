package srs

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/store"
)

func toCard(p store.UpsertCardParams) store.Card {
	return store.Card{
		UserID: p.UserID, ItemID: p.ItemID, Due: p.Due, Stability: p.Stability, Difficulty: p.Difficulty,
		ElapsedDays: p.ElapsedDays, ScheduledDays: p.ScheduledDays, Reps: p.Reps, Lapses: p.Lapses,
		State: p.State, LastReview: p.LastReview,
	}
}

func TestCorrectAnswersPushDueFurther(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	var user pgtype.UUID

	first := Review(user, "x", nil, true, now)
	if first.Reps != 1 || !first.Due.Time.After(now) {
		t.Fatalf("primera revisión inesperada: %+v", first)
	}

	c := toCard(first)
	second := Review(user, "x", &c, true, first.Due.Time)
	gap1 := first.Due.Time.Sub(now)
	gap2 := second.Due.Time.Sub(first.Due.Time)
	if gap2 <= gap1 {
		t.Fatalf("acertar otra vez debía espaciar más: %v → %v", gap1, gap2)
	}
}

func TestWrongAnswerComesBackSooner(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	var user pgtype.UUID
	good := Review(user, "x", nil, true, now)
	again := Review(user, "x", nil, false, now)
	if !again.Due.Time.Before(good.Due.Time) {
		t.Fatalf("errar debía volver antes: again=%v good=%v", again.Due.Time, good.Due.Time)
	}
}
