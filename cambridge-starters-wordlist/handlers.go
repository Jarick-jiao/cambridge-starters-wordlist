package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

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
