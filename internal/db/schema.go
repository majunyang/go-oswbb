package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

// EnsureTable creates the table and timestamp index if they don't exist.
func EnsureTable(db *sql.DB, tableName string) error {
	createSQL := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		host TEXT,
		timestamp TEXT,
		file TEXT,
		line_no INTEGER,
		content TEXT
	)`, tableName)

	if _, err := db.Exec(createSQL); err != nil {
		return fmt.Errorf("create table %s: %w", tableName, err)
	}

	indexSQL := fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_timestamp ON %s(timestamp)`, tableName, tableName)
	if _, err := db.Exec(indexSQL); err != nil {
		return fmt.Errorf("create index on %s: %w", tableName, err)
	}

	return nil
}

// DropAllTables drops all user-created tables (not sqlite internal tables).
func DropAllTables(db *sql.DB) error {
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		return fmt.Errorf("query tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("scan table name: %w", err)
		}
		tables = append(tables, name)
	}

	for _, table := range tables {
		dropSQL := fmt.Sprintf("DROP TABLE IF EXISTS %s", table)
		if _, err := db.Exec(dropSQL); err != nil {
			return fmt.Errorf("drop table %s: %w", table, err)
		}
	}

	// Vacuum to reclaim space
	if _, err := db.Exec("VACUUM"); err != nil {
		return fmt.Errorf("vacuum: %w", err)
	}

	return nil
}

// CleanupOldData deletes data older than retentionDays from all tables.
// Returns the total number of deleted rows.
func CleanupOldData(db *sql.DB, retentionDays int) (int, error) {
	if retentionDays <= 0 {
		return 0, nil
	}

	cutoffTime := time.Now().AddDate(0, 0, -retentionDays).Format("2006-01-02 15:04:05")
	log.Printf("清理 %d 天前的数据 (截止时间: %s)", retentionDays, cutoffTime)

	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		return 0, fmt.Errorf("query tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return 0, fmt.Errorf("scan table name: %w", err)
		}
		tables = append(tables, name)
	}

	totalDeleted := 0
	for _, table := range tables {
		deleteSQL := fmt.Sprintf("DELETE FROM %s WHERE timestamp < ?", table)
		result, err := db.Exec(deleteSQL, cutoffTime)
		if err != nil {
			log.Printf("清理表 %s 失败: %v", table, err)
			continue
		}
		affected, _ := result.RowsAffected()
		if affected > 0 {
			log.Printf("从表 %s 删除了 %d 条记录", table, affected)
			totalDeleted += int(affected)
		}
	}

	// Vacuum to reclaim space if data was deleted
	if totalDeleted > 0 {
		log.Printf("执行 VACUUM 回收空间...")
		if _, err := db.Exec("VACUUM"); err != nil {
			log.Printf("VACUUM 失败: %v", err)
		}
	}

	return totalDeleted, nil
}

// StartCleanupScheduler starts a background goroutine that periodically cleans up old data.
func StartCleanupScheduler(db *sql.DB, retentionDays int, interval time.Duration) {
	if retentionDays <= 0 {
		return
	}

	go func() {
		// Run cleanup immediately on start
		deleted, err := CleanupOldData(db, retentionDays)
		if err != nil {
			log.Printf("初始清理失败: %v", err)
		} else if deleted > 0 {
			log.Printf("初始清理完成: 删除了 %d 条记录", deleted)
		}

		// Then run periodically
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			deleted, err := CleanupOldData(db, retentionDays)
			if err != nil {
				log.Printf("定期清理失败: %v", err)
			} else if deleted > 0 {
				log.Printf("定期清理完成: 删除了 %d 条记录", deleted)
			}
		}
	}()
}
