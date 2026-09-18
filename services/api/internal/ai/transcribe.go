package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

const (
	groqTranscribeURL = "https://api.groq.com/openai/v1/audio/transcriptions"
	// Turbo alcanza y sobra para 30 segundos de voz, y es el más barato en el free tier.
	DefaultSTTModel = "whisper-large-v3-turbo"
)

// Transcriber pasa audio a texto.
type Transcriber interface {
	Transcribe(ctx context.Context, audio io.Reader, filename string) (string, error)
}

// Transcribe manda el audio a Whisper.
//
// Dos decisiones importantes: `temperature=0` para que no invente, y un prompt
// que pide transcripción literal. Whisper tiende a "arreglar" la gramática, y
// justamente lo que se está midiendo son los errores.
func (g *Groq) Transcribe(ctx context.Context, audio io.Reader, filename string) (string, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)

	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, audio); err != nil {
		return "", err
	}
	for field, value := range map[string]string{
		"model":           DefaultSTTModel,
		"language":        "en",
		"response_format": "json",
		"temperature":     "0",
		"prompt":          "Transcribe exactly what the speaker says, including grammar mistakes. Do not correct anything.",
	} {
		if err := w.WriteField(field, value); err != nil {
			return "", err
		}
	}
	if err := w.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, groqTranscribeURL, &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+g.key)
	req.Header.Set("Content-Type", w.FormDataContentType())

	res, err := g.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("transcribir: %w", err)
	}
	defer res.Body.Close()

	var parsed struct {
		Text  string `json:"text"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("respuesta ilegible de la transcripción (HTTP %d): %w", res.StatusCode, err)
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("la transcripción falló (HTTP %d): %s", res.StatusCode, parsed.Error.Message)
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("la transcripción falló (HTTP %d)", res.StatusCode)
	}
	return parsed.Text, nil
}
