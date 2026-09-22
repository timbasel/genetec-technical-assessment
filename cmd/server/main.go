package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/timbasel/genetec-technical-assessment/internal/store"
	"github.com/timbasel/genetec-technical-assessment/internal/utils"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, err := store.Open(ctx, utils.GetEnv("BOOKS_DATABASE_PATH", "data/books.db"))
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
}
