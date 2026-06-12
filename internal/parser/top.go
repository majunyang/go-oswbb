package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go-oswbb/internal/model"
)

type TopParser struct{}

func (p *TopParser) ParseFile(filePath string) (*model.TopParsedData, error) {
	content, err := ReadFileContent(filePath)
	if err != nil {
		return nil, fmt.Errorf("read top file: %w", err)
	}
	return p.ParseContent(content)
}

var (
	topLoadRe   = regexp.MustCompile(`load average:\s*([\d.]+),\s*([\d.]+),\s*([\d.]+)`)
	topTasksRe  = regexp.MustCompile(`(\d+)\s+total,\s*(\d+)\s+running,\s*(\d+)\s+sleeping`)
	topCpuRe    = regexp.MustCompile(`([\d.]+)%us,\s*([\d.]+)%sy,\s*([\d.]+)%ni,\s*([\d.]+)%id,\s*([\d.]+)%wa`)
	topMemRe    = regexp.MustCompile(`(\d+)k\s+total,\s*(\d+)k\s+used,\s*(\d+)k\s+free`)
)

func (p *TopParser) ParseContent(content string) (*model.TopParsedData, error) {
	lines := strings.Split(content, "\n")
	result := &model.TopParsedData{}

	var currentTimestamp time.Time
	var currentSnapshot *model.TopSnapshot
	var procs []model.TopProcess
	inProcSection := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "zzz ***") {
			// Save previous snapshot
			if currentSnapshot != nil && !currentTimestamp.IsZero() {
				result.Timestamps = append(result.Timestamps, currentTimestamp)
				result.Snapshots = append(result.Snapshots, *currentSnapshot)
				if len(procs) > 0 {
					result.TopProcs = procs // keep the latest
				}
			}
			ts, err := ParseTimestamp(line)
			if err == nil {
				currentTimestamp = ts
			}
			currentSnapshot = &model.TopSnapshot{Timestamp: currentTimestamp}
			procs = nil
			inProcSection = false
			continue
		}

		if currentSnapshot == nil {
			continue
		}

		// Parse "top -" header line for load average
		if strings.HasPrefix(line, "top -") {
			if m := topLoadRe.FindStringSubmatch(line); m != nil {
				currentSnapshot.Load1, _ = strconv.ParseFloat(m[1], 64)
				currentSnapshot.Load5, _ = strconv.ParseFloat(m[2], 64)
				currentSnapshot.Load15, _ = strconv.ParseFloat(m[3], 64)
			}
			inProcSection = false
			continue
		}

		// Parse Tasks line
		if strings.HasPrefix(line, "Tasks:") {
			if m := topTasksRe.FindStringSubmatch(line); m != nil {
				currentSnapshot.TasksTotal, _ = strconv.Atoi(m[1])
				currentSnapshot.TasksRunning, _ = strconv.Atoi(m[2])
				currentSnapshot.TasksSleeping, _ = strconv.Atoi(m[3])
			}
			inProcSection = false
			continue
		}

		// Parse Cpu(s) line
		if strings.HasPrefix(line, "Cpu(s):") {
			if m := topCpuRe.FindStringSubmatch(line); m != nil {
				currentSnapshot.CpuUs, _ = strconv.ParseFloat(m[1], 64)
				currentSnapshot.CpuSy, _ = strconv.ParseFloat(m[2], 64)
				currentSnapshot.CpuNi, _ = strconv.ParseFloat(m[3], 64)
				currentSnapshot.CpuId, _ = strconv.ParseFloat(m[4], 64)
				currentSnapshot.CpuWa, _ = strconv.ParseFloat(m[5], 64)
			}
			inProcSection = false
			continue
		}

		// Parse Mem line
		if strings.HasPrefix(line, "Mem:") {
			if m := topMemRe.FindStringSubmatch(line); m != nil {
				currentSnapshot.MemTotal, _ = strconv.ParseInt(m[1], 10, 64)
				currentSnapshot.MemUsed, _ = strconv.ParseInt(m[2], 10, 64)
				currentSnapshot.MemFree, _ = strconv.ParseInt(m[3], 10, 64)
			}
			inProcSection = false
			continue
		}

		// Parse Swap line
		if strings.HasPrefix(line, "Swap:") {
			if m := topMemRe.FindStringSubmatch(line); m != nil {
				currentSnapshot.SwapTotal, _ = strconv.ParseInt(m[1], 10, 64)
				currentSnapshot.SwapUsed, _ = strconv.ParseInt(m[2], 10, 64)
				currentSnapshot.SwapFree, _ = strconv.ParseInt(m[3], 10, 64)
			}
			inProcSection = false
			continue
		}

		// PID header line marks start of process section
		if strings.HasPrefix(line, "PID") && strings.Contains(line, "%CPU") {
			inProcSection = true
			continue
		}

		// Parse process lines
		if inProcSection {
			proc := parseTopProcessLine(line)
			if proc != nil {
				procs = append(procs, *proc)
			}
		}
	}

	// Save last snapshot
	if currentSnapshot != nil && !currentTimestamp.IsZero() {
		result.Timestamps = append(result.Timestamps, currentTimestamp)
		result.Snapshots = append(result.Snapshots, *currentSnapshot)
		if len(procs) > 0 {
			result.TopProcs = procs
		}
	}

	return result, nil
}

func parseTopProcessLine(line string) *model.TopProcess {
	parts := strings.Fields(line)
	if len(parts) < 12 {
		return nil
	}

	pid, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil
	}

	cpu, _ := strconv.ParseFloat(parts[8], 64)
	mem, _ := strconv.ParseFloat(parts[9], 64)
	cmd := parts[11]
	if len(parts) > 12 {
		cmd = strings.Join(parts[11:], " ")
	}

	return &model.TopProcess{
		PID:     pid,
		User:    parts[1],
		CPU:     cpu,
		Mem:     mem,
		Command: cmd,
	}
}

// GetChartData generates chart data for a specific metric type.
func (p *TopParser) GetChartData(parsed *model.TopParsedData, metric string) *model.ChartResponse {
	if len(parsed.Snapshots) == 0 {
		return &model.ChartResponse{}
	}

	categories := make([]string, len(parsed.Timestamps))
	for i, ts := range parsed.Timestamps {
		categories[i] = ts.Format("01-02 15:04")
	}

	var series []model.ChartSeries

	switch metric {
	case "load":
		series = []model.ChartSeries{
			{Name: "1分钟负载", Data: extractTopField(parsed.Snapshots, "load1")},
			{Name: "5分钟负载", Data: extractTopField(parsed.Snapshots, "load5")},
			{Name: "15分钟负载", Data: extractTopField(parsed.Snapshots, "load15")},
		}
	case "tasks":
		series = []model.ChartSeries{
			{Name: "总任务数", Data: extractTopField(parsed.Snapshots, "tasksTotal")},
			{Name: "运行中", Data: extractTopField(parsed.Snapshots, "tasksRunning")},
			{Name: "睡眠中", Data: extractTopField(parsed.Snapshots, "tasksSleeping")},
		}
	case "cpu":
		series = []model.ChartSeries{
			{Name: "用户态(%)", Data: extractTopField(parsed.Snapshots, "cpuUs")},
			{Name: "系统态(%)", Data: extractTopField(parsed.Snapshots, "cpuSy")},
			{Name: "空闲(%)", Data: extractTopField(parsed.Snapshots, "cpuId")},
			{Name: "IO等待(%)", Data: extractTopField(parsed.Snapshots, "cpuWa")},
		}
	case "memory":
		series = []model.ChartSeries{
			{Name: "内存总量(MB)", Data: extractTopField(parsed.Snapshots, "memTotal")},
			{Name: "内存已用(MB)", Data: extractTopField(parsed.Snapshots, "memUsed")},
			{Name: "内存空闲(MB)", Data: extractTopField(parsed.Snapshots, "memFree")},
			{Name: "Swap总量(MB)", Data: extractTopField(parsed.Snapshots, "swapTotal")},
			{Name: "Swap已用(MB)", Data: extractTopField(parsed.Snapshots, "swapUsed")},
		}
	case "topcpu":
		return p.getTopCpuChartData(parsed)
	}

	return &model.ChartResponse{Series: series, Categories: categories}
}

func (p *TopParser) getTopCpuChartData(parsed *model.TopParsedData) *model.ChartResponse {
	if len(parsed.TopProcs) == 0 {
		return &model.ChartResponse{}
	}

	// Take top 15 by CPU
	procs := make([]model.TopProcess, len(parsed.TopProcs))
	copy(procs, parsed.TopProcs)
	sortTopProcsByCpu(procs)
	if len(procs) > 15 {
		procs = procs[:15]
	}

	categories := make([]string, len(procs))
	cpuData := make([]float64, len(procs))
	memData := make([]float64, len(procs))
	for i, proc := range procs {
		label := fmt.Sprintf("%d(%s)", proc.PID, proc.Command)
		if len(label) > 20 {
			label = label[:20] + "..."
		}
		categories[i] = label
		cpuData[i] = round2(proc.CPU)
		memData[i] = round2(proc.Mem)
	}

	series := []model.ChartSeries{
		{Name: "CPU(%)", Data: cpuData},
		{Name: "MEM(%)", Data: memData},
	}

	return &model.ChartResponse{Series: series, Categories: categories}
}

func sortTopProcsByCpu(procs []model.TopProcess) {
	for i := 1; i < len(procs); i++ {
		for j := i; j > 0 && procs[j].CPU > procs[j-1].CPU; j-- {
			procs[j], procs[j-1] = procs[j-1], procs[j]
		}
	}
}

func extractTopField(data []model.TopSnapshot, field string) []float64 {
	r := make([]float64, len(data))
	for i, d := range data {
		switch field {
		case "load1":
			r[i] = round2(d.Load1)
		case "load5":
			r[i] = round2(d.Load5)
		case "load15":
			r[i] = round2(d.Load15)
		case "tasksTotal":
			r[i] = float64(d.TasksTotal)
		case "tasksRunning":
			r[i] = float64(d.TasksRunning)
		case "tasksSleeping":
			r[i] = float64(d.TasksSleeping)
		case "cpuUs":
			r[i] = round2(d.CpuUs)
		case "cpuSy":
			r[i] = round2(d.CpuSy)
		case "cpuNi":
			r[i] = round2(d.CpuNi)
		case "cpuId":
			r[i] = round2(d.CpuId)
		case "cpuWa":
			r[i] = round2(d.CpuWa)
		case "cpuSt":
			r[i] = round2(d.CpuSt)
		case "memTotal":
			r[i] = round2(float64(d.MemTotal) / 1024.0)
		case "memUsed":
			r[i] = round2(float64(d.MemUsed) / 1024.0)
		case "memFree":
			r[i] = round2(float64(d.MemFree) / 1024.0)
		case "swapTotal":
			r[i] = round2(float64(d.SwapTotal) / 1024.0)
		case "swapUsed":
			r[i] = round2(float64(d.SwapUsed) / 1024.0)
		case "swapFree":
			r[i] = round2(float64(d.SwapFree) / 1024.0)
		}
	}
	return r
}
