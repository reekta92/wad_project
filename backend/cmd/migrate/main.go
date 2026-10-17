package main

import (
	"context"
	"log"
	"time"

	"github.com/reekta92/wad_project/backend/internal/config"
	"github.com/reekta92/wad_project/backend/internal/store"
)

func main() {
	u, err := config.MigrationURL()
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := store.Migrate(ctx, u); err != nil {
		log.Fatal(err)
	}
	log.Print("Database migrations applied.")
}
