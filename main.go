package main

import (
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"go-oswbb/internal/config"
	"go-oswbb/internal/db"
	"go-oswbb/internal/web"
)

//go:embed web/templates/*.html
var templateFS embed.FS

//go:embed web/static
var staticFS embed.FS

func main() {
	cfg := config.Parse()

	// Ensure directories exist
	os.MkdirAll(cfg.DataDir, 0755)
	os.MkdirAll(cfg.UploadDir, 0755)

	// Log archive path if specified
	if cfg.ArchivePath != "" {
		log.Printf("使用指定归档路径: %s", cfg.ArchivePath)
	}

	// Initialize database
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	// Start cleanup scheduler if retention is configured
	if cfg.RetentionDays > 0 {
		log.Printf("数据保留天数: %d 天", cfg.RetentionDays)
		db.StartCleanupScheduler(database, cfg.RetentionDays, 1*time.Hour)
	}

	// Initialize template manager
	tmpl := web.NewTemplateManager(templateFS)

	// Create and start server
	srv := web.NewServer(cfg, database, tmpl, staticFS)

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("OSWBBGraph Go server starting on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
