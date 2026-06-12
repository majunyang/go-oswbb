package web

import (
	"database/sql"
	"embed"
	"io/fs"
	"net/http"

	"go-oswbb/internal/config"
)

type Server struct {
	cfg       *config.Config
	db        *sql.DB
	mux       *http.ServeMux
	tmpl      *TemplateManager
	cache     *ChartCache
	staticFS  embed.FS
}

func NewServer(cfg *config.Config, db *sql.DB, tmpl *TemplateManager, staticFS embed.FS) *Server {
	s := &Server{
		cfg:      cfg,
		db:       db,
		mux:      http.NewServeMux(),
		tmpl:     tmpl,
		cache:    NewChartCache(),
		staticFS: staticFS,
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	// Static files - use fs.Sub to serve from web/static subdirectory
	staticSub, err := fs.Sub(s.staticFS, "web/static")
	if err != nil {
		panic("failed to create static sub filesystem: " + err.Error())
	}
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))

	// Pages
	s.mux.HandleFunc("GET /{$}", s.handleIndex)
	s.mux.HandleFunc("GET /viewer", s.handleViewer)
	s.mux.HandleFunc("GET /404", s.handle404)

	// API
	s.mux.HandleFunc("POST /api/upload/archive", s.handleUpload)
	s.mux.HandleFunc("GET /api/statistics", s.handleStatistics)
	s.mux.HandleFunc("GET /api/data/{table}", s.handleData)
	s.mux.HandleFunc("GET /api/chart/vmstat/{metric}", s.handleVmstatChart)
	s.mux.HandleFunc("GET /api/chart/ifconfig/{metric}", s.handleIfconfigChart)
	s.mux.HandleFunc("GET /api/chart/ifconfig-interfaces", s.handleIfconfigInterfaces)
	s.mux.HandleFunc("GET /api/chart/iostat/{metric}", s.handleIostatChart)
	s.mux.HandleFunc("GET /api/chart/iostat-devices", s.handleIostatDevices)
	s.mux.HandleFunc("GET /api/chart/mpstat/{metric}", s.handleMpstatChart)
	s.mux.HandleFunc("GET /api/chart/mpstat-cpus", s.handleMpstatCpus)
	s.mux.HandleFunc("GET /api/chart/meminfo/{metric}", s.handleMeminfoChart)
	s.mux.HandleFunc("GET /api/chart/top/{metric}", s.handleTopChart)
	s.mux.HandleFunc("GET /api/chart/types", s.handleChartTypes)
	s.mux.HandleFunc("GET /api/archive-path", s.handleArchivePath)
}

func (s *Server) Handler() http.Handler {
	return s.mux
}
