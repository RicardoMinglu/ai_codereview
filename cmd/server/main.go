package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/RicardoMinglu/ai_codereview/internal/ai"
	"github.com/RicardoMinglu/ai_codereview/internal/config"
	"github.com/RicardoMinglu/ai_codereview/internal/notify"
	"github.com/RicardoMinglu/ai_codereview/internal/project"
	"github.com/RicardoMinglu/ai_codereview/internal/report"
	"github.com/RicardoMinglu/ai_codereview/internal/reviewer"
	"github.com/RicardoMinglu/ai_codereview/internal/web"
)

func publicBaseURL(cfg *config.Config) string {
	base := strings.TrimSuffix(strings.TrimSpace(cfg.Server.BaseURL), "/")
	if base == "" {
		base = fmt.Sprintf("http://127.0.0.1:%d", cfg.Server.Port)
	}
	return base
}

func printAccessURLs(baseURL string) {
	line := strings.Repeat("═", 58)
	log.Println()
	log.Println(line)
	log.Println("  访问地址    " + baseURL)
	log.Println(line)
	log.Printf("  • %-14s %s/\n", "评审列表", baseURL)
	log.Printf("  • %-14s %s/admin/projects\n", "项目配置", baseURL)
	log.Printf("  • %-14s %s/webhook/github\n", "GitHub Webhook", baseURL)
	log.Println(line)
	log.Println()
}

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	// Load config
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	store, err := report.NewStore(&cfg.Storage)
	if err != nil {
		log.Fatalf("init store: %v", err)
	}
	defer store.Close()

	ms, ok := store.(*report.MySQLStore)
	if !ok {
		log.Fatal("仅支持 storage.type=mysql：GitHub/评审/通知等逻辑配置从表 github_projects 读取")
	}

	baseURL := publicBaseURL(cfg)
	printAccessURLs(baseURL)

	checkCtx := context.Background()
	hasProject, err := ms.AnyProjectRow(checkCtx)
	if err != nil {
		log.Printf("检查 github_projects: %v", err)
	} else if !hasProject {
		log.Println("  【注意】尚未登记任何 GitHub 仓库，Webhook 不会触发评审。")
		log.Println("          请先完成项目配置：")
		log.Printf("            · %s/admin/projects\n", baseURL)
		log.Printf("            · %s/setup（同上页的快捷入口）\n", baseURL)
		log.Println("          填写各仓库 Token 后，再在 GitHub 仓库中配置 Webhook。")
		log.Println()
	}

	provider, err := ai.NewProvider(&cfg.AI)
	if err != nil {
		log.Fatalf("init AI provider: %v", err)
	}
	log.Printf("AI provider: %s", provider.Name())

	revCfg := config.DefaultReviewConfig()
	rev := reviewer.New(provider, &revCfg)
	notifier := notify.NewMultiNotifier(&config.NotifyConfig{})

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

	projReader := project.Reader(ms)
	srv := web.NewServer(cfg, projReader, store, rev, notifier)
	if err := srv.Start(ctx); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
