package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDevVerifier(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		wantErr bool
		wantUID string
	}{
		{name: "válido", token: "dev:lucho:lucho@example.com", wantUID: "lucho"},
		{name: "email con dos puntos", token: "dev:lucho:a:b@example.com", wantUID: "lucho"},
		{name: "sin prefijo", token: "lucho:lucho@example.com", wantErr: true},
		{name: "uid vacío", token: "dev::lucho@example.com", wantErr: true},
		{name: "email vacío", token: "dev:lucho:", wantErr: true},
		{name: "vacío", token: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := DevVerifier{}.Verify(context.Background(), tt.token)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && id.UID != tt.wantUID {
				t.Fatalf("UID = %q, want %q", id.UID, tt.wantUID)
			}
		})
	}
}

func TestMiddleware(t *testing.T) {
	var got Identity
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = FromContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	})
	h := Middleware(DevVerifier{})(next)

	tests := []struct {
		name       string
		header     string
		wantStatus int
	}{
		{name: "sin header", header: "", wantStatus: http.StatusUnauthorized},
		{name: "sin Bearer", header: "dev:lucho:l@example.com", wantStatus: http.StatusUnauthorized},
		{name: "token inválido", header: "Bearer nope", wantStatus: http.StatusUnauthorized},
		{name: "token válido", header: "Bearer dev:lucho:l@example.com", wantStatus: http.StatusNoContent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got = Identity{}
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantStatus == http.StatusNoContent && got.UID != "lucho" {
				t.Fatalf("identidad no llegó al handler: %+v", got)
			}
		})
	}
}
