package achievement

import (
	"os"
	"reflect"
	"testing"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/content"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/placement"
)

// El contenido de prueba: dos obras con piezas y una vacía, que es el caso del
// cimiento y del refugio en el catálogo real.
const fixture = `{
  "version": "test",
  "obras": [
    {"id": 1, "slug": "pilar", "name": "Pilar", "topic_en": "Verb tenses", "topic_es": "Tiempos verbales",
     "pieces": [
       {"id": "1.1", "name_en": "Present", "name_es": "Presente", "skills": ["a.uno", "a.dos"]},
       {"id": "1.2", "name_en": "Past", "name_es": "Pasado", "skills": ["a.tres"]}
     ]},
    {"id": 2, "slug": "anillo", "name": "Anillo", "topic_en": "Meetings", "topic_es": "Reuniones",
     "pieces": [
       {"id": "2.1", "name_en": "Standup", "name_es": "Standup", "skills": ["b.uno"]}
     ]},
    {"id": 3, "slug": "refugio", "name": "Refugio", "topic_en": "The interview", "topic_es": "La entrevista"}
  ],
  "skills": [
    {"id": "a.uno", "piece": "1.1", "obra": 1, "name_en": "One", "name_es": "Uno"},
    {"id": "a.dos", "piece": "1.1", "obra": 1, "name_en": "Two", "name_es": "Dos"},
    {"id": "a.tres", "piece": "1.2", "obra": 1, "name_en": "Three", "name_es": "Tres"},
    {"id": "b.uno", "piece": "2.1", "obra": 2, "name_en": "Four", "name_es": "Cuatro"}
  ],
  "placement": {"items_per_skill": 0, "parts": []},
  "items": [],
  "glossary": {"verbs": [], "regular_verbs": [], "rules": [], "terms": [], "cheatsheets": []},
  "lessons": [],
  "drills": []
}`

func testCatalog(t *testing.T) *Catalog {
	t.Helper()
	c, err := content.Parse([]byte(fixture))
	if err != nil {
		t.Fatal(err)
	}
	return NewCatalog(c)
}

func TestCatalogHasOneDistincionPerSkillAndPerObra(t *testing.T) {
	cat := testCatalog(t)
	want := []Code{
		"pieza:a.uno", "pieza:a.dos", "pieza:a.tres", "obra:1",
		"pieza:b.uno", "obra:2",
		"obra:3",
	}
	got := make([]Code, 0, cat.Len())
	for _, d := range cat.All() {
		got = append(got, d.Code)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("catálogo = %v\nwant %v", got, want)
	}

	d, ok := cat.Get("pieza:a.dos")
	if !ok || d.Kind != Pieza || d.NameEn != "Two" || d.Obra != 1 || d.ObraName != "Pilar" {
		t.Fatalf("la distinción de pieza trae mal los datos: %+v (ok=%v)", d, ok)
	}
	if d, ok := cat.Get("obra:2"); !ok || d.Kind != Obra || d.NameEn != "Meetings" {
		t.Fatalf("la distinción de obra trae mal los datos: %+v (ok=%v)", d, ok)
	}
	if _, ok := cat.Get("pieza:no.existe"); ok {
		t.Fatal("un código inventado no tendría que existir")
	}
}

// El catálogo real: 34 piezas y 7 obras, las 41 del diseño (SDD §3).
func TestCatalogRealTiene41(t *testing.T) {
	data, err := os.ReadFile("../content/bundle.json")
	if err != nil {
		t.Fatal(err)
	}
	c, err := content.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	cat := NewCatalog(c)
	piezas, obras := 0, 0
	for _, d := range cat.All() {
		switch d.Kind {
		case Pieza:
			piezas++
		case Obra:
			obras++
		}
	}
	if piezas != 34 || obras != 7 || cat.Len() != 41 {
		t.Fatalf("piezas = %d, obras = %d, total = %d; want 34, 7 y 41", piezas, obras, cat.Len())
	}
}

func TestEarned(t *testing.T) {
	tests := []struct {
		name    string
		mastery map[string]placement.State
		want    []Code
	}{
		{
			name:    "sin progreso no hay nada",
			mastery: map[string]placement.State{},
		},
		{
			name: "suspendida no alcanza",
			mastery: map[string]placement.State{
				"a.uno": placement.Suspendida, "a.dos": placement.Plano,
			},
		},
		{
			name:    "una pieza calzada da su distinción",
			mastery: map[string]placement.State{"a.uno": placement.Calzada},
			want:    []Code{"pieza:a.uno"},
		},
		{
			name: "la obra sale recién con todas sus piezas",
			mastery: map[string]placement.State{
				"a.uno": placement.Calzada, "a.dos": placement.Calzada,
			},
			want: []Code{"pieza:a.uno", "pieza:a.dos"},
		},
		{
			name: "todas las piezas de la obra, y sale la obra",
			mastery: map[string]placement.State{
				"a.uno": placement.Calzada, "a.dos": placement.Calzada, "a.tres": placement.Calzada,
			},
			want: []Code{"pieza:a.uno", "pieza:a.dos", "pieza:a.tres", "obra:1"},
		},
		{
			name: "una obra de una sola pieza también",
			mastery: map[string]placement.State{
				"b.uno": placement.Calzada,
			},
			want: []Code{"pieza:b.uno", "obra:2"},
		},
		{
			name: "todo calzado: las 4 piezas y las 2 obras con contenido",
			mastery: map[string]placement.State{
				"a.uno": placement.Calzada, "a.dos": placement.Calzada,
				"a.tres": placement.Calzada, "b.uno": placement.Calzada,
			},
			want: []Code{
				"pieza:a.uno", "pieza:a.dos", "pieza:a.tres", "obra:1",
				"pieza:b.uno", "obra:2",
			},
		},
		{
			// La obra sin piezas no se gana por vacío: "todas sus piezas están
			// calzadas" sería cierto sin haber respondido nada.
			name:    "la obra vacía nunca se gana",
			mastery: map[string]placement.State{"a.uno": placement.Calzada},
			want:    []Code{"pieza:a.uno"},
		},
		{
			// La primera mitad de la decisión central: Earned dice cómo estás
			// hoy, y hoy la pieza oxidada no está calzada.
			name: "oxidarse saca la pieza de la cuenta de hoy",
			mastery: map[string]placement.State{
				"a.uno": "oxidada", "a.dos": placement.Calzada, "a.tres": placement.Calzada,
			},
			want: []Code{"pieza:a.dos", "pieza:a.tres"},
		},
		{
			name: "una habilidad que no está en el catálogo se ignora",
			mastery: map[string]placement.State{
				"no.existe": placement.Calzada, "b.uno": placement.Calzada,
			},
			want: []Code{"pieza:b.uno", "obra:2"},
		},
	}

	cat := testCatalog(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Earned(cat, tt.mastery)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Earned = %v\nwant %v", got, tt.want)
			}
		})
	}
}

// La segunda mitad de la decisión central: perder el dominio no borra la
// distinción, porque lo que se guardó no se vuelve a calcular. Earned es puro,
// así que acá se prueba lo que sí depende de él: la misma entrada da siempre lo
// mismo, y lo ganado antes ya no es asunto suyo. Lo otro —que la fila sobreviva
// al óxido— se prueba contra la base en session.
func TestEarnedEsDeterministaYSinMemoria(t *testing.T) {
	cat := testCatalog(t)
	calzada := map[string]placement.State{
		"a.uno": placement.Calzada, "a.dos": placement.Calzada, "a.tres": placement.Calzada,
	}
	first := Earned(cat, calzada)
	for i := 0; i < 10; i++ {
		if !reflect.DeepEqual(Earned(cat, calzada), first) {
			t.Fatal("dos llamadas con la misma entrada tienen que dar lo mismo")
		}
	}

	oxidada := map[string]placement.State{
		"a.uno": "oxidada", "a.dos": placement.Calzada, "a.tres": placement.Calzada,
	}
	if got := Earned(cat, oxidada); reflect.DeepEqual(got, first) {
		t.Fatal("Earned no tiene que acordarse de lo que devolvió antes")
	}
	// Y al recuperarse vuelve a dar lo mismo que la primera vez.
	if got := Earned(cat, calzada); !reflect.DeepEqual(got, first) {
		t.Fatalf("Earned = %v, want %v", got, first)
	}
}

func TestCodes(t *testing.T) {
	got := Codes([]Code{PiezaCode("a.uno"), ObraCode(7)})
	want := []string{"pieza:a.uno", "obra:7"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Codes = %v, want %v", got, want)
	}
}
