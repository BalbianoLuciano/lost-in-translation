package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/auth"
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

type fakePinger struct{ err error }

func (p fakePinger) Ping(context.Context) error { return p.err }

func newTestRouter(s store.Querier, ping error) http.Handler {
	return NewRouter(Deps{
		Store:       s,
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
