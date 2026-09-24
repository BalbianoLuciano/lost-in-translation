package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/content"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/placement"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/srs"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/store"
)

var (
	ErrNoLesson       = errors.New("no hay lección para esa habilidad")
	ErrLessonNotReady = errors.New("esa lección todavía no toca")
	ErrNoItem         = errors.New("no hay ítem para responder en ese bloque")
	ErrUnexpectedItem = errors.New("ese no es el ítem que toca")
)

// recentWindow es cuántos intentos mira el dominio de una habilidad.
const recentWindow = 8

// maxReviewBatch es el tope de repaso por sesión: diez minutos, no una lista infinita.
const maxReviewBatch = 20

type Service struct {
	pool    *pgxpool.Pool
	catalog *content.Catalog
	view    *CatalogView
	now     func() time.Time
}

func NewService(pool *pgxpool.Pool, catalog *content.Catalog) *Service {
	return &Service{pool: pool, catalog: catalog, view: NewCatalogView(catalog), now: time.Now}
}

type LessonRef struct {
	Skill    string `json:"skill"`
	SkillEn  string `json:"skillEn"`
	TitleEn  string `json:"titleEn"`
	GoalEn   string `json:"goalEn"`
	Minutes  int    `json:"minutes"`
	Status   string `json:"status"` // not_started | in_progress | done
	Obra     int    `json:"obra"`
	ObraName string `json:"obraName"`
}

type PracticeRef struct {
	Skill     string `json:"skill"`
	SkillEn   string `json:"skillEn"`
	Done      int    `json:"done"`
	Total     int    `json:"total"`
	Locked    bool   `json:"locked"` // hasta que la lección esté leída
	LessonFor string `json:"lessonFor,omitempty"`
}

type ReviewRef struct {
	Due  int `json:"due"`
	Done int `json:"done"`
}

// State es la sesión de hoy.
type State struct {
	Date         string       `json:"date"`
	Jornal       int          `json:"jornal"`
	ColadaToday  int          `json:"coladaToday"`
	ColadaTotal  int          `json:"coladaTotal"`
	MinutesToday int          `json:"minutesToday"`
	Review       ReviewRef    `json:"review"`
	Lesson       *LessonRef   `json:"lesson"`
	Practice     *PracticeRef `json:"practice"`
}

func (s *Service) Today(ctx context.Context, userID pgtype.UUID) (State, error) {
	q := store.New(s.pool)
	now := s.now()

	st := State{Date: now.Format("2006-01-02")}

	due, err := q.CountDueCards(ctx, userID)
	if err != nil {
		return st, err
	}
	st.Review.Due = int(due)

	if log, err := q.GetTodayLog(ctx, userID); err == nil {
		st.ColadaToday, st.MinutesToday = int(log.Colada), int(log.Seconds)/60
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return st, err
	}

	total, err := q.SumColada(ctx, userID)
	if err != nil {
		return st, err
	}
	st.ColadaTotal = int(total)

	days, err := q.ListRecentDays(ctx, store.ListRecentDaysParams{UserID: userID, Limit: 120})
	if err != nil {
		return st, err
	}
	history := make([]Day, 0, len(days))
	for _, d := range days {
		history = append(history, Day{Date: d.Day.Time, Answers: int(d.Answers)})
	}
	st.Jornal = Jornal(history, now)

	candidate, progress, err := s.currentLesson(ctx, q, userID)
	if err != nil {
		return st, err
	}
	if candidate == nil {
		return st, nil
	}

	ref := &LessonRef{
		Skill: candidate.SkillID, TitleEn: candidate.Lesson.TitleEn,
		GoalEn: candidate.Lesson.GoalEn, Minutes: candidate.Lesson.Minutes, Status: "not_started",
	}
	if sk, ok := s.catalog.Skill(candidate.SkillID); ok {
		ref.SkillEn, ref.Obra = sk.NameEn, sk.Obra
		for _, o := range s.catalog.Obras {
			if o.ID == sk.Obra {
				ref.ObraName = o.Name
			}
		}
	}
	if progress != nil {
		ref.Status = progress.Status
	}
	st.Lesson = ref

	answered, err := s.answeredToday(ctx, q, userID)
	if err != nil {
		return st, err
	}
	items := s.catalog.PracticeItems(candidate.SkillID)
	done := 0
	for _, it := range items {
		if answered[it.ID] {
			done++
		}
	}
	st.Practice = &PracticeRef{
		Skill: candidate.SkillID, SkillEn: ref.SkillEn, Done: done, Total: len(items),
		Locked: ref.Status != "done", LessonFor: candidate.SkillID,
	}
	return st, nil
}

// currentLesson devuelve el tema que toca: el más flojo con lección escrita.
func (s *Service) currentLesson(ctx context.Context, q *store.Queries, userID pgtype.UUID) (*Candidate, *store.LessonProgress, error) {
	mastery, err := q.ListSkillMastery(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	byID := make(map[string]Mastery, len(mastery))
	for _, m := range mastery {
		byID[m.SkillID] = Mastery{SkillID: m.SkillID, State: placement.State(m.State), Mastery: m.Mastery}
	}

	progress, err := q.ListLessonProgress(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	done := map[string]bool{}
	started := map[string]bool{}
	current := map[string]store.LessonProgress{}
	for _, p := range progress {
		current[p.SkillID] = p
		started[p.SkillID] = true
		if p.Status != "done" {
			continue
		}
		// Un tema deja de proponerse cuando se leyó la lección Y se recorrió todo
		// su banco de práctica. A partir de ahí vuelve por el repaso espaciado,
		// no como lección.
		answered, err := q.CountAnsweredItemsBySkill(ctx, store.CountAnsweredItemsBySkillParams{
			UserID: userID, SkillID: p.SkillID,
		})
		if err != nil {
			return nil, nil, err
		}
		done[p.SkillID] = int(answered) >= len(s.catalog.PracticeItems(p.SkillID))
	}

	candidate := NextLesson(s.view, byID, done, started)
	if candidate == nil {
		return nil, nil, nil
	}
	if p, ok := current[candidate.SkillID]; ok {
		return candidate, &p, nil
	}
	return candidate, nil, nil
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

// ── Lección ───────────────────────────────────────────────────────────────

type LessonView struct {
	content.Lesson
	Status  string `json:"status"`
	SkillEn string `json:"skillEn"`
}

// Lesson devuelve la lección y la marca como empezada.
func (s *Service) Lesson(ctx context.Context, userID pgtype.UUID, skill string) (LessonView, error) {
	lesson, ok := s.catalog.Lesson(skill)
	if !ok {
		return LessonView{}, ErrNoLesson
	}
	q := store.New(s.pool)
	p, err := q.StartLesson(ctx, store.StartLessonParams{UserID: userID, SkillID: skill})
	if err != nil {
		return LessonView{}, err
	}
	view := LessonView{Lesson: *lesson, Status: p.Status}
	if sk, ok := s.catalog.Skill(skill); ok {
		view.SkillEn = sk.NameEn
	}
	return view, nil
}

// CompleteLesson marca la lección como leída y destraba la práctica.
func (s *Service) CompleteLesson(ctx context.Context, userID pgtype.UUID, skill string) (State, error) {
	if _, ok := s.catalog.Lesson(skill); !ok {
		return State{}, ErrNoLesson
	}
	if _, err := store.New(s.pool).CompleteLesson(ctx, store.CompleteLessonParams{UserID: userID, SkillID: skill}); err != nil {
		return State{}, err
	}
	return s.Today(ctx, userID)
}

// ── Qué ítem toca ─────────────────────────────────────────────────────────

type NextItem struct {
	Block   Block               `json:"block"`
	Skill   string              `json:"skill"`
	SkillEn string              `json:"skillEn"`
	Item    *content.PublicItem `json:"item"`
	Left    int                 `json:"left"`
}

func (s *Service) Next(ctx context.Context, userID pgtype.UUID, block Block) (*NextItem, error) {
	q := store.New(s.pool)
	switch block {
	case Review:
		cards, err := q.ListDueCards(ctx, store.ListDueCardsParams{UserID: userID, Limit: maxReviewBatch})
		if err != nil {
			return nil, err
		}
		for _, c := range cards {
			it, ok := s.catalog.Item(c.ItemID)
			if !ok {
				continue // el ítem salió del banco: la tarjeta se ignora
			}
			return s.next(block, it, len(cards)-1), nil
		}
		return nil, nil

	case Practice:
		st, err := s.Today(ctx, userID)
		if err != nil {
			return nil, err
		}
		if st.Practice == nil || st.Practice.Locked {
			return nil, nil
		}
		answered, err := s.answeredToday(ctx, q, userID)
		if err != nil {
			return nil, err
		}
		items := s.catalog.PracticeItems(st.Practice.Skill)
		left := 0
		var pick *content.Item
		for _, it := range items {
			if answered[it.ID] {
				continue
			}
			if pick == nil {
				pick = it
			}
			left++
		}
		if pick == nil {
			return nil, nil
		}
		return s.next(block, pick, left-1), nil
	}
	return nil, fmt.Errorf("bloque desconocido: %s", block)
}

func (s *Service) next(block Block, it *content.Item, left int) *NextItem {
	p := it.Public()
	n := &NextItem{Block: block, Skill: it.Skill, Item: &p, Left: left}
	if sk, ok := s.catalog.Skill(it.Skill); ok {
		n.SkillEn = sk.NameEn
	}
	return n
}

// ── Responder ─────────────────────────────────────────────────────────────

type AnswerResult struct {
	content.Result
	ItemID    string            `json:"itemId"`
	Rule      string            `json:"rule"`
	ExplainEs content.ExplainEs `json:"explainEs"`
	Colada    int               `json:"colada"`
	State     State             `json:"state"`
	Next      *NextItem         `json:"next"`
}

// Answer corrige, guarda el intento, mueve la tarjeta de repaso y el dominio de
// la habilidad, y suma al día. Todo en una transacción.
func (s *Service) Answer(
	ctx context.Context, userID pgtype.UUID, block Block, itemID string,
	resp content.Response, latencyMs int, consulted bool,
) (AnswerResult, error) {
	it, ok := s.catalog.Item(itemID)
	if !ok {
		return AnswerResult{}, ErrUnexpectedItem
	}
	expected, err := s.Next(ctx, userID, block)
	if err != nil {
		return AnswerResult{}, err
	}
	if expected == nil || expected.Item.ID != itemID {
		return AnswerResult{}, ErrUnexpectedItem
	}

	result, err := content.Grade(it, resp)
	if err != nil {
		return AnswerResult{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return AnswerResult{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op si ya se hizo commit
	q := store.New(tx)

	raw, err := json.Marshal(resp)
	if err != nil {
		return AnswerResult{}, err
	}
	attemptContext := "lesson"
	if block == Review {
		attemptContext = "review"
	}
	if _, err := q.InsertAttempt(ctx, store.InsertAttemptParams{
		UserID: userID, ItemID: it.ID, SkillID: it.Skill, Context: attemptContext,
		Response: raw, Correct: result.Correct, Consulted: consulted,
		LatencyMs: pgtype.Int4{Int32: int32(latencyMs), Valid: latencyMs > 0},
	}); err != nil {
		return AnswerResult{}, fmt.Errorf("guardar intento: %w", err)
	}

	if err := s.review(ctx, q, userID, it.ID, result.Correct && !consulted); err != nil {
		return AnswerResult{}, err
	}
	if err := s.updateMastery(ctx, q, userID, it.Skill); err != nil {
		return AnswerResult{}, err
	}

	colada := 0
	if result.Correct {
		colada = Colada(it)
	}
	if _, err := q.AddToDailyLog(ctx, store.AddToDailyLogParams{
		UserID: userID, Colada: int32(colada), Seconds: int32(latencyMs / 1000),
	}); err != nil {
		return AnswerResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return AnswerResult{}, err
	}

	out := AnswerResult{
		Result: result, ItemID: it.ID, Rule: it.Rule, ExplainEs: it.ExplainEs, Colada: colada,
	}
	if out.State, err = s.Today(ctx, userID); err != nil {
		return out, err
	}
	if out.Next, err = s.Next(ctx, userID, block); err != nil {
		return out, err
	}
	return out, nil
}

func (s *Service) review(ctx context.Context, q *store.Queries, userID pgtype.UUID, itemID string, good bool) error {
	var prev *store.Card
	card, err := q.GetCard(ctx, store.GetCardParams{UserID: userID, ItemID: itemID})
	switch {
	case err == nil:
		prev = &card
	case !errors.Is(err, pgx.ErrNoRows):
		return err
	}
	return q.UpsertCard(ctx, srs.Review(userID, itemID, prev, good, s.now()))
}

func (s *Service) updateMastery(ctx context.Context, q *store.Queries, userID pgtype.UUID, skill string) error {
	rows, err := q.ListRecentSkillAttempts(ctx, store.ListRecentSkillAttemptsParams{
		UserID: userID, SkillID: skill, Limit: recentWindow,
	})
	if err != nil {
		return err
	}
	attempts := make([]Attempt, 0, len(rows))
	for _, r := range rows {
		attempts = append(attempts, Attempt{Correct: r.Correct, Consulted: r.Consulted})
	}

	previous := placement.Plano
	wasCalzada := false
	all, err := q.ListSkillMastery(ctx, userID)
	if err != nil {
		return err
	}
	for _, m := range all {
		if m.SkillID == skill {
			previous, wasCalzada = placement.State(m.State), m.WasCalzada
		}
	}

	state, score := RecomputeMastery(previous, wasCalzada, attempts)
	return q.UpsertSkillMastery(ctx, store.UpsertSkillMasteryParams{
		UserID: userID, SkillID: skill, Mastery: score, State: string(state), Source: "practice",
	})
}
