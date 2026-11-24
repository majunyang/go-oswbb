package config

import (
	"errors"
	"flag"
	"net"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Bind          string
	Port          int
	DataDir       string
	UploadDir     string
	DBPath        string
	ArchivePath   string // Pre-extracted archive directory path
	RetentionDays int    // Data retention days (0 = no cleanup)
	AdminUser     string // Admin username for web login
	AdminPass     string // Admin password for web login
	TLSCert       string
	TLSKey        string
}

func Parse() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.Bind, "bind", "127.0.0.1", "Listen address (HTTPS required outside loopback)")
	flag.IntVar(&cfg.Port, "port", 3001, "Server port")
	flag.StringVar(&cfg.DataDir, "data-dir", "./data", "Data directory")
	flag.StringVar(&cfg.UploadDir, "upload-dir", "./uploads", "Upload directory")
	flag.StringVar(&cfg.DBPath, "db-path", "", "Database file path (default: data-dir/oswbb.db)")
	flag.StringVar(&cfg.ArchivePath, "archive", "", "Pre-extracted OSWatcher archive directory path")
	flag.IntVar(&cfg.RetentionDays, "retention-days", 3, "Data retention days (0 = no cleanup)")
	flag.StringVar(&cfg.AdminUser, "admin-user", "admin", "Admin username for web login")
	flag.StringVar(&cfg.AdminPass, "admin-pass", "", "Required admin password for web login")
	flag.StringVar(&cfg.TLSCert, "tls-cert", "", "HTTPS certificate file (PEM)")
	flag.StringVar(&cfg.TLSKey, "tls-key", "", "HTTPS private key file (PEM)")
	flag.Parse()

	if cfg.DBPath == "" {
		cfg.DBPath = cfg.DataDir + "/oswbb.db"
	}

	// Environment variable overrides
	if v := os.Getenv("OSWBB_BIND"); v != "" {
		cfg.Bind = v
	}
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
	if v := os.Getenv("OSWBB_ADMIN_USER"); v != "" {
		cfg.AdminUser = v
	}
	if v := os.Getenv("OSWBB_ADMIN_PASS"); v != "" {
		cfg.AdminPass = v
	}
	if v := os.Getenv("OSWBB_TLS_CERT"); v != "" {
		cfg.TLSCert = v
	}
	if v := os.Getenv("OSWBB_TLS_KEY"); v != "" {
		cfg.TLSKey = v
	}
	if v := os.Getenv("OSWBB_RETENTION_DAYS"); v != "" {
		if d, err := strconv.Atoi(v); err == nil {
			cfg.RetentionDays = d
		}
	}

	return cfg
}

func (cfg *Config) Validate() error {
	password := strings.TrimSpace(cfg.AdminPass)
	if password == "" || strings.EqualFold(password, "admin") {
		return errors.New("请通过 OSWBB_ADMIN_PASS 或 -admin-pass 设置非默认管理员密码")
	}
	if (cfg.TLSCert == "") != (cfg.TLSKey == "") {
		return errors.New("HTTPS 需要同时配置 -tls-cert 和 -tls-key")
	}
	if cfg.TLSCert == "" && !net.ParseIP(cfg.Bind).IsLoopback() {
		return errors.New("HTTP 仅允许监听回环 IP，其他监听地址需要配置 HTTPS")
	}
	return nil
}

func (cfg *Config) ListenAddress() string {
	return net.JoinHostPort(cfg.Bind, strconv.Itoa(cfg.Port))
}
