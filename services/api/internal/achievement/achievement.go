// Package achievement calcula las distinciones: qué logró una persona, alguna
// vez, con el progreso que tiene hoy.
//
// La decisión que ordena todo el paquete: Earned mira el presente y nada más.
// No sabe qué se ganó antes ni tiene por qué saberlo. Que un logro no se pierda
// cuando el tema se oxida no lo resuelve esta función sino el ON CONFLICT DO
// NOTHING al guardar (docs/sdd-distinciones.md §3). Separarlo así es lo que
// permite probar el cálculo con una tabla, sin base y sin estado escondido.
package achievement

import (
	"strconv"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/content"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/placement"
)

// Code identifica una distinción. Es lo que se guarda en la base y, más
// adelante, lo que va a viajar al contrato: no cambia nunca.
type Code string

// Kind separa las dos formas del código.
type Kind string

const (
	Pieza Kind = "pieza"
	Obra  Kind = "obra"
)

// PiezaCode es la distinción de un tema calzado.
func PiezaCode(skillID string) Code { return Code(string(Pieza) + ":" + skillID) }

// ObraCode es la distinción de una obra entera.
func ObraCode(n int) Code { return Code(string(Obra) + ":" + strconv.Itoa(n)) }

// Distincion es uno de los logros posibles, con lo que hace falta para
// mostrarlo y para decidir si está ganado.
type Distincion struct {
	Code     Code   `json:"code"`
	Kind     Kind   `json:"kind"`
	NameEn   string `json:"nameEn"`
	NameEs   string `json:"nameEs"`
	Obra     int    `json:"obra"`
	ObraName string `json:"obraName"`
	// Skills son las habilidades que hay que tener calzadas para ganarla: una
	// sola en las de pieza, todas las de la obra en las de obra.
	Skills []string `json:"-"`
}

// Catalog son las distinciones posibles, fijas, derivadas del contenido.
type Catalog struct {
	all    []Distincion
	byCode map[Code]int
}

// NewCatalog arma las distinciones desde el contenido: una por habilidad y una
// por obra.
//
// Las obras sin piezas (el cimiento, que es el diagnóstico, y el refugio, que
// todavía no tiene contenido) entran igual en el catálogo —son parte de las 41
// del diseño— pero no se pueden ganar: ver Earned.
func NewCatalog(c *content.Catalog) *Catalog {
	cat := &Catalog{byCode: map[Code]int{}}
	for _, o := range c.Obras {
		var skills []string
		for _, p := range o.Pieces {
			for _, sid := range p.Skills {
				skills = append(skills, sid)
				d := Distincion{
					Code: PiezaCode(sid), Kind: Pieza,
					Obra: o.ID, ObraName: o.Name, Skills: []string{sid},
				}
				if sk, ok := c.Skill(sid); ok {
					d.NameEn, d.NameEs = sk.NameEn, sk.NameEs
				}
				cat.add(d)
			}
		}
		// La obra va al final de su grupo porque es lo último que se gana.
		cat.add(Distincion{
			Code: ObraCode(o.ID), Kind: Obra,
			NameEn: o.TopicEn, NameEs: o.TopicEs,
			Obra: o.ID, ObraName: o.Name, Skills: skills,
		})
	}
	return cat
}

func (c *Catalog) add(d Distincion) {
	c.byCode[d.Code] = len(c.all)
	c.all = append(c.all, d)
}

// All devuelve las distinciones en orden de currículum: las piezas de cada obra
// y después la obra.
func (c *Catalog) All() []Distincion { return c.all }

func (c *Catalog) Len() int { return len(c.all) }

func (c *Catalog) Get(code Code) (Distincion, bool) {
	i, ok := c.byCode[code]
	if !ok {
		return Distincion{}, false
	}
	return c.all[i], true
}

// Earned devuelve, para un progreso dado, todas las distinciones que
// corresponden. Es pura y determinista: la misma entrada da siempre lo mismo,
// en el mismo orden.
//
// Sólo cuenta `calzada`. Una pieza oxidada no aparece acá, y eso es correcto:
// esta función dice cómo estás hoy. Que la distinción ya ganada siga estando es
// cosa de la base, no de este cálculo.
func Earned(c *Catalog, mastery map[string]placement.State) []Code {
	var out []Code
	for _, d := range c.all {
		// Una obra sin piezas no se gana por vacío: "todas sus piezas están
		// calzadas" sería cierto sin haber hecho nada.
		if len(d.Skills) == 0 {
			continue
		}
		if allCalzadas(d.Skills, mastery) {
			out = append(out, d.Code)
		}
	}
	return out
}

func allCalzadas(skills []string, mastery map[string]placement.State) bool {
	for _, s := range skills {
		if mastery[s] != placement.Calzada {
			return false
		}
	}
	return true
}

// Codes pasa los códigos a texto plano, que es como los guarda la base.
func Codes(earned []Code) []string {
	out := make([]string, 0, len(earned))
	for _, c := range earned {
		out = append(out, string(c))
	}
	return out
}
