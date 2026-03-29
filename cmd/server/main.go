package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/RicardoMinglu/ai_codereview/internal/ai"
	"github.com/RicardoMinglu/ai_codereview/internal/config"
	"github.com/RicardoMinglu/ai_codereview/internal/notify"
	"github.com/RicardoMinglu/ai_codereview/internal/report"
	"github.com/RicardoMinglu/ai_codereview/internal/reviewer"
	"github.com/RicardoMinglu/ai_codereview/internal/web"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	// Load config
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// Init store（默认 mysql，见 config 与 store_factory）
	store, err := report.NewStore(&cfg.Storage)
	if err != nil {
		log.Fatalf("init store: %v", err)
	}
	defer store.Close()

	// Init AI provider
	provider, err := ai.NewProvider(&cfg.AI)
	if err != nil {
		log.Fatalf("init AI provider: %v", err)
	}
	log.Printf("AI provider: %s", provider.Name())

	// Init reviewer
	rev := reviewer.New(provider, &cfg.Review)

	// Init notifier
	notifier := notify.NewMultiNotifier(&cfg.Notify)

	// Start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("received signal %v, shutting down...", sig)
		cancel()
	}()

	srv := web.NewServer(cfg, store, rev, notifier)
	if err := srv.Start(ctx); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
