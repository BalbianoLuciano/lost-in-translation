package httpapi

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/auth"
)

// El presupuesto diario pone el techo de lo que se gasta; esto pone el techo de
// lo rápido que se puede gastar. Hacen falta los dos: sin ritmo, una sola tarde
// de pedidos automáticos consume el día entero de todos en un minuto.
//
// Las cuentas viven en memoria, así que el límite es por instancia. Con una sola
// instancia es exacto; con varias, cada una deja pasar su parte, y el tope
// diario —que sí está en la base— sigue siendo el techo real.
type limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	every   rate.Limit
	burst   int
	idle    time.Duration
	swept   time.Time
	now     func() time.Time
}

type bucket struct {
	*rate.Limiter
	seen time.Time
}

// newLimiter arma un limitador de perMinute pedidos por minuto, con un pico de
// burst seguidos.
func newLimiter(perMinute, burst int) *limiter {
	return &limiter{
		buckets: map[string]*bucket{},
		every:   rate.Limit(float64(perMinute) / 60),
		burst:   burst,
		idle:    10 * time.Minute,
		now:     time.Now,
	}
}

func (l *limiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.sweep(now)

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{Limiter: rate.NewLimiter(l.every, l.burst)}
		l.buckets[key] = b
	}
	b.seen = now
	return b.Allow()
}

// sweep tira los baldes que nadie tocó hace rato, para que el mapa no crezca
// con cada IP que pasa. Se hace acá y no en una goroutine: sin hilo de fondo no
// hay nada que apagar al cerrar.
func (l *limiter) sweep(now time.Time) {
	if now.Sub(l.swept) < l.idle {
		return
	}
	l.swept = now
	for k, b := range l.buckets {
		if now.Sub(b.seen) > l.idle {
			delete(l.buckets, k)
		}
	}
}

func (l *limiter) middleware(keyOf func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.allow(keyOf(r)) {
				w.Header().Set("Retry-After", strconv.Itoa(int(l.retryAfter().Seconds())))
				writeError(w, http.StatusTooManyRequests, "estás yendo demasiado rápido: probá de nuevo en un momento")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (l *limiter) retryAfter() time.Duration {
	if l.every <= 0 {
		return time.Minute
	}
	return time.Duration(float64(time.Second) / float64(l.every))
}

// byIP: la IP ya viene resuelta por el middleware RealIP.
func byIP(r *http.Request) string { return r.RemoteAddr }

// byUser limita por cuenta, no por IP: lo caro se paga por usuario, y detrás de
// una misma red puede haber varias personas.
func byUser(r *http.Request) string {
	if id, ok := auth.FromContext(r.Context()); ok && id.UID != "" {
		return id.UID
	}
	return r.RemoteAddr
}
