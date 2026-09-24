// Package placement implementa el test de ubicación de la obra 0.
//
// Por habilidad se preguntan los ítems de ubicación de fácil a difícil. Si los
// dos primeros salen bien, la habilidad se da por dominada y no se pregunta el
// resto: no se gasta tiempo en lo que ya sabés (PLAN.md §4).
package placement

import "github.com/BalbianoLuciano/lost-in-translation/services/api/internal/content"

// Answer es una respuesta ya corregida dentro de una corrida.
type Answer struct {
	ItemID string
	// Correct es el veredicto que se le mostró al usuario.
	Correct bool
	// Consulted marca que se abrió el glosario mientras se respondía. Un acierto
	// consultado vale medio punto y no alcanza para dar la habilidad por dominada.
	Consulted bool
}

// score es cuánto vale una respuesta al medir el nivel.
func (a Answer) score() float32 {
	switch {
	case !a.Correct:
		return 0
	case a.Consulted:
		return 0.5
	default:
		return 1
	}
}

type State string

const (
	Plano      State = "plano"
	Suspendida State = "suspendida"
	Calzada    State = "calzada"
	// Oxidada no la produce el diagnóstico: una pieza se oxida recién cuando
	// estuvo calzada y después el dominio se le cayó (PLAN.md §8). Vive acá igual
	// porque es uno de los cuatro estados de una pieza, y así deja de andar
	// suelta como literal por el resto del código.
	Oxidada State = "oxidada"
)

type SkillOutcome struct {
	SkillID string `json:"skillId"`
	Asked   int    `json:"asked"`
	// Score suma 1 por acierto y 0.5 si se consultó el glosario.
	Score   float32 `json:"score"`
	Done    bool    `json:"done"`
	State   State   `json:"state"`
	Mastery float32 `json:"mastery"`
}

// earlyStop: con estos aciertos seguidos desde el principio, la habilidad queda dominada.
const earlyStop = 2

func index(answers []Answer) map[string]Answer {
	got := make(map[string]Answer, len(answers))
	for _, a := range answers {
		if _, seen := got[a.ItemID]; !seen {
			got[a.ItemID] = a
		}
	}
	return got
}

// skillStatus recorre los ítems de una habilidad y dice cuál sigue.
func skillStatus(items []*content.Item, got map[string]Answer) (next *content.Item, asked int, score float32) {
	for _, it := range items {
		// Dos de dos, sin consultar: se da por dominada y no se pregunta más.
		if asked == earlyStop && score == earlyStop {
			return nil, asked, score
		}
		a, answered := got[it.ID]
		if !answered {
			return it, asked, score
		}
		asked++
		score += a.score()
	}
	return nil, asked, score
}

// Next devuelve el próximo ítem de la parte, o nil si la parte terminó.
func Next(c *content.Catalog, part *content.PlacementPart, answers []Answer) *content.Item {
	got := index(answers)
	for _, skill := range part.Skills {
		if next, _, _ := skillStatus(c.PlacementItems(skill), got); next != nil {
			return next
		}
	}
	return nil
}

func Outcomes(c *content.Catalog, part *content.PlacementPart, answers []Answer) []SkillOutcome {
	got := index(answers)
	out := make([]SkillOutcome, 0, len(part.Skills))
	for _, skill := range part.Skills {
		next, asked, score := skillStatus(c.PlacementItems(skill), got)
		o := SkillOutcome{SkillID: skill, Asked: asked, Score: score, Done: next == nil}
		o.State, o.Mastery = classify(asked, score)
		out = append(out, o)
	}
	return out
}

func classify(asked int, score float32) (State, float32) {
	switch {
	case asked == 0:
		return Plano, 0
	case score == float32(asked) && asked >= earlyStop:
		return Calzada, 0.9
	case score/float32(asked) >= 0.66:
		return Suspendida, 0.6
	case score >= 1:
		return Plano, 0.3
	default:
		return Plano, 0.1
	}
}

type Progress struct {
	Answered int `json:"answered"`
	// Max es el máximo posible de preguntas; puede terminar antes por las
	// habilidades dominadas.
	Max int `json:"max"`
}

func ProgressOf(c *content.Catalog, part *content.PlacementPart, answers []Answer) Progress {
	got := index(answers)
	p := Progress{}
	for _, skill := range part.Skills {
		items := c.PlacementItems(skill)
		next, asked, _ := skillStatus(items, got)
		p.Answered += asked
		if next == nil {
			p.Max += asked
		} else {
			p.Max += len(items)
		}
	}
	return p
}
