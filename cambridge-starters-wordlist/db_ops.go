package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

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
