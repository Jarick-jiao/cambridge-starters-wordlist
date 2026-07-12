package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// ========== MCP Client ==========

// MCPRequest is a JSON-RPC request to the MCP server
type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// MCPResponse is a JSON-RPC response from the MCP server
type MCPResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

// MCPError represents a JSON-RPC error
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCPClient communicates with mcp-server-sqlite via stdio
type MCPClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	reader *bufio.Reader
	mu     sync.Mutex
	nextID int
}

func NewMCPClient(dbPath string) (*MCPClient, error) {
	mcpBin, err := exec.LookPath("mcp-server-sqlite")
	if err != nil {
		// Try npx fallback
		mcpBin = "npx"
	}

	var cmd *exec.Cmd
	if strings.Contains(mcpBin, "mcp-server-sqlite") {
		cmd = exec.Command(mcpBin)
	} else {
		cmd = exec.Command(mcpBin, "mcp-server-sqlite")
	}
	// mcp-server-sqlite uses SQLITE_DB_PATH env var instead of --db flag
	cmd.Env = append(os.Environ(), "SQLITE_DB_PATH="+dbPath)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stderr = os.Stderr // MCP server logs go to our stderr for debugging

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start mcp-server-sqlite: %w", err)
	}

	client := &MCPClient{cmd: cmd, stdin: stdin, stdout: stdout, reader: bufio.NewReader(stdout), nextID: 1}

	// Send initialize
	initResult, err := client.call("initialize", json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"starters-server","version":"1.0.0"}}`))
	if err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("MCP initialize: %w", err)
	}
	log.Printf("[MCP] Initialized: %s", string(initResult))

	// Send initialized notification
	client.notify("notifications/initialized", nil)

	return client, nil
}

func (c *MCPClient) call(method string, params json.RawMessage) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := c.nextID
	c.nextID++

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	encoder := json.NewEncoder(c.stdin)
	if err := encoder.Encode(&req); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	// Read response lines, skip any non-JSON log output and mismatched IDs
	for {
		line, err := c.reader.ReadBytes('\n')
		if err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		// Skip non-JSON lines (e.g. "PRAGMA synchronous = NORMAL")
		if line[0] != '{' {
			log.Printf("[MCP] Skip non-JSON: %s", string(line))
			continue
		}
		var resp MCPResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			return nil, fmt.Errorf("parse response: %w (raw: %s)", err, string(line))
		}
		// Skip responses with mismatched IDs (e.g. notify responses)
		if resp.ID != id {
			log.Printf("[MCP] Skip mismatched ID: got %d, want %d", resp.ID, id)
			continue
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("MCP error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	}
}

func (c *MCPClient) notify(method string, params json.RawMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      0, // notifications have id 0 or null
		Method:  method,
		Params:  params,
	}

	encoder := json.NewEncoder(c.stdin)
	encoder.Encode(&req)
}

func (c *MCPClient) Close() {
	c.stdin.Close()
	c.stdout.Close()
	c.cmd.Process.Kill()
	c.cmd.Wait()
}
