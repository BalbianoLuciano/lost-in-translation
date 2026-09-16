package content

import (
	"errors"
	"fmt"
	"slices"
)

// Response es lo que manda el cliente. Cada tipo usa un subconjunto:
// cloze → Text; choice y explain_why → Choice; fix_error → TokenIndex + Text.
type Response struct {
	Text       string `json:"text,omitempty"`
	Choice     *int   `json:"choice,omitempty"`
	TokenIndex *int   `json:"tokenIndex,omitempty"`
}

type Result struct {
	Correct bool `json:"correct"`
	// Expected es la respuesta correcta para mostrar (Ley 02: nunca un
	// "incorrecto" sin la respuesta y la regla al lado).
	Expected string `json:"expected"`
	// WrongIndex, en fix_error, es la palabra que estaba mal.
	WrongIndex *int `json:"wrongIndex,omitempty"`
	// PartOk, en fix_error: se marcó la palabra correcta pero la corrección no.
	PartOk bool `json:"partOk,omitempty"`
}

var ErrBadResponse = errors.New("respuesta con formato inválido para este tipo de ítem")

func Grade(it *Item, r Response) (Result, error) {
	switch it.Type {
	case Cloze:
		got := Normalize(r.Text)
		ok := got != "" && slices.ContainsFunc(it.Answers, func(a string) bool { return Normalize(a) == got })
		return Result{Correct: ok, Expected: it.Answers[0]}, nil

	case Choice, ExplainWhy:
		if r.Choice == nil || *r.Choice < 0 || *r.Choice >= len(it.Options) {
			return Result{}, ErrBadResponse
		}
		return Result{Correct: *r.Choice == it.Answer, Expected: it.Options[it.Answer]}, nil

	case FixError:
		if r.TokenIndex == nil || *r.TokenIndex < 0 || *r.TokenIndex >= len(Tokens(it.Text)) {
			return Result{}, ErrBadResponse
		}
		wrong := it.wrongIndex
		rightToken := *r.TokenIndex == wrong
		got := Normalize(r.Text)
		rightFix := got != "" && slices.ContainsFunc(it.Corrections, func(c string) bool { return Normalize(c) == got })
		return Result{
			Correct:    rightToken && rightFix,
			Expected:   fmt.Sprintf("%s → %s", it.Wrong, it.Corrections[0]),
			WrongIndex: &wrong,
			PartOk:     rightToken && !rightFix,
		}, nil
	}
	return Result{}, fmt.Errorf("tipo de ítem desconocido: %s", it.Type)
}

// PublicItem es el ítem sin respuestas, para mandar al cliente.
type PublicItem struct {
	ID         string   `json:"id"`
	Skill      string   `json:"skill"`
	Type       ItemType `json:"type"`
	Difficulty int      `json:"difficulty"`
	Context    string   `json:"context,omitempty"`
	Text       string   `json:"text"`
	Options    []string `json:"options,omitempty"`
	Focus      string   `json:"focus,omitempty"`
	Question   string   `json:"question,omitempty"`
	Tokens     []string `json:"tokens,omitempty"`
}

func (it *Item) Public() PublicItem {
	p := PublicItem{
		ID:         it.ID,
		Skill:      it.Skill,
		Type:       it.Type,
		Difficulty: it.Difficulty,
		Context:    it.Context,
		Text:       it.Text,
		Options:    it.Options,
		Focus:      it.Focus,
		Question:   it.Question,
	}
	if it.Type == FixError {
		p.Tokens = Tokens(it.Text)
	}
	return p
}
