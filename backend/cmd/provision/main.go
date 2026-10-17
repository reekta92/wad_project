package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/reekta92/wad_project/backend/internal/config"
	"github.com/reekta92/wad_project/backend/internal/provision"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Demo login configured. Private credentials: .local/demo-accounts.json (repository root).")
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ownerURL, err := config.MigrationURL()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	return provision.Run(ctx, cfg.Issuer, ownerURL, os.Getenv("KEYCLOAK_ADMIN_PASSWORD"), "..")
}
