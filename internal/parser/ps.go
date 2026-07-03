package parser

import (
	"strconv"
	"strings"
	"time"
)

type PsRecord struct {
	Timestamp time.Time
	User      string
	PID       int
	PPID      int
	PRI       int
	CPU       float64
	Mem       float64
	VSZ       int64
	RSS       int64
	Wchan     string
	State     string
	Command   string
}

type PsParser struct{}

// ParsePsLine parses a single line from a ps output file.
func (p *PsParser) ParsePsLine(line string, ts time.Time) *PsRecord {
	parts := strings.Fields(line)
	if len(parts) < 11 {
		return nil
	}
	
	// Skip header line
	if parts[0] == "USER" && parts[1] == "PID" {
		return nil
	}

	pid, _ := strconv.Atoi(parts[1])
	ppid, _ := strconv.Atoi(parts[2])
	pri, _ := strconv.Atoi(parts[3])
	cpu, _ := strconv.ParseFloat(parts[4], 64)
	mem, _ := strconv.ParseFloat(parts[5], 64)
	vsz, _ := strconv.ParseInt(parts[6], 10, 64)
	rss, _ := strconv.ParseInt(parts[7], 10, 64)
	wchan := parts[8]
	state := parts[9]

	// Determine command based on STARTED column length (usually 1 or 2 parts)
	var cmd string
	// If column index 10 contains a colon, it's like "10:03:26" (1 token for STARTED)
	// Otherwise it's like "Sep 25" (2 tokens for STARTED)
	if strings.Contains(parts[10], ":") {
		// parts[10] = STARTED, parts[11] = TIME
		if len(parts) > 12 {
			cmd = strings.Join(parts[12:], " ")
		} else if len(parts) == 12 {
			cmd = parts[11]
		}
	} else {
		// parts[10], parts[11] = STARTED, parts[12] = TIME
		if len(parts) > 13 {
			cmd = strings.Join(parts[13:], " ")
		} else if len(parts) == 13 {
			cmd = parts[12]
		}
	}

	return &PsRecord{
		Timestamp: ts,
		User:      parts[0],
		PID:       pid,
		PPID:      ppid,
		PRI:       pri,
		CPU:       cpu,
		Mem:       mem,
		VSZ:       vsz,
		RSS:       rss,
		Wchan:     wchan,
		State:     state,
		Command:   cmd,
	}
}
