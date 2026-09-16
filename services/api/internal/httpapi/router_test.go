package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/auth"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/content"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/placement"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/store"
)

// fakeStore guarda usuarios en memoria por firebase_uid.
type fakeStore struct {
	users map[string]store.User
}

func newFakeStore() *fakeStore { return &fakeStore{users: map[string]store.User{}} }

func (f *fakeStore) UpsertUser(_ context.Context, p store.UpsertUserParams) (store.User, error) {
	u, ok := f.users[p.FirebaseUid]
	if !ok {
		u = store.User{FirebaseUid: p.FirebaseUid, Theme: "system"}
	}
	u.Email, u.DisplayName = p.Email, p.DisplayName
	f.users[p.FirebaseUid] = u
	return u, nil
}

func (f *fakeStore) UpdateUserTheme(_ context.Context, p store.UpdateUserThemeParams) (store.User, error) {
	u, ok := f.users[p.FirebaseUid]
	if !ok {
		return store.User{}, pgx.ErrNoRows
	}
	u.Theme = p.Theme
	f.users[p.FirebaseUid] = u
	return u, nil
}

func (f *fakeStore) GetUserByFirebaseUID(_ context.Context, uid string) (store.User, error) {
	u, ok := f.users[uid]
	if !ok {
		return store.User{}, pgx.ErrNoRows
	}
	return u, nil
}

func (f *fakeStore) ListSkillMastery(context.Context, pgtype.UUID) ([]store.SkillMastery, error) {
	return []store.SkillMastery{{SkillID: "tense.fixture_b", State: "calzada", Mastery: 0.9}}, nil
}

// fakePlacement devuelve el error que se le configure, para probar el mapeo a HTTP.
type fakePlacement struct{ err error }

func (f fakePlacement) Overview(context.Context, pgtype.UUID) ([]placement.PartView, error) {
	return []placement.PartView{{ID: "tenses", Status: "not_started"}}, f.err
}
func (f fakePlacement) Start(context.Context, pgtype.UUID, string) (placement.RunState, error) {
	return placement.RunState{RunID: "r"}, f.err
}
func (f fakePlacement) Get(context.Context, pgtype.UUID, pgtype.UUID) (placement.RunState, error) {
	return placement.RunState{}, f.err
}
func (f fakePlacement) Answer(context.Context, pgtype.UUID, pgtype.UUID, string, content.Response, int) (placement.AnswerResult, error) {
	return placement.AnswerResult{}, f.err
}

type fakePinger struct{ err error }

func (p fakePinger) Ping(context.Context) error { return p.err }

func testCatalog() *content.Catalog {
	data, err := os.ReadFile("../content/testdata/bundle.json")
	if err != nil {
		panic(err)
	}
	c, err := content.Parse(data)
	if err != nil {
		panic(err)
	}
	return c
}

func newTestRouter(s Users, ping error) http.Handler {
	return newTestRouterWith(s, fakePlacement{}, ping)
}

func newTestRouterWith(s Users, p Placement, ping error) http.Handler {
	return NewRouter(Deps{
		Users:       s,
		Placement:   p,
		Catalog:     testCatalog(),
		DB:          fakePinger{err: ping},
		Verifier:    auth.DevVerifier{},
		CORSOrigins: []string{"http://localhost:5173"},
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

func do(t *testing.T, h http.Handler, method, path, body string, authed bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if authed {
		req.Header.Set("Authorization", "Bearer dev:lucho:lucho@example.com")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHealth(t *testing.T) {
	if rec := do(t, newTestRouter(newFakeStore(), nil), http.MethodGet, "/healthz", "", false); rec.Code != http.StatusOK {
		t.Fatalf("db arriba: status = %d, want 200", rec.Code)
	}
	if rec := do(t, newTestRouter(newFakeStore(), errors.New("down")), http.MethodGet, "/healthz", "", false); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("db abajo: status = %d, want 503", rec.Code)
	}
}

func TestMeRequiresAuth(t *testing.T) {
	rec := do(t, newTestRouter(newFakeStore(), nil), http.MethodGet, "/v1/me", "", false)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestMeRegistersUser(t *testing.T) {
	s := newFakeStore()
	rec := do(t, newTestRouter(s, nil), http.MethodGet, "/v1/me", "", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
	}
	var got userResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Email != "lucho@example.com" || got.Theme != "system" {
		t.Fatalf("respuesta inesperada: %+v", got)
	}
	if _, ok := s.users["lucho"]; !ok {
		t.Fatal("el usuario no quedó registrado")
	}
}

func TestUpdateSettings(t *testing.T) {
	tests := []struct {
		name       string
		register   bool
		body       string
		wantStatus int
		wantTheme  string
	}{
		{name: "tema válido", register: true, body: `{"theme":"light"}`, wantStatus: http.StatusOK, wantTheme: "light"},
		{name: "tema inválido", register: true, body: `{"theme":"sepia"}`, wantStatus: http.StatusBadRequest},
		{name: "json roto", register: true, body: `{`, wantStatus: http.StatusBadRequest},
		{name: "usuario sin registrar", register: false, body: `{"theme":"dark"}`, wantStatus: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newFakeStore()
			h := newTestRouter(s, nil)
			if tt.register {
				do(t, h, http.MethodGet, "/v1/me", "", true)
			}
			rec := do(t, h, http.MethodPatch, "/v1/me/settings", tt.body, true)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantTheme != "" && s.users["lucho"].Theme != tt.wantTheme {
				t.Fatalf("theme = %q, want %q", s.users["lucho"].Theme, tt.wantTheme)
			}
		})
	}
}

func TestSkillMapMergesMastery(t *testing.T) {
	rec := do(t, newTestRouter(newFakeStore(), nil), http.MethodGet, "/v1/map", "", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	var body struct {
		Obras []mapObra `json:"obras"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	states := map[string]string{}
	for _, o := range body.Obras {
		for _, p := range o.Pieces {
			for _, s := range p.Skills {
				states[s.ID] = s.State
			}
		}
	}
	if states["tense.fixture_a"] != "plano" || states["tense.fixture_b"] != "calzada" {
		t.Fatalf("estados inesperados: %v", states)
	}
}

func TestPlacementErrorMapping(t *testing.T) {
	const run = "/v1/placement/runs/00000000-0000-0000-0000-000000000001"
	tests := []struct {
		name       string
		err        error
		method     string
		path       string
		body       string
		wantStatus int
	}{
		{"overview ok", nil, http.MethodGet, "/v1/placement/", "", http.StatusOK},
		{"parte inexistente", placement.ErrPartNotFound, http.MethodPost, "/v1/placement/parts/nope/runs", "", http.StatusNotFound},
		{"corrida con id inválido", nil, http.MethodGet, "/v1/placement/runs/no-es-uuid", "", http.StatusNotFound},
		{"corrida terminada", placement.ErrRunDone, http.MethodPost, run + "/answers", `{"itemId":"a-01","response":{"text":"x"}}`, http.StatusConflict},
		{"ítem que no toca", placement.ErrUnexpectedItem, http.MethodPost, run + "/answers", `{"itemId":"a-01","response":{"text":"x"}}`, http.StatusConflict},
		{"respuesta mal formada", content.ErrBadResponse, http.MethodPost, run + "/answers", `{"itemId":"a-02","response":{}}`, http.StatusBadRequest},
		{"body sin itemId", nil, http.MethodPost, run + "/answers", `{}`, http.StatusBadRequest},
		{"error inesperado", errors.New("boom"), http.MethodPost, "/v1/placement/parts/tenses/runs", "", http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestRouterWith(newFakeStore(), fakePlacement{err: tt.err}, nil)
			rec := do(t, h, tt.method, tt.path, tt.body, true)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tt.wantStatus, rec.Body)
			}
		})
	}
}
