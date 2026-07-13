package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// ========== Existing Handlers ==========

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
	c.JSON(http.StatusOK, gin.H{"mastered": progress})
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

// ========== Math Handlers ==========

type GenerateMathRequest struct {
	Operation string `json:"operation"` // + - * /
	Difficulty string `json:"difficulty"` // L1 L2 L3
	Count     int    `json:"count"`
}

type MathQuestion struct {
	Num1     int    `json:"num1"`
	Num2     int    `json:"num2"`
	Operator string `json:"operator"`
	Answer   int    `json:"answer"`
}

var mathSessionStore = make(map[int]*MathSessionState)

type MathSessionState struct {
	SessionID    int
	Questions    []MathQuestion
	StartTime    time.Time
	CorrectCount int
}

func apiGenerateMath(c *gin.Context) {
	var req GenerateMathRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.Operation == "" {
		req.Operation = "+"
	}
	if req.Difficulty == "" {
		req.Difficulty = "L1"
	}
	if req.Count <= 0 || req.Count > 50 {
		req.Count = 20
	}

	questions := generateQuestions(req.Operation, req.Difficulty, req.Count)

	// Create session in DB
	sessionID, err := mcpClient.createMathSession(req.Operation, req.Difficulty, req.Count)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	mathSessionStore[sessionID] = &MathSessionState{
		SessionID: sessionID,
		Questions: questions,
		StartTime: time.Now(),
	}

	// Return questions without answers
	var publicQuestions []gin.H
	for _, q := range questions {
		publicQuestions = append(publicQuestions, gin.H{
			"num1":     q.Num1,
			"num2":     q.Num2,
			"operator": q.Operator,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionID,
		"questions":  publicQuestions,
	})
}

type SubmitMathRequest struct {
	SessionID int `json:"session_id"`
	Answers   []int `json:"answers"`
}

func apiSubmitMath(c *gin.Context) {
	var req SubmitMathRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	state, ok := mathSessionStore[req.SessionID]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session not found"})
		return
	}

	correctCount := 0
	results := make([]gin.H, 0, len(state.Questions))

	for i, q := range state.Questions {
		userAns := 0
		if i < len(req.Answers) {
			userAns = req.Answers[i]
		}
		isCorrect := userAns == q.Answer
		if isCorrect {
			correctCount++
		}

		questionStr := strconv.Itoa(q.Num1) + " " + q.Operator + " " + strconv.Itoa(q.Num2)
		isCorrectInt := 0
		if isCorrect {
			isCorrectInt = 1
		}
		mcpClient.recordMathAnswer(req.SessionID, questionStr, userAns, q.Answer, isCorrectInt)

		results = append(results, gin.H{
			"question":     questionStr,
			"user_answer":  userAns,
			"correct":      q.Answer,
			"is_correct":   isCorrect,
		})
	}

	timeSpent := int(time.Since(state.StartTime).Seconds())
	mcpClient.updateMathSession(req.SessionID, correctCount, timeSpent)
	mcpClient.logActivity("math_practice", len(state.Questions), fmt.Sprintf("%s/%s correct:%d", state.Questions[0].Operator, req.Answers, correctCount))

	accuracy := 0.0
	if len(state.Questions) > 0 {
		accuracy = float64(correctCount) / float64(len(state.Questions))
	}

	delete(mathSessionStore, req.SessionID)

	c.JSON(http.StatusOK, gin.H{
		"session_id":    req.SessionID,
		"total":         len(state.Questions),
		"correct":       correctCount,
		"accuracy":      accuracy,
		"time_spent":    timeSpent,
		"results":       results,
	})
}

func generateQuestions(op, difficulty string, count int) []MathQuestion {
	var questions []MathQuestion

	for i := 0; i < count; i++ {
		q := MathQuestion{Operator: op}
		switch difficulty {
		case "L1":
			q.Num1 = rand.Intn(9) + 1
			q.Num2 = rand.Intn(9) + 1
		case "L2":
			q.Num1 = rand.Intn(90) + 10
			q.Num2 = rand.Intn(90) + 10
		case "L3":
			q.Num1 = rand.Intn(900) + 100
			q.Num2 = rand.Intn(900) + 100
		default:
			q.Num1 = rand.Intn(9) + 1
			q.Num2 = rand.Intn(9) + 1
		}

		switch op {
		case "+":
			q.Answer = q.Num1 + q.Num2
		case "-":
			// Ensure non-negative
			if q.Num1 < q.Num2 {
				q.Num1, q.Num2 = q.Num2, q.Num1
			}
			q.Answer = q.Num1 - q.Num2
		case "*":
			if difficulty == "L1" {
				q.Num1 = rand.Intn(9) + 1
				q.Num2 = rand.Intn(9) + 1
			} else {
				q.Num1 = rand.Intn(12) + 2
				q.Num2 = rand.Intn(12) + 2
			}
			q.Answer = q.Num1 * q.Num2
		case "/":
			// Ensure integer result
			q.Num2 = rand.Intn(9) + 1
			if difficulty == "L2" || difficulty == "L3" {
				q.Num2 = rand.Intn(12) + 2
			}
			q.Answer = rand.Intn(9) + 1
			if difficulty == "L2" || difficulty == "L3" {
				q.Answer = rand.Intn(12) + 2
			}
			q.Num1 = q.Num2 * q.Answer
		}
		questions = append(questions, q)
	}
	return questions
}

// ========== Task Handlers ==========

func apiGetTasks(c *gin.Context) {
	date := c.Query("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	tasks, err := mcpClient.getTasksByDate(date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tasks": tasks, "date": date})
}

func apiCreateTask(c *gin.Context) {
	var t Task
	if err := c.ShouldBindJSON(&t); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if t.TaskDate == "" {
		t.TaskDate = time.Now().Format("2006-01-02")
	}
	if t.TargetCount <= 0 {
		t.TargetCount = 10
	}
	id, err := mcpClient.createTask(&t)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "status": "created"})
}

type UpdateTaskProgressRequest struct {
	CompletedCount int     `json:"completed_count"`
	TimeSpent      int     `json:"time_spent"`
	Accuracy       float64 `json:"accuracy"`
}

func apiUpdateTaskProgress(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req UpdateTaskProgressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := mcpClient.updateTaskProgress(id, req.CompletedCount, req.TimeSpent, req.Accuracy); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Check if task should be auto-completed
	task, _ := mcpClient.getTaskByID(id)
	if task != nil && task.CompletedCount >= task.TargetCount {
		mcpClient.completeTask(id)
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func apiDeleteTask(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := mcpClient.deleteTask(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// ========== Dashboard & Stats ==========

func apiGetDashboard(c *gin.Context) {
	date := c.Query("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	data, err := mcpClient.getDashboardData(date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

func apiGetTimeline(c *gin.Context) {
	days := 30
	if d := c.Query("days"); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v > 0 {
			days = v
		}
	}
	end := time.Now().Format("2006-01-02")
	start := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	logs, err := mcpClient.getLearningLogByDateRange(start, end)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"start": start, "end": end, "logs": logs})
}

func apiGetStatsSummary(c *gin.Context) {
	days := 7
	if d := c.Query("days"); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v > 0 {
			days = v
		}
	}
	data, err := mcpClient.getStatsSummary(days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}
