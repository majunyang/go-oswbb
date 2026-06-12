package parser

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"go-oswbb/internal/model"
)

type VmstatParser struct{}

func (p *VmstatParser) ParseFile(filePath string) (*model.VmstatParsedData, error) {
	content, err := ReadFileContent(filePath)
	if err != nil {
		return nil, fmt.Errorf("read vmstat file: %w", err)
	}
	return p.ParseContent(content)
}

func (p *VmstatParser) ParseContent(content string) (*model.VmstatParsedData, error) {
	lines := strings.Split(content, "\n")
	result := &model.VmstatParsedData{}

	var currentTimestamp time.Time
	var dataLines [][]string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "Linux OSWbb") {
			result.Metadata.Version = line
		} else if strings.HasPrefix(line, "SNAP_INTERVAL") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				result.Metadata.SnapInterval, _ = strconv.Atoi(parts[1])
			}
		} else if strings.HasPrefix(line, "CPU_CORES") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				result.Metadata.CPUCores, _ = strconv.Atoi(parts[1])
			}
		} else if strings.HasPrefix(line, "VCPUS") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				result.Metadata.VCPUs, _ = strconv.Atoi(parts[1])
			}
		} else if strings.HasPrefix(line, "zzz ***") {
			if !currentTimestamp.IsZero() && len(dataLines) > 0 {
				p.processDataBlock(result, currentTimestamp, dataLines)
			}
			ts, err := ParseTimestamp(line)
			if err == nil {
				currentTimestamp = ts
			}
			dataLines = nil
		} else if !currentTimestamp.IsZero() && !strings.Contains(line, "procs") && !strings.Contains(line, "---") {
			parts := strings.Fields(line)
			if len(parts) >= 17 {
				dataLines = append(dataLines, parts)
			}
		}
	}

	if !currentTimestamp.IsZero() && len(dataLines) > 0 {
		p.processDataBlock(result, currentTimestamp, dataLines)
	}

	return result, nil
}

func (p *VmstatParser) processDataBlock(result *model.VmstatParsedData, timestamp time.Time, dataLines [][]string) {
	result.Timestamps = append(result.Timestamps, timestamp)
	avg := calculateAverage(dataLines)

	result.Procs = append(result.Procs, model.VmstatProcsRecord{
		Timestamp: timestamp,
		R: avg[0],
		B: avg[1],
	})

	result.Memory = append(result.Memory, model.VmstatMemoryRecord{
		Timestamp: timestamp,
		Free:  avg[3],
		Buff:  avg[4],
		Cache: avg[5],
	})

	result.Swap = append(result.Swap, model.VmstatSwapRecord{
		Timestamp: timestamp,
		Swpd: avg[2],
		Si:   avg[6],
		So:   avg[7],
	})

	result.IO = append(result.IO, model.VmstatIORecord{
		Timestamp: timestamp,
		Bi: avg[8],
		Bo: avg[9],
	})

	result.System = append(result.System, model.VmstatSystemRecord{
		Timestamp: timestamp,
		In: avg[10],
		Cs: avg[11],
	})

	result.CPU = append(result.CPU, model.VmstatCPURecord{
		Timestamp: timestamp,
		Us: avg[12],
		Sy: avg[13],
		Id: avg[14],
		Wa: avg[15],
		St: avg[16],
	})
}

func calculateAverage(dataLines [][]string) []int {
	if len(dataLines) == 0 {
		return nil
	}
	if len(dataLines) == 1 {
		result := make([]int, len(dataLines[0]))
		for i, v := range dataLines[0] {
			result[i], _ = strconv.Atoi(v)
		}
		return result
	}

	colCount := len(dataLines[0])
	sums := make([]int, colCount)
	for _, line := range dataLines {
		for i, v := range line {
			if i < colCount {
				val, _ := strconv.Atoi(v)
				sums[i] += val
			}
		}
	}

	result := make([]int, colCount)
	for i, sum := range sums {
		result[i] = int(math.Round(float64(sum) / float64(len(dataLines))))
	}
	return result
}

// GetChartData generates chart data for a specific metric type.
func (p *VmstatParser) GetChartData(parsed *model.VmstatParsedData, metric string) *model.ChartResponse {
	var timestamps []time.Time
	switch metric {
	case "cpu":
		timestamps = make([]time.Time, len(parsed.CPU))
		for i, r := range parsed.CPU {
			timestamps[i] = r.Timestamp
		}
	case "memory":
		timestamps = make([]time.Time, len(parsed.Memory))
		for i, r := range parsed.Memory {
			timestamps[i] = r.Timestamp
		}
	case "io":
		timestamps = make([]time.Time, len(parsed.IO))
		for i, r := range parsed.IO {
			timestamps[i] = r.Timestamp
		}
	case "swap":
		timestamps = make([]time.Time, len(parsed.Swap))
		for i, r := range parsed.Swap {
			timestamps[i] = r.Timestamp
		}
	case "system":
		timestamps = make([]time.Time, len(parsed.System))
		for i, r := range parsed.System {
			timestamps[i] = r.Timestamp
		}
	case "procs":
		timestamps = make([]time.Time, len(parsed.Procs))
		for i, r := range parsed.Procs {
			timestamps[i] = r.Timestamp
		}
	default:
		return &model.ChartResponse{}
	}

	categories := make([]string, len(timestamps))
	for i, ts := range timestamps {
		categories[i] = ts.Format("01-02 15:04")
	}

	var series []model.ChartSeries

	switch metric {
	case "memory":
		series = []model.ChartSeries{
			{Name: "空闲内存(KB)", Data: extractMemoryFree(parsed.Memory)},
			{Name: "缓冲区(KB)", Data: extractMemoryBuff(parsed.Memory)},
			{Name: "缓存(KB)", Data: extractMemoryCache(parsed.Memory)},
		}
	case "cpu":
		series = []model.ChartSeries{
			{Name: "用户时间(%)", Data: extractCPUUs(parsed.CPU)},
			{Name: "系统时间(%)", Data: extractCPUSy(parsed.CPU)},
			{Name: "空闲时间(%)", Data: extractCPUId(parsed.CPU)},
			{Name: "等待IO(%)", Data: extractCPUWa(parsed.CPU)},
		}
	case "io":
		series = []model.ChartSeries{
			{Name: "读取(blocks/s)", Data: extractIOBi(parsed.IO)},
			{Name: "写入(blocks/s)", Data: extractIOBo(parsed.IO)},
		}
	case "swap":
		series = []model.ChartSeries{
			{Name: "已用交换空间(KB)", Data: extractSwapSwpd(parsed.Swap)},
			{Name: "换入(KB/s)", Data: extractSwapSi(parsed.Swap)},
			{Name: "换出(KB/s)", Data: extractSwapSo(parsed.Swap)},
		}
	case "system":
		series = []model.ChartSeries{
			{Name: "中断次数(/s)", Data: extractSystemIn(parsed.System)},
			{Name: "上下文切换(/s)", Data: extractSystemCs(parsed.System)},
		}
	case "procs":
		series = []model.ChartSeries{
			{Name: "运行队列(r)", Data: extractProcsR(parsed.Procs)},
			{Name: "阻塞队列(b)", Data: extractProcsB(parsed.Procs)},
		}
	}

	return &model.ChartResponse{Series: series, Categories: categories}
}

func extractMemoryFree(data []model.VmstatMemoryRecord) []float64 {
	r := make([]float64, len(data))
	for i, d := range data {
		r[i] = float64(d.Free)
	}
	return r
}
func extractMemoryBuff(data []model.VmstatMemoryRecord) []float64 {
	r := make([]float64, len(data))
	for i, d := range data {
		r[i] = float64(d.Buff)
	}
	return r
}
func extractMemoryCache(data []model.VmstatMemoryRecord) []float64 {
	r := make([]float64, len(data))
	for i, d := range data {
		r[i] = float64(d.Cache)
	}
	return r
}
func extractCPUUs(data []model.VmstatCPURecord) []float64 {
	r := make([]float64, len(data))
	for i, d := range data {
		r[i] = float64(d.Us)
	}
	return r
}
func extractCPUSy(data []model.VmstatCPURecord) []float64 {
	r := make([]float64, len(data))
	for i, d := range data {
		r[i] = float64(d.Sy)
	}
	return r
}
func extractCPUId(data []model.VmstatCPURecord) []float64 {
	r := make([]float64, len(data))
	for i, d := range data {
		r[i] = float64(d.Id)
	}
	return r
}
func extractCPUWa(data []model.VmstatCPURecord) []float64 {
	r := make([]float64, len(data))
	for i, d := range data {
		r[i] = float64(d.Wa)
	}
	return r
}
func extractIOBi(data []model.VmstatIORecord) []float64 {
	r := make([]float64, len(data))
	for i, d := range data {
		r[i] = float64(d.Bi)
	}
	return r
}
func extractIOBo(data []model.VmstatIORecord) []float64 {
	r := make([]float64, len(data))
	for i, d := range data {
		r[i] = float64(d.Bo)
	}
	return r
}
func extractSwapSwpd(data []model.VmstatSwapRecord) []float64 {
	r := make([]float64, len(data))
	for i, d := range data {
		r[i] = float64(d.Swpd)
	}
	return r
}
func extractSwapSi(data []model.VmstatSwapRecord) []float64 {
	r := make([]float64, len(data))
	for i, d := range data {
		r[i] = float64(d.Si)
	}
	return r
}
func extractSwapSo(data []model.VmstatSwapRecord) []float64 {
	r := make([]float64, len(data))
	for i, d := range data {
		r[i] = float64(d.So)
	}
	return r
}
func extractSystemIn(data []model.VmstatSystemRecord) []float64 {
	r := make([]float64, len(data))
	for i, d := range data {
		r[i] = float64(d.In)
	}
	return r
}
func extractSystemCs(data []model.VmstatSystemRecord) []float64 {
	r := make([]float64, len(data))
	for i, d := range data {
		r[i] = float64(d.Cs)
	}
	return r
}
func extractProcsR(data []model.VmstatProcsRecord) []float64 {
	r := make([]float64, len(data))
	for i, d := range data {
		r[i] = float64(d.R)
	}
	return r
}
func extractProcsB(data []model.VmstatProcsRecord) []float64 {
	r := make([]float64, len(data))
	for i, d := range data {
		r[i] = float64(d.B)
	}
	return r
}
