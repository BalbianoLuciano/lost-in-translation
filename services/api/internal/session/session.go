// Package session arma la sesión diaria: repaso, lección y práctica.
//
// Las tres decisiones que toma este paquete:
//   - qué ítem toca ahora (el repaso vencido manda, después la práctica del tema);
//   - qué tema se estudia (el más flojo primero, según el diagnóstico y la práctica);
//   - cómo se mueve el dominio de una habilidad con cada respuesta.
package session

import (
	"sort"
	"time"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/content"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/placement"
)

// Block es cada bloque de la sesión (PLAN.md §3).
type Block string

const (
	Review   Block = "review"
	Practice Block = "practice"
)

// Colada: los adobes que suma cada respuesta correcta (design.md §3).
func Colada(it *content.Item) int {
	if it.Type == content.ExplainWhy {
		return 3 // pensar el porqué vale más que acertar la forma
	}
	return 1
}

// ── Qué tema toca estudiar ────────────────────────────────────────────────

// Mastery es lo que la app sabe hoy de una habilidad.
type Mastery struct {
	SkillID string
	State   placement.State
	Mastery float32
}

// Candidate es una habilidad que puede ser la próxima lección.
type Candidate struct {
	SkillID string
	Lesson  *content.Lesson
	State   placement.State
	Mastery float32
	// Started: ya se abrió la lección o se practicó algo del tema.
	Started bool
}

// El orden de urgencia entre estados: lo que está en plano primero.
var stateRank = map[placement.State]int{
	placement.Plano:      0,
	placement.Oxidada:    1,
	placement.Suspendida: 2,
	placement.Calzada:    3,
}

// NextLesson elige el próximo tema.
//
// Un tema empezado se termina: nada de saltar de tema a mitad de camino. Entre
// los que no arrancaron gana el más flojo; con el mismo estado, el de menor
// dominio; y a igualdad, el que viene antes en el currículum.
func NextLesson(c *CatalogView, mastery map[string]Mastery, done, started map[string]bool) *Candidate {
	var out []Candidate
	for _, l := range c.Lessons() {
		if done[l.Skill] {
			continue
		}
		m, ok := mastery[l.Skill]
		if !ok {
			m = Mastery{SkillID: l.Skill, State: placement.Plano}
		}
		out = append(out, Candidate{
			SkillID: l.Skill, Lesson: l, State: m.State, Mastery: m.Mastery, Started: started[l.Skill],
		})
	}
	if len(out) == 0 {
		return nil
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Started != b.Started {
			return a.Started
		}
		if ra, rb := stateRank[a.State], stateRank[b.State]; ra != rb {
			return ra < rb
		}
		if a.Mastery != b.Mastery {
			return a.Mastery < b.Mastery
		}
		return c.Order(a.SkillID) < c.Order(b.SkillID)
	})
	return &out[0]
}

// CatalogView es lo que este paquete necesita del catálogo: se define acá para
// poder probar la lógica sin cargar el bundle entero.
type CatalogView struct {
	lessons []*content.Lesson
	order   map[string]int
}

func NewCatalogView(c *content.Catalog) *CatalogView {
	v := &CatalogView{order: map[string]int{}}
	for i := range c.Lessons {
		v.lessons = append(v.lessons, &c.Lessons[i])
	}
	n := 0
	for _, o := range c.Obras {
		for _, p := range o.Pieces {
			for _, s := range p.Skills {
				v.order[s] = n
				n++
			}
		}
	}
	sort.SliceStable(v.lessons, func(i, j int) bool {
		return v.order[v.lessons[i].Skill] < v.order[v.lessons[j].Skill]
	})
	return v
}

func (v *CatalogView) Lessons() []*content.Lesson { return v.lessons }

func (v *CatalogView) Order(skill string) int {
	if n, ok := v.order[skill]; ok {
		return n
	}
	return 1 << 30
}

// ── Cómo se mueve el dominio ──────────────────────────────────────────────

// Attempt es una respuesta de práctica o repaso, de la más nueva a la más vieja.
type Attempt struct {
	Correct   bool
	Consulted bool
}

// MinAttempts es cuántas respuestas hacen falta para mover el estado: con menos,
// una distracción cambiaría el mapa.
const MinAttempts = 4

// Los umbrales del dominio (PLAN.md §8).
const (
	// calzadaFloor: de acá para arriba la pieza está firme.
	calzadaFloor = 0.85
	// oxidoFloor: una pieza que estuvo calzada y baja de acá chorrea óxido.
	oxidoFloor = 0.7
	// suspendidaFloor: la mitad para arriba es "lo estás aprendiendo".
	suspendidaFloor = 0.5
)

// RecomputeMastery recalcula el dominio de una habilidad con sus últimos
// intentos. Los más nuevos pesan más: lo de hoy dice más que lo de la semana
// pasada. Un acierto consultado vale medio, igual que en el diagnóstico.
//
// wasCalzada dice si la pieza estuvo calzada alguna vez, y es lo que habilita el
// óxido. Hace falta memoria y no alcanza con el estado anterior: como el
// promedio es pesado, el dominio no se desploma de un intento para el otro, así
// que una pieza calzada siempre pasa por suspendida en la bajada. Mirando sólo
// el estado de ayer, cuando por fin cruzaba el piso ya venía de suspendida y
// terminaba en plano: el óxido no le tocaba nunca a nadie.
func RecomputeMastery(previous placement.State, wasCalzada bool, attempts []Attempt) (placement.State, float32) {
	if len(attempts) < MinAttempts {
		return previous, masteryFor(previous)
	}
	var sum, weights float32
	w := float32(1)
	for _, a := range attempts { // vienen de la más nueva a la más vieja
		score := float32(0)
		if a.Correct {
			score = 1
			if a.Consulted {
				score = 0.5
			}
		}
		sum += score * w
		weights += w
		w *= 0.85
	}
	score := sum / weights

	switch {
	case score >= calzadaFloor:
		// Volver a quedar firme limpia el óxido; la memoria de haber calzado no
		// se borra, y por eso la próxima caída vuelve a oxidar.
		return placement.Calzada, score
	case wasCalzada && score < oxidoFloor:
		// Estaba firme y se cayó: eso es óxido, no volver a empezar. Va antes que
		// suspendida porque una pieza que se afloja no está aprendiéndose, se
		// está perdiendo, y el mapa la tiene que llamar a repaso.
		return placement.Oxidada, score
	// La mitad para arriba es "lo estás aprendiendo": entra acá el que acierta
	// todo pero consultando el glosario, que sabe la regla y todavía no la tiene.
	case score >= suspendidaFloor:
		return placement.Suspendida, score
	default:
		return placement.Plano, score
	}
}

func masteryFor(s placement.State) float32 {
	switch s {
	case placement.Calzada:
		return 0.9
	case placement.Suspendida:
		return 0.6
	case placement.Oxidada:
		return 0.5
	default:
		return 0.2
	}
}

// ── El jornal ─────────────────────────────────────────────────────────────

// MinAnswersForStreak es lo mínimo que cuenta como día de obra.
const MinAnswersForStreak = 5

// Day es un día con actividad.
type Day struct {
	Date    time.Time
	Answers int
}

// Jornal cuenta los días seguidos de obra hasta hoy. Se permite un día libre
// entre medio (design.md §3): dos días sin nada cortan la racha.
func Jornal(days []Day, today time.Time) int {
	valid := make([]time.Time, 0, len(days))
	for _, d := range days {
		if d.Answers >= MinAnswersForStreak {
			valid = append(valid, truncate(d.Date))
		}
	}
	if len(valid) == 0 {
		return 0
	}
	sort.Slice(valid, func(i, j int) bool { return valid[i].After(valid[j]) })

	today = truncate(today)
	if gap := int(today.Sub(valid[0]).Hours() / 24); gap > 1 {
		return 0 // hace más de un día que no hay obra
	}
	streak := 1
	for i := 1; i < len(valid); i++ {
		gap := int(valid[i-1].Sub(valid[i]).Hours() / 24)
		if gap > 2 { // un día libre entre medio no corta
			break
		}
		streak++
	}
	return streak
}

func truncate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
