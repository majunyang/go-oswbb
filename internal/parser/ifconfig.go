package parser

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go-oswbb/internal/model"
)

var ifaceRe = regexp.MustCompile(`^(\d+):\s+(\S+?):\s+<.*>`)

type IfconfigParser struct{}

func (p *IfconfigParser) ParseFile(filePath string) (*model.IfconfigParsedData, error) {
	content, err := ReadFileContent(filePath)
	if err != nil {
		return nil, fmt.Errorf("read ifconfig file: %w", err)
	}
	return p.ParseContent(content)
}

func (p *IfconfigParser) ParseContent(content string) (*model.IfconfigParsedData, error) {
	lines := strings.Split(content, "\n")
	result := &model.IfconfigParsedData{
		Interfaces: make(map[string][]model.IfconfigRecord),
	}

	var currentTimestamp time.Time
	var currentInterface string
	expectingRxData := false
	expectingTxData := false
	gotRxData := false

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
			}
			continue
		}

		if currentTimestamp.IsZero() {
			continue
		}

		// Parse interface line (e.g., "10: team0: <BROADCAST,MULTICAST,UP,LOWER_UP>")
		if m := ifaceRe.FindStringSubmatch(line); m != nil {
			currentInterface = strings.SplitN(m[2], "@", 2)[0]
			if currentInterface == "lo" {
				currentInterface = ""
			}
			expectingRxData = false
			expectingTxData = false
			gotRxData = false
			continue
		}

		if currentInterface == "" {
			continue
		}

		// RX statistics header
		if strings.HasPrefix(line, "RX:") && strings.Contains(line, "bytes") {
			expectingRxData = true
			expectingTxData = false
			continue
		}

		// TX statistics header
		if strings.HasPrefix(line, "TX:") && strings.Contains(line, "bytes") {
			expectingTxData = true
			expectingRxData = false
			continue
		}

		// Skip error lines
		if strings.HasPrefix(line, "RX errors:") || strings.HasPrefix(line, "TX errors:") {
			continue
		}

		// Parse RX data
		if expectingRxData {
			values := parseIntValues(line)
			if len(values) >= 2 {
				// Find or create record for this timestamp
				records := result.Interfaces[currentInterface]
				found := false
				for i := range records {
					if records[i].Timestamp.Equal(currentTimestamp) {
						records[i].RxBytes = values[0]
						records[i].RxPackets = values[1]
						if len(values) > 2 {
							records[i].RxErrors = values[2]
						}
						if len(values) > 3 {
							records[i].RxDropped = values[3]
						}
						if len(values) > 4 {
							records[i].RxMissed = values[4]
						}
						found = true
						break
					}
				}
				if !found {
					record := model.IfconfigRecord{
						Timestamp: currentTimestamp,
						Interface: currentInterface,
						RxBytes:   values[0],
						RxPackets: values[1],
					}
					if len(values) > 2 {
						record.RxErrors = values[2]
					}
					if len(values) > 3 {
						record.RxDropped = values[3]
					}
					if len(values) > 4 {
						record.RxMissed = values[4]
					}
					records = append(records, record)
				}
				result.Interfaces[currentInterface] = records
				gotRxData = true
			}
			expectingRxData = false
			continue
		}

		// Parse TX data
		if expectingTxData {
			values := parseIntValues(line)
			if len(values) >= 2 && gotRxData {
				records := result.Interfaces[currentInterface]
				for i := range records {
					if records[i].Timestamp.Equal(currentTimestamp) {
						records[i].TxBytes = values[0]
						records[i].TxPackets = values[1]
						if len(values) > 2 {
							records[i].TxErrors = values[2]
						}
						if len(values) > 3 {
							records[i].TxDropped = values[3]
						}
						break
					}
				}
				result.Interfaces[currentInterface] = records
			}
			expectingTxData = false
			continue
		}
	}

	// Collect all timestamps
	tsSet := make(map[int64]bool)
	for _, records := range result.Interfaces {
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

func parseIntValues(line string) []int64 {
	parts := strings.Fields(line)
	var values []int64
	for _, p := range parts {
		if v, err := strconv.ParseInt(p, 10, 64); err == nil {
			values = append(values, v)
		}
	}
	return values
}

// CalculateThroughput computes per-second rates by diffing consecutive snapshots.
func (p *IfconfigParser) CalculateThroughput(records []model.IfconfigRecord) []model.IfconfigThroughput {
	if len(records) < 2 {
		return nil
	}

	throughput := make([]model.IfconfigThroughput, 0, len(records)-1)
	for i := 1; i < len(records); i++ {
		prev := records[i-1]
		curr := records[i]
		timeDiff := curr.Timestamp.Sub(prev.Timestamp).Seconds()

		if timeDiff > 0 {
			throughput = append(throughput, model.IfconfigThroughput{
				Timestamp:       curr.Timestamp,
				RxBytesPerSec:   math.Max(0, float64(curr.RxBytes-prev.RxBytes)/timeDiff),
				TxBytesPerSec:   math.Max(0, float64(curr.TxBytes-prev.TxBytes)/timeDiff),
				RxPacketsPerSec: math.Max(0, float64(curr.RxPackets-prev.RxPackets)/timeDiff),
				TxPacketsPerSec: math.Max(0, float64(curr.TxPackets-prev.TxPackets)/timeDiff),
				RxErrorsPerSec:  math.Max(0, float64(curr.RxErrors-prev.RxErrors)/timeDiff),
				TxErrorsPerSec:  math.Max(0, float64(curr.TxErrors-prev.TxErrors)/timeDiff),
				RxDroppedPerSec: math.Max(0, float64(curr.RxDropped-prev.RxDropped)/timeDiff),
				TxDroppedPerSec: math.Max(0, float64(curr.TxDropped-prev.TxDropped)/timeDiff),
				RxMissedPerSec:  math.Max(0, float64(curr.RxMissed-prev.RxMissed)/timeDiff),
			})
		}
	}
	return throughput
}

// GetInterfaceList returns sorted interface names (excluding lo).
func (p *IfconfigParser) GetInterfaceList(parsed *model.IfconfigParsedData) []string {
	var names []string
	for name := range parsed.Interfaces {
		if name != "lo" {
			names = append(names, name)
		}
	}
	sortStrings(names)
	return names
}

// GetAllInterfacesChartData generates chart data for all interfaces.
func (p *IfconfigParser) GetAllInterfacesChartData(parsed *model.IfconfigParsedData, metric string, selectedInterfaces []string) *model.ChartResponse {
	interfaceNames := p.GetInterfaceList(parsed)
	if len(selectedInterfaces) > 0 {
		interfaceNames = filterStrings(interfaceNames, selectedInterfaces)
	}

	if len(interfaceNames) == 0 {
		return &model.ChartResponse{}
	}

	// Compute throughput for all interfaces
	allThroughput := make(map[string][]model.IfconfigThroughput)
	for _, name := range interfaceNames {
		if records, ok := parsed.Interfaces[name]; ok && len(records) >= 2 {
			allThroughput[name] = p.CalculateThroughput(records)
		}
	}

	// Find first interface with data for categories
	var categories []string
	for _, name := range interfaceNames {
		if tp, ok := allThroughput[name]; ok && len(tp) > 0 {
			categories = make([]string, len(tp))
			for i, item := range tp {
				categories[i] = item.Timestamp.Format("01-02 15:04")
			}
			break
		}
	}

	var series []model.ChartSeries

	switch metric {
	case "throughput":
		for _, name := range interfaceNames {
			if tp, ok := allThroughput[name]; ok {
				series = append(series,
					model.ChartSeries{
						Name: name + " RX (MB/s)",
						Data: extractThroughputField(tp, "rxBytesPerSec", 1024*1024),
					},
					model.ChartSeries{
						Name: name + " TX (MB/s)",
						Data: extractThroughputField(tp, "txBytesPerSec", 1024*1024),
					},
				)
			}
		}
	case "packets":
		for _, name := range interfaceNames {
			if tp, ok := allThroughput[name]; ok {
				series = append(series,
					model.ChartSeries{Name: name + " RX pkts/s", Data: extractThroughputInt(tp, "rxPacketsPerSec")},
					model.ChartSeries{Name: name + " TX pkts/s", Data: extractThroughputInt(tp, "txPacketsPerSec")},
				)
			}
		}
	case "errors":
		for _, name := range interfaceNames {
			if tp, ok := allThroughput[name]; ok {
				series = append(series,
					model.ChartSeries{Name: name + " RX Errors/s", Data: extractThroughputField(tp, "rxErrorsPerSec", 1)},
					model.ChartSeries{Name: name + " TX Errors/s", Data: extractThroughputField(tp, "txErrorsPerSec", 1)},
				)
			}
		}
	case "dropped":
		for _, name := range interfaceNames {
			if tp, ok := allThroughput[name]; ok {
				series = append(series,
					model.ChartSeries{Name: name + " RX Dropped/s", Data: extractThroughputField(tp, "rxDroppedPerSec", 1)},
					model.ChartSeries{Name: name + " TX Dropped/s", Data: extractThroughputField(tp, "txDroppedPerSec", 1)},
					model.ChartSeries{Name: name + " RX Missed/s", Data: extractThroughputField(tp, "rxMissedPerSec", 1)},
				)
			}
		}
	}

	return &model.ChartResponse{Series: series, Categories: categories, Interfaces: interfaceNames}
}

func extractThroughputField(data []model.IfconfigThroughput, field string, divisor float64) []float64 {
	r := make([]float64, len(data))
	for i, d := range data {
		var v float64
		switch field {
		case "rxBytesPerSec":
			v = d.RxBytesPerSec
		case "txBytesPerSec":
			v = d.TxBytesPerSec
		case "rxErrorsPerSec":
			v = d.RxErrorsPerSec
		case "txErrorsPerSec":
			v = d.TxErrorsPerSec
		case "rxDroppedPerSec":
			v = d.RxDroppedPerSec
		case "txDroppedPerSec":
			v = d.TxDroppedPerSec
		case "rxMissedPerSec":
			v = d.RxMissedPerSec
		}
		r[i] = math.Round(v/divisor*100) / 100
	}
	return r
}

func extractThroughputInt(data []model.IfconfigThroughput, field string) []float64 {
	r := make([]float64, len(data))
	for i, d := range data {
		var v float64
		switch field {
		case "rxPacketsPerSec":
			v = d.RxPacketsPerSec
		case "txPacketsPerSec":
			v = d.TxPacketsPerSec
		}
		r[i] = math.Round(v)
	}
	return r
}
