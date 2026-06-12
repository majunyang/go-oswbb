package config

import (
	"flag"
	"os"
	"strconv"
)

type Config struct {
	Port          int
	DataDir       string
	UploadDir     string
	DBPath        string
	ArchivePath   string // Pre-extracted archive directory path
	RetentionDays int    // Data retention days (0 = no cleanup)
}

func Parse() *Config {
	cfg := &Config{}

	flag.IntVar(&cfg.Port, "port", 3001, "Server port")
	flag.StringVar(&cfg.DataDir, "data-dir", "./data", "Data directory")
	flag.StringVar(&cfg.UploadDir, "upload-dir", "./uploads", "Upload directory")
	flag.StringVar(&cfg.DBPath, "db-path", "", "Database file path (default: data-dir/oswbb.db)")
	flag.StringVar(&cfg.ArchivePath, "archive", "", "Pre-extracted OSWatcher archive directory path")
	flag.IntVar(&cfg.RetentionDays, "retention-days", 3, "Data retention days (0 = no cleanup)")
	flag.Parse()

	if cfg.DBPath == "" {
		cfg.DBPath = cfg.DataDir + "/oswbb.db"
	}

	// Environment variable overrides
	if v := os.Getenv("OSWBB_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Port = p
		}
	}
	if v := os.Getenv("OSWBB_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v := os.Getenv("OSWBB_UPLOAD_DIR"); v != "" {
		cfg.UploadDir = v
	}
	if v := os.Getenv("OSWBB_ARCHIVE"); v != "" {
		cfg.ArchivePath = v
	}
	if v := os.Getenv("OSWBB_RETENTION_DAYS"); v != "" {
		if d, err := strconv.Atoi(v); err == nil {
			cfg.RetentionDays = d
		}
	}

	return cfg
}
