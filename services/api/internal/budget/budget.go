// Package budget pone el techo de lo que se paga por afuera.
//
// Todo lo que sale de la app hacia el proveedor de IA —una pregunta al profesor,
// un audio a transcribir— se cuenta acá antes de gastarse. Hay dos topes y los
// dos hacen falta:
//
//   - el de cada usuario, para que nadie se lleve el día entero;
//   - el de todos juntos, porque la clave del proveedor es una sola y el tope
//     por usuario no sirve de nada cuando los usuarios son muchos.
//
// Cuando se llega a un tope, la funcionalidad se apaga sola y el resto de la
// app sigue andando: la app ya sabe funcionar sin IA.
package budget

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/store"
)

// Kind es cada cosa que se paga por separado.
type Kind string

const (
	// Ask es una pregunta al profesor de IA.
	Ask Kind = "ask"
	// Speaking es un audio mandado a transcribir.
	Speaking Kind = "speaking"
)

var (
	// ErrUserLimit: a esta persona no le queda margen por hoy.
	ErrUserLimit = errors.New("llegaste al límite de hoy")
	// ErrGlobalLimit: no le queda margen a la app entera por hoy.
	ErrGlobalLimit = errors.New("la app llegó a su tope de uso por hoy")
)

// NoLimit se devuelve como margen restante cuando no hay tope configurado.
const NoLimit = -1

// Limits son los topes diarios de un Kind. Cero significa sin tope.
type Limits struct {
	PerUser int
	Global  int
}

// counter es lo que este paquete necesita de la base.
type counter interface {
	CountUsageToday(ctx context.Context, arg store.CountUsageTodayParams) (int32, error)
	CountUsageTodayAll(ctx context.Context, kind string) (int32, error)
	AddUsage(ctx context.Context, arg store.AddUsageParams) error
}

type Budget struct {
	q      counter
	limits map[Kind]Limits
}

func New(pool *pgxpool.Pool, limits map[Kind]Limits) *Budget {
	return &Budget{q: store.New(pool), limits: limits}
}

// Check mira si queda margen y devuelve cuántos usos le quedan hoy al usuario.
//
// El tope del usuario se informa antes que el global: al que todavía tiene
// margen propio le sirve más saber que la app se quedó sin cupo que creer que
// se quedó sin el suyo.
func (b *Budget) Check(ctx context.Context, userID pgtype.UUID, kind Kind) (int, error) {
	lim := b.limits[kind]

	if lim.PerUser > 0 {
		used, err := b.q.CountUsageToday(ctx, store.CountUsageTodayParams{
			UserID: userID, Kind: string(kind),
		})
		if err != nil {
			return 0, err
		}
		if int(used) >= lim.PerUser {
			return 0, ErrUserLimit
		}
		if err := b.checkGlobal(ctx, kind, lim); err != nil {
			return 0, err
		}
		return lim.PerUser - int(used), nil
	}

	if err := b.checkGlobal(ctx, kind, lim); err != nil {
		return 0, err
	}
	return NoLimit, nil
}

func (b *Budget) checkGlobal(ctx context.Context, kind Kind, lim Limits) error {
	if lim.Global <= 0 {
		return nil
	}
	used, err := b.q.CountUsageTodayAll(ctx, string(kind))
	if err != nil {
		return err
	}
	if int(used) >= lim.Global {
		return ErrGlobalLimit
	}
	return nil
}

// Add anota un uso. Se llama después de que la llamada salió bien: lo que falla
// no se le cobra a nadie.
func (b *Budget) Add(ctx context.Context, userID pgtype.UUID, kind Kind) error {
	return b.q.AddUsage(ctx, store.AddUsageParams{UserID: userID, Kind: string(kind)})
}

// Left son los usos que le quedan hoy al usuario, sin gastar ninguno.
// Devuelve NoLimit si ese Kind no tiene tope por usuario.
func (b *Budget) Left(ctx context.Context, userID pgtype.UUID, kind Kind) (int, error) {
	lim := b.limits[kind]
	if lim.PerUser <= 0 {
		return NoLimit, nil
	}
	used, err := b.q.CountUsageToday(ctx, store.CountUsageTodayParams{
		UserID: userID, Kind: string(kind),
	})
	if err != nil {
		return 0, err
	}
	if left := lim.PerUser - int(used); left > 0 {
		return left, nil
	}
	return 0, nil
}
