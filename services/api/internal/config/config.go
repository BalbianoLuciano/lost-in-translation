// Package config lee la configuración de la API desde variables de entorno.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
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

	// FakeTranscript activa un transcriptor de mentira para los tests de punta a
	// punta. Se ignora en producción.
	FakeTranscript string

	// AllowedEmails es la lista de quienes pueden darse de alta. Vacía en
	// producción significa que no se aceptan altas nuevas: la puerta se cierra,
	// los que ya están adentro siguen entrando.
	AllowedEmails []string

	// Ask y Speaking son los topes diarios de lo que se le paga al proveedor.
	Ask      Limits
	Speaking Limits

	// Chain es opcional: sin las cuatro variables no hay distinciones en la
	// cadena y la app anda igual, como sin clave de Groq.
	Chain Chain
}

// Chain es la configuración de la cadena, tal como llega del entorno. Se
// interpreta en internal/wallet, que es quien sabe qué es una dirección.
//
// SignerKey es el secreto más serio del proyecto. Vive sólo como variable de
// entorno en producción, nunca en el repo ni en un .env commiteado, y el
// contrato tiene setFirmante para rotarla sin redesplegar. No tiene fondos: no
// puede gastar, sólo firmar.
type Chain struct {
	RPCURL    string
	ChainID   uint64
	Contract  string
	SignerKey string
}

// Limits son los topes diarios de un tipo de gasto. Cero es sin tope.
type Limits struct {
	PerUser int
	Global  int
}

// Los topes por omisión. Los de cada usuario están pensados para una hora de
// estudio con margen de sobra; los globales, para que una sola clave de
// proveedor no se queme aunque haya varias personas usando la app el mismo día.
const (
	defaultAskPerUser      = 60
	defaultAskGlobal       = 600
	defaultSpeakingPerUser = 40
	defaultSpeakingGlobal  = 400
)

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
		FakeTranscript:          os.Getenv("FAKE_TRANSCRIPT"),
		AllowedEmails:           splitList(strings.ToLower(os.Getenv("ALLOWED_EMAILS"))),
		Ask: Limits{
			PerUser: getint("ASK_DAILY_PER_USER", defaultAskPerUser),
			Global:  getint("ASK_DAILY_GLOBAL", defaultAskGlobal),
		},
		Speaking: Limits{
			PerUser: getint("SPEAKING_DAILY_PER_USER", defaultSpeakingPerUser),
			Global:  getint("SPEAKING_DAILY_GLOBAL", defaultSpeakingGlobal),
		},
		Chain: Chain{
			RPCURL:    os.Getenv("CHAIN_RPC_URL"),
			ChainID:   uint64(getint("CHAIN_ID", 0)), //nolint:gosec // un chain id no es negativo
			Contract:  os.Getenv("CHAIN_CONTRACT"),
			SignerKey: os.Getenv("CHAIN_SIGNER_KEY"),
		},
	}
	return c, c.validate()
}

func (c Config) IsProduction() bool { return c.Env == "production" }

// OpenSignups dice si cualquiera puede darse de alta. Sólo fuera de producción
// y sólo mientras no haya lista: en producción, sin lista, la puerta está
// cerrada para los que no tienen cuenta todavía.
func (c Config) OpenSignups() bool {
	return len(c.AllowedEmails) == 0 && !c.IsProduction()
}

// UseFakeTranscriber: sólo fuera de producción y sólo si se pidió explícitamente.
func (c Config) UseFakeTranscriber() bool {
	return c.FakeTranscript != "" && !c.IsProduction()
}

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

func getint(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
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
