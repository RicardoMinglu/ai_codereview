package github

import (
	"context"
	"fmt"
	"strings"

	gh "github.com/google/go-github/v60/github"
	"golang.org/x/oauth2"

	"github.com/RicardoMinglu/ai_codereview/internal/config"
)

type Client struct {
	client *gh.Client
	cfg    *config.GitHubConfig
}

func NewClient(cfg *config.GitHubConfig) *Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: cfg.Token})
	tc := oauth2.NewClient(context.Background(), ts)
	return &Client{
		client: gh.NewClient(tc),
		cfg:    cfg,
	}
}

type DiffResult struct {
	Files    []FileDiff
	TotalAdd int
	TotalDel int
}

type FileDiff struct {
	Filename string
	Status   string // added, removed, modified, renamed
	Patch    string
	AddLines int
	DelLines int
}

// GetCommitDiff fetches the diff for a specific commit.
func (c *Client) GetCommitDiff(ctx context.Context, owner, repo, sha string) (*DiffResult, error) {
	commit, _, err := c.client.Repositories.GetCommit(ctx, owner, repo, sha, nil)
	if err != nil {
		return nil, fmt.Errorf("get commit: %w", err)
	}

	result := &DiffResult{}
	for _, f := range commit.Files {
		fd := FileDiff{
			Filename: f.GetFilename(),
			Status:   f.GetStatus(),
			Patch:    f.GetPatch(),
			AddLines: f.GetAdditions(),
			DelLines: f.GetDeletions(),
		}
		result.Files = append(result.Files, fd)
		result.TotalAdd += fd.AddLines
		result.TotalDel += fd.DelLines
	}
	return result, nil
}

// GetPRDiff fetches the diff for a pull request.
// Supports pagination for PRs with more than 100 files.
func (c *Client) GetPRDiff(ctx context.Context, owner, repo string, prNumber int) (*DiffResult, error) {
	result := &DiffResult{}
	page := 1
	perPage := 100

	for {
		files, resp, err := c.client.PullRequests.ListFiles(ctx, owner, repo, prNumber, &gh.ListOptions{
			Page:    page,
			PerPage: perPage,
		})
		if err != nil {
			return nil, fmt.Errorf("list PR files: %w", err)
		}

		for _, f := range files {
			fd := FileDiff{
				Filename: f.GetFilename(),
				Status:   f.GetStatus(),
				Patch:    f.GetPatch(),
				AddLines: f.GetAdditions(),
				DelLines: f.GetDeletions(),
			}
			result.Files = append(result.Files, fd)
			result.TotalAdd += fd.AddLines
			result.TotalDel += fd.DelLines
		}

		if resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}
	return result, nil
}

// GetCompareCommits fetches the diff between two commits (for push events with multiple commits).
func (c *Client) GetCompareCommits(ctx context.Context, owner, repo, base, head string) (*DiffResult, error) {
	comp, _, err := c.client.Repositories.CompareCommits(ctx, owner, repo, base, head, nil)
	if err != nil {
		return nil, fmt.Errorf("compare commits: %w", err)
	}

	result := &DiffResult{}
	for _, f := range comp.Files {
		fd := FileDiff{
			Filename: f.GetFilename(),
			Status:   f.GetStatus(),
			Patch:    f.GetPatch(),
			AddLines: f.GetAdditions(),
			DelLines: f.GetDeletions(),
		}
		result.Files = append(result.Files, fd)
		result.TotalAdd += fd.AddLines
		result.TotalDel += fd.DelLines
	}
	return result, nil
}

// ParseRepoFullName splits "owner/repo" into owner and repo.
func ParseRepoFullName(fullName string) (owner, repo string, err error) {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid repo full name: %s", fullName)
	}
	return parts[0], parts[1], nil
}
