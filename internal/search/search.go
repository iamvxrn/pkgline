package search

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Result is a repo found containing pkgline.toml
type Result struct {
	Repo        string `json:"repo"` // e.g. "user/repo"
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
	Stars       int    `json:"stars,omitempty"`
	Path        string `json:"path,omitempty"` // file path, usually pkgline.toml
}

// githubCodeSearchResponse mirrors GitHub code search JSON
type githubCodeSearchResponse struct {
	TotalCount int `json:"total_count"`
	Items      []struct {
		Name       string `json:"name"`
		Path       string `json:"path"`
		Repository struct {
			FullName        string `json:"full_name"`
			HTMLURL         string `json:"html_url"`
			Description     string `json:"description"`
			StargazersCount int    `json:"stargazers_count"`
		} `json:"repository"`
	} `json:"items"`
	Message string `json:"message"`
}

// SearchOptions tunes the query
type SearchOptions struct {
	Query string
	Limit int
	Token string // optional GITHUB_TOKEN / GH_TOKEN
	// Client for testing
	Client *http.Client
	// BaseURL for testing (default https://api.github.com)
	BaseURL string
}

// SearchGitHub queries GitHub code search for pkgline.toml
func SearchGitHub(opts SearchOptions) ([]Result, error) {
	q := strings.TrimSpace(opts.Query)
	if q == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 30 {
		limit = 30
	}
	token := opts.Token
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
		if token == "" {
			token = os.Getenv("GH_TOKEN")
		}
	}
	// Build query: pkgline.toml plus user terms
	// Use filename search for precision: "pkgline.toml in:path <terms>"
	query := "pkgline.toml in:path " + q
	base := opts.BaseURL
	if base == "" {
		base = "https://api.github.com"
	}
	u := fmt.Sprintf("%s/search/code?q=%s&per_page=%d", strings.TrimRight(base, "/"), url.QueryEscape(query), limit)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "pkgline/search")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github search: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		// Parse message for hint
		var e struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(body, &e)
		if e.Message != "" && strings.Contains(strings.ToLower(e.Message), "rate limit") && token == "" {
			return nil, fmt.Errorf("GitHub API rate limited or requires auth: set GITHUB_TOKEN env and retry (%s)", e.Message)
		}
		if resp.StatusCode == 401 && token == "" {
			return nil, fmt.Errorf("GitHub code search requires authentication: set GITHUB_TOKEN (https://github.com/settings/tokens) and retry")
		}
		if e.Message != "" {
			return nil, fmt.Errorf("github search %d: %s", resp.StatusCode, e.Message)
		}
		return nil, fmt.Errorf("github search %d", resp.StatusCode)
	}
	if resp.StatusCode != 200 {
		var e struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(body, &e)
		msg := e.Message
		if msg == "" {
			msg = string(body)
			if len(msg) > 200 {
				msg = msg[:200]
			}
		}
		return nil, fmt.Errorf("github search %d: %s", resp.StatusCode, msg)
	}

	var r githubCodeSearchResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("decode github response: %w", err)
	}

	// Deduplicate repos
	seen := make(map[string]bool)
	var out []Result
	for _, it := range r.Items {
		repo := it.Repository.FullName
		if repo == "" || seen[repo] {
			continue
		}
		seen[repo] = true
		out = append(out, Result{
			Repo:        repo,
			URL:         it.Repository.HTMLURL,
			Description: it.Repository.Description,
			Stars:       it.Repository.StargazersCount,
			Path:        it.Path,
		})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}
