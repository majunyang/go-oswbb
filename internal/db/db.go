package db

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go-oswbb/internal/parser"

	_ "modernc.org/sqlite"
)

// Open opens a SQLite database with WAL mode and busy timeout.
func Open(dbPath string) (*sql.DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}

	return db, nil
}

// ImportFile imports a .dat, .dat.gz, or .zip file into the specified table.
// Returns the number of lines imported.
func ImportFile(db *sql.DB, tableName, filePath, host string) (int, error) {
	if err := EnsureTable(db, tableName); err != nil {
		return 0, err
	}

	var lines []string
	ext := strings.ToLower(filepath.Ext(filePath))

	if ext == ".zip" {
		// For zip files, read content using parser.ReadFileContent
		content, err := parser.ReadFileContent(filePath)
		if err != nil {
			return 0, fmt.Errorf("read zip file: %w", err)
		}
		lines = strings.Split(content, "\n")
	} else {
		// For .dat and .dat.gz files, use scanner
		f, err := os.Open(filePath)
		if err != nil {
			return 0, fmt.Errorf("open file: %w", err)
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return 0, fmt.Errorf("scan file: %w", err)
		}
	}

	var currentTs string
	fallbackTs := parser.ParseTimestampFromFilename(filePath)

	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}

	insertSQL := fmt.Sprintf("INSERT INTO %s(host, timestamp, file, line_no, content) VALUES (?, ?, ?, ?, ?)", tableName)
	stmt, err := tx.Prepare(insertSQL)
	if err != nil {
		tx.Rollback()
		return 0, fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	lineNo := 0
	batchCount := 0

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		lineNo++
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "zzz ***") {
			ts := parser.ParseTimestampFormatted(line)
			if ts != "" {
				currentTs = ts
			}
			continue
		}

		ts := currentTs
		if ts == "" {
			ts = fallbackTs
		}

		if _, err := stmt.Exec(host, ts, filePath, lineNo, line); err != nil {
			tx.Rollback()
			return 0, fmt.Errorf("insert row: %w", err)
		}

		batchCount++
		if batchCount >= 2000 {
			if err := tx.Commit(); err != nil {
				return 0, fmt.Errorf("commit batch: %w", err)
			}
			tx, err = db.Begin()
			if err != nil {
				return 0, fmt.Errorf("begin transaction: %w", err)
			}
			stmt, err = tx.Prepare(insertSQL)
			if err != nil {
				tx.Rollback()
				return 0, fmt.Errorf("prepare insert: %w", err)
			}
			batchCount = 0
		}
	}

	if batchCount > 0 {
		if err := tx.Commit(); err != nil {
			return 0, fmt.Errorf("commit final batch: %w", err)
		}
	} else {
		tx.Rollback()
	}

	return lineNo, nil
}

// GetTableStats returns row counts for all tables in the database.
func GetTableStats(db *sql.DB) (map[string]int, error) {
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		return nil, fmt.Errorf("query tables: %w", err)
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		var count int
		countSQL := fmt.Sprintf("SELECT COUNT(*) FROM %s", name)
		if err := db.QueryRow(countSQL).Scan(&count); err != nil {
			continue
		}
		stats[name] = count
	}
	return stats, nil
}

// GetDataPaginated returns paginated data from a table with optional time range filter.
func GetDataPaginated(db *sql.DB, table string, page, limit int, startTime, endTime string) ([]map[string]interface{}, int, error) {
	// Sanitize table name
	safeName := parser.SanitizeTableName(table)

	var total int
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM %s", safeName)
	if startTime != "" && endTime != "" {
		countSQL += " WHERE timestamp >= ? AND timestamp <= ?"
		if err := db.QueryRow(countSQL, startTime, endTime).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("count rows: %w", err)
		}
	} else {
		if err := db.QueryRow(countSQL).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("count rows: %w", err)
		}
	}

	offset := (page - 1) * limit
	query := fmt.Sprintf("SELECT id, host, timestamp, file, line_no, content FROM %s", safeName)
	var args []interface{}
	if startTime != "" && endTime != "" {
		query += " WHERE timestamp >= ? AND timestamp <= ?"
		args = append(args, startTime, endTime)
	}
	query += " ORDER BY timestamp LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query data: %w", err)
	}
	defer rows.Close()

	columns, _ := rows.Columns()
	var result []map[string]interface{}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		ptrs := make([]interface{}, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			continue
		}
		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = values[i]
		}
		result = append(result, row)
	}

	return result, total, nil
}
