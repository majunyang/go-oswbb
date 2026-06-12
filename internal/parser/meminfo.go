package parser

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"go-oswbb/internal/model"
)

type MeminfoParser struct{}

func (p *MeminfoParser) ParseFile(filePath string) (*model.MeminfoParsedData, error) {
	content, err := ReadFileContent(filePath)
	if err != nil {
		return nil, fmt.Errorf("read meminfo file: %w", err)
	}
	return p.ParseContent(content)
}

func (p *MeminfoParser) ParseContent(content string) (*model.MeminfoParsedData, error) {
	lines := strings.Split(content, "\n")
	result := &model.MeminfoParsedData{}

	var currentTimestamp time.Time
	var currentRecord *model.MeminfoRecord

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "zzz ***") {
			// Save previous record
			if currentRecord != nil && !currentTimestamp.IsZero() {
				result.Timestamps = append(result.Timestamps, currentTimestamp)
				result.Records = append(result.Records, *currentRecord)
			}
			ts, err := ParseTimestamp(line)
			if err == nil {
				currentTimestamp = ts
			}
			currentRecord = &model.MeminfoRecord{Timestamp: currentTimestamp}
			continue
		}

		if currentRecord == nil {
			continue
		}

		// Parse key: value kB lines
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		valStr := strings.TrimSpace(line[idx+1:])
		valStr = strings.TrimSuffix(valStr, " kB")
		valStr = strings.TrimSpace(valStr)

		val, err := strconv.ParseInt(valStr, 10, 64)
		if err != nil {
			continue
		}

		switch key {
		case "MemTotal":
			currentRecord.MemTotal = val
		case "MemFree":
			currentRecord.MemFree = val
		case "Buffers":
			currentRecord.Buffers = val
		case "Cached":
			currentRecord.Cached = val
		case "SwapTotal":
			currentRecord.SwapTotal = val
		case "SwapFree":
			currentRecord.SwapFree = val
		case "SwapCached":
			currentRecord.SwapCached = val
		case "Active":
			currentRecord.Active = val
		case "Inactive":
			currentRecord.Inactive = val
		case "Active(anon)":
			currentRecord.ActiveAnon = val
		case "Inactive(anon)":
			currentRecord.InactiveAnon = val
		case "Active(file)":
			currentRecord.ActiveFile = val
		case "Inactive(file)":
			currentRecord.InactiveFile = val
		case "Dirty":
			currentRecord.Dirty = val
		case "Writeback":
			currentRecord.Writeback = val
		case "AnonPages":
			currentRecord.AnonPages = val
		case "Mapped":
			currentRecord.Mapped = val
		case "Shmem":
			currentRecord.Shmem = val
		case "Slab":
			currentRecord.Slab = val
		case "SReclaimable":
			currentRecord.SReclaimable = val
		case "SUnreclaim":
			currentRecord.SUnreclaim = val
		case "PageTables":
			currentRecord.PageTables = val
		case "KernelStack":
			currentRecord.KernelStack = val
		case "HugePages_Total":
			currentRecord.HugePagesTotal = val
		case "HugePages_Free":
			currentRecord.HugePagesFree = val
		case "Committed_AS":
			currentRecord.CommittedAS = val
		}
	}

	// Don't forget the last snapshot
	if currentRecord != nil && !currentTimestamp.IsZero() {
		result.Timestamps = append(result.Timestamps, currentTimestamp)
		result.Records = append(result.Records, *currentRecord)
	}

	return result, nil
}

// GetChartData generates chart data for a specific metric type.
func (p *MeminfoParser) GetChartData(parsed *model.MeminfoParsedData, metric string) *model.ChartResponse {
	if len(parsed.Records) == 0 {
		return &model.ChartResponse{}
	}

	categories := make([]string, len(parsed.Timestamps))
	for i, ts := range parsed.Timestamps {
		categories[i] = ts.Format("01-02 15:04")
	}

	var series []model.ChartSeries

	switch metric {
	case "memory":
		series = []model.ChartSeries{
			{Name: "总内存(MB)", Data: extractMeminfoField(parsed.Records, "memTotal")},
			{Name: "空闲内存(MB)", Data: extractMeminfoField(parsed.Records, "memFree")},
			{Name: "缓冲区(MB)", Data: extractMeminfoField(parsed.Records, "buffers")},
			{Name: "缓存(MB)", Data: extractMeminfoField(parsed.Records, "cached")},
		}
	case "swap":
		series = []model.ChartSeries{
			{Name: "Swap总量(MB)", Data: extractMeminfoField(parsed.Records, "swapTotal")},
			{Name: "Swap空闲(MB)", Data: extractMeminfoField(parsed.Records, "swapFree")},
			{Name: "SwapCached(MB)", Data: extractMeminfoField(parsed.Records, "swapCached")},
		}
	case "detail":
		series = []model.ChartSeries{
			{Name: "Active(MB)", Data: extractMeminfoField(parsed.Records, "active")},
			{Name: "Inactive(MB)", Data: extractMeminfoField(parsed.Records, "inactive")},
			{Name: "AnonPages(MB)", Data: extractMeminfoField(parsed.Records, "anonPages")},
			{Name: "Mapped(MB)", Data: extractMeminfoField(parsed.Records, "mapped")},
			{Name: "Shmem(MB)", Data: extractMeminfoField(parsed.Records, "shmem")},
			{Name: "Slab(MB)", Data: extractMeminfoField(parsed.Records, "slab")},
		}
	case "hugepages":
		series = []model.ChartSeries{
			{Name: "HugePages总量", Data: extractMeminfoField(parsed.Records, "hugePagesTotal")},
			{Name: "HugePages空闲", Data: extractMeminfoField(parsed.Records, "hugePagesFree")},
		}
	}

	return &model.ChartResponse{Series: series, Categories: categories}
}

func extractMeminfoField(records []model.MeminfoRecord, field string) []float64 {
	r := make([]float64, len(records))
	for i, d := range records {
		var val int64
		switch field {
		case "memTotal":
			val = d.MemTotal
		case "memFree":
			val = d.MemFree
		case "buffers":
			val = d.Buffers
		case "cached":
			val = d.Cached
		case "swapTotal":
			val = d.SwapTotal
		case "swapFree":
			val = d.SwapFree
		case "swapCached":
			val = d.SwapCached
		case "active":
			val = d.Active
		case "inactive":
			val = d.Inactive
		case "activeAnon":
			val = d.ActiveAnon
		case "inactiveAnon":
			val = d.InactiveAnon
		case "activeFile":
			val = d.ActiveFile
		case "inactiveFile":
			val = d.InactiveFile
		case "dirty":
			val = d.Dirty
		case "writeback":
			val = d.Writeback
		case "anonPages":
			val = d.AnonPages
		case "mapped":
			val = d.Mapped
		case "shmem":
			val = d.Shmem
		case "slab":
			val = d.Slab
		case "sReclaimable":
			val = d.SReclaimable
		case "sUnreclaim":
			val = d.SUnreclaim
		case "pageTables":
			val = d.PageTables
		case "kernelStack":
			val = d.KernelStack
		case "hugePagesTotal":
			val = d.HugePagesTotal
		case "hugePagesFree":
			val = d.HugePagesFree
		case "committedAS":
			val = d.CommittedAS
		}
		// Convert kB to MB for most fields (except hugepages which are counts)
		if field != "hugePagesTotal" && field != "hugePagesFree" {
			r[i] = float64(val) / 1024.0
		} else {
			r[i] = float64(val)
		}
	}
	return r
}
