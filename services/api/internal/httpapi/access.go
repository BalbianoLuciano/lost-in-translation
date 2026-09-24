package httpapi

import (
	"errors"
	"strings"
)

// ErrNotAllowed: la cuenta existe en Google, pero no está invitada a la app.
var ErrNotAllowed = errors.New("esta cuenta no está habilitada")

// Gate decide quién puede darse de alta.
//
// Cierra la puerta sin dejar a nadie afuera: sólo se consulta cuando el usuario
// todavía no existe. Quien ya tiene cuenta sigue entrando aunque no figure en la
// lista, así que activar la lista en una app en uso no echa a nadie.
type Gate struct {
	allowed map[string]bool
	open    bool
}

// NewGate arma la puerta. Con open en true cualquiera se da de alta: es lo que
// hace falta en desarrollo, y config.OpenSignups lo prohíbe en producción.
func NewGate(emails []string, open bool) *Gate {
	g := &Gate{allowed: make(map[string]bool, len(emails)), open: open}
	for _, e := range emails {
		if e = strings.ToLower(strings.TrimSpace(e)); e != "" {
			g.allowed[e] = true
		}
	}
	return g
}

func (g *Gate) CanRegister(email string) bool {
	if g == nil || g.open {
		return true
	}
	return g.allowed[strings.ToLower(strings.TrimSpace(email))]
}
