package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/achievement"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/ai"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/auth"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/budget"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/config"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/content"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/db"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/httpapi"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/placement"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/session"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/speaking"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/store"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/tutor"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("la API terminó con error", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	catalog, err := content.Load()
	if err != nil {
		return err
	}

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		return err
	}

	verifier, err := newVerifier(ctx, cfg)
	if err != nil {
		return err
	}

	// Sin clave el chat no existe, pero la app funciona igual.
	// Un solo cliente cubre las dos cosas: el chat y la transcripción.
	var llm ai.Client
	var stt ai.Transcriber
	if cfg.GroqAPIKey != "" {
		groq := ai.NewGroq(cfg.GroqAPIKey, cfg.GroqModel)
		llm, stt = groq, groq
	}
	if cfg.UseFakeTranscriber() {
		stt = ai.FakeTranscriber{Text: cfg.FakeTranscript}
		logger.Warn("transcripción de mentira activada: sólo para tests")
	}

	// El techo de gasto: lo que se le paga al proveedor, por usuario y por día
	// entero. Sin esto, una sola clave alcanza para que un usuario deje sin
	// servicio a todos los demás.
	limits := map[budget.Kind]budget.Limits{
		budget.Ask:      {PerUser: cfg.Ask.PerUser, Global: cfg.Ask.Global},
		budget.Speaking: {PerUser: cfg.Speaking.PerUser, Global: cfg.Speaking.Global},
	}
	bud := budget.New(pool, limits)

	gate := httpapi.NewGate(cfg.AllowedEmails, cfg.OpenSignups())
	if cfg.OpenSignups() {
		logger.Warn("altas abiertas: cualquiera con cuenta de Google puede registrarse")
	} else if len(cfg.AllowedEmails) == 0 {
		logger.Warn("sin ALLOWED_EMAILS: no se aceptan altas nuevas, los usuarios existentes siguen entrando")
	}

	srv := &http.Server{
		Addr: ":" + cfg.Port,
		Handler: httpapi.NewRouter(httpapi.Deps{
			Users:        store.New(pool),
			Gate:         gate,
			Placement:    placement.NewService(pool, catalog),
			Session:      session.NewService(pool, catalog),
			Achievements: achievement.NewService(pool, catalog),
			Tutor:        tutor.New(pool, catalog, llm, bud),
			Speaking:     speaking.NewService(pool, catalog, stt, bud),
			Catalog:      catalog,
			DB:           pool,
			Verifier:     verifier,
			CORSOrigins:  cfg.CORSOrigins,
			Logger:       logger,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		logger.Info("API escuchando", "port", cfg.Port, "env", cfg.Env, "auth", cfg.AuthMode, "content", catalog.Version, "ai", llm != nil)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		logger.Info("apagando la API")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func newVerifier(ctx context.Context, cfg config.Config) (auth.Verifier, error) {
	if cfg.AuthMode == config.AuthDev {
		return auth.DevVerifier{}, nil
	}
	return auth.NewFirebaseVerifier(ctx, cfg.FirebaseProjectID, cfg.FirebaseCredentialsJSON)
}
