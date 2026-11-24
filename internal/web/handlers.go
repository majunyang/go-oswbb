package web

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"go-oswbb/internal/archive"
	"go-oswbb/internal/db"
	"go-oswbb/internal/model"
	"go-oswbb/internal/parser"
)

// New uploads are visible to readers only after the database records their path.
const uploadDirPrefix = ".oswbb-upload-"

// ChartCache provides thread-safe caching for parsed chart data.
type ChartCache struct {
	mu    sync.RWMutex
	data  map[string]interface{}
	paths map[string]string // tracks which archive path is cached
}

func NewChartCache() *ChartCache {
	return &ChartCache{
		data:  make(map[string]interface{}),
		paths: make(map[string]string),
	}
}

func (c *ChartCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.data[key]
	return v, ok
}

func (c *ChartCache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = value
}

func (c *ChartCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[string]interface{})
}

// findArchivePath resolves the configured, committed, or legacy archive directory.
func (s *Server) findArchivePath() string {
	// Use configured archive path if specified
	if s.cfg.ArchivePath != "" {
		if _, err := os.Stat(s.cfg.ArchivePath); err == nil {
			return s.cfg.ArchivePath
		}
		log.Printf("配置的归档路径不存在: %s", s.cfg.ArchivePath)
	}

	activeUpload, err := db.GetActiveUpload(s.db)
	if err != nil {
		log.Printf("读取已导入归档路径失败: %v", err)
		return ""
	}
	if activeUpload != "" {
		if !filepath.IsLocal(activeUpload) {
			log.Printf("已导入归档路径无效: %s", activeUpload)
			return ""
		}
		path := filepath.Join(s.cfg.UploadDir, activeUpload)
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path
		}
		return ""
	}

	// Uploads created before the state table used timestamp-named directories.
	entries, err := os.ReadDir(s.cfg.UploadDir)
	if err != nil {
		return ""
	}

	// Sort by name (timestamp) descending
	var dirs []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), uploadDirPrefix) {
			dirs = append(dirs, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dirs)))

	for _, dir := range dirs {
		extractedPath := filepath.Join(s.cfg.UploadDir, dir, "extracted")
		if _, err := os.Stat(extractedPath); err == nil {
			return archive.FindArchivePath(extractedPath)
		}
	}
	return ""
}

// Page handlers

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	stats, _ := db.GetTableStats(s.db)
	hasData := len(stats) > 0

	data := map[string]interface{}{
		"HasData": hasData,
		"Tables":  stats,
	}
	if err := s.tmpl.Execute(w, "web/templates/index.html", data); err != nil {
		http.Error(w, err.Error(), 500)
	}
}

func (s *Server) handleViewer(w http.ResponseWriter, r *http.Request) {
	table := r.URL.Query().Get("table")
	if table == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "chart"
	}

	data := map[string]interface{}{
		"Table":     table,
		"TableType": getTableType(table),
		"Mode":      mode,
	}
	if err := s.tmpl.Execute(w, "web/templates/viewer.html", data); err != nil {
		http.Error(w, err.Error(), 500)
	}
}

func (s *Server) handle404(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	s.tmpl.Execute(w, "web/templates/404.html", nil)
}

// API handlers

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(500 << 20) // 500MB

	file, header, err := r.FormFile("archive")
	if err != nil {
		jsonError(w, "请上传 tar 或 tar.gz 文件", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validate file extension
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".tar" && ext != ".tgz" && !strings.HasSuffix(strings.ToLower(header.Filename), ".tar.gz") {
		jsonError(w, "只支持 .tar, .tar.gz, .tgz 格式的文件", http.StatusBadRequest)
		return
	}

	uploadPath, err := os.MkdirTemp(s.cfg.UploadDir, uploadDirPrefix)
	if err != nil {
		jsonError(w, "创建上传目录失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	imported := false
	defer func() {
		if !imported {
			if err := archive.CleanupDir(uploadPath); err != nil {
				log.Printf("清理未完成的上传失败: %v", err)
			}
		}
	}()

	destPath := filepath.Join(uploadPath, header.Filename)
	dest, err := os.Create(destPath)
	if err != nil {
		jsonError(w, "保存文件失败: "+err.Error(), 500)
		return
	}
	_, copyErr := io.Copy(dest, file)
	if err := errors.Join(copyErr, dest.Close()); err != nil {
		jsonError(w, "写入文件失败: "+err.Error(), 500)
		return
	}

	// Extract archive
	extractDir := filepath.Join(uploadPath, "extracted")
	archivePath, err := archive.ExtractTarArchive(destPath, extractDir)
	if err != nil {
		jsonError(w, "解压失败: "+err.Error(), 500)
		return
	}
	log.Printf("解压完成, archive路径: %s", archivePath)

	activeUpload, err := filepath.Rel(s.cfg.UploadDir, archivePath)
	if err != nil {
		jsonError(w, "读取上传归档路径失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Import to SQLite
	log.Printf("开始导入到SQLite: %s", s.cfg.DBPath)
	summary, err := s.importArchiveData(r.Context(), archivePath, activeUpload)
	if err != nil {
		jsonError(w, "导入失败: "+err.Error(), 500)
		return
	}
	imported = true
	log.Printf("导入完成")

	jsonResponse(w, map[string]interface{}{
		"success": true,
		"message": "上传并导入成功",
		"data": map[string]interface{}{
			"originalFile":  header.Filename,
			"fileSize":      header.Size,
			"archivePath":   archivePath,
			"dbPath":        s.cfg.DBPath,
			"importSummary": summary,
		},
	})
}

// ImportConfiguredArchive imports data from the archive path configured via the
// -archive flag at startup, so the web UI has data to display without requiring
// an upload through the web interface.
func (s *Server) ImportConfiguredArchive(ctx context.Context) error {
	if s.cfg.ArchivePath == "" {
		return nil
	}
	if _, err := os.Stat(s.cfg.ArchivePath); err != nil {
		return fmt.Errorf("配置的归档路径不存在: %s", s.cfg.ArchivePath)
	}
	// Resolve the actual OSWatcher archive root (may be nested under a hostname dir)
	root := archive.FindArchivePath(s.cfg.ArchivePath)
	log.Printf("从配置的归档路径自动导入数据: %s", root)

	_, err := s.importArchiveData(ctx, root, "")
	return err
}

func (s *Server) importArchiveData(ctx context.Context, archivePath, activeUpload string) (map[string]interface{}, error) {
	entries, err := os.ReadDir(archivePath)
	if err != nil {
		return nil, err
	}

	s.importMu.Lock()
	defer s.importMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("开始归档导入事务: %w", err)
	}
	defer tx.Rollback()

	if err := db.DropDataTables(tx); err != nil {
		return nil, fmt.Errorf("替换旧归档表: %w", err)
	}

	summary := make(map[string]interface{})
	importedRows := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		tableName := parser.SanitizeTableName(entry.Name())
		subdirPath := filepath.Join(archivePath, entry.Name())

		files, err := collectDatFiles(subdirPath)
		if err != nil {
			return nil, fmt.Errorf("读取归档目录 %s: %w", subdirPath, err)
		}
		if len(files) > 0 {
			if err := db.EnsureTable(tx, tableName); err != nil {
				return nil, err
			}
		}
		totalLines := 0
		for _, f := range files {
			host := parser.GetHostFromFilename(f)
			lines, err := db.ImportFile(tx, tableName, f, host)
			if err != nil {
				return nil, fmt.Errorf("导入文件 %s: %w", f, err)
			}
			totalLines += lines
		}
		importedRows += totalLines

		summary[entry.Name()] = map[string]interface{}{
			"table": tableName,
			"files": len(files),
			"lines": totalLines,
		}
	}
	if importedRows == 0 {
		return nil, errors.New("归档没有可导入的记录")
	}
	if activeUpload != "" {
		if err := db.SetActiveUpload(tx, activeUpload); err != nil {
			return nil, fmt.Errorf("记录已导入归档路径: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("提交归档导入: %w", err)
	}
	s.cache.Invalidate()

	return summary, nil
}

func collectDatFiles(dirPath string) ([]string, error) {
	var files []string
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			name := entry.Name()
			if strings.HasSuffix(name, ".dat") || strings.HasSuffix(name, ".dat.gz") {
				files = append(files, filepath.Join(dirPath, name))
			}
		}
	}
	sort.Strings(files)
	return files, nil
}

func (s *Server) handleStatistics(w http.ResponseWriter, r *http.Request) {
	if _, err := os.Stat(s.cfg.DBPath); os.IsNotExist(err) {
		jsonResponse(w, map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"hasData": false,
				"message": "暂无导入数据",
			},
		})
		return
	}

	stats, err := db.GetTableStats(s.db)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"hasData": true,
			"tables":  stats,
			"dbPath":  s.cfg.DBPath,
		},
	})
}

func (s *Server) handleData(w http.ResponseWriter, r *http.Request) {
	table := r.PathValue("table")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	startTime := formatFilterTime(r.URL.Query().Get("startTime"), false)
	endTime := formatFilterTime(r.URL.Query().Get("endTime"), true)

	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	rows, total, err := db.GetDataPaginated(s.db, table, page, limit, startTime, endTime)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"rows":  rows,
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func (s *Server) handleVmstatChart(w http.ResponseWriter, r *http.Request) {
	metric := r.PathValue("metric")
	validMetrics := map[string]bool{
		"cpu": true, "memory": true, "io": true, "swap": true, "system": true, "procs": true,
	}
	if !validMetrics[metric] {
		jsonError(w, "无效的指标类型", 400)
		return
	}

	archivePath := s.findArchivePath()
	if archivePath == "" {
		jsonResponse(w, map[string]interface{}{
			"success": true,
			"data":    &model.ChartResponse{},
		})
		return
	}

	// Check cache
	cacheKey := fmt.Sprintf("vmstat_%s_%s", archivePath, metric)
	if cached, ok := s.cache.Get(cacheKey); ok {
		jsonResponse(w, map[string]interface{}{"success": true, "data": cached})
		return
	}

	// Parse all vmstat files
	files := archive.ScanDirectory(archivePath)["vmstat"]
	p := &parser.VmstatParser{}
	merged := &model.VmstatParsedData{}

	for _, f := range files {
		parsed, err := p.ParseFile(f)
		if err != nil {
			log.Printf("解析vmstat文件失败: %s: %v", f, err)
			continue
		}
		mergeVmstatData(merged, parsed)
	}

	chartData := p.GetChartData(merged, metric)
	s.cache.Set(cacheKey, chartData)

	jsonResponse(w, map[string]interface{}{"success": true, "data": chartData})
}

func mergeVmstatData(merged, parsed *model.VmstatParsedData) {
	merged.Timestamps = append(merged.Timestamps, parsed.Timestamps...)
	merged.Procs = append(merged.Procs, parsed.Procs...)
	merged.Memory = append(merged.Memory, parsed.Memory...)
	merged.Swap = append(merged.Swap, parsed.Swap...)
	merged.IO = append(merged.IO, parsed.IO...)
	merged.System = append(merged.System, parsed.System...)
	merged.CPU = append(merged.CPU, parsed.CPU...)
}

func (s *Server) handleIfconfigChart(w http.ResponseWriter, r *http.Request) {
	metric := r.PathValue("metric")
	validMetrics := map[string]bool{
		"throughput": true, "packets": true, "errors": true, "dropped": true,
	}
	if !validMetrics[metric] {
		jsonError(w, "无效的指标类型", 400)
		return
	}

	selectedInterfaces := parseCommaSeparated(r.URL.Query().Get("interfaces"))

	archivePath := s.findArchivePath()
	if archivePath == "" {
		jsonResponse(w, map[string]interface{}{"success": true, "data": &model.ChartResponse{}})
		return
	}

	cacheKey := fmt.Sprintf("ifconfig_%s_%s_%v", archivePath, metric, selectedInterfaces)
	if cached, ok := s.cache.Get(cacheKey); ok {
		jsonResponse(w, map[string]interface{}{"success": true, "data": cached})
		return
	}

	files := archive.ScanDirectory(archivePath)["ifconfig"]
	p := &parser.IfconfigParser{}
	merged := &model.IfconfigParsedData{
		Interfaces: make(map[string][]model.IfconfigRecord),
	}

	for _, f := range files {
		parsed, err := p.ParseFile(f)
		if err != nil {
			log.Printf("解析ifconfig文件失败: %s: %v", f, err)
			continue
		}
		for name, records := range parsed.Interfaces {
			merged.Interfaces[name] = append(merged.Interfaces[name], records...)
		}
	}

	// Sort records by timestamp for each interface
	for name := range merged.Interfaces {
		records := merged.Interfaces[name]
		sort.Slice(records, func(i, j int) bool {
			return records[i].Timestamp.Before(records[j].Timestamp)
		})
		merged.Interfaces[name] = records
	}

	chartData := p.GetAllInterfacesChartData(merged, metric, selectedInterfaces)
	s.cache.Set(cacheKey, chartData)

	jsonResponse(w, map[string]interface{}{"success": true, "data": chartData})
}

func (s *Server) handleIfconfigInterfaces(w http.ResponseWriter, r *http.Request) {
	archivePath := s.findArchivePath()
	if archivePath == "" {
		jsonResponse(w, map[string]interface{}{"success": true, "data": map[string]interface{}{"interfaces": []string{}}})
		return
	}

	files := archive.ScanDirectory(archivePath)["ifconfig"]
	p := &parser.IfconfigParser{}
	merged := &model.IfconfigParsedData{
		Interfaces: make(map[string][]model.IfconfigRecord),
	}

	for _, f := range files {
		parsed, err := p.ParseFile(f)
		if err != nil {
			continue
		}
		for name, records := range parsed.Interfaces {
			merged.Interfaces[name] = append(merged.Interfaces[name], records...)
		}
	}

	jsonResponse(w, map[string]interface{}{
		"success": true,
		"data":    map[string]interface{}{"interfaces": p.GetInterfaceList(merged)},
	})
}

func (s *Server) handleIostatChart(w http.ResponseWriter, r *http.Request) {
	metric := r.PathValue("metric")
	validMetrics := map[string]bool{
		"iops": true, "throughput": true, "await": true, "util": true, "queue": true, "reqsize": true,
	}
	if !validMetrics[metric] {
		jsonError(w, "无效的指标类型", 400)
		return
	}

	selectedDevices := parseCommaSeparated(r.URL.Query().Get("devices"))

	archivePath := s.findArchivePath()
	if archivePath == "" {
		jsonResponse(w, map[string]interface{}{"success": true, "data": &model.ChartResponse{}})
		return
	}

	cacheKey := fmt.Sprintf("iostat_%s_%s_%v", archivePath, metric, selectedDevices)
	if cached, ok := s.cache.Get(cacheKey); ok {
		jsonResponse(w, map[string]interface{}{"success": true, "data": cached})
		return
	}

	files := archive.ScanDirectory(archivePath)["iostat"]
	p := &parser.IostatParser{}
	merged := &model.IostatParsedData{
		Devices: make(map[string][]model.IostatRecord),
	}

	for _, f := range files {
		parsed, err := p.ParseFile(f)
		if err != nil {
			log.Printf("解析iostat文件失败: %s: %v", f, err)
			continue
		}
		for name, records := range parsed.Devices {
			merged.Devices[name] = append(merged.Devices[name], records...)
		}
	}

	for name := range merged.Devices {
		records := merged.Devices[name]
		sort.Slice(records, func(i, j int) bool {
			return records[i].Timestamp.Before(records[j].Timestamp)
		})
		merged.Devices[name] = records
	}

	chartData := p.GetAllDevicesChartData(merged, metric, selectedDevices)
	s.cache.Set(cacheKey, chartData)

	jsonResponse(w, map[string]interface{}{"success": true, "data": chartData})
}

func (s *Server) handleIostatDevices(w http.ResponseWriter, r *http.Request) {
	archivePath := s.findArchivePath()
	if archivePath == "" {
		jsonResponse(w, map[string]interface{}{"success": true, "data": map[string]interface{}{"devices": []string{}}})
		return
	}

	files := archive.ScanDirectory(archivePath)["iostat"]
	p := &parser.IostatParser{}
	merged := &model.IostatParsedData{
		Devices: make(map[string][]model.IostatRecord),
	}

	for _, f := range files {
		parsed, err := p.ParseFile(f)
		if err != nil {
			continue
		}
		for name, records := range parsed.Devices {
			merged.Devices[name] = append(merged.Devices[name], records...)
		}
	}

	jsonResponse(w, map[string]interface{}{
		"success": true,
		"data":    map[string]interface{}{"devices": p.GetDeviceList(merged)},
	})
}

func (s *Server) handleMpstatChart(w http.ResponseWriter, r *http.Request) {
	metric := r.PathValue("metric")
	validMetrics := map[string]bool{
		"usage": true, "user": true, "system": true, "iowait": true, "interrupt": true, "steal": true,
	}
	if !validMetrics[metric] {
		jsonError(w, "无效的指标类型", 400)
		return
	}

	selectedCpus := parseCommaSeparated(r.URL.Query().Get("cpus"))

	archivePath := s.findArchivePath()
	if archivePath == "" {
		jsonResponse(w, map[string]interface{}{"success": true, "data": &model.ChartResponse{}})
		return
	}

	cacheKey := fmt.Sprintf("mpstat_%s_%s_%v", archivePath, metric, selectedCpus)
	if cached, ok := s.cache.Get(cacheKey); ok {
		jsonResponse(w, map[string]interface{}{"success": true, "data": cached})
		return
	}

	files := archive.ScanDirectory(archivePath)["mpstat"]
	p := &parser.MpstatParser{}
	merged := &model.MpstatParsedData{
		CPUs: make(map[string][]model.MpstatRecord),
	}

	for _, f := range files {
		parsed, err := p.ParseFile(f)
		if err != nil {
			log.Printf("解析mpstat文件失败: %s: %v", f, err)
			continue
		}
		for cpuID, records := range parsed.CPUs {
			merged.CPUs[cpuID] = append(merged.CPUs[cpuID], records...)
		}
	}

	for cpuID := range merged.CPUs {
		records := merged.CPUs[cpuID]
		sort.Slice(records, func(i, j int) bool {
			return records[i].Timestamp.Before(records[j].Timestamp)
		})
		merged.CPUs[cpuID] = records
	}

	chartData := p.GetAllCpuChartData(merged, metric, selectedCpus)
	s.cache.Set(cacheKey, chartData)

	jsonResponse(w, map[string]interface{}{"success": true, "data": chartData})
}

func (s *Server) handleMpstatCpus(w http.ResponseWriter, r *http.Request) {
	archivePath := s.findArchivePath()
	if archivePath == "" {
		jsonResponse(w, map[string]interface{}{"success": true, "data": map[string]interface{}{"cpus": []string{}}})
		return
	}

	files := archive.ScanDirectory(archivePath)["mpstat"]
	p := &parser.MpstatParser{}
	merged := &model.MpstatParsedData{
		CPUs: make(map[string][]model.MpstatRecord),
	}

	for _, f := range files {
		parsed, err := p.ParseFile(f)
		if err != nil {
			continue
		}
		for cpuID, records := range parsed.CPUs {
			merged.CPUs[cpuID] = append(merged.CPUs[cpuID], records...)
		}
	}

	jsonResponse(w, map[string]interface{}{
		"success": true,
		"data":    map[string]interface{}{"cpus": p.GetCpuList(merged)},
	})
}

func (s *Server) handleMeminfoChart(w http.ResponseWriter, r *http.Request) {
	metric := r.PathValue("metric")
	validMetrics := map[string]bool{
		"memory": true, "swap": true, "detail": true, "hugepages": true,
	}
	if !validMetrics[metric] {
		jsonError(w, "无效的指标类型", 400)
		return
	}

	archivePath := s.findArchivePath()
	if archivePath == "" {
		jsonResponse(w, map[string]interface{}{"success": true, "data": &model.ChartResponse{}})
		return
	}

	cacheKey := fmt.Sprintf("meminfo_%s_%s", archivePath, metric)
	if cached, ok := s.cache.Get(cacheKey); ok {
		jsonResponse(w, map[string]interface{}{"success": true, "data": cached})
		return
	}

	files := archive.ScanDirectory(archivePath)["meminfo"]
	p := &parser.MeminfoParser{}
	merged := &model.MeminfoParsedData{}

	for _, f := range files {
		parsed, err := p.ParseFile(f)
		if err != nil {
			log.Printf("解析meminfo文件失败: %s: %v", f, err)
			continue
		}
		merged.Timestamps = append(merged.Timestamps, parsed.Timestamps...)
		merged.Records = append(merged.Records, parsed.Records...)
	}

	chartData := p.GetChartData(merged, metric)
	s.cache.Set(cacheKey, chartData)

	jsonResponse(w, map[string]interface{}{"success": true, "data": chartData})
}

func (s *Server) handleTopChart(w http.ResponseWriter, r *http.Request) {
	metric := r.PathValue("metric")
	validMetrics := map[string]bool{
		"load": true, "tasks": true, "cpu": true, "memory": true, "topcpu": true,
	}
	if !validMetrics[metric] {
		jsonError(w, "无效的指标类型", 400)
		return
	}

	archivePath := s.findArchivePath()
	if archivePath == "" {
		jsonResponse(w, map[string]interface{}{"success": true, "data": &model.ChartResponse{}})
		return
	}

	cacheKey := fmt.Sprintf("top_%s_%s", archivePath, metric)
	if cached, ok := s.cache.Get(cacheKey); ok {
		jsonResponse(w, map[string]interface{}{"success": true, "data": cached})
		return
	}

	files := archive.ScanDirectory(archivePath)["top"]
	p := &parser.TopParser{}
	merged := &model.TopParsedData{}

	for _, f := range files {
		parsed, err := p.ParseFile(f)
		if err != nil {
			log.Printf("解析top文件失败: %s: %v", f, err)
			continue
		}
		merged.Timestamps = append(merged.Timestamps, parsed.Timestamps...)
		merged.Snapshots = append(merged.Snapshots, parsed.Snapshots...)
		if len(parsed.TopProcs) > 0 {
			merged.TopProcs = parsed.TopProcs
		}
	}

	chartData := p.GetChartData(merged, metric)
	s.cache.Set(cacheKey, chartData)

	jsonResponse(w, map[string]interface{}{"success": true, "data": chartData})
}

func (s *Server) handleChartTypes(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"vmstat":   []string{"cpu", "memory", "io", "swap", "system", "procs"},
			"iostat":   []string{"iops", "throughput", "await", "util", "queue", "reqsize"},
			"ifconfig": []string{"throughput", "packets", "errors", "dropped"},
			"mpstat":   []string{"usage", "user", "system", "iowait", "interrupt", "steal"},
			"meminfo":  []string{"memory", "swap", "detail", "hugepages"},
			"top":      []string{"load", "tasks", "cpu", "memory", "topcpu"},
		},
	})
}

func (s *Server) handleArchivePath(w http.ResponseWriter, r *http.Request) {
	path := s.findArchivePath()
	jsonResponse(w, map[string]interface{}{
		"success": true,
		"data":    map[string]interface{}{"archivePath": path},
	})
}

// handleRefresh re-imports the configured archive directory on demand so the UI
// can pick up newly written .dat files while running in -archive mode.
func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "仅支持 POST 方法", http.StatusMethodNotAllowed)
		return
	}
	if s.cfg.ArchivePath == "" {
		jsonError(w, "未配置归档路径(-archive),无法刷新。请改用 Web 上传方式导入数据。", http.StatusBadRequest)
		return
	}
	if _, err := os.Stat(s.cfg.ArchivePath); err != nil {
		jsonError(w, "归档路径不存在: "+s.cfg.ArchivePath, http.StatusBadRequest)
		return
	}
	if err := s.ImportConfiguredArchive(r.Context()); err != nil {
		jsonError(w, "刷新失败: "+err.Error(), 500)
		return
	}
	jsonResponse(w, map[string]interface{}{
		"success": true,
		"message": "已重新导入归档数据",
	})
}

// Helpers

func jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"message": message,
	})
}

// hashCreds returns a stable hash of "user:pass" for constant-time comparison.
func hashCreds(user, pass string) string {
	sum := sha256.Sum256([]byte(user + ":" + pass))
	return hex.EncodeToString(sum[:])
}

// handleLoginPage renders the standalone login page.
func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.RenderRaw(w, "web/templates/login.html", nil); err != nil {
		log.Printf("渲染登录页失败: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleLoginAPI validates credentials and issues a session cookie.
func (s *Server) handleLoginAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "仅支持 POST 方法", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		User string `json:"user"`
		Pass string `json:"pass"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "请求体解析失败", http.StatusBadRequest)
		return
	}
	want := hashCreds(req.User, req.Pass)
	if subtle.ConstantTimeCompare([]byte(want), []byte(s.adminHash)) != 1 {
		jsonError(w, "用户名或密码错误", http.StatusUnauthorized)
		return
	}
	tok := s.sessions.create()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(s.sessions.timeout.Seconds()),
	})
	jsonResponse(w, map[string]interface{}{"success": true})
}

// handleLogoutAPI clears the session cookie and invalidates the token.
func (s *Server) handleLogoutAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "仅支持 POST 方法", http.StatusMethodNotAllowed)
		return
	}
	if c, err := r.Cookie(sessionCookieName); err == nil {
		s.sessions.remove(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		MaxAge:   -1,
	})
	jsonResponse(w, map[string]interface{}{"success": true})
}

func parseCommaSeparated(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func getTableType(table string) string {
	if strings.Contains(table, "vmstat") {
		return "vmstat"
	}
	if strings.Contains(table, "iostat") {
		return "iostat"
	}
	if strings.Contains(table, "ifconfig") {
		return "ifconfig"
	}
	if strings.Contains(table, "mpstat") {
		return "mpstat"
	}
	if strings.Contains(table, "meminfo") {
		return "meminfo"
	}
	if strings.Contains(table, "top") {
		return "top"
	}
	return "generic"
}

func (s *Server) handleOswpsAnalysis(w http.ResponseWriter, r *http.Request) {
	archivePath := s.findArchivePath()
	startTime := formatFilterTime(r.URL.Query().Get("startTime"), false)
	endTime := formatFilterTime(r.URL.Query().Get("endTime"), true)

	cacheKey := fmt.Sprintf("oswps_analysis_%s_%s_%s", archivePath, startTime, endTime)
	if cached, ok := s.cache.Get(cacheKey); ok {
		jsonResponse(w, map[string]interface{}{"success": true, "data": cached})
		return
	}

	analysis, err := db.AnalyzeOswps(s.db, startTime, endTime)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	s.cache.Set(cacheKey, analysis)
	jsonResponse(w, map[string]interface{}{"success": true, "data": analysis})
}

func formatFilterTime(t string, isEnd bool) string {
	if t == "" {
		return ""
	}
	t = strings.Replace(t, "T", " ", 1)
	if len(t) == 16 { // YYYY-MM-DD HH:MM
		if isEnd {
			return t + ":59"
		}
		return t + ":00"
	}
	return t
}
