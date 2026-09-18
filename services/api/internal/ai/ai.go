// Package ai habla con el proveedor de modelos.
//
// La interfaz existe para que el resto del código no sepa cuál es: hoy es Groq
// por su free tier, mañana puede ser otro sin tocar el tutor.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

var ErrNotConfigured = errors.New("el chat de IA no está configurado")

type Client interface {
	// Complete devuelve la respuesta del modelo, o un error si no se pudo.
	Complete(ctx context.Context, system, user string) (string, error)
	Model() string
}

const (
	groqURL = "https://api.groq.com/openai/v1/chat/completions"
	// El catálogo de Groq cambia seguido: este es el modelo grande disponible hoy
	// (2026-09). Se puede pisar con GROQ_MODEL sin tocar código.
	DefaultModel = "openai/gpt-oss-120b"
)

type Groq struct {
	key   string
	model string
	http  *http.Client
}

func NewGroq(key, model string) *Groq {
	if model == "" {
		model = DefaultModel
	}
	return &Groq{key: key, model: model, http: &http.Client{Timeout: 30 * time.Second}}
}

func (g *Groq) Model() string { return g.model }

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type request struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
}

type response struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (g *Groq) Complete(ctx context.Context, system, user string) (string, error) {
	body, err := json.Marshal(request{
		Model: g.model,
		Messages: []message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature: 0.2, // es un profesor, no un poeta
		MaxTokens:   500,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, groqURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+g.key)
	req.Header.Set("Content-Type", "application/json")

	res, err := g.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("llamar al modelo: %w", err)
	}
	defer res.Body.Close()

	var parsed response
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("respuesta ilegible del modelo (HTTP %d): %w", res.StatusCode, err)
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("el modelo rechazó la consulta (HTTP %d): %s", res.StatusCode, parsed.Error.Message)
	}
	if res.StatusCode != http.StatusOK || len(parsed.Choices) == 0 {
		return "", fmt.Errorf("el modelo no respondió (HTTP %d)", res.StatusCode)
	}
	return parsed.Choices[0].Message.Content, nil
}
