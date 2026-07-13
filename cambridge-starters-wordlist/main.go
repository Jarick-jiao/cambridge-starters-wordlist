package main

import (
	"log"
	"os"
	"path/filepath"
)

// global MCP client (used by handlers)
var mcpClient *MCPClient

func main() {
	port := "8080"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	dir := "."
	if d := os.Getenv("STATIC_DIR"); d != "" {
		dir = d
	}

	dbDir := filepath.Join(dir, "database")
	dbPath := filepath.Join(dbDir, "progress.db")

	// Ensure database directory exists
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		log.Fatalf("Failed to create database directory: %v", err)
	}

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

	// Start server
	if err := serve(dir, port, dbPath); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
