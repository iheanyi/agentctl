package mcpclient

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/iheanyi/agentctl/pkg/mcp"
)

// Tool represents an MCP tool exposed by a server
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ToolCallResult represents the result of calling a tool
type ToolCallResult struct {
	Success bool          `json:"success"`
	Content []string      `json:"content,omitempty"` // Text content from the result
	IsError bool          `json:"isError,omitempty"`
	Error   error         `json:"error,omitempty"`
	Latency time.Duration `json:"latency"`
}

// HealthResult represents the result of a health check
type HealthResult struct {
	Healthy    bool          `json:"healthy"`
	ServerName string        `json:"serverName,omitempty"`
	Version    string        `json:"version,omitempty"`
	Tools      []Tool        `json:"tools,omitempty"`
	Error      error         `json:"error,omitempty"`
	Latency    time.Duration `json:"latency"`
}

// Client wraps the MCP SDK client for health checks and tool discovery
type Client struct {
	timeout time.Duration
}

// NewClient creates a new MCP client wrapper
func NewClient() *Client {
	return &Client{
		timeout: 10 * time.Second,
	}
}

// WithTimeout sets the timeout for client operations
func (c *Client) WithTimeout(d time.Duration) *Client {
	c.timeout = d
	return c
}

// CheckHealth performs a health check on an MCP server
// It connects, performs the initialize handshake, and optionally lists tools
func (c *Client) CheckHealth(ctx context.Context, server *mcp.Server) HealthResult {
	start := time.Now()

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	result := HealthResult{}

	// Create transport based on server config
	transport, err := c.createTransport(server)
	if err != nil {
		result.Error = fmt.Errorf("failed to create transport: %w", err)
		result.Latency = time.Since(start)
		return result
	}

	// Create client
	client := mcpsdk.NewClient(&mcpsdk.Implementation{
		Name:    "agentctl",
		Version: "1.0.0",
	}, nil)

	// Connect to server (this performs the initialize handshake)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		result.Error = fmt.Errorf("failed to connect: %w", err)
		result.Latency = time.Since(start)
		return result
	}
	defer session.Close()

	// List tools to verify server is working
	toolsResult, err := session.ListTools(ctx, nil)
	if err != nil {
		result.Error = fmt.Errorf("failed to list tools: %w", err)
		result.Latency = time.Since(start)
		return result
	}

	// Success - populate result
	result.Healthy = true
	result.Latency = time.Since(start)

	// Convert tools
	for _, t := range toolsResult.Tools {
		result.Tools = append(result.Tools, Tool{
			Name:        t.Name,
			Description: t.Description,
		})
	}

	return result
}

// ListTools connects to a server and returns its available tools
func (c *Client) ListTools(ctx context.Context, server *mcp.Server) ([]Tool, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	transport, err := c.createTransport(server)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}

	client := mcpsdk.NewClient(&mcpsdk.Implementation{
		Name:    "agentctl",
		Version: "1.0.0",
	}, nil)

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer session.Close()

	toolsResult, err := session.ListTools(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	var tools []Tool
	for _, t := range toolsResult.Tools {
		tools = append(tools, Tool{
			Name:        t.Name,
			Description: t.Description,
		})
	}

	return tools, nil
}

// CallTool connects to a server and calls a specific tool with arguments
func (c *Client) CallTool(ctx context.Context, server *mcp.Server, toolName string, arguments map[string]any) ToolCallResult {
	start := time.Now()

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	result := ToolCallResult{}

	transport, err := c.createTransport(server)
	if err != nil {
		result.Error = fmt.Errorf("failed to create transport: %w", err)
		result.Latency = time.Since(start)
		return result
	}

	client := mcpsdk.NewClient(&mcpsdk.Implementation{
		Name:    "agentctl",
		Version: "1.0.0",
	}, nil)

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		result.Error = fmt.Errorf("failed to connect: %w", err)
		result.Latency = time.Since(start)
		return result
	}
	defer session.Close()

	// Call the tool
	callResult, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      toolName,
		Arguments: arguments,
	})
	if err != nil {
		result.Error = fmt.Errorf("tool call failed: %w", err)
		result.Latency = time.Since(start)
		return result
	}

	result.Success = true
	result.Latency = time.Since(start)
	result.IsError = callResult.IsError

	// Extract text content from result
	for _, content := range callResult.Content {
		if textContent, ok := content.(*mcpsdk.TextContent); ok {
			result.Content = append(result.Content, textContent.Text)
		}
	}

	return result
}

// createTransport creates the appropriate transport based on server config
func (c *Client) createTransport(server *mcp.Server) (mcpsdk.Transport, error) {
	switch server.Transport {
	case mcp.TransportHTTP, mcp.TransportSSE:
		if server.URL == "" {
			return nil, fmt.Errorf("HTTP/SSE server requires URL")
		}
		return &mcpsdk.StreamableClientTransport{
			Endpoint: server.URL,
			HTTPClient: &http.Client{
				Timeout: c.timeout,
			},
		}, nil

	case mcp.TransportStdio, "":
		if server.Command == "" {
			return nil, fmt.Errorf("stdio server requires command")
		}
		cmd := exec.Command(server.Command, server.Args...)

		// Set environment variables
		if len(server.Env) > 0 {
			for k, v := range server.Env {
				cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
			}
		}

		return &mcpsdk.CommandTransport{
			Command: cmd,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported transport: %s", server.Transport)
	}
}
