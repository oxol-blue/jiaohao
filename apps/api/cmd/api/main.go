package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"jiaohao/internal/canteen"
	"jiaohao/internal/config"
	"jiaohao/internal/db"
	"jiaohao/internal/httpapi"
	"jiaohao/internal/identity"
	"jiaohao/internal/logx"
	"jiaohao/internal/queue"
	"jiaohao/internal/realtime"
)

func main() {
	log := logx.New(os.Getenv("LOG_LEVEL"))
	cfg, err := config.Load()
	if err != nil {
		log.Error("config.load", map[string]any{"outcome": "error", "error": err.Error()})
		os.Exit(1)
	}
	log = logx.New(cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL, cfg.DatabaseSchema)
	if err != nil {
		log.Error("db.connect", map[string]any{"outcome": "error", "error": err.Error()})
		os.Exit(1)
	}
	defer pool.Close()

	if err := identity.SeedAccounts(ctx, identity.NewStore(pool), cfg, log); err != nil {
		log.Error("auth.seed", map[string]any{"outcome": "error", "error": err.Error()})
		os.Exit(1)
	}
	if err := canteen.SeedDemo(ctx, canteen.NewStore(pool), cfg.SeedStaffStudentID, log); err != nil {
		log.Error("canteen.seed", map[string]any{"outcome": "error", "error": err.Error()})
		os.Exit(1)
	}

	loc, err := time.LoadLocation(cfg.AppTZ)
	if err != nil {
		log.Error("config.tz", map[string]any{"outcome": "error", "error": err.Error()})
		os.Exit(1)
	}

	hub := realtime.NewHub()
	qs := queue.NewService(pool, loc, log, hub)
	queue.StartAutoSkip(ctx, qs, log, time.Second)

	srv := &http.Server{
		Addr:              cfg.APIAddr,
		Handler:           httpapi.NewRouter(cfg, pool, loc, log, qs, hub),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Info("api.listen", map[string]any{"addr": cfg.APIAddr, "outcome": "ok"})
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("api.listen", map[string]any{"outcome": "error", "error": err.Error()})
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("api.shutdown", map[string]any{"outcome": "error", "error": err.Error()})
	}
}
