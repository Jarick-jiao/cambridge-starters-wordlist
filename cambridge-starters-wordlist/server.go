package main

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func serve(dir, port, dbPath string) error {
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
		// Word progress
		api.GET("/progress", apiGetProgress)
		api.PUT("/progress/toggle", apiToggleWord)
		api.DELETE("/progress", apiResetProgress)

		// Math
		api.POST("/math/generate", apiGenerateMath)
		api.POST("/math/submit", apiSubmitMath)

		// Tasks
		api.GET("/tasks", apiGetTasks)
		api.POST("/tasks", apiCreateTask)
		api.PUT("/tasks/:id/progress", apiUpdateTaskProgress)
		api.DELETE("/tasks/:id", apiDeleteTask)

		// Dashboard & Stats
		api.GET("/dashboard", apiGetDashboard)
		api.GET("/stats/timeline", apiGetTimeline)
		api.GET("/stats/summary", apiGetStatsSummary)

		// Points
		api.GET("/points", apiGetPoints)
		api.POST("/points", apiAddPoints)
	}

	// --- Page routes ---
	r.GET("/", func(c *gin.Context) {
		c.File(filepath.Join(dir, "cambridge-starters-wordlist.html"))
	})
	r.StaticFS("/assets", http.Dir(filepath.Join(dir, "assets")))
	r.StaticFS("/_shared", http.Dir(filepath.Join(dir, "_shared")))
	r.StaticFS("/docs", http.Dir(filepath.Join(dir, "docs")))
	r.NoRoute(func(c *gin.Context) {
		c.File(filepath.Join(dir, "cambridge-starters-wordlist.html"))
	})

	// Print startup banner
	fmt.Println()
	fmt.Println("  ================================================")
	fmt.Println("    Starters Learning Server (Gin + MCP SQLite)")
	fmt.Println("  ================================================")
	fmt.Printf("    Serving:   %s\n", absDir)
	fmt.Printf("    Database:  %s\n", dbPath)
	fmt.Printf("    Local:     http://localhost:%s\n", port)
	fmt.Printf("    Network:   http://<your-lan-ip>:%s\n", port)
	fmt.Println("  ================================================")
	fmt.Println()

	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
	return nil
}
