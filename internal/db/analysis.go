package db

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"go-oswbb/internal/model"
	"go-oswbb/internal/parser"
)

type procStats struct {
	command   string
	shortName string
	totalCpu  float64
	count     int
	maxCpu    float64
	maxMem    int64
	totalMem  int64
	pids      map[int]bool
}

type abnormalKey struct {
	pid   int
	state string
}

// AnalyzeOswps queries the oswps table and aggregates diagnostics in Go memory with time filtering
func AnalyzeOswps(db *sql.DB, startTime, endTime string) (*model.OswpsAnalysisResult, error) {
	// Check if table exists
	var exists int
	err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='oswps'").Scan(&exists)
	if err != nil || exists == 0 {
		return nil, fmt.Errorf("table oswps does not exist or db error: %w", err)
	}

	query := "SELECT timestamp, content FROM oswps"
	var args []interface{}
	if startTime != "" && endTime != "" {
		query += " WHERE timestamp >= ? AND timestamp <= ?"
		args = append(args, startTime, endTime)
	} else if startTime != "" {
		query += " WHERE timestamp >= ?"
		args = append(args, startTime)
	} else if endTime != "" {
		query += " WHERE timestamp <= ?"
		args = append(args, endTime)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query oswps: %w", err)
	}
	defer rows.Close()

	psParser := &parser.PsParser{}
	procMap := make(map[string]*procStats)
	abnormalMap := make(map[abnormalKey]*model.AbnormalStateProc)
	tsSet := make(map[string]bool)

	zombieCount := 0
	dStateCount := 0

	for rows.Next() {
		var tsStr, content string
		if err := rows.Scan(&tsStr, &content); err != nil {
			continue
		}

		if tsStr != "" {
			tsSet[tsStr] = true
		}

		ts, err := time.Parse("2006-01-02 15:04:05", tsStr)
		if err != nil {
			ts = time.Time{}
		}

		r := psParser.ParsePsLine(content, ts)
		if r == nil {
			continue
		}

		shortName := getProcessShortName(r.Command)

		// 1. Accumulate resource usage
		stats, ok := procMap[shortName]
		if !ok {
			stats = &procStats{
				command:   r.Command,
				shortName: shortName,
				pids:      make(map[int]bool),
			}
			procMap[shortName] = stats
		}
		stats.totalCpu += r.CPU
		stats.count++
		if r.CPU > stats.maxCpu {
			stats.maxCpu = r.CPU
		}
		if r.RSS > stats.maxMem {
			stats.maxMem = r.RSS
		}
		stats.totalMem += r.RSS
		stats.pids[r.PID] = true

		// 2. Track abnormal states (Zombie or Uninterruptible Sleep D)
		if r.State == "Z" || r.State == "D" {
			if r.State == "Z" {
				zombieCount++
			} else {
				dStateCount++
			}

			key := abnormalKey{pid: r.PID, state: r.State}
			item, exist := abnormalMap[key]
			formattedTs := tsStr
			if formattedTs == "" {
				formattedTs = "未知"
			}
			if !exist {
				item = &model.AbnormalStateProc{
					FirstTimestamp: formattedTs,
					LastTimestamp:  formattedTs,
					PID:            r.PID,
					PPID:           r.PPID,
					User:           r.User,
					State:          r.State,
					Command:        r.Command,
					Count:          1,
				}
				abnormalMap[key] = item
			} else {
				item.Count++
				if formattedTs != "未知" {
					if item.FirstTimestamp == "未知" || formattedTs < item.FirstTimestamp {
						item.FirstTimestamp = formattedTs
					}
					if item.LastTimestamp == "未知" || formattedTs > item.LastTimestamp {
						item.LastTimestamp = formattedTs
					}
				}
			}
		}
	}

	result := &model.OswpsAnalysisResult{
		TotalSnapshots: len(tsSet),
		ZombieCount:    zombieCount,
		DStateCount:    dStateCount,
	}

	var cpuList []model.HighResourceProc
	var memList []model.HighResourceProc
	var forkList []model.ForkRateProc

	for name, stats := range procMap {
		avgCpu := stats.totalCpu / float64(stats.count)
		avgMem := float64(stats.totalMem) / float64(stats.count) / 1024.0 // Convert RSS KB to MB

		cpuList = append(cpuList, model.HighResourceProc{
			Command:   stats.command,
			ShortName: name,
			MaxVal:    stats.maxCpu,
			AvgVal:    roundFloat(avgCpu),
		})

		memList = append(memList, model.HighResourceProc{
			Command:   stats.command,
			ShortName: name,
			MaxVal:    roundFloat(float64(stats.maxMem) / 1024.0),
			AvgVal:    roundFloat(avgMem),
		})

		forkList = append(forkList, model.ForkRateProc{
			Command:   stats.command,
			ShortName: name,
			ForkCount: len(stats.pids),
		})
	}

	// Sort CPU list
	sort.Slice(cpuList, func(i, j int) bool {
		if cpuList[i].MaxVal != cpuList[j].MaxVal {
			return cpuList[i].MaxVal > cpuList[j].MaxVal
		}
		return cpuList[i].AvgVal > cpuList[j].AvgVal
	})
	if len(cpuList) > 10 {
		result.HighCpuProcs = cpuList[:10]
	} else {
		result.HighCpuProcs = cpuList
	}

	// Sort Memory list
	sort.Slice(memList, func(i, j int) bool {
		if memList[i].MaxVal != memList[j].MaxVal {
			return memList[i].MaxVal > memList[j].MaxVal
		}
		return memList[i].AvgVal > memList[j].AvgVal
	})
	if len(memList) > 10 {
		result.HighMemProcs = memList[:10]
	} else {
		result.HighMemProcs = memList
	}

	// Sort Fork/PIDs list (only active fork rates > 1 PID)
	sort.Slice(forkList, func(i, j int) bool {
		return forkList[i].ForkCount > forkList[j].ForkCount
	})
	var activeForks []model.ForkRateProc
	for _, f := range forkList {
		if f.ForkCount > 1 {
			activeForks = append(activeForks, f)
		}
	}
	if len(activeForks) > 10 {
		result.ForkBombProcs = activeForks[:10]
	} else {
		result.ForkBombProcs = activeForks
	}

	// Sort Abnormal states
	var abnormalList []model.AbnormalStateProc
	for _, item := range abnormalMap {
		abnormalList = append(abnormalList, *item)
	}
	sort.Slice(abnormalList, func(i, j int) bool {
		if abnormalList[i].LastTimestamp != abnormalList[j].LastTimestamp {
			return abnormalList[i].LastTimestamp > abnormalList[j].LastTimestamp
		}
		return abnormalList[i].Count > abnormalList[j].Count
	})
	if len(abnormalList) > 30 {
		result.AbnormalProcs = abnormalList[:30]
	} else {
		result.AbnormalProcs = abnormalList
	}

	return result, nil
}

func getProcessShortName(cmd string) string {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return "unknown"
	}
	binaryPath := parts[0]
	binaryPath = strings.TrimPrefix(binaryPath, "[")
	binaryPath = strings.TrimSuffix(binaryPath, "]")

	idx := strings.LastIndex(binaryPath, "/")
	if idx >= 0 {
		return binaryPath[idx+1:]
	}
	idxBack := strings.LastIndex(binaryPath, "\\")
	if idxBack >= 0 {
		return binaryPath[idxBack+1:]
	}
	return binaryPath
}

func roundFloat(v float64) float64 {
	return float64(int(v*100)) / 100
}
