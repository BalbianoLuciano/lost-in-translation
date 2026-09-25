package wallet

import (
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/achievement"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/content"
)

// Éste es el test que justifica que la numeración esté escrita a mano, y es el
// más importante del paquete.
//
// El número de pieza es lo único de la app que viaja a la cadena y que no se
// puede corregir después: queda adentro del id del token. El contrato lo recibe
// como índice del catálogo que se le pasa al constructor, o sea
// contracts/script/catalogo.json. Si esa lista y ésta no dicen exactamente lo
// mismo, en el mismo orden, cada reclamo acuña la pieza equivocada y **nada se
// queja**: la firma es válida, el índice existe, el token se acuña. Se descubre
// meses después mirando un explorador de bloques.
//
// Ya pasó una vez, cuando las dos listas se escribieron en paralelo: una
// arrancaba con el cimiento y la otra con la primera habilidad. Estaban corridas
// enteras y las dos suites pasaban.
func TestLaNumeracionCanonicaEsLaQueSeDespliega(t *testing.T) {
	data, err := os.ReadFile("../../../../contracts/script/catalogo.json")
	if err != nil {
		t.Fatalf("no se pudo leer el catálogo del despliegue: %v", err)
	}
	var catalogo struct {
		Distinciones []struct {
			Codigo achievement.Code `json:"codigo"`
		} `json:"distinciones"`
	}
	if err := json.Unmarshal(data, &catalogo); err != nil {
		t.Fatal(err)
	}

	canonica := NumeracionCanonica()
	if len(catalogo.Distinciones) != canonica.Len() {
		t.Fatalf("el catálogo del despliegue tiene %d distinciones y la numeración del servidor %d",
			len(catalogo.Distinciones), canonica.Len())
	}
	for i, d := range catalogo.Distinciones {
		if canonica.Orden()[i] != d.Codigo {
			t.Fatalf("la posición %d se despliega como %q y el servidor la numera como %q.\n"+
				"Las dos listas tienen que decir lo mismo: si no, el voucher firma una pieza "+
				"y el contrato acuña otra, sin error.",
				i, d.Codigo, canonica.Orden()[i])
		}
	}
}

// Y contra el contenido: que no falte ni sobre ninguna distinción.
//
// Acá se compara el conjunto, no el orden. El orden lo manda el catálogo del
// despliegue (test de arriba), que se congela el día que se despliega; el
// contenido puede reordenarse internamente sin que eso importe, pero no puede
// tener temas que el contrato no conozca.
func TestNoFaltaNiSobraNingunaDistincion(t *testing.T) {
	data, err := os.ReadFile("../content/bundle.json")
	if err != nil {
		t.Fatal(err)
	}
	c, err := content.Parse(data)
	if err != nil {
		t.Fatal(err)
	}

	enElContenido := map[achievement.Code]bool{}
	for _, d := range achievement.NewCatalog(c).All() {
		enElContenido[d.Code] = true
	}
	enElContrato := map[achievement.Code]bool{}
	for _, code := range NumeracionCanonica().Orden() {
		enElContrato[code] = true
	}

	for code := range enElContenido {
		if !enElContrato[code] {
			t.Errorf("%q existe en el contenido y el contrato no la conoce: "+
				"hace falta desplegar una versión nueva", code)
		}
	}
	for code := range enElContrato {
		if !enElContenido[code] {
			t.Errorf("%q está en el contrato y ya no existe en el contenido: "+
				"el número queda reservado igual, no se puede reusar", code)
		}
	}
}

// Y a la inversa: toda distinción del catálogo tiene número, y es el mismo que
// su posición. Es lo que usa Voucher para traducir un código a un uint16.
func TestPieza(t *testing.T) {
	n := NumeracionCanonica()

	tests := []struct {
		name string
		code achievement.Code
		want uint16
		ok   bool
	}{
		// Primero las 34 piezas en orden de currículum y después las 7 obras:
		// es la forma del catálogo que recibe el constructor.
		{"la primera pieza", "pieza:tense.present.simple_vs_continuous", 0, true},
		{"la última pieza", "pieza:reported_speech.basic", 33, true},
		// El cimiento no lo puede ganar nadie (no tiene piezas), pero ocupa su
		// lugar igual para que las demás obras no se corran.
		{"el cimiento", "obra:0", 34, true},
		{"una obra en el medio", "obra:2", 36, true},
		{"la última", "obra:6", 40, true},
		{"un código inventado no tiene número", "pieza:no.existe", 0, false},
		{"el vacío tampoco", "", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := n.Pieza(tt.code)
			if ok != tt.ok || (tt.ok && got != tt.want) {
				t.Fatalf("Pieza(%q) = %d, %v; want %d, %v", tt.code, got, ok, tt.want, tt.ok)
			}
		})
	}
}

// Un código repetido dejaría dos distinciones apuntando a la misma pieza del
// contrato. No hay forma razonable de elegir cuál gana, así que no se elige.
func TestNumeracionRechazaRepetidos(t *testing.T) {
	_, err := NuevaNumeracion([]achievement.Code{"obra:0", "pieza:a", "obra:0"})
	if !errors.Is(err, ErrNumeracionAmbigua) {
		t.Fatalf("err = %v, want ErrNumeracionAmbigua", err)
	}
}
