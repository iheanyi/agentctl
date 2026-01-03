package mcpclient

import (
	"testing"
	"time"

	"github.com/iheanyi/agentctl/pkg/mcp"
)

func TestNewClient(t *testing.T) {
	client := NewClient()
	if client == nil {
		t.Fatal("NewClient() returned nil")
	}
	if client.timeout != 10*time.Second {
		t.Errorf("Default timeout = %v, want %v", client.timeout, 10*time.Second)
	}
}

func TestWithTimeout(t *testing.T) {
	client := NewClient().WithTimeout(5 * time.Second)
	if client.timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want %v", client.timeout, 5*time.Second)
	}
}

func TestCreateTransportStdio(t *testing.T) {
	client := NewClient()
	server := &mcp.Server{
		Command: "echo",
		Args:    []string{"hello"},
	}

	transport, err := client.createTransport(server)
	if err != nil {
		t.Fatalf("createTransport() error = %v", err)
	}
	if transport == nil {
		t.Fatal("createTransport() returned nil transport")
	}
}

func TestCreateTransportStdioNoCommand(t *testing.T) {
	client := NewClient()
	server := &mcp.Server{
		Transport: mcp.TransportStdio,
	}

	_, err := client.createTransport(server)
	if err == nil {
		t.Fatal("createTransport() should error when no command provided")
	}
}

func TestCreateTransportHTTP(t *testing.T) {
	client := NewClient()
	server := &mcp.Server{
		Transport: mcp.TransportHTTP,
		URL:       "http://localhost:8080",
	}

	transport, err := client.createTransport(server)
	if err != nil {
		t.Fatalf("createTransport() error = %v", err)
	}
	if transport == nil {
		t.Fatal("createTransport() returned nil transport")
	}
}

func TestCreateTransportHTTPNoURL(t *testing.T) {
	client := NewClient()
	server := &mcp.Server{
		Transport: mcp.TransportHTTP,
	}

	_, err := client.createTransport(server)
	if err == nil {
		t.Fatal("createTransport() should error when no URL provided for HTTP")
	}
}

func TestCreateTransportSSE(t *testing.T) {
	client := NewClient()
	server := &mcp.Server{
		Transport: mcp.TransportSSE,
		URL:       "http://localhost:8080/sse",
	}

	transport, err := client.createTransport(server)
	if err != nil {
		t.Fatalf("createTransport() error = %v", err)
	}
	if transport == nil {
		t.Fatal("createTransport() returned nil transport")
	}
}

func TestHealthResultDefaults(t *testing.T) {
	result := HealthResult{}
	if result.Healthy {
		t.Error("Default HealthResult.Healthy should be false")
	}
	if result.Error != nil {
		t.Error("Default HealthResult.Error should be nil")
	}
	if len(result.Tools) != 0 {
		t.Error("Default HealthResult.Tools should be empty")
	}
}

func TestToolStruct(t *testing.T) {
	tool := Tool{
		Name:        "test-tool",
		Description: "A test tool",
	}
	if tool.Name != "test-tool" {
		t.Errorf("Tool.Name = %q, want %q", tool.Name, "test-tool")
	}
	if tool.Description != "A test tool" {
		t.Errorf("Tool.Description = %q, want %q", tool.Description, "A test tool")
	}
}
