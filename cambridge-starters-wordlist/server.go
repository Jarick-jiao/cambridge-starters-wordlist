package main

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func serve(dir, port string) error {
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

	fmt.Println()
	fmt.Println("  ================================================")
	fmt.Println("    Starters Learning Server (Gin + MCP SQLite)")
	fmt.Println("  ================================================")
	fmt.Printf("    Serving:   %s\n", absDir)
	fmt.Printf("    Local:     http://localhost:%s\n", port)
	fmt.Printf("    Network:   http://<your-lan-ip>:%s\n", port)
	fmt.Println("  ================================================")
	fmt.Println()

	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
	return nil
}
