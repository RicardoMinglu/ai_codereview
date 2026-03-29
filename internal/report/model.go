package report

import "time"

type ReviewReport struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`

	// Source info
	RepoFullName string `json:"repo_full_name"` // e.g. "owner/repo"
	EventType    string `json:"event_type"`      // "push" or "pull_request"
	Ref          string `json:"ref"`             // branch or PR number
	CommitSHA    string `json:"commit_sha"`
	Author       string `json:"author"`
	CommitMsg    string `json:"commit_msg"`
	HTMLURL      string `json:"html_url"` // link to commit/PR on GitHub

	// Review result
	Status   string    `json:"status"`   // success | failed | pending
	ErrorMsg string    `json:"error_msg,omitempty"` // 失败原因
	Score    int       `json:"score"`    // 0-100
	Summary  string    `json:"summary"`  // overall summary
	Issues   []Issue   `json:"issues"`
	FilesNum int       `json:"files_num"`
	LinesNum int       `json:"lines_num"`
	AIModel  string    `json:"ai_model"` // which model was used
	Duration float64   `json:"duration"` // review duration in seconds
}

type Issue struct {
	File     string `json:"file"`
	Line     int    `json:"line,omitempty"`
	Severity string `json:"severity"` // critical, warning, info, suggestion
	Category string `json:"category"` // bug, security, performance, style, best_practice
	Message  string `json:"message"`
	Suggest  string `json:"suggest,omitempty"` // suggested fix
}

type SeverityCounts struct {
	Critical   int `json:"critical"`
	Warning    int `json:"warning"`
	Info       int `json:"info"`
	Suggestion int `json:"suggestion"`
}

func (r *ReviewReport) SeverityCounts() SeverityCounts {
	var counts SeverityCounts
	if r.Issues == nil {
		return counts
	}
	for _, issue := range r.Issues {
		switch issue.Severity {
		case "critical":
			counts.Critical++
		case "warning":
			counts.Warning++
		case "info":
			counts.Info++
		case "suggestion":
			counts.Suggestion++
		}
	}
	return counts
}
