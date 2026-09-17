// Package srs adapta FSRS (repaso espaciado) a las tarjetas guardadas en la base.
package srs

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	fsrs "github.com/open-spaced-repetition/go-fsrs/v3"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/store"
)

var scheduler = fsrs.NewFSRS(fsrs.DefaultParam())

// Review aplica una respuesta a la tarjeta (nil si es la primera vez) y
// devuelve los parámetros para guardarla.
//
// Por ahora las respuestas son binarias: correcta → Good, incorrecta → Again.
func Review(userID pgtype.UUID, itemID string, prev *store.Card, correct bool, now time.Time) store.UpsertCardParams {
	card := fsrs.NewCard()
	if prev != nil {
		card = fromStore(*prev)
	}
	rating := fsrs.Again
	if correct {
		rating = fsrs.Good
	}
	next := scheduler.Next(card, now, rating).Card
	return store.UpsertCardParams{
		UserID:        userID,
		ItemID:        itemID,
		Due:           ts(next.Due),
		Stability:     next.Stability,
		Difficulty:    next.Difficulty,
		ElapsedDays:   int32(next.ElapsedDays),
		ScheduledDays: int32(next.ScheduledDays),
		Reps:          int32(next.Reps),
		Lapses:        int32(next.Lapses),
		State:         int16(next.State),
		LastReview:    ts(next.LastReview),
	}
}

func fromStore(c store.Card) fsrs.Card {
	card := fsrs.Card{
		Due:           c.Due.Time,
		Stability:     c.Stability,
		Difficulty:    c.Difficulty,
		ElapsedDays:   uint64(max(c.ElapsedDays, 0)),
		ScheduledDays: uint64(max(c.ScheduledDays, 0)),
		Reps:          uint64(max(c.Reps, 0)),
		Lapses:        uint64(max(c.Lapses, 0)),
		State:         fsrs.State(c.State),
	}
	if c.LastReview.Valid {
		card.LastReview = c.LastReview.Time
	}
	return card
}

func ts(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t, Valid: true}
}
