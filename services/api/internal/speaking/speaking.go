// Package speaking corrige lo que decís en voz alta.
//
// El problema de fondo: Whisper a veces "arregla" la gramática al transcribir,
// así que no se puede confiar en la transcripción para detectar errores. La
// salida es que el drill declara de antemano qué pronombres corresponden, y la
// corrección compara contra eso. Si decís "he" hablando de Sofía, el error se ve
// aunque el resto de la oración venga limpia.
package speaking

import (
	"regexp"
	"sort"
	"strings"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/content"
)

// Analysis es lo que se muestra después de hablar.
type Analysis struct {
	Transcript string `json:"transcript"`
	Words      int    `json:"words"`
	// Used: cuántas veces apareció cada pronombre que interesa.
	Used map[string]int `json:"used"`
	// Missing: los que el drill pedía y no aparecieron.
	Missing []string `json:"missing"`
	// Wrong: los que delatan el error de género.
	Wrong []string `json:"wrong"`
	// Correct: usó los que iban y ninguno de los que no.
	Correct bool `json:"correct"`
	// TooShort: no habló lo suficiente como para medir nada.
	TooShort bool `json:"tooShort"`
}

// minWords: menos que esto no es una respuesta, es un titubeo.
const minWords = 6

var wordRe = regexp.MustCompile(`[a-zA-Z']+`)

// Words devuelve las palabras de un texto, en minúsculas.
func Words(text string) []string {
	out := wordRe.FindAllString(strings.ToLower(text), -1)
	for i, w := range out {
		out[i] = strings.Trim(w, "'")
	}
	return out
}

// Analyze corrige la transcripción contra lo que el drill esperaba.
func Analyze(drill *content.Drill, transcript string) Analysis {
	words := Words(transcript)
	// Listas vacías, nunca nil: el cliente no tiene que defenderse de null.
	a := Analysis{
		Transcript: strings.TrimSpace(transcript),
		Words:      len(words),
		Used:       map[string]int{},
		Missing:    []string{},
		Wrong:      []string{},
	}
	if len(words) < minWords {
		a.TooShort = true
		return a
	}

	counts := map[string]int{}
	for _, w := range words {
		counts[w]++
	}

	// Sólo se cuentan los pronombres que el drill nombra: el resto no dice nada.
	for _, p := range append(append([]string{}, drill.Expect...), drill.Avoid...) {
		key := strings.ToLower(p)
		if n := counts[key]; n > 0 {
			a.Used[key] = n
		}
	}

	for _, p := range drill.Expect {
		key := strings.ToLower(p)
		if counts[key] == 0 {
			a.Missing = append(a.Missing, key)
		}
	}
	for _, p := range drill.Avoid {
		key := strings.ToLower(p)
		if counts[key] > 0 {
			a.Wrong = append(a.Wrong, key)
		}
	}
	sort.Strings(a.Missing)
	sort.Strings(a.Wrong)

	if drill.Kind == "free" {
		// Sin consigna de pronombres: alcanza con haber hablado.
		a.Correct = true
		return a
	}
	a.Correct = len(a.Missing) == 0 && len(a.Wrong) == 0
	return a
}
