package wallet

import (
	"errors"
	"fmt"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/achievement"
)

// El mapeo de una distinción a un número, que es lo único que viaja a la cadena.
//
// El voucher lleva un `uint16 pieza` y el contrato lo usa como índice de su
// array `piezas[]`, cargado una sola vez al desplegar. De ahí sale la
// restricción que ordena todo este archivo: **el número es la posición en el
// catálogo en el momento del despliegue, y después no puede cambiar nunca más.**
// Un token ya acuñado guarda su pieza adentro del id (`uint16(tokenId)`), así
// que renumerar no reescribe el token: lo hace mentir.
//
// La tentación es calcularlo al vuelo con el índice de `achievement.Catalog`.
// Anda perfecto hasta el día en que alguien agrega un tema en el medio de
// content/skills.yaml —algo absolutamente razonable de hacer— y ese día todas
// las distinciones posteriores se corren un lugar en silencio: los vouchers
// nuevos apuntan a la pieza equivocada y los tokens viejos quedan dibujando
// otra cosa. No falla nada, no avisa nadie.
//
// Por eso la numeración está escrita acá, a mano, y es la fuente de verdad. Lo
// que se calcula del catálogo es la **verificación**: hay un test que arma la
// numeración desde el contenido real y exige que dé exactamente esto. Si el
// orden del catálogo cambia, ese test rompe, que es lo que se quería: la
// discusión pasa a ser "¿desplegamos una v2 o dejamos el orden como está?", y
// no un bug que aparece meses después en un explorador de bloques.

// ordenCanonico son las 41 distinciones en el orden en que el contrato las
// tiene cargadas. La posición **es** el número de pieza; el índice 0 existe y
// es `obra:0`, el cimiento, que nadie puede ganar (no tiene piezas) pero ocupa
// su lugar igual para que los demás no se corran.
var ordenCanonico = []achievement.Code{
	"obra:0", // 0 · Placement
	"pieza:tense.present.simple_vs_continuous",      // 1
	"pieza:tense.present.stative_verbs",             // 2
	"pieza:tense.past.simple_vs_continuous",         // 3
	"pieza:tense.past.used_to",                      // 4
	"pieza:tense.present_perfect.result_experience", // 5
	"pieza:tense.present_perfect.just_already_yet",  // 6
	"pieza:tense.perfect_vs_past.finished_time",     // 7
	"pieza:tense.perfect_vs_past.for_since_ago",     // 8
	"pieza:tense.perfect_continuous.duration",       // 9
	"pieza:tense.past_perfect.sequence",             // 10
	"pieza:tense.future.will_vs_going_to",           // 11
	"pieza:tense.future.present_for_future",         // 12
	"pieza:tense.future.perfect_and_continuous",     // 13
	"pieza:pronunciation.ed_endings",                // 14
	"obra:1",                                        // 15 · Verb tenses
	"pieza:pronoun.subject_gender",                  // 16
	"pieza:pronoun.possessive_gender",               // 17
	"pieza:article.a_an_the",                        // 18
	"pieza:article.zero",                            // 19
	"pieza:preposition.time_place",                  // 20
	"pieza:preposition.dependent",                   // 21
	"pieza:noun.uncountable",                        // 22
	"pieza:verb.make_do",                            // 23
	"pieza:verb.say_tell",                           // 24
	"pieza:false_friend.common",                     // 25
	"pieza:question.word_order",                     // 26
	"pieza:question.indirect",                       // 27
	"obra:2",                                        // 28 · The small pieces
	"pieza:chunk.standup",                           // 29
	"pieza:chunk.incidents",                         // 30
	"pieza:phrasal.it",                              // 31
	"obra:3",                                        // 32 · IT English
	"pieza:modal.politeness",                        // 33
	"pieza:conditional.first_second",                // 34
	"pieza:conditional.third",                       // 35
	"pieza:passive.voice",                           // 36
	"obra:4",                                        // 37 · The client
	"pieza:reported_speech.basic",                   // 38
	"obra:5",                                        // 39 · Leadership
	"obra:6",                                        // 40 · The interview
}

// ErrNumeracionAmbigua: dos distinciones querrían el mismo número, o hay más de
// las que entran en un uint16. Es un error de programa, no de datos de entrada.
var ErrNumeracionAmbigua = errors.New("wallet: la numeración de piezas es ambigua")

// Numeracion traduce un código de distinción al número que espera el contrato.
type Numeracion struct {
	porCodigo map[achievement.Code]uint16
	orden     []achievement.Code
}

// NuevaNumeracion arma una numeración desde una lista ordenada de códigos.
//
// Es pública y toma la lista por parámetro para que los tests puedan numerar un
// catálogo de fixture sin tocar el canónico, que es el que está desplegado.
func NuevaNumeracion(orden []achievement.Code) (*Numeracion, error) {
	if len(orden) > 1<<16 {
		return nil, fmt.Errorf("%w: %d distinciones no entran en un uint16", ErrNumeracionAmbigua, len(orden))
	}
	n := &Numeracion{porCodigo: make(map[achievement.Code]uint16, len(orden)), orden: orden}
	for i, code := range orden {
		if antes, repetido := n.porCodigo[code]; repetido {
			return nil, fmt.Errorf("%w: %q está en la posición %d y en la %d", ErrNumeracionAmbigua, code, antes, i)
		}
		n.porCodigo[code] = uint16(i)
	}
	return n, nil
}

// NumeracionCanonica es la que está cargada en el contrato. Es la que usa el
// servidor en producción.
func NumeracionCanonica() *Numeracion {
	n, err := NuevaNumeracion(ordenCanonico)
	if err != nil {
		// La lista es una constante del programa: si tiene un repetido, el
		// binario está mal y no hay nada que hacer en tiempo de ejecución.
		panic(err)
	}
	return n
}

// NumeracionDeCatalogo deriva la numeración del orden de currículum del
// contenido. No se usa para firmar: existe para que el test la compare contra
// la canónica y grite si el catálogo se reordenó.
func NumeracionDeCatalogo(c *achievement.Catalog) (*Numeracion, error) {
	orden := make([]achievement.Code, 0, c.Len())
	for _, d := range c.All() {
		orden = append(orden, d.Code)
	}
	return NuevaNumeracion(orden)
}

// Pieza devuelve el número de una distinción.
func (n *Numeracion) Pieza(code achievement.Code) (uint16, bool) {
	p, ok := n.porCodigo[code]
	return p, ok
}

// Orden son los códigos en su orden, para compararlos.
func (n *Numeracion) Orden() []achievement.Code { return n.orden }

func (n *Numeracion) Len() int { return len(n.orden) }
