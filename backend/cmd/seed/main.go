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
	created, err := store.Seed(ctx, u)
	if err != nil {
		log.Fatal(err)
	}
	if created {
		log.Print("Demo schedule created.")
	} else {
		log.Print("Existing demo data preserved.")
	}
}
