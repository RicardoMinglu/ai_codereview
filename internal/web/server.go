package web

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/RicardoMinglu/ai_codereview/internal/config"
	"github.com/RicardoMinglu/ai_codereview/internal/notify"
	"github.com/RicardoMinglu/ai_codereview/internal/project"
	"github.com/RicardoMinglu/ai_codereview/internal/report"
	"github.com/RicardoMinglu/ai_codereview/internal/reviewer"
	"github.com/RicardoMinglu/ai_codereview/internal/webhook"
)

type Server struct {
	cfg      *config.Config
	mux      *http.ServeMux
	store    report.Store
	proj     project.Reader
	reviewer *reviewer.Reviewer
	notifier notify.Notifier
}

func NewServer(cfg *config.Config, proj project.Reader, store report.Store, rev *reviewer.Reviewer, notifier notify.Notifier) *Server {
	s := &Server{
		cfg:      cfg,
		mux:      http.NewServeMux(),
		store:    store,
		proj:     proj,
		reviewer: rev,
		notifier: notifier,
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	// Webhook endpoint
	wh := webhook.NewHandler(s.cfg, s.proj, s.reviewer, s.store, s.notifier)
	s.mux.HandleFunc("POST /webhook/github", wh.Handle)

	// Web UI
	h := NewHandler(s.cfg, s.store, s.proj, wh)
	s.mux.HandleFunc("GET /", h.Index)
	s.mux.HandleFunc("GET /report/{id}", h.Report)
	s.mux.HandleFunc("POST /report/{id}/retry", h.Retry)
	s.mux.HandleFunc("GET /setup", h.SetupRedirect)

	// Admin UI - Project Management
	admin := NewAdminHandler(s.store)
	s.mux.HandleFunc("GET /admin/projects", admin.ProjectsPage)
	s.mux.HandleFunc("POST /admin/projects/add", admin.AddProject)
	s.mux.HandleFunc("POST /admin/projects/edit", admin.EditProject)
	s.mux.HandleFunc("POST /admin/projects/delete", admin.DeleteProject)

	// Health check
	s.mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})
}

func (s *Server) Start(ctx context.Context) error {
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", s.cfg.Server.Port),
		Handler:      s.mux,
		ReadTimeout:  15 * time.Second,
		// 再次评审等路径可能长时间无响应体写入，过短的 WriteTimeout 会关连接；与 Retry 独立 context 搭配使用
		WriteTimeout: 30 * time.Minute,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("server shutdown error: %v", err)
		}
	}()

	log.Printf("server starting on :%d", s.cfg.Server.Port)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}
