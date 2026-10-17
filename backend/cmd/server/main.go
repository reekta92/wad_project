package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reekta92/wad_project/backend/internal/api"
	"github.com/reekta92/wad_project/backend/internal/config"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	pool, err := pgxpool.New(ctx, cfg.ReaderURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	app := api.New(cfg, pool)
	finished := make(chan error, 1)
	go func() {
		log.Printf("API listening on %s", cfg.Address)
		if cfg.TLSCert != "" {
			finished <- app.ListenTLS(cfg.Address, cfg.TLSCert, cfg.TLSKey)
		} else {
			finished <- app.Listen(cfg.Address)
		}
	}()
	select {
	case err := <-finished:
		return err
	case <-ctx.Done():
		return app.ShutdownWithTimeout(10 * time.Second)
	}
}
