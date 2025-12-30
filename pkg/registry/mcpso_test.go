package registry

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewMCPSoClient(t *testing.T) {
	client := NewMCPSoClient()
	if client.BaseURL != "https://mcp.so/api" {
		t.Errorf("BaseURL mismatch: got %q", client.BaseURL)
	}
	if client.HTTPClient == nil {
		t.Error("HTTPClient should not be nil")
	}
}

func TestSearchMockServer(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}

		query := r.URL.Query().Get("q")
		if query == "" {
			t.Error("Expected query parameter")
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"results": [
				{
					"name": "filesystem-mcp",
					"description": "File system operations",
					"url": "github.com/modelcontextprotocol/servers",
					"author": "Anthropic",
					"runtime": "node"
				}
			],
			"total": 1
		}`))
	}))
	defer server.Close()

	client := &MCPSoClient{
		BaseURL:    server.URL,
		HTTPClient: http.DefaultClient,
	}

	results, err := client.Search("filesystem")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results.Results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results.Results))
	}

	if results.Results[0].Name != "filesystem-mcp" {
		t.Errorf("Name mismatch: got %q", results.Results[0].Name)
	}
}

func TestGetMockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/servers/filesystem" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"name": "filesystem",
			"description": "File system MCP server",
			"url": "github.com/modelcontextprotocol/servers",
			"author": "Anthropic",
			"runtime": "node",
			"stars": 100
		}`))
	}))
	defer server.Close()

	client := &MCPSoClient{
		BaseURL:    server.URL,
		HTTPClient: http.DefaultClient,
	}

	result, err := client.Get("filesystem")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result should not be nil")
	}

	if result.Name != "filesystem" {
		t.Errorf("Name mismatch: got %q", result.Name)
	}

	if result.Stars != 100 {
		t.Errorf("Stars mismatch: got %d", result.Stars)
	}
}

func TestGetNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := &MCPSoClient{
		BaseURL:    server.URL,
		HTTPClient: http.DefaultClient,
	}

	result, err := client.Get("nonexistent")
	if err != nil {
		t.Fatalf("Get should not error on 404: %v", err)
	}

	if result != nil {
		t.Error("Result should be nil for 404")
	}
}

func TestListMockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/servers" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}

		limit := r.URL.Query().Get("limit")
		if limit == "" {
			t.Error("Expected limit parameter")
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"results": [
				{"name": "server1", "description": "First server"},
				{"name": "server2", "description": "Second server"}
			],
			"total": 2
		}`))
	}))
	defer server.Close()

	client := &MCPSoClient{
		BaseURL:    server.URL,
		HTTPClient: http.DefaultClient,
	}

	results, err := client.List(10)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(results.Results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results.Results))
	}
}

func TestSearchError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := &MCPSoClient{
		BaseURL:    server.URL,
		HTTPClient: http.DefaultClient,
	}

	_, err := client.Search("test")
	if err == nil {
		t.Error("Expected error for 500 response")
	}
}

func TestSearchInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	client := &MCPSoClient{
		BaseURL:    server.URL,
		HTTPClient: http.DefaultClient,
	}

	_, err := client.Search("test")
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}
