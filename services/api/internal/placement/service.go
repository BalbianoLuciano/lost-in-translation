package placement

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
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/store"
)

var (
	ErrPartNotFound   = errors.New("parte de ubicación inexistente")
	ErrRunNotFound    = errors.New("corrida inexistente")
	ErrRunDone        = errors.New("la corrida ya terminó")
	ErrUnexpectedItem = errors.New("ese no es el ítem que toca")
)

type Service struct {
	pool    *pgxpool.Pool
	catalog *content.Catalog
	now     func() time.Time
}

func NewService(pool *pgxpool.Pool, catalog *content.Catalog) *Service {
	return &Service{pool: pool, catalog: catalog, now: time.Now}
}

type SkillRef struct {
	ID     string `json:"id"`
	NameEn string `json:"nameEn"`
	NameEs string `json:"nameEs"`
}

type RunState struct {
	RunID     string              `json:"runId"`
	Part      string              `json:"part"`
	PartName  string              `json:"partName"`
	Status    string              `json:"status"`
	Next      *content.PublicItem `json:"next"`
	NextSkill *SkillRef           `json:"nextSkill,omitempty"`
	Progress  Progress            `json:"progress"`
	Summary   []SkillSummary      `json:"summary,omitempty"`
	Missed    []Missed            `json:"missed,omitempty"`
}

type SkillSummary struct {
	SkillOutcome
	NameEn string `json:"nameEn"`
	NameEs string `json:"nameEs"`
	Piece  string `json:"piece"`
}

// Missed es un ítem que se erró, con todo lo necesario para repasarlo sin
// volver a preguntar a la API.
type Missed struct {
	ItemID    string            `json:"itemId"`
	SkillID   string            `json:"skillId"`
	SkillName string            `json:"skillName"`
	Text      string            `json:"text"`
	Question  string            `json:"question,omitempty"`
	Expected  string            `json:"expected"`
	Rule      string            `json:"rule"`
	ExplainEs content.ExplainEs `json:"explainEs"`
}

type PartView struct {
	ID            string         `json:"id"`
	NameEn        string         `json:"nameEn"`
	NameEs        string         `json:"nameEs"`
	DescriptionEn string         `json:"descriptionEn"`
	Skills        int            `json:"skills"`
	Status        string         `json:"status"` // not_started | in_progress | done
	RunID         string         `json:"runId,omitempty"`
	Progress      *Progress      `json:"progress,omitempty"`
	Summary       []SkillSummary `json:"summary,omitempty"`
	Missed        []Missed       `json:"missed,omitempty"`
}

type AnswerResult struct {
	content.Result
	ItemID    string            `json:"itemId"`
	Rule      string            `json:"rule"`
	ExplainEs content.ExplainEs `json:"explainEs"`
	State     RunState          `json:"state"`
}

func (s *Service) Overview(ctx context.Context, userID pgtype.UUID) ([]PartView, error) {
	q := store.New(s.pool)
	runs, err := q.ListLatestPlacementRuns(ctx, userID)
	if err != nil {
		return nil, err
	}
	latest := map[string]store.PlacementRun{}
	for _, r := range runs {
		latest[r.Part] = r
	}
	views := make([]PartView, 0, len(s.catalog.Placement.Parts))
	for i := range s.catalog.Placement.Parts {
		part := &s.catalog.Placement.Parts[i]
		v := PartView{
			ID: part.ID, NameEn: part.NameEn, NameEs: part.NameEs,
			DescriptionEn: part.DescriptionEn, Skills: len(part.Skills), Status: "not_started",
		}
		if run, ok := latest[part.ID]; ok {
			answers, err := s.answers(ctx, q, run.ID)
			if err != nil {
				return nil, err
			}
			v.Status, v.RunID = run.Status, run.ID.String()
			p := ProgressOf(s.catalog, part, answers)
			v.Progress = &p
			if run.Status == "done" {
				v.Summary = s.summary(part, answers)
				v.Missed = s.missed(answers)
			}
		}
		views = append(views, v)
	}
	return views, nil
}

// Start abre una corrida para la parte o retoma la que está abierta.
func (s *Service) Start(ctx context.Context, userID pgtype.UUID, partID string) (RunState, error) {
	part, ok := s.catalog.Part(partID)
	if !ok {
		return RunState{}, ErrPartNotFound
	}
	q := store.New(s.pool)
	run, err := q.GetOpenPlacementRun(ctx, store.GetOpenPlacementRunParams{UserID: userID, Part: partID})
	if errors.Is(err, pgx.ErrNoRows) {
		run, err = q.CreatePlacementRun(ctx, store.CreatePlacementRunParams{
			UserID: userID, Part: partID, ContentVersion: s.catalog.Version,
		})
	}
	if err != nil {
		return RunState{}, err
	}
	answers, err := s.answers(ctx, q, run.ID)
	if err != nil {
		return RunState{}, err
	}
	return s.state(part, run, answers), nil
}

func (s *Service) Get(ctx context.Context, userID, runID pgtype.UUID) (RunState, error) {
	q := store.New(s.pool)
	run, err := q.GetPlacementRun(ctx, store.GetPlacementRunParams{ID: runID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return RunState{}, ErrRunNotFound
	}
	if err != nil {
		return RunState{}, err
	}
	part, ok := s.catalog.Part(run.Part)
	if !ok {
		return RunState{}, ErrPartNotFound
	}
	answers, err := s.answers(ctx, q, run.ID)
	if err != nil {
		return RunState{}, err
	}
	return s.state(part, run, answers), nil
}

// Answer corrige, guarda el intento y, si la parte terminó, el dominio de cada
// habilidad. Todo en una transacción.
//
// El diagnóstico NO crea tarjetas de repaso a propósito: si estos ítems
// volvieran en el repaso diario, la próxima vez que midas tu nivel estarías
// midiendo memoria y no inglés. Los ítems de ubicación quedan reservados para
// medir; la práctica usa los demás.
func (s *Service) Answer(ctx context.Context, userID, runID pgtype.UUID, itemID string, resp content.Response, latencyMs int) (AnswerResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return AnswerResult{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op si ya se hizo commit
	q := store.New(tx)

	run, err := q.GetPlacementRunForUpdate(ctx, store.GetPlacementRunForUpdateParams{ID: runID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return AnswerResult{}, ErrRunNotFound
	}
	if err != nil {
		return AnswerResult{}, err
	}
	if run.Status == "done" {
		return AnswerResult{}, ErrRunDone
	}
	part, ok := s.catalog.Part(run.Part)
	if !ok {
		return AnswerResult{}, ErrPartNotFound
	}
	answers, err := s.answers(ctx, q, run.ID)
	if err != nil {
		return AnswerResult{}, err
	}
	expected := Next(s.catalog, part, answers)
	if expected == nil || expected.ID != itemID {
		return AnswerResult{}, ErrUnexpectedItem
	}

	result, err := content.Grade(expected, resp)
	if err != nil {
		return AnswerResult{}, err
	}

	raw, err := json.Marshal(resp)
	if err != nil {
		return AnswerResult{}, err
	}
	if _, err := q.InsertAttempt(ctx, store.InsertAttemptParams{
		UserID: userID, ItemID: expected.ID, SkillID: expected.Skill, Context: "placement",
		PlacementRunID: run.ID, Response: raw, Correct: result.Correct,
		LatencyMs: pgtype.Int4{Int32: int32(latencyMs), Valid: latencyMs > 0},
	}); err != nil {
		return AnswerResult{}, fmt.Errorf("guardar intento: %w", err)
	}

	answers = append(answers, Answer{ItemID: expected.ID, Correct: result.Correct})
	if Next(s.catalog, part, answers) == nil {
		if err := q.FinishPlacementRun(ctx, run.ID); err != nil {
			return AnswerResult{}, err
		}
		run.Status = "done"
		for _, o := range Outcomes(s.catalog, part, answers) {
			if err := q.UpsertSkillMastery(ctx, store.UpsertSkillMasteryParams{
				UserID: userID, SkillID: o.SkillID, Mastery: o.Mastery, State: string(o.State), Source: "placement",
			}); err != nil {
				return AnswerResult{}, fmt.Errorf("guardar dominio: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return AnswerResult{}, err
	}
	return AnswerResult{
		Result:    result,
		ItemID:    expected.ID,
		Rule:      expected.Rule,
		ExplainEs: expected.ExplainEs,
		State:     s.state(part, run, answers),
	}, nil
}

func (s *Service) answers(ctx context.Context, q *store.Queries, runID pgtype.UUID) ([]Answer, error) {
	rows, err := q.ListRunAttempts(ctx, runID)
	if err != nil {
		return nil, err
	}
	out := make([]Answer, 0, len(rows))
	for _, r := range rows {
		out = append(out, Answer{ItemID: r.ItemID, Correct: r.Correct})
	}
	return out, nil
}

func (s *Service) state(part *content.PlacementPart, run store.PlacementRun, answers []Answer) RunState {
	st := RunState{
		RunID:    run.ID.String(),
		Part:     part.ID,
		PartName: part.NameEn,
		Status:   run.Status,
		Progress: ProgressOf(s.catalog, part, answers),
	}
	if run.Status == "done" {
		st.Summary = s.summary(part, answers)
		st.Missed = s.missed(answers)
		return st
	}
	if next := Next(s.catalog, part, answers); next != nil {
		p := next.Public()
		st.Next = &p
		if sk, ok := s.catalog.Skill(next.Skill); ok {
			st.NextSkill = &SkillRef{ID: sk.ID, NameEn: sk.NameEn, NameEs: sk.NameEs}
		}
	}
	return st
}

// missed arma la lista de errores en el orden en que se respondieron.
func (s *Service) missed(answers []Answer) []Missed {
	var out []Missed
	for _, a := range answers {
		if a.Correct {
			continue
		}
		it, ok := s.catalog.Item(a.ItemID)
		if !ok {
			continue
		}
		m := Missed{
			ItemID: it.ID, SkillID: it.Skill, Text: it.Text, Question: it.Question,
			Rule: it.Rule, ExplainEs: it.ExplainEs,
		}
		if sk, ok := s.catalog.Skill(it.Skill); ok {
			m.SkillName = sk.NameEn
		}
		m.Expected = expectedOf(it)
		out = append(out, m)
	}
	return out
}

// expectedOf devuelve la respuesta correcta de un ítem, para mostrarla en el repaso.
func expectedOf(it *content.Item) string {
	switch it.Type {
	case content.Choice, content.ExplainWhy:
		if it.Answer < len(it.Options) {
			return it.Options[it.Answer]
		}
	case content.FixError:
		if len(it.Corrections) > 0 {
			return it.Wrong + " → " + it.Corrections[0]
		}
	case content.Cloze:
		if len(it.Answers) > 0 {
			return it.Answers[0]
		}
	}
	return ""
}

func (s *Service) summary(part *content.PlacementPart, answers []Answer) []SkillSummary {
	outcomes := Outcomes(s.catalog, part, answers)
	out := make([]SkillSummary, 0, len(outcomes))
	for _, o := range outcomes {
		sum := SkillSummary{SkillOutcome: o}
		if sk, ok := s.catalog.Skill(o.SkillID); ok {
			sum.NameEn, sum.NameEs, sum.Piece = sk.NameEn, sk.NameEs, sk.Piece
		}
		out = append(out, sum)
	}
	return out
}
