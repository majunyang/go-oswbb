package parser

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"go-oswbb/internal/model"
)

type IostatParser struct{}

func (p *IostatParser) ParseFile(filePath string) (*model.IostatParsedData, error) {
	content, err := ReadFileContent(filePath)
	if err != nil {
		return nil, fmt.Errorf("read iostat file: %w", err)
	}
	return p.ParseContent(content)
}

func (p *IostatParser) ParseContent(content string) (*model.IostatParsedData, error) {
	lines := strings.Split(content, "\n")
	result := &model.IostatParsedData{
		Devices: make(map[string][]model.IostatRecord),
	}

	var currentTimestamp time.Time
	inDataSection := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "zzz ***") {
			ts, err := ParseTimestamp(line)
			if err == nil {
				currentTimestamp = ts
			}
			inDataSection = false
			continue
		}

		if currentTimestamp.IsZero() {
			continue
		}

		if strings.HasPrefix(line, "Device") {
			inDataSection = true
			continue
		}

		if inDataSection {
			parts := strings.Fields(line)
			if len(parts) < 2 || strings.Contains(parts[0], ":") {
				continue
			}

			deviceName := parts[0]

			// Skip loop devices only (keep dm- devices for LVM/RAID)
			if strings.HasPrefix(deviceName, "loop") {
				continue
			}

			record := model.IostatRecord{
				Timestamp: currentTimestamp,
				Device:    deviceName,
				RS:        parseFloat(parts, 1),
				WS:        parseFloat(parts, 2),
				RKBs:      parseFloat(parts, 3),
				WKBs:      parseFloat(parts, 4),
				RRQMs:     parseFloat(parts, 5),
				WRQMs:     parseFloat(parts, 6),
				PRRQM:     parseFloat(parts, 7),
				PWRQM:     parseFloat(parts, 8),
				RAwait:    parseFloat(parts, 9),
				WAwait:    parseFloat(parts, 10),
				AquSz:     parseFloat(parts, 11),
				RareqSz:   parseFloat(parts, 12),
				WareqSz:   parseFloat(parts, 13),
				Svctm:     parseFloat(parts, 14),
				Util:      parseFloat(parts, 15),
			}

			result.Devices[deviceName] = append(result.Devices[deviceName], record)
		}
	}

	// Collect all timestamps
	tsSet := make(map[int64]bool)
	for _, records := range result.Devices {
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

func parseFloat(parts []string, index int) float64 {
	if index >= len(parts) {
		return 0
	}
	v, _ := strconv.ParseFloat(parts[index], 64)
	return v
}

func sortTimestamps(ts []time.Time) {
	for i := 1; i < len(ts); i++ {
		for j := i; j > 0 && ts[j].Before(ts[j-1]); j-- {
			ts[j], ts[j-1] = ts[j-1], ts[j]
		}
	}
}

// GetDeviceList returns sorted device names.
func (p *IostatParser) GetDeviceList(parsed *model.IostatParsedData) []string {
	var devices []string
	for name := range parsed.Devices {
		devices = append(devices, name)
	}
	sortStrings(devices)
	return devices
}

// GetAllDevicesChartData generates chart data for all devices.
func (p *IostatParser) GetAllDevicesChartData(parsed *model.IostatParsedData, metric string, selectedDevices []string) *model.ChartResponse {
	deviceNames := p.GetDeviceList(parsed)
	if len(selectedDevices) > 0 {
		deviceNames = filterStrings(deviceNames, selectedDevices)
	}

	if len(deviceNames) == 0 {
		return &model.ChartResponse{}
	}

	// Use first device's timestamps as categories
	firstDevice := deviceNames[0]
	deviceData := parsed.Devices[firstDevice]
	if len(deviceData) == 0 {
		return &model.ChartResponse{}
	}

	categories := make([]string, len(deviceData))
	for i, item := range deviceData {
		categories[i] = item.Timestamp.Format("01-02 15:04")
	}

	var series []model.ChartSeries

	switch metric {
	case "iops":
		for _, name := range deviceNames {
			if data, ok := parsed.Devices[name]; ok {
				series = append(series,
					model.ChartSeries{Name: name + " r/s", Data: extractField(data, "rs")},
					model.ChartSeries{Name: name + " w/s", Data: extractField(data, "ws")},
				)
			}
		}
	case "throughput":
		for _, name := range deviceNames {
			if data, ok := parsed.Devices[name]; ok {
				series = append(series,
					model.ChartSeries{Name: name + " rkB/s", Data: extractField(data, "rkBs")},
					model.ChartSeries{Name: name + " wkB/s", Data: extractField(data, "wkBs")},
				)
			}
		}
	case "await":
		for _, name := range deviceNames {
			if data, ok := parsed.Devices[name]; ok {
				series = append(series,
					model.ChartSeries{Name: name + " r_await", Data: extractField(data, "rAwait")},
					model.ChartSeries{Name: name + " w_await", Data: extractField(data, "wAwait")},
				)
			}
		}
	case "util":
		for _, name := range deviceNames {
			if data, ok := parsed.Devices[name]; ok {
				series = append(series,
					model.ChartSeries{Name: name + " %util", Data: extractField(data, "util")},
				)
			}
		}
	case "queue":
		for _, name := range deviceNames {
			if data, ok := parsed.Devices[name]; ok {
				series = append(series,
					model.ChartSeries{Name: name + " aqu-sz", Data: extractField(data, "aquSz")},
				)
			}
		}
	case "reqsize":
		for _, name := range deviceNames {
			if data, ok := parsed.Devices[name]; ok {
				series = append(series,
					model.ChartSeries{Name: name + " rareq-sz", Data: extractField(data, "rareqSz")},
					model.ChartSeries{Name: name + " wareq-sz", Data: extractField(data, "wareqSz")},
				)
			}
		}
	}

	return &model.ChartResponse{Series: series, Categories: categories, Devices: deviceNames}
}

func extractField(data []model.IostatRecord, field string) []float64 {
	r := make([]float64, len(data))
	for i, d := range data {
		switch field {
		case "rs":
			r[i] = round2(d.RS)
		case "ws":
			r[i] = round2(d.WS)
		case "rkBs":
			r[i] = round2(d.RKBs)
		case "wkBs":
			r[i] = round2(d.WKBs)
		case "rAwait":
			r[i] = round2(d.RAwait)
		case "wAwait":
			r[i] = round2(d.WAwait)
		case "aquSz":
			r[i] = round2(d.AquSz)
		case "util":
			r[i] = round2(d.Util)
		case "rareqSz":
			r[i] = round2(d.RareqSz)
		case "wareqSz":
			r[i] = round2(d.WareqSz)
		}
	}
	return r
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func filterStrings(all, selected []string) []string {
	set := make(map[string]bool)
	for _, s := range selected {
		set[s] = true
	}
	var result []string
	for _, s := range all {
		if set[s] {
			result = append(result, s)
		}
	}
	return result
}
