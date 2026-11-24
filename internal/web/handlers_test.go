package web

import (
	"archive/tar"
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-oswbb/internal/config"
	"go-oswbb/internal/db"
)

const vmstatLogHeader = "zzz ***Tue Nov 24 12:00:00 UTC 2026\n"

type archiveEntry struct {
	name    string
	content string
}

func newArchiveTestServer(t *testing.T) *Server {
	t.Helper()
	root := t.TempDir()
	cfg := &config.Config{DBPath: filepath.Join(root, "test.db"), UploadDir: filepath.Join(root, "uploads")}
	if err := os.Mkdir(cfg.UploadDir, 0700); err != nil {
		t.Fatal(err)
	}
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	return &Server{cfg: cfg, db: database, cache: NewChartCache()}
}

func uploadTestArchive(t *testing.T, s *Server, entries ...archiveEntry) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile("archive", "sample.tar")
	if err != nil {
		t.Fatal(err)
	}
	archive := tar.NewWriter(part)
	for _, entry := range entries {
		if err := archive.WriteHeader(&tar.Header{Name: entry.name, Mode: 0600, Size: int64(len(entry.content))}); err != nil {
			t.Fatal(err)
		}
		if _, err := archive.Write([]byte(entry.content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/upload/archive", &body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	rec := httptest.NewRecorder()
	s.handleUpload(rec, req)
	return rec
}

func assertArchiveRecord(t *testing.T, s *Server, want string) {
	t.Helper()
	stats, err := db.GetTableStats(s.db)
	if err != nil || len(stats) != 1 || stats["oswvmstat"] != 1 {
		t.Fatalf("archive table stats = %v, err = %v, want one VMstat row", stats, err)
	}
	var content string
	if err := s.db.QueryRow("SELECT content FROM oswvmstat").Scan(&content); err != nil {
		t.Fatal(err)
	}
	if content != want {
		t.Fatalf("archive record = %q, want %q", content, want)
	}
}

func TestUploadImportCommitsDataAndArchiveTogether(t *testing.T) {
	s := newArchiveTestServer(t)
	const firstRecord = "1 0 0 2048 128 256 0 0 1 2 10 9 20 5 75 0 0"
	const nextRecord = "2 0 0 4096 128 256 0 0 1 2 10 9 20 5 75 0 0"
	oldFile := archiveEntry{"oswvmstat/host_vmstat_26.11.24.1200.dat", vmstatLogHeader + firstRecord + "\n"}
	newFile := archiveEntry{"oswvmstat/host_vmstat_26.11.24.1201.dat", vmstatLogHeader + nextRecord + "\n"}

	if rec := uploadTestArchive(t, s, oldFile); rec.Code != http.StatusOK {
		t.Fatalf("first upload: status = %d, body = %s", rec.Code, rec.Body)
	}
	oldPath := s.findArchivePath()
	if oldPath == "" {
		t.Fatal("first successful upload has no chart source")
	}
	assertArchiveRecord(t, s, firstRecord)

	badFile := archiveEntry{"oswvmstat/host_vmstat_26.11.24.1202.dat", strings.Repeat("x", 1024*1024+1)}
	if rec := uploadTestArchive(t, s, newFile, badFile); rec.Code != http.StatusInternalServerError {
		t.Fatalf("invalid upload: status = %d, body = %s", rec.Code, rec.Body)
	}
	assertArchiveRecord(t, s, firstRecord)
	if got := s.findArchivePath(); got != oldPath {
		t.Fatalf("failed upload changed chart source from %q to %q", oldPath, got)
	}
	if entries, err := os.ReadDir(s.cfg.UploadDir); err != nil || len(entries) != 1 {
		t.Fatalf("failed upload left temporary files: directories = %v, err = %v", entries, err)
	}

	if rec := uploadTestArchive(t, s, newFile); rec.Code != http.StatusOK {
		t.Fatalf("replacement upload: status = %d, body = %s", rec.Code, rec.Body)
	}
	assertArchiveRecord(t, s, nextRecord)
	newPath := s.findArchivePath()
	if newPath == "" || newPath == oldPath {
		t.Fatalf("replacement upload did not publish its chart source: %q", newPath)
	}

	reopened, err := db.Open(s.cfg.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	restarted := &Server{cfg: s.cfg, db: reopened}
	if got := restarted.findArchivePath(); got != newPath {
		t.Fatalf("reopened database chart source = %q, want %q", got, newPath)
	}
	assertArchiveRecord(t, restarted, nextRecord)
}

func TestRefreshRejectsEmptyArchiveAndKeepsData(t *testing.T) {
	s := newArchiveTestServer(t)
	if rec := uploadTestArchive(t, s, archiveEntry{"oswvmstat/host_vmstat_26.11.24.1200.dat", vmstatLogHeader + "original record\n"}); rec.Code != http.StatusOK {
		t.Fatalf("initial upload: status = %d, body = %s", rec.Code, rec.Body)
	}
	s.cfg.ArchivePath = t.TempDir()
	if err := s.ImportConfiguredArchive(context.Background()); err == nil {
		t.Fatal("startup import accepted an empty archive")
	}
	req := httptest.NewRequest(http.MethodPost, "/api/refresh", nil)
	rec := httptest.NewRecorder()
	s.handleRefresh(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("refresh accepting an empty archive: status = %d, body = %s", rec.Code, rec.Body)
	}
	assertArchiveRecord(t, s, "original record")
}

func TestArchiveLookupSkipsUncommittedUploads(t *testing.T) {
	s := newArchiveTestServer(t)
	incomplete := filepath.Join(s.cfg.UploadDir, uploadDirPrefix+"incomplete", "extracted")
	if err := os.MkdirAll(incomplete, 0700); err != nil {
		t.Fatal(err)
	}
	if got := s.findArchivePath(); got != "" {
		t.Fatalf("uncommitted upload became the chart source: %q", got)
	}
	legacy := filepath.Join(s.cfg.UploadDir, "20261124120000", "extracted")
	if err := os.MkdirAll(legacy, 0700); err != nil {
		t.Fatal(err)
	}
	if got := s.findArchivePath(); got != legacy {
		t.Fatalf("chart source = %q, want legacy source %q", got, legacy)
	}
}

func TestLoginCookieFollowsTransport(t *testing.T) {
	s := &Server{
		sessions:  newSessionStore(sessionTimeout),
		adminHash: hashCreds("sample-admin", "test-session-password"),
	}
	for _, scheme := range []string{"http", "https"} {
		t.Run(scheme, func(t *testing.T) {
			server := httptest.NewUnstartedServer(http.HandlerFunc(s.handleLoginAPI))
			defer server.Close()
			if scheme == "https" {
				server.StartTLS()
			} else {
				server.Start()
			}
			body := strings.NewReader(`{"user":"sample-admin","pass":"test-session-password"}`)
			resp, err := server.Client().Post(server.URL+"/api/login", "application/json", body)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			cookies := resp.Cookies()
			if resp.StatusCode != http.StatusOK || len(cookies) != 1 {
				t.Fatalf("login: status = %d, cookie count = %d", resp.StatusCode, len(cookies))
			}
			cookie := cookies[0]
			if cookie.Name != sessionCookieName || !cookie.HttpOnly || cookie.Secure != (scheme == "https") {
				t.Fatalf("wrong session Cookie flags on %s", scheme)
			}
		})
	}
}
