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

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/auth"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/config"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/content"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/db"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/httpapi"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/placement"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/session"
	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/store"
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

	srv := &http.Server{
		Addr: ":" + cfg.Port,
		Handler: httpapi.NewRouter(httpapi.Deps{
			Users:       store.New(pool),
			Placement:   placement.NewService(pool, catalog),
			Session:     session.NewService(pool, catalog),
			Catalog:     catalog,
			DB:          pool,
			Verifier:    verifier,
			CORSOrigins: cfg.CORSOrigins,
			Logger:      logger,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		logger.Info("API escuchando", "port", cfg.Port, "env", cfg.Env, "auth", cfg.AuthMode, "content", catalog.Version)
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
