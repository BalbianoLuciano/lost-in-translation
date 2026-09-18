package speaking

import (
	"testing"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/content"
)

func drill(kind string, expect, avoid []string) *content.Drill {
	return &content.Drill{ID: "d-01", Kind: kind, Expect: expect, Avoid: avoid}
}

func TestWords(t *testing.T) {
	got := Words("She's testing it, and he FIXED the bug.")
	want := []string{"she's", "testing", "it", "and", "he", "fixed", "the", "bug"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestAnalyzePronouns(t *testing.T) {
	d := drill("pronouns", []string{"she"}, []string{"he"})

	tests := []struct {
		name        string
		transcript  string
		wantCorrect bool
		wantWrong   []string
		wantMissing []string
		wantShort   bool
	}{
		{
			name:        "usó el que iba",
			transcript:  "She found the bug yesterday and she is testing the fix today.",
			wantCorrect: true,
		},
		{
			name:       "usó el equivocado",
			transcript: "She found the bug and he is testing the fix today.",
			wantWrong:  []string{"he"},
		},
		{
			name:        "no usó ninguno",
			transcript:  "The bug was found yesterday and the fix is being tested today.",
			wantMissing: []string{"she"},
		},
		{
			name:       "dijo dos palabras",
			transcript: "She fixed it.",
			wantShort:  true,
		},
		{
			name:       "no dijo nada",
			transcript: "",
			wantShort:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := Analyze(d, tt.transcript)
			if a.Correct != tt.wantCorrect {
				t.Errorf("correct = %v, want %v", a.Correct, tt.wantCorrect)
			}
			if a.TooShort != tt.wantShort {
				t.Errorf("tooShort = %v, want %v", a.TooShort, tt.wantShort)
			}
			if len(a.Wrong) != len(tt.wantWrong) {
				t.Errorf("wrong = %v, want %v", a.Wrong, tt.wantWrong)
			}
			if len(a.Missing) != len(tt.wantMissing) {
				t.Errorf("missing = %v, want %v", a.Missing, tt.wantMissing)
			}
		})
	}
}

func TestAnalyzeCountsOnlyTheRelevantPronouns(t *testing.T) {
	d := drill("pronouns", []string{"she", "he"}, nil)
	a := Analyze(d, "She tested it and he deployed it, and they told the client about it.")
	if !a.Correct {
		t.Fatalf("tendría que estar bien: %+v", a)
	}
	if a.Used["she"] != 1 || a.Used["he"] != 1 {
		t.Errorf("used = %v, want she y he una vez cada uno", a.Used)
	}
	if _, counted := a.Used["they"]; counted {
		t.Error("they no estaba en la consigna: no se cuenta")
	}
}

func TestAnalyzeSingularThey(t *testing.T) {
	d := drill("pronouns", []string{"they", "their"}, []string{"he", "she", "his", "her"})
	ok := Analyze(d, "They start on Monday and their laptop is already set up for them.")
	if !ok.Correct {
		t.Fatalf("el singular they tendría que valer: %+v", ok)
	}
	guessed := Analyze(d, "He starts on Monday and his laptop is already set up.")
	if guessed.Correct || len(guessed.Wrong) != 2 {
		t.Fatalf("adivinar el género es el error que se busca: %+v", guessed)
	}
}

// El cliente hace result.wrong.length sin defenderse: nunca puede venir null.
func TestAnalyzeNeverReturnsNilLists(t *testing.T) {
	a := Analyze(drill("pronouns", []string{"she"}, []string{"he"}), "She fixed the flaky test this morning.")
	if a.Missing == nil || a.Wrong == nil || a.Used == nil {
		t.Fatalf("las listas tienen que venir vacías, no nulas: %+v", a)
	}
}

func TestAnalyzeFreeDrillOnlyNeedsYouToTalk(t *testing.T) {
	d := drill("free", nil, nil)
	a := Analyze(d, "Yesterday I reviewed a pull request and today I will deploy the fix.")
	if !a.Correct || a.Words == 0 {
		t.Fatalf("un drill libre se aprueba hablando: %+v", a)
	}
}
