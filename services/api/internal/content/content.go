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
	Glossary  Glossary  `json:"glossary"`
	Lessons   []Lesson  `json:"lessons"`
	Drills    []Drill   `json:"drills"`
}

// Drill es un ejercicio oral. En los de pronombres la app sabe de antemano qué
// corresponde, así que la corrección no depende de que la transcripción salga
// perfecta (Whisper a veces "corrige" la gramática).
type Drill struct {
	ID       string   `json:"id"`
	Skill    string   `json:"skill"`
	Kind     string   `json:"kind"` // pronouns | free
	Seconds  int      `json:"seconds"`
	Context  string   `json:"context"`
	PromptEn string   `json:"prompt_en"`
	HintEs   string   `json:"hint_es,omitempty"`
	Expect   []string `json:"expect,omitempty"`
	Avoid    []string `json:"avoid,omitempty"`
}

// Lesson es lo que se lee antes de practicar una habilidad.
type Lesson struct {
	Skill       string        `json:"skill"`
	TitleEn     string        `json:"title_en"`
	GoalEn      string        `json:"goal_en"`
	Minutes     int           `json:"minutes"`
	Blocks      []LessonBlock `json:"blocks"`
	Cheatsheets []string      `json:"cheatsheets,omitempty"`
}

type LessonBlock struct {
	Kind     string       `json:"kind"`
	TitleEn  string       `json:"title_en"`
	BodyEn   string       `json:"body_en"`
	BodyEs   string       `json:"body_es,omitempty"`
	Examples []string     `json:"examples,omitempty"`
	Pairs    []LessonPair `json:"pairs,omitempty"`
}

type LessonPair struct {
	A            string `json:"a"`
	B            string `json:"b"`
	DifferenceEs string `json:"difference_es"`
}

// Glossary es lo que se consulta, no lo que se practica: se busca al instante y
// sin internet desde el panel flotante.
type Glossary struct {
	Verbs        []Verb        `json:"verbs"`
	RegularVerbs []RegularVerb `json:"regular_verbs"`
	Rules        []Rule        `json:"rules"`
	Terms        []Term        `json:"terms"`
	Cheatsheets  []Cheatsheet  `json:"cheatsheets"`
}

// RegularVerb existe por una sola cosa: cómo suena su pasado.
type RegularVerb struct {
	Base    string `json:"base"`
	Past    string `json:"past"`
	Sound   string `json:"sound"` // t | d | id
	Es      string `json:"es"`
	Example string `json:"example"`
	NoteEs  string `json:"note_es,omitempty"`
}

type Verb struct {
	Base       string `json:"base"`
	Past       string `json:"past"`
	Participle string `json:"participle"`
	Es         string `json:"es"`
	Example    string `json:"example"`
	NoteEs     string `json:"note_es,omitempty"`
}

type Rule struct {
	ID       string   `json:"id"`
	TitleEn  string   `json:"title_en"`
	WhenEs   string   `json:"when_es"`
	Examples []string `json:"examples"`
	NoteEs   string   `json:"note_es,omitempty"`
}

type Term struct {
	Term    string `json:"term"`
	Type    string `json:"type"`
	Es      string `json:"es"`
	Example string `json:"example"`
	NoteEs  string `json:"note_es,omitempty"`
}

type CheatRow struct {
	Name    string `json:"name"`
	Form    string `json:"form"`
	UseEs   string `json:"use_es"`
	Example string `json:"example"`
}

type Cheatsheet struct {
	ID        string     `json:"id"`
	TitleEn   string     `json:"title_en"`
	TitleEs   string     `json:"title_es"`
	SummaryEs string     `json:"summary_es"`
	Rows      []CheatRow `json:"rows"`
	NotesEs   []string   `json:"notes_es,omitempty"`
	Skills    []string   `json:"skills,omitempty"`
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
	practiceItems  map[string][]*Item // por habilidad, de fácil a difícil
	lessons        map[string]*Lesson
	drills         map[string]*Drill
	drillsBySkill  map[string][]*Drill
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
		practiceItems:  map[string][]*Item{},
		lessons:        map[string]*Lesson{},
		drills:         map[string]*Drill{},
		drillsBySkill:  map[string][]*Drill{},
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
		} else {
			c.practiceItems[it.Skill] = append(c.practiceItems[it.Skill], it)
		}
	}
	for _, index := range []map[string][]*Item{c.placementItems, c.practiceItems} {
		for skill, items := range index {
			sort.SliceStable(items, func(a, b int) bool { return items[a].Difficulty < items[b].Difficulty })
			index[skill] = items
		}
	}
	for i := range c.Lessons {
		c.lessons[c.Lessons[i].Skill] = &c.Lessons[i]
	}
	for i := range c.Drills {
		d := &c.Drills[i]
		if _, ok := c.skills[d.Skill]; !ok {
			return nil, fmt.Errorf("drill %s: habilidad inexistente %s", d.ID, d.Skill)
		}
		c.drills[d.ID] = d
		c.drillsBySkill[d.Skill] = append(c.drillsBySkill[d.Skill], d)
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

// Lesson devuelve la lección de una habilidad, si existe.
func (c *Catalog) Lesson(skill string) (*Lesson, bool) {
	l, ok := c.lessons[skill]
	return l, ok
}

// Drill devuelve un ejercicio oral por id.
func (c *Catalog) Drill(id string) (*Drill, bool) {
	d, ok := c.drills[id]
	return d, ok
}

// DrillsFor son los ejercicios orales de una habilidad, en el orden del archivo.
func (c *Catalog) DrillsFor(skill string) []*Drill {
	return c.drillsBySkill[skill]
}

// SkillsWithDrills lista las habilidades que tienen práctica oral.
func (c *Catalog) SkillsWithDrills() []string {
	out := make([]string, 0, len(c.drillsBySkill))
	for skill := range c.drillsBySkill {
		out = append(out, skill)
	}
	sort.Strings(out)
	return out
}

// PracticeItems son los ítems que NO son de ubicación: los de practicar.
func (c *Catalog) PracticeItems(skill string) []*Item {
	return c.practiceItems[skill]
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
