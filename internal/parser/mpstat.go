package parser

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"go-oswbb/internal/model"
)

type MpstatParser struct{}

func (p *MpstatParser) ParseFile(filePath string) (*model.MpstatParsedData, error) {
	content, err := ReadFileContent(filePath)
	if err != nil {
		return nil, fmt.Errorf("read mpstat file: %w", err)
	}
	return p.ParseContent(content)
}

func (p *MpstatParser) ParseContent(content string) (*model.MpstatParsedData, error) {
	lines := strings.Split(content, "\n")
	result := &model.MpstatParsedData{
		CPUs: make(map[string][]model.MpstatRecord),
	}

	var currentTimestamp time.Time
	var currentDate string
	inDataSection := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse timestamp
		if strings.HasPrefix(line, "zzz ***") {
			ts, err := ParseTimestamp(line)
			if err == nil {
				currentTimestamp = ts
				currentDate = ts.Format("2006-01-02")
			}
			inDataSection = false
			continue
		}

		if currentTimestamp.IsZero() {
			continue
		}

		// Skip Average lines
		if strings.HasPrefix(line, "Average:") {
			continue
		}

		// Detect header line
		if strings.Contains(line, "CPU") && strings.Contains(line, "%usr") && strings.Contains(line, "%idle") {
			inDataSection = true
			continue
		}

		// Parse data lines
		if inDataSection {
			parts := strings.Fields(line)
			if len(parts) < 12 {
				continue
			}

			timeStr := parts[0] // HH:MM:SS
			cpuID := parts[1]   // all, 0, 1, 2, ...

			// Skip non-CPU data lines
			if cpuID != "all" {
				if _, err := strconv.Atoi(cpuID); err != nil {
					continue
				}
			}

			// Build full timestamp by combining date with time
			fullTs, err := time.Parse("2006-01-02 15:04:05", currentDate+" "+timeStr)
			if err != nil {
				fullTs = currentTimestamp
			}

			record := model.MpstatRecord{
				Timestamp: fullTs,
				CPUID:     cpuID,
				Usr:       parseFloat(parts, 2),
				Nice:      parseFloat(parts, 3),
				Sys:       parseFloat(parts, 4),
				IOWait:    parseFloat(parts, 5),
				IRQ:       parseFloat(parts, 6),
				Soft:      parseFloat(parts, 7),
				Steal:     parseFloat(parts, 8),
				Guest:     parseFloat(parts, 9),
				GNice:     parseFloat(parts, 10),
				Idle:      parseFloat(parts, 11),
			}

			result.CPUs[cpuID] = append(result.CPUs[cpuID], record)
		}
	}

	// Collect all timestamps
	tsSet := make(map[int64]bool)
	for _, records := range result.CPUs {
		for _, r := range records {
			tsSet[r.Timestamp.Unix()] = true
		}
	}
	var timestamps []time.Time
	for ts := range tsSet {
		timestamps = append(timestamps, time.Unix(ts, 0))
	}
	sortTimestamps(timestamps)
	result.Timestamps = timestamps

	return result, nil
}

// GetCpuList returns sorted CPU IDs with "all" first.
func (p *MpstatParser) GetCpuList(parsed *model.MpstatParsedData) []string {
	var ids []string
	hasAll := false
	for id := range parsed.CPUs {
		if id == "all" {
			hasAll = true
		} else {
			ids = append(ids, id)
		}
	}
	sortStrings(ids)
	if hasAll {
		ids = append([]string{"all"}, ids...)
	}
	return ids
}

// GetAllCpuChartData generates chart data for all CPUs.
func (p *MpstatParser) GetAllCpuChartData(parsed *model.MpstatParsedData, metric string, selectedCpus []string) *model.ChartResponse {
	cpuIDs := p.GetCpuList(parsed)
	if len(selectedCpus) > 0 {
		cpuIDs = filterStrings(cpuIDs, selectedCpus)
	}

	if len(cpuIDs) == 0 {
		return &model.ChartResponse{}
	}

	// Use "all" or first CPU's timestamps as categories
	refID := cpuIDs[0]
	if contains(cpuIDs, "all") {
		refID = "all"
	}
	refData := parsed.CPUs[refID]
	if len(refData) == 0 {
		return &model.ChartResponse{}
	}

	categories := make([]string, len(refData))
	for i, item := range refData {
		categories[i] = item.Timestamp.Format("01-02 15:04")
	}

	var series []model.ChartSeries

	for _, cpuID := range cpuIDs {
		data, ok := parsed.CPUs[cpuID]
		if !ok {
			continue
		}

		name := "CPU " + cpuID
		if cpuID == "all" {
			name = "CPU All"
		}

		var values []float64
		switch metric {
		case "usage":
			values = extractMpstatField(data, "usage")
		case "user":
			values = extractMpstatField(data, "user")
		case "system":
			values = extractMpstatField(data, "system")
		case "iowait":
			values = extractMpstatField(data, "iowait")
		case "interrupt":
			values = extractMpstatField(data, "interrupt")
		case "steal":
			values = extractMpstatField(data, "steal")
		default:
			values = extractMpstatField(data, "usage")
		}

		series = append(series, model.ChartSeries{Name: name, Data: values})
	}

	return &model.ChartResponse{Series: series, Categories: categories, CPUs: cpuIDs}
}

func extractMpstatField(data []model.MpstatRecord, field string) []float64 {
	r := make([]float64, len(data))
	for i, d := range data {
		var v float64
		switch field {
		case "usage":
			v = 100 - d.Idle
		case "user":
			v = d.Usr + d.Nice
		case "system":
			v = d.Sys
		case "iowait":
			v = d.IOWait
		case "interrupt":
			v = d.IRQ + d.Soft
		case "steal":
			v = d.Steal
		}
		r[i] = math.Round(v*100) / 100
	}
	return r
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Ensure IQR field is correctly referenced
func (p *MpstatParser) GetMetricOptions() []map[string]string {
	return []map[string]string{
		{"value": "usage", "label": "CPU 使用率 (%)"},
		{"value": "user", "label": "用户态 (%usr+%nice)"},
		{"value": "system", "label": "系统态 (%sys)"},
		{"value": "iowait", "label": "I/O 等待 (%iowait)"},
		{"value": "interrupt", "label": "中断 (%irq+%soft)"},
		{"value": "steal", "label": "虚拟化偷取 (%steal)"},
	}
}
