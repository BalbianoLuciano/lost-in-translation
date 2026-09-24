package config

import "testing"

func TestFakeTranscriberNeverRunsInProduction(t *testing.T) {
	dev := Config{Env: "development", FakeTranscript: "hola"}
	if !dev.UseFakeTranscriber() {
		t.Error("en desarrollo, con texto configurado, tiene que usarse")
	}
	prod := Config{Env: "production", FakeTranscript: "hola"}
	if prod.UseFakeTranscriber() {
		t.Error("en producción no se usa nunca")
	}
	sinTexto := Config{Env: "development"}
	if sinTexto.UseFakeTranscriber() {
		t.Error("sin texto configurado no se activa")
	}
}

func TestOpenSignups(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want bool
	}{
		{name: "en desarrollo y sin lista, la puerta está abierta", cfg: Config{Env: "development"}, want: true},
		{
			name: "en producción y sin lista, no se acepta ninguna alta nueva",
			cfg:  Config{Env: "production"},
		},
		{
			name: "con lista, sólo entra la lista",
			cfg:  Config{Env: "development", AllowedEmails: []string{"uno@example.com"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.OpenSignups(); got != tt.want {
				t.Fatalf("OpenSignups() = %v, want %v", got, tt.want)
			}
		})
	}
}
