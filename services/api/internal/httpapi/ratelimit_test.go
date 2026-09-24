package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLimiterDejaPasarElPicoYDespuésFrena(t *testing.T) {
	l := newLimiter(60, 3)

	for i := 0; i < 3; i++ {
		if !l.allow("uno") {
			t.Fatalf("el pedido %d entra en el pico", i+1)
		}
	}
	if l.allow("uno") {
		t.Fatal("el cuarto tiene que esperar: el pico era de tres")
	}
}

// Que uno se pase de rosca no puede frenar a los demás.
func TestLimiterSepararPorClave(t *testing.T) {
	l := newLimiter(60, 1)

	if !l.allow("uno") {
		t.Fatal("el primero entra")
	}
	if l.allow("uno") {
		t.Fatal("el mismo, no")
	}
	if !l.allow("otro") {
		t.Fatal("otra clave tiene su propio balde")
	}
}

// El mapa no puede crecer con cada IP que pasa una vez.
func TestLimiterTiraLosBaldesViejos(t *testing.T) {
	now := time.Now()
	l := newLimiter(60, 1)
	l.now = func() time.Time { return now }

	l.allow("vieja")
	if len(l.buckets) != 1 {
		t.Fatalf("baldes = %d, want 1", len(l.buckets))
	}

	now = now.Add(l.idle + time.Minute)
	l.allow("nueva")

	if _, ok := l.buckets["vieja"]; ok {
		t.Error("el balde sin uso tenía que desaparecer")
	}
	if _, ok := l.buckets["nueva"]; !ok {
		t.Error("el balde recién usado tiene que quedar")
	}
}

func TestLimiterMiddlewareContesta429(t *testing.T) {
	l := newLimiter(60, 1)
	h := l.middleware(byIP)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"

	first := httptest.NewRecorder()
	h.ServeHTTP(first, req)
	if first.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", first.Code)
	}

	second := httptest.NewRecorder()
	h.ServeHTTP(second, req)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", second.Code)
	}
	if second.Header().Get("Retry-After") == "" {
		t.Error("un 429 sin Retry-After no le dice al cliente cuándo volver")
	}
}
