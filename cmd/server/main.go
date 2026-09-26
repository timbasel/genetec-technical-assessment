package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/timbasel/genetec-technical-assessment/internal/api"
	"github.com/timbasel/genetec-technical-assessment/internal/books"
	"github.com/timbasel/genetec-technical-assessment/internal/health"
	"github.com/timbasel/genetec-technical-assessment/internal/store"
	"github.com/timbasel/genetec-technical-assessment/internal/utils"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, err := store.Open(ctx, utils.GetEnv("DATABASE_PATH", "data/books.db"))
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer store.Close()

	addr := utils.GetEnv("HTTP_ADDRESS", "127.0.0.1:8080")
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}
	defer listener.Close()

	server := &http.Server{
		Addr:              addr,
		Handler:           api.NewServer(books.NewService(store), health.NewService(store)).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() { serverErr <- server.Serve(listener) }()
	log.Printf("Server listening on %s", listener.Addr())
	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	case <-ctx.Done():
		stop()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			return fmt.Errorf("shutdown HTTP server: %w", err)
		}
		return nil
	}
}
