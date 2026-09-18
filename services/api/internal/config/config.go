// Package config lee la configuración de la API desde variables de entorno.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type AuthMode string

const (
	// AuthFirebase verifica ID tokens de Firebase Authentication.
	AuthFirebase AuthMode = "firebase"
	// AuthDev acepta tokens "dev:<uid>:<email>". Sólo para desarrollo local.
	AuthDev AuthMode = "dev"
)

type Config struct {
	Env         string
	Port        string
	DatabaseURL string
	CORSOrigins []string
	AuthMode    AuthMode

	FirebaseProjectID string
	// FirebaseCredentialsJSON es opcional: verificar ID tokens sólo necesita el project ID.
	FirebaseCredentialsJSON string

	// GroqAPIKey es opcional: sin clave, la app anda igual y el chat no aparece.
	GroqAPIKey string
	GroqModel  string
}

func Load() (Config, error) {
	c := Config{
		Env:                     getenv("APP_ENV", "development"),
		Port:                    getenv("PORT", "8080"),
		DatabaseURL:             os.Getenv("DATABASE_URL"),
		CORSOrigins:             splitList(getenv("CORS_ORIGINS", "http://localhost:5173")),
		AuthMode:                AuthMode(getenv("AUTH_MODE", string(AuthFirebase))),
		FirebaseProjectID:       os.Getenv("FIREBASE_PROJECT_ID"),
		FirebaseCredentialsJSON: os.Getenv("FIREBASE_CREDENTIALS_JSON"),
		GroqAPIKey:              os.Getenv("GROQ_API_KEY"),
		GroqModel:               os.Getenv("GROQ_MODEL"),
	}
	return c, c.validate()
}

func (c Config) IsProduction() bool { return c.Env == "production" }

func (c Config) validate() error {
	var errs []error
	if c.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL es obligatoria"))
	}
	switch c.AuthMode {
	case AuthFirebase:
		if c.FirebaseProjectID == "" {
			errs = append(errs, errors.New("FIREBASE_PROJECT_ID es obligatoria con AUTH_MODE=firebase"))
		}
	case AuthDev:
		if c.IsProduction() {
			errs = append(errs, errors.New("AUTH_MODE=dev no se permite con APP_ENV=production"))
		}
	default:
		errs = append(errs, fmt.Errorf("AUTH_MODE inválido: %q (firebase|dev)", c.AuthMode))
	}
	return errors.Join(errs...)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
