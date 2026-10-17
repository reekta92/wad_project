package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/reekta92/wad_project/backend/internal/api"
	"github.com/reekta92/wad_project/backend/internal/config"
	"github.com/reekta92/wad_project/backend/internal/store"
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
	databases, err := store.Open(ctx, cfg)
	if err != nil {
		return err
	}
	defer databases.Close()
	app := api.New(cfg, databases)
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
