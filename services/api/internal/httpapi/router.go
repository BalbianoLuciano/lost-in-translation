// Package httpapi define las rutas HTTP de la API.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/auth"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/content"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/placement"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/session"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/speaking"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/store"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/tutor"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

// Users es lo que la API necesita de la tabla de usuarios y su progreso.
type Users interface {
	UpsertUser(ctx context.Context, arg store.UpsertUserParams) (store.User, error)
	GetUserByFirebaseUID(ctx context.Context, firebaseUid string) (store.User, error)
	UpdateUserTheme(ctx context.Context, arg store.UpdateUserThemeParams) (store.User, error)
	ListSkillMastery(ctx context.Context, userID pgtype.UUID) ([]store.SkillMastery, error)
}

// Placement es el test de ubicación (placement.Service).
type Placement interface {
	Overview(ctx context.Context, userID pgtype.UUID) ([]placement.PartView, error)
	Start(ctx context.Context, userID pgtype.UUID, part string) (placement.RunState, error)
	Get(ctx context.Context, userID, runID pgtype.UUID) (placement.RunState, error)
	Answer(ctx context.Context, userID, runID pgtype.UUID, itemID string, resp content.Response, latencyMs int, consulted bool) (placement.AnswerResult, error)
}

// Session es la sesión diaria (session.Service).
type Session interface {
	Today(ctx context.Context, userID pgtype.UUID) (session.State, error)
	Next(ctx context.Context, userID pgtype.UUID, block session.Block) (*session.NextItem, error)
	Answer(ctx context.Context, userID pgtype.UUID, block session.Block, itemID string, resp content.Response, latencyMs int, consulted bool) (session.AnswerResult, error)
	Lesson(ctx context.Context, userID pgtype.UUID, skill string) (session.LessonView, error)
	CompleteLesson(ctx context.Context, userID pgtype.UUID, skill string) (session.State, error)
}

// Tutor responde lo que el glosario no cubre (tutor.Tutor).
type Tutor interface {
	Configured() bool
	Ask(ctx context.Context, userID pgtype.UUID, question, itemID string) (tutor.Answer, error)
}

// Speaking es la práctica oral (speaking.Service).
type Speaking interface {
	Configured() bool
	Next(ctx context.Context, userID pgtype.UUID) (*speaking.NextDrill, error)
	Answer(ctx context.Context, userID pgtype.UUID, drillID string, audio io.Reader, filename string, seconds int) (speaking.Result, error)
}

type Deps struct {
	Users        Users
	Account      Account
	Gate         *Gate
	Tutor        Tutor
	Speaking     Speaking
	Placement    Placement
	Session      Session
	Achievements Achievements
	Catalog      *content.Catalog
	DB           Pinger
	Verifier     auth.Verifier
	CORSOrigins  []string
	Logger       *slog.Logger
}

// Cuánto se puede pedir por minuto. El general protege de una inundación; el
// caro protege la clave del proveedor, y por eso va por cuenta y no por IP.
const (
	generalPerMinute = 240
	generalBurst     = 60
	costlyPerMinute  = 12
	costlyBurst      = 6
)

func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(requestLogger(d.Logger))
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: d.CORSOrigins,
		AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowedHeaders: []string{"Authorization", "Content-Type"},
		MaxAge:         600,
	}))

	h := handlers{deps: d}
	general := newLimiter(generalPerMinute, generalBurst)
	costly := newLimiter(costlyPerMinute, costlyBurst)

	r.Get("/healthz", h.health)

	r.Route("/v1", func(r chi.Router) {
		r.Use(general.middleware(byIP))
		r.Use(auth.Middleware(d.Verifier))
		r.Get("/me", h.me)
		r.Patch("/me/settings", h.updateSettings)
		r.Delete("/me", h.deleteAccount)
		// Exportar recorre todas las tablas de la persona: es barato de pedir y
		// caro de servir, así que va con el límite de los caros.
		r.With(costly.middleware(byUser)).Get("/me/export", h.exportAccount)
		r.Get("/map", h.skillMap)
		r.Get("/glossary", h.glossary)
		r.Get("/achievements", h.achievements)
		r.With(costly.middleware(byUser)).Post("/ask", h.ask)

		r.Route("/session", func(r chi.Router) {
			r.Get("/", h.sessionToday)
			r.Get("/next", h.sessionNext)
			r.Post("/answers", h.sessionAnswer)
		})

		r.Route("/speaking", func(r chi.Router) {
			r.Get("/next", h.speakingNext)
			r.With(costly.middleware(byUser)).Post("/{drill}/answers", h.speakingAnswer)
		})

		r.Route("/lessons", func(r chi.Router) {
			r.Get("/{skill}", h.lesson)
			r.Post("/{skill}/complete", h.completeLesson)
		})

		r.Route("/placement", func(r chi.Router) {
			r.Get("/", h.placementOverview)
			r.Post("/parts/{part}/runs", h.placementStart)
			r.Get("/runs/{run}", h.placementGet)
			r.Post("/runs/{run}/answers", h.placementAnswer)
		})
	})

	return r
}

type handlers struct {
	deps Deps
}

func (h handlers) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.deps.DB.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "degraded", "db": "down"})
		return
	}
	body := map[string]any{"status": "ok", "db": "up"}
	if h.deps.Catalog != nil {
		body["content"] = h.deps.Catalog.Version
	}
	if h.deps.Tutor != nil {
		body["ai"] = h.deps.Tutor.Configured()
	}
	if h.deps.Speaking != nil {
		body["speaking"] = h.deps.Speaking.Configured()
	}
	writeJSON(w, http.StatusOK, body)
}

type userResponse struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Theme       string `json:"theme"`
}

func toUserResponse(u store.User) userResponse {
	return userResponse{
		ID:          u.ID.String(),
		Email:       u.Email,
		DisplayName: u.DisplayName,
		Theme:       u.Theme,
	}
}

// me registra al usuario en el primer llamado y devuelve su perfil.
func (h handlers) me(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.FromContext(r.Context())
	if _, err := h.deps.Users.GetUserByFirebaseUID(r.Context(), id.UID); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			h.internalError(w, r, err)
			return
		}
		if !h.deps.Gate.CanRegister(id.Email) {
			writeError(w, http.StatusForbidden, ErrNotAllowed.Error())
			return
		}
	}
	u, err := h.deps.Users.UpsertUser(r.Context(), store.UpsertUserParams{
		FirebaseUid: id.UID,
		Email:       id.Email,
		DisplayName: id.Name,
	})
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toUserResponse(u))
}

// currentUser devuelve el usuario registrado; si todavía no existe y está
// invitado, lo crea.
func (h handlers) currentUser(r *http.Request) (store.User, error) {
	id, _ := auth.FromContext(r.Context())
	u, err := h.deps.Users.GetUserByFirebaseUID(r.Context(), id.UID)
	if errors.Is(err, pgx.ErrNoRows) {
		if !h.deps.Gate.CanRegister(id.Email) {
			return store.User{}, ErrNotAllowed
		}
		return h.deps.Users.UpsertUser(r.Context(), store.UpsertUserParams{
			FirebaseUid: id.UID, Email: id.Email, DisplayName: id.Name,
		})
	}
	return u, err
}

type settingsRequest struct {
	Theme string `json:"theme"`
}

var validThemes = map[string]bool{"system": true, "dark": true, "light": true}

func (h handlers) updateSettings(w http.ResponseWriter, r *http.Request) {
	var req settingsRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "body inválido")
		return
	}
	if !validThemes[req.Theme] {
		writeError(w, http.StatusBadRequest, "theme debe ser system, dark o light")
		return
	}
	id, _ := auth.FromContext(r.Context())
	u, err := h.deps.Users.UpdateUserTheme(r.Context(), store.UpdateUserThemeParams{
		FirebaseUid: id.UID,
		Theme:       req.Theme,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "usuario no registrado: llamar primero a GET /v1/me")
		return
	}
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toUserResponse(u))
}

func (h handlers) internalError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, ErrNotAllowed) {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	h.deps.Logger.ErrorContext(r.Context(), "error interno",
		"err", err, "path", r.URL.Path, "request_id", middleware.GetReqID(r.Context()))
	writeError(w, http.StatusInternalServerError, "error interno")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			logger.InfoContext(r.Context(), "request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", middleware.GetReqID(r.Context()),
			)
		})
	}
}
