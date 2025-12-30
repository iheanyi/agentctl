package registry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// MCPSoClient is a client for the mcp.so registry
type MCPSoClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// ServerResult represents a search result from mcp.so
type ServerResult struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	URL         string   `json:"url"`
	Author      string   `json:"author"`
	Tags        []string `json:"tags"`
	Runtime     string   `json:"runtime"` // node, python, go, etc.
	Stars       int      `json:"stars"`
}

// SearchResponse represents the response from mcp.so search
type SearchResponse struct {
	Results []ServerResult `json:"results"`
	Total   int            `json:"total"`
}

// NewMCPSoClient creates a new mcp.so client
func NewMCPSoClient() *MCPSoClient {
	return &MCPSoClient{
		BaseURL: "https://mcp.so/api",
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Search searches for MCP servers on mcp.so
func (c *MCPSoClient) Search(query string) (*SearchResponse, error) {
	u, err := url.Parse(c.BaseURL + "/search")
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("q", query)
	u.RawQuery = q.Encode()

	resp, err := c.HTTPClient.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search failed with status %d", resp.StatusCode)
	}

	var result SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// Get retrieves details for a specific MCP server
func (c *MCPSoClient) Get(name string) (*ServerResult, error) {
	u := c.BaseURL + "/servers/" + url.PathEscape(name)

	resp, err := c.HTTPClient.Get(u)
	if err != nil {
		return nil, fmt.Errorf("failed to get server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get failed with status %d", resp.StatusCode)
	}

	var result ServerResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// List retrieves the most popular MCP servers
func (c *MCPSoClient) List(limit int) (*SearchResponse, error) {
	u, err := url.Parse(c.BaseURL + "/servers")
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("limit", fmt.Sprintf("%d", limit))
	q.Set("sort", "popular")
	u.RawQuery = q.Encode()

	resp, err := c.HTTPClient.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("failed to list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list failed with status %d", resp.StatusCode)
	}

	var result SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}
