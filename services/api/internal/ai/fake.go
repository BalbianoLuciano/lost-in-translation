package ai

import (
	"context"
	"io"
)

// FakeTranscriber devuelve siempre el mismo texto, sin llamar a nadie.
//
// Existe para que los tests de punta a punta puedan recorrer la práctica oral
// completa sin gastar transcripciones reales. `config` sólo lo habilita fuera de
// producción.
type FakeTranscriber struct {
	Text string
}

func (f FakeTranscriber) Transcribe(_ context.Context, audio io.Reader, _ string) (string, error) {
	_, _ = io.Copy(io.Discard, audio) // se consume el audio y se descarta
	if f.Text == "" {
		return "She found the bug yesterday and she is testing the fix today.", nil
	}
	return f.Text, nil
}
