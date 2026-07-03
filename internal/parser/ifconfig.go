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

// regex rules for interface detection
var (
	ifaceRe1 = regexp.MustCompile(`^(\d+):\s+(\S+?):\s+<.*>`)             // ip -s link
	ifaceRe2 = regexp.MustCompile(`^([a-zA-Z0-9_\-\.:]+):\s+flags=\d+<.*>`) // new ifconfig
	ifaceRe3 = regexp.MustCompile(`^([a-zA-Z0-9_\-\.:]+)\s+Link encap:`)   // old ifconfig
)

// regex rules for data parsing in traditional ifconfig output
var (
	rxPacketsNewRe = regexp.MustCompile(`^RX\s+packets\s+(\d+)\s+bytes\s+(\d+)`)
	rxErrorsNewRe  = regexp.MustCompile(`^RX\s+errors\s+(\d+)\s+dropped\s+(\d+)`)
	txPacketsNewRe = regexp.MustCompile(`^TX\s+packets\s+(\d+)\s+bytes\s+(\d+)`)
	txErrorsNewRe  = regexp.MustCompile(`^TX\s+errors\s+(\d+)\s+dropped\s+(\d+)`)

	rxPacketsOldRe = regexp.MustCompile(`^RX\s+packets:(\d+)\s+errors:(\d+)\s+dropped:(\d+)`)
	txPacketsOldRe = regexp.MustCompile(`^TX\s+packets:(\d+)\s+errors:(\d+)\s+dropped:(\d+)`)
	bytesOldRe     = regexp.MustCompile(`RX\s+bytes:(\d+).*?TX\s+bytes:(\d+)`)
)

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

		// Parse interface line
		var matchIface string
		if m := ifaceRe1.FindStringSubmatch(line); m != nil {
			matchIface = strings.SplitN(m[2], "@", 2)[0]
		} else if m := ifaceRe2.FindStringSubmatch(line); m != nil {
			matchIface = m[1]
		} else if m := ifaceRe3.FindStringSubmatch(line); m != nil {
			matchIface = m[1]
		}

		if matchIface != "" {
			currentInterface = matchIface
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

		// --- Traditional ifconfig Format Parsers ---
		// 1. New RX packets
		if m := rxPacketsNewRe.FindStringSubmatch(line); m != nil {
			rxPkts, _ := strconv.ParseInt(m[1], 10, 64)
			rxBytes, _ := strconv.ParseInt(m[2], 10, 64)
			updateRecord(result, currentInterface, currentTimestamp, func(r *model.IfconfigRecord) {
				r.RxPackets = rxPkts
				r.RxBytes = rxBytes
			})
			continue
		}
		// 2. New RX errors
		if m := rxErrorsNewRe.FindStringSubmatch(line); m != nil {
			rxErr, _ := strconv.ParseInt(m[1], 10, 64)
			rxDrop, _ := strconv.ParseInt(m[2], 10, 64)
			updateRecord(result, currentInterface, currentTimestamp, func(r *model.IfconfigRecord) {
				r.RxErrors = rxErr
				r.RxDropped = rxDrop
			})
			continue
		}
		// 3. New TX packets
		if m := txPacketsNewRe.FindStringSubmatch(line); m != nil {
			txPkts, _ := strconv.ParseInt(m[1], 10, 64)
			txBytes, _ := strconv.ParseInt(m[2], 10, 64)
			updateRecord(result, currentInterface, currentTimestamp, func(r *model.IfconfigRecord) {
				r.TxPackets = txPkts
				r.TxBytes = txBytes
			})
			continue
		}
		// 4. New TX errors
		if m := txErrorsNewRe.FindStringSubmatch(line); m != nil {
			txErr, _ := strconv.ParseInt(m[1], 10, 64)
			txDrop, _ := strconv.ParseInt(m[2], 10, 64)
			updateRecord(result, currentInterface, currentTimestamp, func(r *model.IfconfigRecord) {
				r.TxErrors = txErr
				r.TxDropped = txDrop
			})
			continue
		}
		// 5. Old RX packets
		if m := rxPacketsOldRe.FindStringSubmatch(line); m != nil {
			rxPkts, _ := strconv.ParseInt(m[1], 10, 64)
			rxErr, _ := strconv.ParseInt(m[2], 10, 64)
			rxDrop, _ := strconv.ParseInt(m[3], 10, 64)
			updateRecord(result, currentInterface, currentTimestamp, func(r *model.IfconfigRecord) {
				r.RxPackets = rxPkts
				r.RxErrors = rxErr
				r.RxDropped = rxDrop
			})
			continue
		}
		// 6. Old TX packets
		if m := txPacketsOldRe.FindStringSubmatch(line); m != nil {
			txPkts, _ := strconv.ParseInt(m[1], 10, 64)
			txErr, _ := strconv.ParseInt(m[2], 10, 64)
			txDrop, _ := strconv.ParseInt(m[3], 10, 64)
			updateRecord(result, currentInterface, currentTimestamp, func(r *model.IfconfigRecord) {
				r.TxPackets = txPkts
				r.TxErrors = txErr
				r.TxDropped = txDrop
			})
			continue
		}
		// 7. Old RX/TX bytes
		if m := bytesOldRe.FindStringSubmatch(line); m != nil {
			rxBytes, _ := strconv.ParseInt(m[1], 10, 64)
			txBytes, _ := strconv.ParseInt(m[2], 10, 64)
			updateRecord(result, currentInterface, currentTimestamp, func(r *model.IfconfigRecord) {
				r.RxBytes = rxBytes
				r.TxBytes = txBytes
			})
			continue
		}

		// --- Original ip -s link Format Parsers ---
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
				updateRecord(result, currentInterface, currentTimestamp, func(r *model.IfconfigRecord) {
					r.RxBytes = values[0]
					r.RxPackets = values[1]
					if len(values) > 2 {
						r.RxErrors = values[2]
					}
					if len(values) > 3 {
						r.RxDropped = values[3]
					}
					if len(values) > 4 {
						r.RxMissed = values[4]
					}
				})
				gotRxData = true
			}
			expectingRxData = false
			continue
		}

		// Parse TX data
		if expectingTxData {
			values := parseIntValues(line)
			if len(values) >= 2 && gotRxData {
				updateRecord(result, currentInterface, currentTimestamp, func(r *model.IfconfigRecord) {
					r.TxBytes = values[0]
					r.TxPackets = values[1]
					if len(values) > 2 {
						r.TxErrors = values[2]
					}
					if len(values) > 3 {
						r.TxDropped = values[3]
					}
				})
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

func updateRecord(result *model.IfconfigParsedData, iface string, ts time.Time, updateFn func(*model.IfconfigRecord)) {
	if iface == "" || ts.IsZero() {
		return
	}
	records := result.Interfaces[iface]
	found := false
	for i := range records {
		if records[i].Timestamp.Equal(ts) {
			updateFn(&records[i])
			found = true
			break
		}
	}
	if !found {
		record := model.IfconfigRecord{
			Timestamp: ts,
			Interface: iface,
		}
		updateFn(&record)
		records = append(records, record)
	}
	result.Interfaces[iface] = records
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
