// Package placement implementa el test de ubicación de la obra 0.
//
// Por habilidad se preguntan los ítems de ubicación de fácil a difícil. Si los
// dos primeros salen bien, la habilidad se da por dominada y no se pregunta el
// resto: no se gasta tiempo en lo que ya sabés (PLAN.md §4).
package placement

import "github.com/BalbianoLuciano/lost-in-translation/services/api/internal/content"

// Answer es una respuesta ya corregida dentro de una corrida.
type Answer struct {
	ItemID  string
	Correct bool
}

type State string

const (
	Plano      State = "plano"
	Suspendida State = "suspendida"
	Calzada    State = "calzada"
)

type SkillOutcome struct {
	SkillID string  `json:"skillId"`
	Asked   int     `json:"asked"`
	Correct int     `json:"correct"`
	Done    bool    `json:"done"`
	State   State   `json:"state"`
	Mastery float32 `json:"mastery"`
}

// earlyStop: con estos aciertos seguidos desde el principio, la habilidad queda dominada.
const earlyStop = 2

func index(answers []Answer) map[string]bool {
	got := make(map[string]bool, len(answers))
	for _, a := range answers {
		if _, seen := got[a.ItemID]; !seen {
			got[a.ItemID] = a.Correct
		}
	}
	return got
}

// skillStatus recorre los ítems de una habilidad y dice cuál sigue.
func skillStatus(items []*content.Item, got map[string]bool) (next *content.Item, asked, correct int) {
	for _, it := range items {
		if asked == earlyStop && correct == earlyStop {
			return nil, asked, correct
		}
		ok, answered := got[it.ID]
		if !answered {
			return it, asked, correct
		}
		asked++
		if ok {
			correct++
		}
	}
	return nil, asked, correct
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
		next, asked, correct := skillStatus(c.PlacementItems(skill), got)
		o := SkillOutcome{SkillID: skill, Asked: asked, Correct: correct, Done: next == nil}
		o.State, o.Mastery = classify(asked, correct)
		out = append(out, o)
	}
	return out
}

func classify(asked, correct int) (State, float32) {
	switch {
	case asked == 0:
		return Plano, 0
	case correct == asked && asked >= earlyStop:
		return Calzada, 0.9
	case float32(correct)/float32(asked) >= 0.66:
		return Suspendida, 0.6
	case correct >= 1:
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
