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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	// Deduplicate repos
	seen := make(map[string]bool)
	var out []Result
	for page := 1; len(out) < limit; page++ {
		u := fmt.Sprintf("%s/search/code?q=%s&per_page=%d&page=%d", strings.TrimRight(base, "/"), url.QueryEscape(query), limit, page)
		req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "pkgline/search")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("github search: %w", err)
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read github response: %w", readErr)
		}
		if err := checkResponse(resp.StatusCode, body, token); err != nil {
			return nil, err
		}

		var r githubCodeSearchResponse
		if err := json.Unmarshal(body, &r); err != nil {
			return nil, fmt.Errorf("decode github response: %w", err)
		}
		for _, it := range r.Items {
			repo := it.Repository.FullName
			if repo == "" || seen[repo] {
				continue
			}
			seen[repo] = true
			out = append(out, Result{Repo: repo, URL: it.Repository.HTMLURL, Description: it.Repository.Description, Stars: it.Repository.StargazersCount, Path: it.Path})
			if len(out) >= limit {
				break
			}
		}
		if len(r.Items) == 0 || page*limit >= r.TotalCount {
			break
		}
	}
	return out, nil
}

func checkResponse(status int, body []byte, token string) error {
	var e struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(body, &e)
	if status == 401 || status == 403 {
		if strings.Contains(strings.ToLower(e.Message), "rate limit") && token == "" {
			return fmt.Errorf("GitHub API rate limited or requires auth: set GITHUB_TOKEN env and retry (%s)", e.Message)
		}
		if status == 401 && token == "" {
			return fmt.Errorf("GitHub code search requires authentication: set GITHUB_TOKEN (https://github.com/settings/tokens) and retry")
		}
		if e.Message != "" {
			return fmt.Errorf("github search %d: %s", status, e.Message)
		}
		return fmt.Errorf("github search %d", status)
	}
	if status != http.StatusOK {
		msg := e.Message
		if msg == "" {
			msg = string(body)
			if len(msg) > 200 {
				msg = msg[:200]
			}
		}
		return fmt.Errorf("github search %d: %s", status, msg)
	}
	return nil
}
