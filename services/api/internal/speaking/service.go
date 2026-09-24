package speaking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/achievement"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/ai"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/budget"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/content"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/placement"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/session"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/store"
)

var (
	ErrNotConfigured = errors.New("la práctica oral necesita la clave del proveedor de transcripción")
	ErrNoDrill       = errors.New("no existe ese ejercicio oral")
	ErrEmptyAudio    = errors.New("no llegó audio")
)

// Colada de un drill oral: hablar cuesta más que marcar una opción (design.md §3).
const colada = 5

type Service struct {
	pool         *pgxpool.Pool
	catalog      *content.Catalog
	distinciones *achievement.Catalog
	stt          ai.Transcriber
	budget       *budget.Budget
	now          func() time.Time
}

func NewService(pool *pgxpool.Pool, catalog *content.Catalog, stt ai.Transcriber, b *budget.Budget) *Service {
	return &Service{
		pool: pool, catalog: catalog, distinciones: achievement.NewCatalog(catalog),
		stt: stt, budget: b, now: time.Now,
	}
}

// Configured: sin proveedor de transcripción, el bloque de hablar no aparece.
func (s *Service) Configured() bool { return s.stt != nil }

type NextDrill struct {
	*content.Drill
	SkillEn string `json:"skillEn"`
	Left    int    `json:"left"`
}

// Next elige el drill: el de la habilidad más floja que tenga práctica oral, sin
// repetir los de hoy.
func (s *Service) Next(ctx context.Context, userID pgtype.UUID) (*NextDrill, error) {
	q := store.New(s.pool)

	mastery, err := q.ListSkillMastery(ctx, userID)
	if err != nil {
		return nil, err
	}
	score := map[string]float32{}
	for _, m := range mastery {
		score[m.SkillID] = m.Mastery
	}

	skills := s.catalog.SkillsWithDrills()
	sort.SliceStable(skills, func(i, j int) bool { return score[skills[i]] < score[skills[j]] })

	answered, err := s.answeredToday(ctx, q, userID)
	if err != nil {
		return nil, err
	}

	for _, skill := range skills {
		drills := s.catalog.DrillsFor(skill)
		left := 0
		var pick *content.Drill
		for _, d := range drills {
			if answered[d.ID] {
				continue
			}
			if pick == nil {
				pick = d
			}
			left++
		}
		if pick == nil {
			continue
		}
		next := &NextDrill{Drill: pick, Left: left - 1}
		if sk, ok := s.catalog.Skill(skill); ok {
			next.SkillEn = sk.NameEn
		}
		return next, nil
	}
	return nil, nil
}

func (s *Service) answeredToday(ctx context.Context, q *store.Queries, userID pgtype.UUID) (map[string]bool, error) {
	ids, err := q.ListAnsweredToday(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out, nil
}

type Result struct {
	Analysis
	DrillID string     `json:"drillId"`
	HintEs  string     `json:"hintEs,omitempty"`
	Colada  int        `json:"colada"`
	Next    *NextDrill `json:"next"`
}

// Answer transcribe lo que dijiste, lo corrige contra la consigna y lo guarda.
func (s *Service) Answer(ctx context.Context, userID pgtype.UUID, drillID string, audio io.Reader, filename string, seconds int) (Result, error) {
	if s.stt == nil {
		return Result{}, ErrNotConfigured
	}
	drill, ok := s.catalog.Drill(drillID)
	if !ok {
		return Result{}, ErrNoDrill
	}
	if audio == nil {
		return Result{}, ErrEmptyAudio
	}

	// Transcribir es lo más caro que hace la app: el tope se chequea antes de
	// mandar el audio, no después.
	if _, err := s.budget.Check(ctx, userID, budget.Speaking); err != nil {
		return Result{}, err
	}

	transcript, err := s.stt.Transcribe(ctx, audio, filename)
	if err != nil {
		return Result{}, err
	}
	analysis := Analyze(drill, transcript)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op si ya se hizo commit
	q := store.New(tx)

	// El audio no se guarda: queda el texto, que es lo que sirve para repasar.
	raw, err := json.Marshal(analysis)
	if err != nil {
		return Result{}, err
	}
	if _, err := q.InsertAttempt(ctx, store.InsertAttemptParams{
		UserID: userID, ItemID: drill.ID, SkillID: drill.Skill, Context: "speaking",
		Response: raw, Correct: analysis.Correct,
		LatencyMs: pgtype.Int4{Int32: int32(seconds * 1000), Valid: seconds > 0},
	}); err != nil {
		return Result{}, fmt.Errorf("guardar el intento: %w", err)
	}

	mastery, err := s.updateMastery(ctx, q, userID, drill.Skill)
	if err != nil {
		return Result{}, err
	}
	if err := achievement.Grant(ctx, q, userID, s.distinciones, mastery); err != nil {
		return Result{}, err
	}

	won := 0
	if analysis.Correct {
		won = colada
	}
	if _, err := q.AddToDailyLog(ctx, store.AddToDailyLogParams{
		UserID: userID, Colada: int32(won), Seconds: int32(seconds),
	}); err != nil {
		return Result{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Result{}, err
	}

	// El audio ya se transcribió: se cobra aunque la respuesta haya estado mal.
	if err := s.budget.Add(ctx, userID, budget.Speaking); err != nil {
		return Result{}, err
	}

	res := Result{Analysis: analysis, DrillID: drill.ID, HintEs: drill.HintEs, Colada: won}
	if res.Next, err = s.Next(ctx, userID); err != nil {
		return res, err
	}
	return res, nil
}

// updateMastery mueve el dominio con los últimos intentos, igual que la práctica
// escrita: hablar y escribir cuentan para el mismo tema. Devuelve el dominio de
// todas las habilidades, que es lo que hace falta para evaluar las distinciones.
func (s *Service) updateMastery(
	ctx context.Context, q *store.Queries, userID pgtype.UUID, skill string,
) (map[string]placement.State, error) {
	rows, err := q.ListRecentSkillAttempts(ctx, store.ListRecentSkillAttemptsParams{
		UserID: userID, SkillID: skill, Limit: 8,
	})
	if err != nil {
		return nil, err
	}
	attempts := make([]session.Attempt, 0, len(rows))
	for _, r := range rows {
		attempts = append(attempts, session.Attempt{Correct: r.Correct, Consulted: r.Consulted})
	}

	previous := placement.Plano
	wasCalzada := false
	all, err := q.ListSkillMastery(ctx, userID)
	if err != nil {
		return nil, err
	}
	states := make(map[string]placement.State, len(all)+1)
	for _, m := range all {
		states[m.SkillID] = placement.State(m.State)
		if m.SkillID == skill {
			previous, wasCalzada = placement.State(m.State), m.WasCalzada
		}
	}
	state, score := session.RecomputeMastery(previous, wasCalzada, attempts)
	if err := q.UpsertSkillMastery(ctx, store.UpsertSkillMasteryParams{
		UserID: userID, SkillID: skill, Mastery: score, State: string(state), Source: "practice",
	}); err != nil {
		return nil, err
	}
	states[skill] = state
	return states, nil
}
