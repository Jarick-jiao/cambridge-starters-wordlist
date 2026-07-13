package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ========== ToolResult & helpers ==========

type ToolResult struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	IsError bool `json:"isError"`
}

func parseToolResult(result json.RawMessage) (*ToolResult, error) {
	var tr ToolResult
	if err := json.Unmarshal(result, &tr); err != nil {
		return nil, fmt.Errorf("unmarshal tool result: %w (raw: %s)", err, string(result))
	}
	return &tr, nil
}

func parseRows[T any](tr *ToolResult) ([]T, error) {
	if len(tr.Content) == 0 || tr.Content[0].Text == "" || tr.Content[0].Text == "[]" {
		return nil, nil
	}
	var rows []T
	if err := json.Unmarshal([]byte(tr.Content[0].Text), &rows); err != nil {
		return nil, fmt.Errorf("unmarshal rows: %w (raw: %s)", err, tr.Content[0].Text)
	}
	return rows, nil
}

func sqlEscape(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// ========== DB Init ==========

func (c *MCPClient) initDB() error {
	// word_progress
	c.call("tools/call", json.RawMessage(`{
		"name": "execute",
		"arguments": {"sql": "CREATE TABLE IF NOT EXISTS word_progress (word TEXT PRIMARY KEY, group_id TEXT NOT NULL, mastered INTEGER NOT NULL DEFAULT 0)"}
	}`))
	c.call("tools/call", json.RawMessage(`{
		"name": "execute",
		"arguments": {"sql": "CREATE INDEX IF NOT EXISTS idx_wp_group ON word_progress(group_id)"}
	}`))

	// tasks
	c.call("tools/call", json.RawMessage(`{
		"name": "execute",
		"arguments": {"sql": "CREATE TABLE IF NOT EXISTS tasks (id INTEGER PRIMARY KEY AUTOINCREMENT, task_date TEXT NOT NULL, subject TEXT NOT NULL, task_type TEXT NOT NULL, target_count INTEGER NOT NULL, difficulty TEXT, topic TEXT, status TEXT NOT NULL DEFAULT 'assigned', completed_count INTEGER NOT NULL DEFAULT 0, accuracy REAL, time_spent INTEGER DEFAULT 0, created_at TEXT, updated_at TEXT)"}
	}`))
	c.call("tools/call", json.RawMessage(`{
		"name": "execute",
		"arguments": {"sql": "CREATE INDEX IF NOT EXISTS idx_tasks_date ON tasks(task_date)"}
	}`))

	// math_sessions
	c.call("tools/call", json.RawMessage(`{
		"name": "execute",
		"arguments": {"sql": "CREATE TABLE IF NOT EXISTS math_sessions (id INTEGER PRIMARY KEY AUTOINCREMENT, session_date TEXT, operation TEXT, difficulty TEXT, total_questions INTEGER, correct_count INTEGER, time_spent INTEGER, created_at TEXT)"}
	}`))

	// math_answers
	c.call("tools/call", json.RawMessage(`{
		"name": "execute",
		"arguments": {"sql": "CREATE TABLE IF NOT EXISTS math_answers (id INTEGER PRIMARY KEY AUTOINCREMENT, session_id INTEGER, question TEXT, user_answer INTEGER, correct_answer INTEGER, is_correct INTEGER, created_at TEXT)"}
	}`))

	// learning_log
	c.call("tools/call", json.RawMessage(`{
		"name": "execute",
		"arguments": {"sql": "CREATE TABLE IF NOT EXISTS learning_log (id INTEGER PRIMARY KEY AUTOINCREMENT, log_date TEXT, activity_type TEXT, count INTEGER, detail TEXT, created_at TEXT)"}
	}`))
	c.call("tools/call", json.RawMessage(`{
		"name": "execute",
		"arguments": {"sql": "CREATE INDEX IF NOT EXISTS idx_log_date ON learning_log(log_date)"}
	}`))

	return nil
}

// ========== Word Progress ==========

func (c *MCPClient) getAllProgress() (map[string]bool, error) {
	result, err := c.call("tools/call", json.RawMessage(`{
		"name": "query",
		"arguments": {"sql": "SELECT word, mastered FROM word_progress"}
	}`))
	if err != nil {
		return nil, err
	}
	tr, err := parseToolResult(result)
	if err != nil {
		return nil, err
	}
	progress := make(map[string]bool)
	rows, _ := parseRows[struct {
		Word     string `json:"word"`
		Mastered int    `json:"mastered"`
	}](tr)
	for _, row := range rows {
		if row.Mastered == 1 {
			progress[row.Word] = true
		}
	}
	return progress, nil
}

func (c *MCPClient) toggleWord(word, groupID string) (bool, error) {
	w := sqlEscape(word)
	g := sqlEscape(groupID)

	result, err := c.call("tools/call", json.RawMessage(fmt.Sprintf(`{
		"name": "query",
		"arguments": {"sql": "SELECT mastered FROM word_progress WHERE word = '%s'"}
	}`, w)))
	if err != nil {
		return false, err
	}
	tr, _ := parseToolResult(result)
	var isMastered bool
	rows, _ := parseRows[struct{ Mastered int `json:"mastered"` }](tr)
	if len(rows) > 0 {
		isMastered = rows[0].Mastered == 1
	}

	if isMastered {
		c.call("tools/call", json.RawMessage(fmt.Sprintf(`{
			"name": "execute",
			"arguments": {"sql": "UPDATE word_progress SET mastered = 0 WHERE word = '%s'"}
		}`, w)))
		c.logActivity("word_unmastered", 1, word)
		return false, nil
	}
	_, err = c.call("tools/call", json.RawMessage(fmt.Sprintf(`{
		"name": "execute",
		"arguments": {"sql": "INSERT INTO word_progress (word, group_id, mastered) VALUES ('%s', '%s', 1) ON CONFLICT(word) DO UPDATE SET mastered = 1, group_id = '%s'"}
	}`, w, g, g)))
	if err != nil {
		return false, err
	}
	c.logActivity("word_mastered", 1, word)
	return true, nil
}

func (c *MCPClient) resetAll() error {
	_, err := c.call("tools/call", json.RawMessage(`{
		"name": "execute",
		"arguments": {"sql": "DELETE FROM word_progress"}
	}`))
	return err
}

func (c *MCPClient) getGroupStats() (map[string][2]int, error) {
	result, err := c.call("tools/call", json.RawMessage(`{
		"name": "query",
		"arguments": {"sql": "SELECT group_id, COUNT(*) as total, SUM(mastered) as mastered FROM word_progress GROUP BY group_id"}
	}`))
	if err != nil {
		return nil, err
	}
	tr, err := parseToolResult(result)
	if err != nil {
		return nil, err
	}
	stats := make(map[string][2]int)
	rows, _ := parseRows[struct {
		GroupID  string `json:"group_id"`
		Total    int    `json:"total"`
		Mastered int    `json:"mastered"`
	}](tr)
	for _, row := range rows {
		stats[row.GroupID] = [2]int{row.Mastered, row.Total}
	}
	return stats, nil
}

// ========== Tasks ==========

type Task struct {
	ID             int     `json:"id"`
	TaskDate       string  `json:"task_date"`
	Subject        string  `json:"subject"`
	TaskType       string  `json:"task_type"`
	TargetCount    int     `json:"target_count"`
	Difficulty     string  `json:"difficulty"`
	Topic          string  `json:"topic"`
	Status         string  `json:"status"`
	CompletedCount int     `json:"completed_count"`
	Accuracy       float64 `json:"accuracy"`
	TimeSpent      int     `json:"time_spent"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

func (c *MCPClient) createTask(t *Task) (int, error) {
	now := time.Now().Format(time.RFC3339)
	t.CreatedAt = now
	t.UpdatedAt = now
	if t.Status == "" {
		t.Status = "assigned"
	}
	_, err := c.call("tools/call", json.RawMessage(fmt.Sprintf(`{
		"name": "execute",
		"arguments": {"sql": "INSERT INTO tasks (task_date, subject, task_type, target_count, difficulty, topic, status, completed_count, accuracy, time_spent, created_at, updated_at) VALUES ('%s', '%s', '%s', %d, '%s', '%s', '%s', %d, %v, %d, '%s', '%s')"}
	}`, sqlEscape(t.TaskDate), sqlEscape(t.Subject), sqlEscape(t.TaskType), t.TargetCount, sqlEscape(t.Difficulty), sqlEscape(t.Topic), sqlEscape(t.Status), t.CompletedCount, t.Accuracy, t.TimeSpent, now, now)))
	if err != nil {
		return 0, err
	}
	// Get last insert id
	r, err := c.call("tools/call", json.RawMessage(`{
		"name": "query",
		"arguments": {"sql": "SELECT last_insert_rowid() as id"}
	}`))
	if err != nil {
		return 0, err
	}
	tr, _ := parseToolResult(r)
	rows, _ := parseRows[struct{ ID int `json:"id"` }](tr)
	if len(rows) > 0 {
		return rows[0].ID, nil
	}
	return 0, nil
}

func (c *MCPClient) getTasksByDate(date string) ([]Task, error) {
	result, err := c.call("tools/call", json.RawMessage(fmt.Sprintf(`{
		"name": "query",
		"arguments": {"sql": "SELECT * FROM tasks WHERE task_date = '%s' ORDER BY id DESC"}
	}`, sqlEscape(date))))
	if err != nil {
		return nil, err
	}
	tr, err := parseToolResult(result)
	if err != nil {
		return nil, err
	}
	return parseRows[Task](tr)
}

func (c *MCPClient) getTaskByID(id int) (*Task, error) {
	result, err := c.call("tools/call", json.RawMessage(fmt.Sprintf(`{
		"name": "query",
		"arguments": {"sql": "SELECT * FROM tasks WHERE id = %d"}
	}`, id)))
	if err != nil {
		return nil, err
	}
	tr, err := parseToolResult(result)
	if err != nil {
		return nil, err
	}
	rows, _ := parseRows[Task](tr)
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

func (c *MCPClient) updateTaskProgress(id, completedCount, timeSpent int, accuracy float64) error {
	now := time.Now().Format(time.RFC3339)
	_, err := c.call("tools/call", json.RawMessage(fmt.Sprintf(`{
		"name": "execute",
		"arguments": {"sql": "UPDATE tasks SET completed_count = %d, time_spent = %d, accuracy = %v, updated_at = '%s', status = CASE WHEN completed_count >= target_count THEN 'completed' ELSE status END WHERE id = %d"}
	}`, completedCount, timeSpent, accuracy, now, id)))
	return err
}

func (c *MCPClient) completeTask(id int) error {
	now := time.Now().Format(time.RFC3339)
	_, err := c.call("tools/call", json.RawMessage(fmt.Sprintf(`{
		"name": "execute",
		"arguments": {"sql": "UPDATE tasks SET status = 'completed', updated_at = '%s' WHERE id = %d"}
	}`, now, id)))
	return err
}

func (c *MCPClient) deleteTask(id int) error {
	_, err := c.call("tools/call", json.RawMessage(fmt.Sprintf(`{
		"name": "execute",
		"arguments": {"sql": "DELETE FROM tasks WHERE id = %d"}
	}`, id)))
	return err
}

// ========== Math ==========

type MathSession struct {
	ID              int    `json:"id"`
	SessionDate     string `json:"session_date"`
	Operation       string `json:"operation"`
	Difficulty      string `json:"difficulty"`
	TotalQuestions  int    `json:"total_questions"`
	CorrectCount    int    `json:"correct_count"`
	TimeSpent       int    `json:"time_spent"`
	CreatedAt       string `json:"created_at"`
}

type MathAnswer struct {
	ID            int    `json:"id"`
	SessionID     int    `json:"session_id"`
	Question      string `json:"question"`
	UserAnswer    int    `json:"user_answer"`
	CorrectAnswer int    `json:"correct_answer"`
	IsCorrect     int    `json:"is_correct"`
	CreatedAt     string `json:"created_at"`
}

func (c *MCPClient) createMathSession(operation, difficulty string, totalQuestions int) (int, error) {
	now := time.Now().Format(time.RFC3339)
	_, err := c.call("tools/call", json.RawMessage(fmt.Sprintf(`{
		"name": "execute",
		"arguments": {"sql": "INSERT INTO math_sessions (session_date, operation, difficulty, total_questions, correct_count, time_spent, created_at) VALUES ('%s', '%s', '%s', %d, 0, 0, '%s')"}
	}`, now[:10], sqlEscape(operation), sqlEscape(difficulty), totalQuestions, now)))
	if err != nil {
		return 0, err
	}
	r, err := c.call("tools/call", json.RawMessage(`{
		"name": "query",
		"arguments": {"sql": "SELECT last_insert_rowid() as id"}
	}`))
	if err != nil {
		return 0, err
	}
	tr, _ := parseToolResult(r)
	rows, _ := parseRows[struct{ ID int `json:"id"` }](tr)
	if len(rows) > 0 {
		return rows[0].ID, nil
	}
	return 0, nil
}

func (c *MCPClient) recordMathAnswer(sessionID int, question string, userAnswer, correctAnswer, isCorrect int) error {
	now := time.Now().Format(time.RFC3339)
	_, err := c.call("tools/call", json.RawMessage(fmt.Sprintf(`{
		"name": "execute",
		"arguments": {"sql": "INSERT INTO math_answers (session_id, question, user_answer, correct_answer, is_correct, created_at) VALUES (%d, '%s', %d, %d, %d, '%s')"}
	}`, sessionID, sqlEscape(question), userAnswer, correctAnswer, isCorrect, now)))
	return err
}

func (c *MCPClient) updateMathSession(sessionID, correctCount, timeSpent int) error {
	_, err := c.call("tools/call", json.RawMessage(fmt.Sprintf(`{
		"name": "execute",
		"arguments": {"sql": "UPDATE math_sessions SET correct_count = %d, time_spent = %d WHERE id = %d"}
	}`, correctCount, timeSpent, sessionID)))
	return err
}

func (c *MCPClient) getMathAnswers(sessionID int) ([]MathAnswer, error) {
	result, err := c.call("tools/call", json.RawMessage(fmt.Sprintf(`{
		"name": "query",
		"arguments": {"sql": "SELECT * FROM math_answers WHERE session_id = %d ORDER BY id"}
	}`, sessionID)))
	if err != nil {
		return nil, err
	}
	tr, err := parseToolResult(result)
	if err != nil {
		return nil, err
	}
	return parseRows[MathAnswer](tr)
}

// ========== Learning Log ==========

func (c *MCPClient) logActivity(activityType string, count int, detail string) error {
	now := time.Now().Format(time.RFC3339)
	date := now[:10]
	_, err := c.call("tools/call", json.RawMessage(fmt.Sprintf(`{
		"name": "execute",
		"arguments": {"sql": "INSERT INTO learning_log (log_date, activity_type, count, detail, created_at) VALUES ('%s', '%s', %d, '%s', '%s')"}
	}`, date, sqlEscape(activityType), count, sqlEscape(detail), now)))
	return err
}

func (c *MCPClient) getLearningLogByDateRange(start, end string) ([]struct {
	LogDate      string `json:"log_date"`
	ActivityType string `json:"activity_type"`
	Count        int    `json:"count"`
	Detail       string `json:"detail"`
}, error) {
	result, err := c.call("tools/call", json.RawMessage(fmt.Sprintf(`{
		"name": "query",
		"arguments": {"sql": "SELECT log_date, activity_type, SUM(count) as count, GROUP_CONCAT(DISTINCT detail) as detail FROM learning_log WHERE log_date BETWEEN '%s' AND '%s' GROUP BY log_date, activity_type ORDER BY log_date"}
	}`, sqlEscape(start), sqlEscape(end))))
	if err != nil {
		return nil, err
	}
	tr, err := parseToolResult(result)
	if err != nil {
		return nil, err
	}
	return parseRows[struct {
		LogDate      string `json:"log_date"`
		ActivityType string `json:"activity_type"`
		Count        int    `json:"count"`
		Detail       string `json:"detail"`
	}](tr)
}

// ========== Dashboard & Stats ==========

func (c *MCPClient) getDashboardData(date string) (map[string]interface{}, error) {
	// Today's tasks
	tasks, err := c.getTasksByDate(date)
	if err != nil {
		return nil, err
	}

	// Word progress stats
	wpResult, err := c.call("tools/call", json.RawMessage(`{
		"name": "query",
		"arguments": {"sql": "SELECT COUNT(*) as total, SUM(mastered) as mastered FROM word_progress"}
	}`))
	if err != nil {
		return nil, err
	}
	tr, _ := parseToolResult(wpResult)
	wpRows, _ := parseRows[struct {
		Total    int `json:"total"`
		Mastered int `json:"mastered"`
	}](tr)
	var wordTotal, wordMastered int
	if len(wpRows) > 0 {
		wordTotal = wpRows[0].Total
		wordMastered = wpRows[0].Mastered
	}

	// Math stats today
	mathResult, err := c.call("tools/call", json.RawMessage(fmt.Sprintf(`{
		"name": "query",
		"arguments": {"sql": "SELECT COUNT(*) as sessions, SUM(correct_count) as correct, SUM(total_questions) as total FROM math_sessions WHERE session_date = '%s'"}
	}`, sqlEscape(date))))
	if err != nil {
		return nil, err
	}
	tr, _ = parseToolResult(mathResult)
	mathRows, _ := parseRows[struct {
		Sessions int `json:"sessions"`
		Correct  int `json:"correct"`
		Total    int `json:"total"`
	}](tr)
	var mathSessions, mathCorrect, mathTotal int
	if len(mathRows) > 0 {
		mathSessions = mathRows[0].Sessions
		mathCorrect = mathRows[0].Correct
		mathTotal = mathRows[0].Total
	}

	// Streak (consecutive days with activity)
	streakResult, err := c.call("tools/call", json.RawMessage(`{
		"name": "query",
		"arguments": {"sql": "SELECT DISTINCT log_date FROM learning_log ORDER BY log_date DESC"}
	}`))
	if err != nil {
		return nil, err
	}
	tr, _ = parseToolResult(streakResult)
	streakRows, _ := parseRows[struct{ LogDate string `json:"log_date"` }](tr)
	streak := 0
	if len(streakRows) > 0 {
		today := time.Now().Format("2006-01-02")
		expected := today
		for _, row := range streakRows {
			if row.LogDate == expected {
				streak++
				// Previous day
				t, _ := time.Parse("2006-01-02", expected)
				expected = t.AddDate(0, 0, -1).Format("2006-01-02")
			} else {
				break
			}
		}
	}

	return map[string]interface{}{
		"tasks":            tasks,
		"word_total":       wordTotal,
		"word_mastered":    wordMastered,
		"math_sessions":    mathSessions,
		"math_correct":     mathCorrect,
		"math_total":       mathTotal,
		"streak_days":      streak,
		"today":            date,
	}, nil
}

func (c *MCPClient) getStatsSummary(days int) (map[string]interface{}, error) {
	end := time.Now().Format("2006-01-02")
	start := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	// Learning log summary
	result, err := c.call("tools/call", json.RawMessage(fmt.Sprintf(`{
		"name": "query",
		"arguments": {"sql": "SELECT activity_type, SUM(count) as total FROM learning_log WHERE log_date BETWEEN '%s' AND '%s' GROUP BY activity_type"}
	}`, start, end)))
	if err != nil {
		return nil, err
	}
	tr, _ := parseToolResult(result)
	activityRows, _ := parseRows[struct {
		ActivityType string `json:"activity_type"`
		Total        int    `json:"total"`
	}](tr)
	activitySummary := make(map[string]int)
	for _, r := range activityRows {
		activitySummary[r.ActivityType] = r.Total
	}

	// Daily activity count
	result, err = c.call("tools/call", json.RawMessage(fmt.Sprintf(`{
		"name": "query",
		"arguments": {"sql": "SELECT log_date, SUM(count) as total FROM learning_log WHERE log_date BETWEEN '%s' AND '%s' GROUP BY log_date ORDER BY log_date"}
	}`, start, end)))
	if err != nil {
		return nil, err
	}
	tr, _ = parseToolResult(result)
	dailyRows, _ := parseRows[struct {
		LogDate string `json:"log_date"`
		Total   int    `json:"total"`
	}](tr)
	dailyActivity := make(map[string]int)
	for _, r := range dailyRows {
		dailyActivity[r.LogDate] = r.Total
	}

	return map[string]interface{}{
		"period_days":    days,
		"activity":       activitySummary,
		"daily_activity": dailyActivity,
	}, nil
}
