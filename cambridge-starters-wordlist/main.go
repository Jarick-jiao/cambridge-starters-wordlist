package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
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

// ========== DB Operations via MCP ==========

// ToolResult is the structure returned by MCP tool calls
type ToolResult struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	IsError bool `json:"isError"`
}

func (c *MCPClient) initDB() error {
	// Create table via execute tool
	_, err := c.call("tools/call", json.RawMessage(`{
		"name": "create-table",
		"arguments": {
			"name": "word_progress",
			"columns": [
				{"name": "word", "type": "TEXT", "primaryKey": true, "notNull": true},
				{"name": "group_id", "type": "TEXT", "notNull": true},
				{"name": "mastered", "type": "INTEGER", "notNull": true, "defaultValue": "0"}
			],
			"ifNotExists": true
		}
	}`))
	if err != nil {
		// Table may already exist, try creating index only
		c.call("tools/call", json.RawMessage(`{
			"name": "execute",
			"arguments": {"sql": "CREATE INDEX IF NOT EXISTS idx_wp_group ON word_progress(group_id)"}
		}`))
	}
	return nil
}

func (c *MCPClient) getAllProgress() (map[string]bool, error) {
	result, err := c.call("tools/call", json.RawMessage(`{
		"name": "query",
		"arguments": {"sql": "SELECT word, mastered FROM word_progress"}
	}`))
	if err != nil {
		return nil, err
	}

	var tr ToolResult
	if err := json.Unmarshal(result, &tr); err != nil {
		return nil, fmt.Errorf("unmarshal tool result: %w (raw: %s)", err, string(result))
	}

	progress := make(map[string]bool)
	if len(tr.Content) > 0 {
		var rows []struct {
			Word     string `json:"word"`
			Mastered int    `json:"mastered"`
		}
		raw := tr.Content[0].Text
		if err := json.Unmarshal([]byte(raw), &rows); err != nil {
			return nil, fmt.Errorf("unmarshal rows: %w (raw: %s)", err, raw)
		}
		for _, row := range rows {
			if row.Mastered == 1 {
				progress[row.Word] = true
			}
		}
	}
	return progress, nil
}

func (c *MCPClient) toggleWord(word, groupID string) (bool, error) {
	w := sqlEscape(word)
	g := sqlEscape(groupID)

	// Check current status
	result, err := c.call("tools/call", json.RawMessage(fmt.Sprintf(`{
		"name": "query",
		"arguments": {"sql": "SELECT mastered FROM word_progress WHERE word = '%s'"}
	}`, w)))
	if err != nil {
		return false, err
	}

	var tr ToolResult
	json.Unmarshal(result, &tr)

	var isMastered bool
	if len(tr.Content) > 0 && tr.Content[0].Text != "[]" {
		var rows []struct {
			Mastered int `json:"mastered"`
		}
		if err := json.Unmarshal([]byte(tr.Content[0].Text), &rows); err == nil && len(rows) > 0 {
			isMastered = rows[0].Mastered == 1
		}
	}

	if isMastered {
		// Un-master
		_, _ = c.call("tools/call", json.RawMessage(fmt.Sprintf(`{
			"name": "execute",
			"arguments": {"sql": "UPDATE word_progress SET mastered = 0 WHERE word = '%s'"}
		}`, w)))
		return false, nil
	}
	// Master (insert or update)
	_, err = c.call("tools/call", json.RawMessage(fmt.Sprintf(`{
		"name": "execute",
		"arguments": {"sql": "INSERT INTO word_progress (word, group_id, mastered) VALUES ('%s', '%s', 1) ON CONFLICT(word) DO UPDATE SET mastered = 1, group_id = '%s'"}
	}`, w, g, g)))
	if err != nil {
		return false, err
	}
	return true, nil
}

func (c *MCPClient) resetAll() error {
	_, err := c.call("tools/call", json.RawMessage(`{
		"name": "execute",
		"arguments": {"sql": "DELETE FROM word_progress"}
	}`))
	return err
}

// sqlEscape escapes single quotes for SQLite string literals
func sqlEscape(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func (c *MCPClient) getGroupStats() (map[string][2]int, error) {
	result, err := c.call("tools/call", json.RawMessage(`{
		"name": "query",
		"arguments": {"sql": "SELECT group_id, COUNT(*) as total, SUM(mastered) as mastered FROM word_progress GROUP BY group_id"}
	}`))
	if err != nil {
		return nil, err
	}

	var tr ToolResult
	if err := json.Unmarshal(result, &tr); err != nil {
		return nil, fmt.Errorf("unmarshal tool result: %w (raw: %s)", err, string(result))
	}

	stats := make(map[string][2]int)
	if len(tr.Content) > 0 {
		var rows []struct {
			GroupID  string `json:"group_id"`
			Total    int    `json:"total"`
			Mastered int    `json:"mastered"`
		}
		raw := tr.Content[0].Text
		if err := json.Unmarshal([]byte(raw), &rows); err != nil {
			return nil, fmt.Errorf("unmarshal rows: %w (raw: %s)", err, raw)
		}
		for _, row := range rows {
			stats[row.GroupID] = [2]int{row.Mastered, row.Total}
		}
	}
	return stats, nil
}

// ========== Global MCP Client ==========

var mcpClient *MCPClient

// ========== API Handlers ==========

type ToggleRequest struct {
	Word  string `json:"word"`
	Group string `json:"group"`
}

func apiGetProgress(c *gin.Context) {
	progress, err := mcpClient.getAllProgress()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"mastered": progress,
	})
}

func apiToggleWord(c *gin.Context) {
	var req ToggleRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Word == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "need word"})
		return
	}

	mastered, err := mcpClient.toggleWord(req.Word, req.Group)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"word": req.Word, "mastered": mastered})
}

func apiResetProgress(c *gin.Context) {
	if err := mcpClient.resetAll(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ========== Main ==========

func main() {
	port := "8080"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	dir := "."
	if d := os.Getenv("STATIC_DIR"); d != "" {
		dir = d
	}

	dbPath := filepath.Join(dir, "progress.db")

	// Init MCP Client (connects to mcp-server-sqlite via stdio)
	var err error
	mcpClient, err = NewMCPClient(dbPath)
	if err != nil {
		log.Fatalf("Failed to start MCP SQLite client: %v", err)
	}
	defer mcpClient.Close()

	// Init database table via MCP
	if err := mcpClient.initDB(); err != nil {
		log.Printf("[WARN] DB init (non-fatal): %v", err)
	}
	log.Printf("[MCP] Database ready: %s", dbPath)

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	absDir, _ := filepath.Abs(dir)

	// Security: block path traversal
	r.Use(func(c *gin.Context) {
		clean := filepath.Clean(c.Request.URL.Path)
		if strings.Contains(clean, "..") {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		c.Next()
	})

	// --- API routes ---
	api := r.Group("/api")
	{
		api.GET("/progress", apiGetProgress)
		api.PUT("/progress/toggle", apiToggleWord)
		api.DELETE("/progress", apiResetProgress)
	}

	// --- Page routes ---
	r.GET("/", func(c *gin.Context) {
		c.File(filepath.Join(dir, "cambridge-starters-wordlist.html"))
	})
	r.StaticFS("/assets", http.Dir(filepath.Join(dir, "assets")))
	r.StaticFS("/_shared", http.Dir(filepath.Join(dir, "_shared")))
	r.NoRoute(func(c *gin.Context) {
		c.File(filepath.Join(dir, "cambridge-starters-wordlist.html"))
	})

	// Graceful shutdown
	// (MCP client is closed via defer in main)

	fmt.Println()
	fmt.Println("  ================================================")
	fmt.Println("    Starters Learning Server (Gin + MCP SQLite)")
	fmt.Println("  ================================================")
	fmt.Printf("    Serving:   %s\n", absDir)
	fmt.Printf("    Database: %s (via MCP Server)\n", dbPath)
	fmt.Printf("    Local:     http://localhost:%s\n", port)
	fmt.Printf("    Network:   http://<your-lan-ip>:%s\n", port)
	fmt.Println("  ================================================")
	fmt.Println()

	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}