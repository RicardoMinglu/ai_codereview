package web

import (
	"database/sql"
	"embed"
	"errors"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/RicardoMinglu/ai_codereview/internal/config"
	"github.com/RicardoMinglu/ai_codereview/internal/report"
	"github.com/RicardoMinglu/ai_codereview/internal/webhook"
)

//go:embed templates/*.html
var templateFS embed.FS

var templates *template.Template

func init() {
	templates = template.Must(template.New("").Funcs(template.FuncMap{
		"scoreColor": scoreColor,
		"or":         func(a, b string) string { if a != "" { return a }; return b },
		"orBool":     func(a, b bool) bool { return a || b },
		"add":        func(a, b int) int { return a + b },
		"subtract":   func(a, b int) int { return a - b },
		"slice": func(s string, start, end int) string {
			if start < 0 {
				start = 0
			}
			if start >= len(s) {
				return ""
			}
			if end > len(s) {
				end = len(s)
			}
			if end <= start {
				return ""
			}
			return s[start:end]
		},
	}).ParseFS(templateFS, "templates/*.html"))
}

func scoreColor(score int) string {
	switch {
	case score >= 80:
		return "#22c55e" // green
	case score >= 60:
		return "#eab308" // yellow
	default:
		return "#ef4444" // red
	}
}

type Handler struct {
	cfg    *config.Config
	store  report.Store
	wh     *webhook.Handler
}

func NewHandler(cfg *config.Config, store report.Store, wh *webhook.Handler) *Handler {
	return &Handler{cfg: cfg, store: store, wh: wh}
}

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	repo := r.URL.Query().Get("repo")
	pageStr := r.URL.Query().Get("page")
	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}
	pageSize := 20

	reports, total, err := h.store.List(r.Context(), repo, page, pageSize)
	if err != nil {
		log.Printf("list reports error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	totalPages := (total + pageSize - 1) / pageSize

	data := map[string]any{
		"Reports":    reports,
		"Repo":       repo,
		"Page":       page,
		"TotalPages": totalPages,
		"Total":      total,
		"BaseURL":    h.cfg.Server.BaseURL,
	}

	if err := templates.ExecuteTemplate(w, "index.html", data); err != nil {
		log.Printf("render index error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) Report(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	rpt, err := h.store.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Report not found", http.StatusNotFound)
			return
		}
		log.Printf("get report error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Report":  rpt,
		"BaseURL": h.cfg.Server.BaseURL,
	}

	if err := templates.ExecuteTemplate(w, "report.html", data); err != nil {
		log.Printf("render report error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) Retry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if err := h.wh.RetryReview(r.Context(), id); err != nil {
		log.Printf("retry review error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/report/"+id, http.StatusSeeOther)
}
