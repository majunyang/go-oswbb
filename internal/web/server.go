package web

import (
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/hex"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"go-oswbb/internal/config"
)

const (
	sessionCookieName = "oswbb_session"
	sessionTimeout    = 24 * time.Hour
)

type Server struct {
	cfg       *config.Config
	db        *sql.DB
	mux       *http.ServeMux
	tmpl      *TemplateManager
	cache     *ChartCache
	importMu  sync.Mutex
	staticFS  embed.FS
	sessions  *sessionStore
	adminHash string
}

func NewServer(cfg *config.Config, db *sql.DB, tmpl *TemplateManager, staticFS embed.FS) *Server {
	s := &Server{
		cfg:       cfg,
		db:        db,
		mux:       http.NewServeMux(),
		tmpl:      tmpl,
		cache:     NewChartCache(),
		staticFS:  staticFS,
		sessions:  newSessionStore(sessionTimeout),
		adminHash: hashCreds(cfg.AdminUser, cfg.AdminPass),
	}
	tmpl.SetArchiveInfo(cfg.ArchivePath != "", cfg.ArchivePath)
	s.routes()
	return s
}

// sessionStore keeps valid login tokens in memory with an expiry.
type sessionStore struct {
	mu      sync.Mutex
	tokens  map[string]time.Time
	timeout time.Duration
}

func newSessionStore(timeout time.Duration) *sessionStore {
	return &sessionStore{
		tokens:  make(map[string]time.Time),
		timeout: timeout,
	}
}

func (st *sessionStore) create() string {
	tok := randToken()
	st.mu.Lock()
	st.tokens[tok] = time.Now().Add(st.timeout)
	st.mu.Unlock()
	return tok
}

func (st *sessionStore) valid(tok string) bool {
	if tok == "" {
		return false
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	exp, ok := st.tokens[tok]
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		delete(st.tokens, tok)
		return false
	}
	return true
}

func (st *sessionStore) remove(tok string) {
	if tok == "" {
		return
	}
	st.mu.Lock()
	delete(st.tokens, tok)
	st.mu.Unlock()
}

func randToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Extremely unlikely; fall back to a time-derived token.
		return hex.EncodeToString([]byte(time.Now().String()))
	}
	return hex.EncodeToString(b)
}

// isAuthenticated reports whether the request carries a valid session cookie.
func (s *Server) isAuthenticated(r *http.Request) bool {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return false
	}
	return s.sessions.valid(c.Value)
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

	// Auth
	s.mux.HandleFunc("GET /login", s.handleLoginPage)
	s.mux.HandleFunc("POST /api/login", s.handleLoginAPI)
	s.mux.HandleFunc("POST /api/logout", s.handleLogoutAPI)

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
	s.mux.HandleFunc("POST /api/refresh", s.handleRefresh)
	s.mux.HandleFunc("GET /api/analysis/oswps", s.handleOswpsAnalysis)
}

// Handler wraps the mux with a global auth gate: only the login page, the
// login/logout APIs and static assets are public; everything else requires a
// valid session cookie.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/login" || path == "/api/login" || path == "/api/logout" || strings.HasPrefix(path, "/static/") {
			s.mux.ServeHTTP(w, r)
			return
		}
		if !s.isAuthenticated(r) {
			if strings.HasPrefix(path, "/api/") {
				jsonError(w, "未授权，请先登录", http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		s.mux.ServeHTTP(w, r)
	})
}
