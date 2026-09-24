package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/auth"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/store"
)

// Account es lo que hace falta para las dos puntas del ciclo de vida de una
// cuenta: llevarse los datos y borrarlos.
//
// Va aparte de Users a propósito. Users es lo que la app usa para estudiar;
// esto se usa dos veces en la vida de una cuenta, y mezclarlo obligaría a todo
// el que implemente Users (los fakes de los tests, sin ir más lejos) a cargar
// con siete métodos que no le importan.
type Account interface {
	// DeleteUser devuelve cuántas filas borró: cero significa que la cuenta ya
	// no estaba, y eso no es un error.
	DeleteUser(ctx context.Context, id pgtype.UUID) (int64, error)

	ListAccountPlacementRuns(ctx context.Context, userID pgtype.UUID) ([]store.PlacementRun, error)
	ListAccountAttempts(ctx context.Context, userID pgtype.UUID) ([]store.Attempt, error)
	ListAccountCards(ctx context.Context, userID pgtype.UUID) ([]store.Card, error)
	ListAccountSkillMastery(ctx context.Context, userID pgtype.UUID) ([]store.SkillMastery, error)
	ListAccountLessonProgress(ctx context.Context, userID pgtype.UUID) ([]store.LessonProgress, error)
	ListAccountDailyLog(ctx context.Context, userID pgtype.UUID) ([]store.DailyLog, error)
	ListAccountUsage(ctx context.Context, userID pgtype.UUID) ([]store.UsageDaily, error)
}

// ── Borrar la cuenta ──────────────────────────────────────────────────────

// deleteAccount borra la cuenta y, con ella, todo lo que la persona hizo.
//
// No usa currentUser: ese método da de alta al usuario si todavía no existe, y
// crear una cuenta para borrarla en el mismo request sería absurdo. Si la
// cuenta no está, el borrado ya está hecho: 204 igual, así reintentar no falla.
func (h handlers) deleteAccount(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.FromContext(r.Context())
	u, err := h.deps.Users.GetUserByFirebaseUID(r.Context(), id.UID)
	if errors.Is(err, pgx.ErrNoRows) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	if _, err := h.deps.Account.DeleteUser(r.Context(), u.ID); err != nil {
		h.internalError(w, r, err)
		return
	}
	h.deps.Logger.InfoContext(r.Context(), "cuenta borrada a pedido del usuario", "user_id", u.ID.String())
	w.WriteHeader(http.StatusNoContent)
}

// ── Llevarse los datos ────────────────────────────────────────────────────

// accountExport es todo lo que la app sabe de una persona, en un solo JSON.
//
// Los tipos de pgtype no se serializan solos de una forma que se pueda leer, y
// esto lo va a abrir alguien que no programó la app: cada fila se traduce a
// campos con nombre y fechas en RFC 3339.
type accountExport struct {
	// Note: el archivo se lo lleva la persona, así que esto va en inglés como
	// el resto de la interfaz.
	Note       string `json:"note"`
	ExportedAt string `json:"exportedAt"`

	Profile       exportProfile      `json:"profile"`
	PlacementRuns []exportPlacement  `json:"placementRuns"`
	Attempts      []exportAttempt    `json:"attempts"`
	Cards         []exportCard       `json:"cards"`
	SkillMastery  []exportMastery    `json:"skillMastery"`
	Lessons       []exportLesson     `json:"lessons"`
	DailyLog      []exportDay        `json:"dailyLog"`
	Usage         []exportUsageEntry `json:"dailyUsage"`
}

const exportNote = "Everything Lost in Translation stores about your account. " +
	"Audio is never stored: spoken answers appear only as the transcript inside attempts. " +
	"AI answers are cached globally by prompt hash, with no link to any account, so they are not part of this file."

type exportProfile struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Theme       string `json:"theme"`
	CreatedAt   string `json:"createdAt"`
	LastSeenAt  string `json:"lastSeenAt"`
}

type exportPlacement struct {
	ID             string  `json:"id"`
	Part           string  `json:"part"`
	ContentVersion string  `json:"contentVersion"`
	Status         string  `json:"status"`
	StartedAt      string  `json:"startedAt"`
	FinishedAt     *string `json:"finishedAt"`
}

type exportAttempt struct {
	ID             int64           `json:"id"`
	ItemID         string          `json:"itemId"`
	SkillID        string          `json:"skillId"`
	Context        string          `json:"context"`
	PlacementRunID *string         `json:"placementRunId"`
	Response       json.RawMessage `json:"response"`
	Correct        bool            `json:"correct"`
	Consulted      bool            `json:"consulted"`
	LatencyMs      *int32          `json:"latencyMs"`
	CreatedAt      string          `json:"createdAt"`
}

type exportCard struct {
	ItemID        string  `json:"itemId"`
	Due           string  `json:"due"`
	Stability     float64 `json:"stability"`
	Difficulty    float64 `json:"difficulty"`
	ElapsedDays   int32   `json:"elapsedDays"`
	ScheduledDays int32   `json:"scheduledDays"`
	Reps          int32   `json:"reps"`
	Lapses        int32   `json:"lapses"`
	State         int16   `json:"state"`
	LastReview    *string `json:"lastReview"`
}

type exportMastery struct {
	SkillID string  `json:"skillId"`
	Mastery float32 `json:"mastery"`
	State   string  `json:"state"`
	// WasCalzada: si el tema estuvo firme alguna vez. Es lo que habilita el
	// óxido, así que forma parte de lo que la app sabe de la persona.
	WasCalzada bool   `json:"wasCalzada"`
	Source     string `json:"source"`
	UpdatedAt  string `json:"updatedAt"`
}

type exportLesson struct {
	SkillID     string  `json:"skillId"`
	Status      string  `json:"status"`
	StartedAt   string  `json:"startedAt"`
	CompletedAt *string `json:"completedAt"`
}

type exportDay struct {
	Day     string `json:"day"`
	Colada  int32  `json:"colada"`
	Answers int32  `json:"answers"`
	Seconds int32  `json:"seconds"`
}

type exportUsageEntry struct {
	Day  string `json:"day"`
	Kind string `json:"kind"`
	N    int32  `json:"n"`
}

// exportAccount devuelve, en un archivo, todo lo que hay guardado de la persona.
func (h handlers) exportAccount(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.FromContext(r.Context())
	u, err := h.deps.Users.GetUserByFirebaseUID(r.Context(), id.UID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "usuario no registrado: llamar primero a GET /v1/me")
		return
	}
	if err != nil {
		h.internalError(w, r, err)
		return
	}

	out, err := h.collectExport(r.Context(), u)
	if err != nil {
		h.internalError(w, r, err)
		return
	}

	// Con esto el navegador lo baja en vez de mostrarlo.
	name := fmt.Sprintf("lost-in-translation-%s.json", time.Now().UTC().Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.WriteHeader(http.StatusOK)

	enc := json.NewEncoder(w)
	// Indentado: el archivo se abre a mano, no lo consume una máquina.
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		// Los headers ya salieron: no se puede devolver un error, sólo dejarlo escrito.
		h.deps.Logger.ErrorContext(r.Context(), "la exportación se cortó a mitad de camino", "err", err)
	}
}

func (h handlers) collectExport(ctx context.Context, u store.User) (accountExport, error) {
	a := h.deps.Account

	runs, err := a.ListAccountPlacementRuns(ctx, u.ID)
	if err != nil {
		return accountExport{}, fmt.Errorf("exportar diagnósticos: %w", err)
	}
	attempts, err := a.ListAccountAttempts(ctx, u.ID)
	if err != nil {
		return accountExport{}, fmt.Errorf("exportar intentos: %w", err)
	}
	cards, err := a.ListAccountCards(ctx, u.ID)
	if err != nil {
		return accountExport{}, fmt.Errorf("exportar tarjetas: %w", err)
	}
	mastery, err := a.ListAccountSkillMastery(ctx, u.ID)
	if err != nil {
		return accountExport{}, fmt.Errorf("exportar dominio: %w", err)
	}
	lessons, err := a.ListAccountLessonProgress(ctx, u.ID)
	if err != nil {
		return accountExport{}, fmt.Errorf("exportar lecciones: %w", err)
	}
	days, err := a.ListAccountDailyLog(ctx, u.ID)
	if err != nil {
		return accountExport{}, fmt.Errorf("exportar jornal: %w", err)
	}
	usage, err := a.ListAccountUsage(ctx, u.ID)
	if err != nil {
		return accountExport{}, fmt.Errorf("exportar uso diario: %w", err)
	}

	out := accountExport{
		Note:       exportNote,
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Profile: exportProfile{
			ID:          u.ID.String(),
			Email:       u.Email,
			DisplayName: u.DisplayName,
			Theme:       u.Theme,
			CreatedAt:   stamp(u.CreatedAt),
			LastSeenAt:  stamp(u.LastSeenAt),
		},
		// Listas vacías, nunca null: el JSON de alguien que recién entra tiene
		// que verse igual que el de alguien con mil intentos.
		PlacementRuns: make([]exportPlacement, 0, len(runs)),
		Attempts:      make([]exportAttempt, 0, len(attempts)),
		Cards:         make([]exportCard, 0, len(cards)),
		SkillMastery:  make([]exportMastery, 0, len(mastery)),
		Lessons:       make([]exportLesson, 0, len(lessons)),
		DailyLog:      make([]exportDay, 0, len(days)),
		Usage:         make([]exportUsageEntry, 0, len(usage)),
	}

	for _, v := range runs {
		out.PlacementRuns = append(out.PlacementRuns, exportPlacement{
			ID:             v.ID.String(),
			Part:           v.Part,
			ContentVersion: v.ContentVersion,
			Status:         v.Status,
			StartedAt:      stamp(v.StartedAt),
			FinishedAt:     stampPtr(v.FinishedAt),
		})
	}
	for _, v := range attempts {
		resp := json.RawMessage(v.Response)
		if len(resp) == 0 {
			resp = json.RawMessage("null")
		}
		out.Attempts = append(out.Attempts, exportAttempt{
			ID:             v.ID,
			ItemID:         v.ItemID,
			SkillID:        v.SkillID,
			Context:        v.Context,
			PlacementRunID: uuidPtr(v.PlacementRunID),
			Response:       resp,
			Correct:        v.Correct,
			Consulted:      v.Consulted,
			LatencyMs:      int32Ptr(v.LatencyMs),
			CreatedAt:      stamp(v.CreatedAt),
		})
	}
	for _, v := range cards {
		out.Cards = append(out.Cards, exportCard{
			ItemID:        v.ItemID,
			Due:           stamp(v.Due),
			Stability:     v.Stability,
			Difficulty:    v.Difficulty,
			ElapsedDays:   v.ElapsedDays,
			ScheduledDays: v.ScheduledDays,
			Reps:          v.Reps,
			Lapses:        v.Lapses,
			State:         v.State,
			LastReview:    stampPtr(v.LastReview),
		})
	}
	for _, v := range mastery {
		out.SkillMastery = append(out.SkillMastery, exportMastery{
			SkillID:    v.SkillID,
			Mastery:    v.Mastery,
			State:      v.State,
			WasCalzada: v.WasCalzada,
			Source:     v.Source,
			UpdatedAt:  stamp(v.UpdatedAt),
		})
	}
	for _, v := range lessons {
		out.Lessons = append(out.Lessons, exportLesson{
			SkillID:     v.SkillID,
			Status:      v.Status,
			StartedAt:   stamp(v.StartedAt),
			CompletedAt: stampPtr(v.CompletedAt),
		})
	}
	for _, v := range days {
		out.DailyLog = append(out.DailyLog, exportDay{
			Day:     date(v.Day),
			Colada:  v.Colada,
			Answers: v.Answers,
			Seconds: v.Seconds,
		})
	}
	for _, v := range usage {
		out.Usage = append(out.Usage, exportUsageEntry{
			Day:  date(v.Day),
			Kind: v.Kind,
			N:    v.N,
		})
	}
	return out, nil
}

// ── Traductores de pgtype a algo que se pueda leer ────────────────────────

func stamp(t pgtype.Timestamptz) string {
	if !t.Valid {
		return ""
	}
	return t.Time.UTC().Format(time.RFC3339)
}

func stampPtr(t pgtype.Timestamptz) *string {
	if !t.Valid {
		return nil
	}
	s := t.Time.UTC().Format(time.RFC3339)
	return &s
}

func date(d pgtype.Date) string {
	if !d.Valid {
		return ""
	}
	return d.Time.Format("2006-01-02")
}

func uuidPtr(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	s := u.String()
	return &s
}

func int32Ptr(n pgtype.Int4) *int32 {
	if !n.Valid {
		return nil
	}
	v := n.Int32
	return &v
}
