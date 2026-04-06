package web

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/RicardoMinglu/ai_codereview/internal/project"
	"github.com/RicardoMinglu/ai_codereview/internal/report"
)

// ProjectManager 定义项目配置管理接口
type ProjectManager interface {
	ListProjects(ctx context.Context) ([]project.Record, error)
	GetProject(ctx context.Context, id int) (*project.Record, error)
	AddProject(ctx context.Context, rec *project.Record) error
	UpdateProject(ctx context.Context, rec *project.Record) error
	DeleteProject(ctx context.Context, id int) error
	AnyProjectRow(ctx context.Context) (bool, error)
}

// AdminHandler 处理项目配置管理
type AdminHandler struct {
	store report.Store
}

func NewAdminHandler(store report.Store) *AdminHandler {
	return &AdminHandler{store: store}
}

// ProjectsPage 显示项目配置列表页面
func (h *AdminHandler) ProjectsPage(w http.ResponseWriter, r *http.Request) {
	// 检查是否支持项目管理
	pm, ok := h.store.(ProjectManager)
	if !ok {
		http.Error(w, "项目管理功能仅在 MySQL 存储时可用", http.StatusNotImplemented)
		return
	}

	ctx := r.Context()

	// 获取所有项目
	projects, err := pm.ListProjects(ctx)
	if err != nil {
		log.Printf("list projects error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 转换为模板数据
	type ProjectData struct {
		ID               int
		RepoFullName     string
		Enabled          bool
		GitHubToken      string
		WebhookSecret    string
		ReviewJSON       string
		NotifyJSON       string
		PushBranchesCSV  string
	}

	var projectsData []ProjectData
	for _, p := range projects {
		pd := ProjectData{
			ID:              p.ID,
			RepoFullName:    p.RepoFullName,
			Enabled:         p.Enabled,
			GitHubToken:     p.GitHubToken,
			WebhookSecret:   p.WebhookSecret,
			PushBranchesCSV: strings.Join(p.PushBranches, ", "),
		}
		if len(p.ReviewJSON) > 0 {
			pd.ReviewJSON = string(p.ReviewJSON)
		}
		if len(p.NotifyJSON) > 0 {
			pd.NotifyJSON = string(p.NotifyJSON)
		}
		projectsData = append(projectsData, pd)
	}

	data := map[string]any{
		"Projects": projectsData,
	}

	if err := templates.ExecuteTemplate(w, "projects.html", data); err != nil {
		log.Printf("render projects page error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

// AddProject 添加项目配置
func (h *AdminHandler) AddProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	pm, ok := h.store.(ProjectManager)
	if !ok {
		http.Error(w, "项目管理功能仅在 MySQL 存储时可用", http.StatusNotImplemented)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	rec := &project.Record{
		RepoFullName:   r.FormValue("repo_full_name"),
		Enabled:        r.FormValue("enabled") == "on",
		GitHubToken:    r.FormValue("github_token"),
		WebhookSecret:  r.FormValue("webhook_secret"),
		PushBranches:   project.ParsePushBranchesInput(r.FormValue("push_branches")),
	}

	// 解析 JSON 字段
	if reviewJson := r.FormValue("review_json"); reviewJson != "" {
		if !isValidJSON(reviewJson) {
			http.Error(w, "评审配置 JSON 格式错误", http.StatusBadRequest)
			return
		}
		rec.ReviewJSON = []byte(reviewJson)
	}

	if notifyJson := r.FormValue("notify_json"); notifyJson != "" {
		if !isValidJSON(notifyJson) {
			http.Error(w, "通知配置 JSON 格式错误", http.StatusBadRequest)
			return
		}
		rec.NotifyJSON = []byte(notifyJson)
	}

	if err := pm.AddProject(r.Context(), rec); err != nil {
		log.Printf("add project error: %v", err)
		http.Error(w, "添加项目失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/projects", http.StatusSeeOther)
}

// EditProject 编辑项目配置
func (h *AdminHandler) EditProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	pm, ok := h.store.(ProjectManager)
	if !ok {
		http.Error(w, "项目管理功能仅在 MySQL 存储时可用", http.StatusNotImplemented)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	idStr := r.FormValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid project ID", http.StatusBadRequest)
		return
	}

	rec := &project.Record{
		RepoFullName:   r.FormValue("repo_full_name"),
		Enabled:        r.FormValue("enabled") == "on",
		GitHubToken:    r.FormValue("github_token"),
		WebhookSecret:  r.FormValue("webhook_secret"),
		PushBranches:   project.ParsePushBranchesInput(r.FormValue("push_branches")),
	}

	// 解析 JSON 字段
	if reviewJson := r.FormValue("review_json"); reviewJson != "" {
		if !isValidJSON(reviewJson) {
			http.Error(w, "评审配置 JSON 格式错误", http.StatusBadRequest)
			return
		}
		rec.ReviewJSON = []byte(reviewJson)
	}

	if notifyJson := r.FormValue("notify_json"); notifyJson != "" {
		if !isValidJSON(notifyJson) {
			http.Error(w, "通知配置 JSON 格式错误", http.StatusBadRequest)
			return
		}
		rec.NotifyJSON = []byte(notifyJson)
	}

	// 需要在 Record 中添加 ID 字段，这里暂时用 context 传递
	ctx := context.WithValue(r.Context(), "project_id", id)
	if err := pm.UpdateProject(ctx, rec); err != nil {
		log.Printf("update project error: %v", err)
		http.Error(w, "更新项目失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/projects", http.StatusSeeOther)
}

// DeleteProject 删除项目配置
func (h *AdminHandler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	pm, ok := h.store.(ProjectManager)
	if !ok {
		http.Error(w, "项目管理功能仅在 MySQL 存储时可用", http.StatusNotImplemented)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	idStr := r.FormValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid project ID", http.StatusBadRequest)
		return
	}

	if err := pm.DeleteProject(r.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Project not found", http.StatusNotFound)
			return
		}
		log.Printf("delete project error: %v", err)
		http.Error(w, "删除项目失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/projects", http.StatusSeeOther)
}

func isValidJSON(s string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(s), &js) == nil
}
