// Package content carga el banco de contenido compilado desde content/ y
// corrige respuestas de forma determinista.
//
// El bundle lo genera `tools` (Python): `uv run lit-tools build`. Las reglas de
// normalización y tokenización tienen que coincidir con tools/src/lit_tools/schema.py.
package content

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type Bundle struct {
	Version   string    `json:"version"`
	Obras     []Obra    `json:"obras"`
	Skills    []Skill   `json:"skills"`
	Placement Placement `json:"placement"`
	Items     []Item    `json:"items"`
}

type Obra struct {
	ID      int     `json:"id"`
	Slug    string  `json:"slug"`
	Name    string  `json:"name"`
	TopicEn string  `json:"topic_en"`
	TopicEs string  `json:"topic_es"`
	Pieces  []Piece `json:"pieces"`
}

type Piece struct {
	ID     string   `json:"id"`
	NameEn string   `json:"name_en"`
	NameEs string   `json:"name_es"`
	Skills []string `json:"skills"`
}

type Skill struct {
	ID     string `json:"id"`
	Piece  string `json:"piece"`
	Obra   int    `json:"obra"`
	NameEn string `json:"name_en"`
	NameEs string `json:"name_es"`
}

type Placement struct {
	ItemsPerSkill int             `json:"items_per_skill"`
	Parts         []PlacementPart `json:"parts"`
}

type PlacementPart struct {
	ID            string   `json:"id"`
	NameEn        string   `json:"name_en"`
	NameEs        string   `json:"name_es"`
	DescriptionEn string   `json:"description_en"`
	Skills        []string `json:"skills"`
}

type ExplainEs struct {
	Rule    string `json:"rule"`
	Analogy string `json:"analogy"`
	Why     string `json:"why"`
}

type ItemType string

const (
	Cloze      ItemType = "cloze"
	Choice     ItemType = "choice"
	ExplainWhy ItemType = "explain_why"
	FixError   ItemType = "fix_error"
)

type Item struct {
	ID         string    `json:"id"`
	Skill      string    `json:"skill"`
	Type       ItemType  `json:"type"`
	Difficulty int       `json:"difficulty"`
	Placement  bool      `json:"placement"`
	Context    string    `json:"context"`
	Text       string    `json:"text"`
	Rule       string    `json:"rule"`
	ExplainEs  ExplainEs `json:"explain_es"`

	Answers     []string `json:"answers,omitempty"`     // cloze
	Options     []string `json:"options,omitempty"`     // choice, explain_why
	Answer      int      `json:"answer,omitempty"`      // choice, explain_why
	Focus       string   `json:"focus,omitempty"`       // explain_why
	Question    string   `json:"question,omitempty"`    // explain_why
	Wrong       string   `json:"wrong,omitempty"`       // fix_error
	Corrections []string `json:"corrections,omitempty"` // fix_error

	wrongIndex int
}

// Catalog es el bundle indexado para consultas rápidas.
type Catalog struct {
	Bundle
	items          map[string]*Item
	skills         map[string]*Skill
	parts          map[string]*PlacementPart
	placementItems map[string][]*Item // por habilidad, de fácil a difícil
}

func Parse(data []byte) (*Catalog, error) {
	var b Bundle
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("bundle inválido: %w", err)
	}
	c := &Catalog{
		Bundle:         b,
		items:          map[string]*Item{},
		skills:         map[string]*Skill{},
		parts:          map[string]*PlacementPart{},
		placementItems: map[string][]*Item{},
	}
	for i := range c.Skills {
		c.skills[c.Skills[i].ID] = &c.Skills[i]
	}
	for i := range c.Items {
		it := &c.Items[i]
		if _, ok := c.skills[it.Skill]; !ok {
			return nil, fmt.Errorf("ítem %s: habilidad inexistente %s", it.ID, it.Skill)
		}
		if it.Type == FixError {
			it.wrongIndex = wrongTokenIndex(it.Text, it.Wrong)
			if it.wrongIndex < 0 {
				return nil, fmt.Errorf("ítem %s: la palabra %q no está en el texto", it.ID, it.Wrong)
			}
		}
		c.items[it.ID] = it
		if it.Placement {
			c.placementItems[it.Skill] = append(c.placementItems[it.Skill], it)
		}
	}
	for skill, items := range c.placementItems {
		sort.SliceStable(items, func(a, b int) bool { return items[a].Difficulty < items[b].Difficulty })
		c.placementItems[skill] = items
	}
	for i := range c.Placement.Parts {
		p := &c.Placement.Parts[i]
		for _, s := range p.Skills {
			if len(c.placementItems[s]) < c.Placement.ItemsPerSkill {
				return nil, fmt.Errorf("parte %s: %s tiene menos de %d ítems de ubicación", p.ID, s, c.Placement.ItemsPerSkill)
			}
		}
		c.parts[p.ID] = p
	}
	return c, nil
}

func (c *Catalog) Item(id string) (*Item, bool) {
	it, ok := c.items[id]
	return it, ok
}

func (c *Catalog) Skill(id string) (*Skill, bool) {
	s, ok := c.skills[id]
	return s, ok
}

func (c *Catalog) Part(id string) (*PlacementPart, bool) {
	p, ok := c.parts[id]
	return p, ok
}

// PlacementItems devuelve los ítems de ubicación de una habilidad, de fácil a
// difícil, recortados a items_per_skill.
func (c *Catalog) PlacementItems(skill string) []*Item {
	items := c.placementItems[skill]
	if len(items) > c.Placement.ItemsPerSkill {
		items = items[:c.Placement.ItemsPerSkill]
	}
	return items
}

// ── Normalización y tokens (idénticas a schema.py) ─────────────────────────

const punct = ".,!?;:\"()"

func Normalize(s string) string {
	s = strings.NewReplacer("’", "'", "‘", "'").Replace(s)
	s = strings.ToLower(s)
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(strings.TrimRight(s, "."))
}

func Tokens(text string) []string {
	return strings.Split(text, " ")
}

func tokenCore(t string) string {
	return strings.Trim(t, punct)
}

func wrongTokenIndex(text, wrong string) int {
	w := Normalize(wrong)
	for i, t := range Tokens(text) {
		if Normalize(tokenCore(t)) == w {
			return i
		}
	}
	return -1
}
