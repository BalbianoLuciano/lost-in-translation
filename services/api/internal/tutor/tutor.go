// Package tutor responde las preguntas que el glosario no cubre.
//
// Tres cosas lo hacen barato y útil: la respuesta se arma con el contexto real
// (el ejercicio que estás haciendo y lo que ya está escrito en el glosario), se
// cachea por pregunta, y hay un tope diario para no quemar el free tier.
package tutor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/ai"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/content"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/store"
)

var (
	ErrNotConfigured = ai.ErrNotConfigured
	ErrEmpty         = errors.New("la pregunta está vacía")
	ErrTooLong       = errors.New("la pregunta es demasiado larga")
	ErrDailyLimit    = errors.New("llegaste al límite de preguntas por hoy")
)

const (
	MaxQuestion = 500
	// DailyLimit: con esto alcanza de sobra para una hora de estudio, y deja
	// margen en el free tier del proveedor.
	DailyLimit = 60
	// maxHints: cuántas entradas del glosario se le pasan al modelo.
	maxHints = 6
)

// El profesor: quién es, para quién habla y qué NO tiene que hacer.
const systemPrompt = `Sos el profesor de inglés de un desarrollador fullstack argentino de nivel B2 que trabaja con clientes internacionales y apunta a roles de team lead.

Cómo respondés:
- En castellano rioplatense, con voseo. Los ejemplos, en inglés.
- Corto: 120 palabras como máximo. Si alcanza con dos líneas, dos líneas.
- Primero la respuesta concreta, después el porqué. Nada de introducciones.
- Siempre un ejemplo de trabajo real: standup, PR, incidente, reunión con cliente, entrevista.
- Si te preguntan por un verbo, dale las tres formas (base, pasado, participio).
- Si el error viene de traducir literal del castellano, decilo: eso es lo que más le sirve.
- Si no estás seguro, decilo en una línea en vez de inventar.
- No inventes reglas ni cites fuentes. No uses emojis.
- Texto plano: nada de markdown, asteriscos ni títulos. Si necesitás destacar una forma, escribila entre comillas.`

type Tutor struct {
	pool    *pgxpool.Pool
	catalog *content.Catalog
	client  ai.Client
	limit   int
}

func New(pool *pgxpool.Pool, catalog *content.Catalog, client ai.Client) *Tutor {
	return &Tutor{pool: pool, catalog: catalog, client: client, limit: DailyLimit}
}

// WithDailyLimit cambia el tope diario de preguntas.
func (t *Tutor) WithDailyLimit(n int) *Tutor {
	t.limit = n
	return t
}

// Configured dice si hay proveedor: sin clave, el front esconde el chat.
func (t *Tutor) Configured() bool { return t.client != nil }

type Answer struct {
	Answer string `json:"answer"`
	// Cached: la respuesta salió de la caché y no gastó una llamada.
	Cached bool `json:"cached"`
	// Left: cuántas preguntas te quedan hoy.
	Left int `json:"left"`
}

func (t *Tutor) Ask(ctx context.Context, userID pgtype.UUID, question, itemID string) (Answer, error) {
	if t.client == nil {
		return Answer{}, ErrNotConfigured
	}
	question = strings.TrimSpace(question)
	switch {
	case question == "":
		return Answer{}, ErrEmpty
	case len(question) > MaxQuestion:
		return Answer{}, ErrTooLong
	}

	q := store.New(t.pool)
	used, err := q.CountAsksToday(ctx, userID)
	if err != nil {
		return Answer{}, err
	}
	if int(used) >= t.limit {
		return Answer{}, ErrDailyLimit
	}

	prompt := t.userPrompt(question, itemID)
	// El system entra en el hash: si cambia cómo responde el profesor, las
	// respuestas viejas dejan de servir.
	hash := hashOf(t.client.Model(), systemPrompt, prompt)

	if cached, err := q.GetCachedAnswer(ctx, hash); err == nil {
		// La caché no gasta pregunta del día: preguntar lo mismo sale gratis.
		return Answer{Answer: cached.Answer, Cached: true, Left: t.limit - int(used)}, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Answer{}, err
	}

	text, err := t.client.Complete(ctx, systemPrompt, prompt)
	if err != nil {
		return Answer{}, err
	}
	text = strings.TrimSpace(text)

	if err := q.SaveAnswer(ctx, store.SaveAnswerParams{
		PromptHash: hash, Question: question, Context: prompt, Answer: text, Model: t.client.Model(),
	}); err != nil {
		return Answer{}, err
	}
	if err := q.AddAsk(ctx, userID); err != nil {
		return Answer{}, err
	}
	return Answer{Answer: text, Left: t.limit - int(used) - 1}, nil
}

// userPrompt arma la pregunta con su contexto: el ejercicio en pantalla y lo que
// ya está escrito en el glosario sobre las palabras que menciona.
func (t *Tutor) userPrompt(question, itemID string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Pregunta: %s\n", question)

	if it, ok := t.catalog.Item(itemID); ok {
		fmt.Fprintf(&b, "\nEstá haciendo este ejercicio: %q\n", it.Text)
		if sk, ok := t.catalog.Skill(it.Skill); ok {
			fmt.Fprintf(&b, "Tema: %s\n", sk.NameEn)
		}
		fmt.Fprintf(&b, "Regla del ejercicio: %s\n", it.Rule)
	}

	if hints := t.hints(question); len(hints) > 0 {
		b.WriteString("\nDel glosario de la app (usalo si sirve, es lo que ya estudió):\n")
		for _, h := range hints {
			fmt.Fprintf(&b, "- %s\n", h)
		}
	}
	return b.String()
}

// hints busca en el glosario las entradas que menciona la pregunta.
func (t *Tutor) hints(question string) []string {
	words := map[string]bool{}
	for _, w := range strings.Fields(content.Normalize(question)) {
		if len(w) > 2 {
			words[strings.Trim(w, ".,:;¿?¡!\"'()")] = true
		}
	}
	if len(words) == 0 {
		return nil
	}

	type hint struct {
		text string
		rank int
	}
	var out []hint

	for _, v := range t.catalog.Glossary.Verbs {
		if words[content.Normalize(v.Base)] || words[content.Normalize(v.Past)] ||
			words[content.Normalize(v.Participle)] || words[content.Normalize(v.Es)] {
			out = append(out, hint{
				text: fmt.Sprintf("%s / %s / %s (%s). Ej: %s", v.Base, v.Past, v.Participle, v.Es, v.Example),
				rank: 0,
			})
		}
	}
	for _, term := range t.catalog.Glossary.Terms {
		normalized := content.Normalize(term.Term)
		if words[normalized] || strings.Contains(content.Normalize(question), normalized) {
			out = append(out, hint{
				text: fmt.Sprintf("%s = %s. Ej: %s", term.Term, term.Es, term.Example),
				rank: 1,
			})
		}
	}
	// Las reglas de escritura sólo entran si la pregunta va por ahí.
	q := content.Normalize(question)
	if strings.Contains(q, "pasado") || strings.Contains(q, "-ed") || strings.Contains(q, "escrib") {
		for _, r := range t.catalog.Glossary.Rules {
			out = append(out, hint{text: fmt.Sprintf("%s: %s", r.TitleEn, r.WhenEs), rank: 2})
		}
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].rank < out[j].rank })
	if len(out) > maxHints {
		out = out[:maxHints]
	}
	texts := make([]string, 0, len(out))
	for _, h := range out {
		texts = append(texts, h.text)
	}
	return texts
}

func hashOf(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n---\n")))
	return hex.EncodeToString(sum[:])
}
