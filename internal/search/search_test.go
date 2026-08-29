package search

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchGitHubSuccess(t *testing.T) {
	// Mock GitHub
	resp := githubCodeSearchResponse{
		TotalCount: 2,
	}
	resp.Items = append(resp.Items, struct {
		Name       string `json:"name"`
		Path       string `json:"path"`
		Repository struct {
			FullName        string `json:"full_name"`
			HTMLURL         string `json:"html_url"`
			Description     string `json:"description"`
			StargazersCount int    `json:"stargazers_count"`
		} `json:"repository"`
	}{Name: "pkgline.toml", Path: "pkgline.toml"})
	resp.Items[0].Repository.FullName = "user/repo1"
	resp.Items[0].Repository.HTMLURL = "https://github.com/user/repo1"
	resp.Items[0].Repository.Description = "A tool"
	resp.Items[0].Repository.StargazersCount = 42

	resp.Items = append(resp.Items, resp.Items[0])
	resp.Items[1].Repository.FullName = "user/repo2"
	resp.Items[1].Repository.HTMLURL = "https://github.com/user/repo2"

	body, _ := json.Marshal(resp)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "pkgline.toml") {
			t.Fatalf("query missing pkgline.toml: %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	results, err := SearchGitHub(SearchOptions{
		Query:   "json parser",
		Limit:   10,
		BaseURL: srv.URL,
		Client:  srv.Client(),
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("want 2 got %d %+v", len(results), results)
	}
	if results[0].Repo != "user/repo1" || results[0].Stars != 42 {
		t.Fatalf("0 %+v", results[0])
	}
}

func TestSearchGitHubDedup(t *testing.T) {
	resp := githubCodeSearchResponse{TotalCount: 2}
	for i := 0; i < 2; i++ {
		var item struct {
			Name       string `json:"name"`
			Path       string `json:"path"`
			Repository struct {
				FullName        string `json:"full_name"`
				HTMLURL         string `json:"html_url"`
				Description     string `json:"description"`
				StargazersCount int    `json:"stargazers_count"`
			} `json:"repository"`
		}
		item.Name = "pkgline.toml"
		item.Path = "pkgline.toml"
		item.Repository.FullName = "user/same"
		item.Repository.HTMLURL = "https://github.com/user/same"
		resp.Items = append(resp.Items, item)
	}
	body, _ := json.Marshal(resp)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	results, err := SearchGitHub(SearchOptions{Query: "test", BaseURL: srv.URL, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("dedup want 1 got %d", len(results))
	}
}

func TestSearchGitHubAuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Bad credentials"})
	}))
	defer srv.Close()
	_, err := SearchGitHub(SearchOptions{Query: "x", BaseURL: srv.URL, Client: srv.Client(), Token: "bad"})
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("want 401 err got %v", err)
	}
}

func TestSearchGitHubEmptyQuery(t *testing.T) {
	_, err := SearchGitHub(SearchOptions{Query: "  "})
	if err == nil {
		t.Fatal("want empty error")
	}
}
