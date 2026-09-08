// Command router serves an ad exchange over HTTP: it offers each lot a
// publisher posts to the demand partners it suits, and answers with what came
// of it.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/andreychh/auction-router/internal/api"
	"github.com/andreychh/auction-router/internal/auction"
	"github.com/andreychh/auction-router/internal/fakedsp"
	"github.com/andreychh/auction-router/internal/jsonfile"
)

const (
	// The bounds a caller is held to: headers arrive promptly, the whole request
	// soon after, and an idle connection is not kept open indefinitely. The handler
	// bounds the size of a body; these bound the time it may take.
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	idleTimeout       = 60 * time.Second

	// writeTimeout covers holding the auction as well as answering, so it has to
	// outlast the budget one round is given.
	writeTimeout = 10 * time.Second

	// shutdownGrace is how long the auctions already under way are given to finish.
	shutdownGrace = 5 * time.Second
)

// PartnerSource hands over the demand partners the exchange may offer lots to.
// It is declared here rather than beside the store it is served by, so that
// changing stores changes one line.
type PartnerSource interface {
	Load(ctx context.Context) ([]auction.Partner, error)
}

func main() {
	if err := run(); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := parseConfig(os.Args[1:], os.Stderr)
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.level}))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var source PartnerSource = jsonfile.NewPartners(cfg.partners)
	partners, err := source.Load(ctx)
	if err != nil {
		return err
	}

	exchange := auction.NewExchange(auction.NewRoster(partners), fakedsp.NewClient(), cfg.budget)

	mux := http.NewServeMux()
	mux.Handle("POST /auction", api.NewAuctionHandler(exchange, logger))

	// Health is the status code and nothing else. A roster that failed to load
	// stopped the router before it served anything, so there is nothing to report.
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	server := &http.Server{
		Addr:              cfg.address,
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	logger.InfoContext(ctx, "serving auctions",
		"address", server.Addr,
		"partners", len(partners),
		"budget_ms", cfg.budget.Milliseconds(),
	)

	// The buffer is what lets this goroutine end: after Shutdown it sends
	// ErrServerClosed to a channel nobody is reading any more.
	failed := make(chan error, 1)
	go func() { failed <- server.ListenAndServe() }()

	select {
	// ListenAndServe never returns nil, so whatever arrives here is a reason to stop.
	case err := <-failed:
		return fmt.Errorf("serving auctions: %w", err)
	// A signal, and nothing to do but leave the select and shut down.
	case <-ctx.Done():
	}

	// Shutdown waits by the context it is handed, and the signal context is already
	// cancelled: handed that, it would wait for nothing.
	stopping := context.WithoutCancel(ctx)
	logger.InfoContext(stopping, "shutting down", "grace_ms", shutdownGrace.Milliseconds())

	grace, cancel := context.WithTimeout(stopping, shutdownGrace)
	defer cancel()
	if err := server.Shutdown(grace); err != nil {
		return fmt.Errorf("shutting down: %w", err)
	}
	return nil
}
