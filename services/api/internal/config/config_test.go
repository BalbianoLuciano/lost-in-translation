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
