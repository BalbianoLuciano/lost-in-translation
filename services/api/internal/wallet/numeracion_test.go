package wallet

import (
	"errors"
	"os"
	"testing"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/achievement"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/content"
)

// Éste es el test que justifica que la numeración esté escrita a mano.
//
// El número de pieza es lo único de la app que viaja a la cadena y que no se
// puede corregir después: queda adentro del id del token. Si alguien agrega un
// tema en el medio de content/skills.yaml, todas las distinciones posteriores
// se corren un lugar y los vouchers nuevos empiezan a apuntar a la pieza
// equivocada. Sin este test eso no falla en ningún lado: se descubre meses
// después mirando un explorador de bloques.
//
// Que rompa acá es lo que se quiere. La pregunta que abre no es "¿cómo hago que
// pase?" sino "¿desplegamos una v2 del contrato o dejamos el orden como está?".
func TestLaNumeracionCanonicaEsLaDelCatalogoReal(t *testing.T) {
	data, err := os.ReadFile("../content/bundle.json")
	if err != nil {
		t.Fatal(err)
	}
	c, err := content.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	delCatalogo, err := NumeracionDeCatalogo(achievement.NewCatalog(c))
	if err != nil {
		t.Fatal(err)
	}
	canonica := NumeracionCanonica()

	if delCatalogo.Len() != canonica.Len() {
		t.Fatalf("el catálogo trae %d distinciones y la numeración del contrato tiene %d.\n"+
			"Si el contenido creció, hay que desplegar una versión nueva del contrato: "+
			"las piezas se cargan una sola vez, al desplegar.",
			delCatalogo.Len(), canonica.Len())
	}
	// Las 41 del diseño (SDD §3), que son las que el contrato tiene cargadas.
	if canonica.Len() != 41 {
		t.Fatalf("la numeración tiene %d entradas y tienen que ser 41", canonica.Len())
	}

	for i, code := range delCatalogo.Orden() {
		if canonica.Orden()[i] != code {
			t.Fatalf("la posición %d del catálogo es %q y en el contrato es %q.\n"+
				"El orden del contenido cambió: renumerar haría mentir a los tokens ya acuñados, "+
				"que guardan su pieza adentro del id.",
				i, code, canonica.Orden()[i])
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
		// El cimiento ocupa el índice 0 aunque nadie pueda ganarlo: si no
		// estuviera, todos los demás se correrían un lugar.
		{"el cimiento ocupa el cero", "obra:0", 0, true},
		{"la primera pieza", "pieza:tense.present.simple_vs_continuous", 1, true},
		{"una obra en el medio", "obra:2", 28, true},
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
