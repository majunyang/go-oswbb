package parser

import (
	"testing"
	"time"
)

func TestParsePsLine(t *testing.T) {
	parser := &PsParser{}
	now := time.Now()

	// Test case 1: STARTED is "Sep 25" (2 tokens)
	line1 := "root     18545     1  19  0.0  1.2 3667740 3265144 poll_s S   Sep 25 02:56:53 /usr/lib/udisks2/udisksd --no-debug"
	r1 := parser.ParsePsLine(line1, now)
	if r1 == nil {
		t.Fatalf("expected non-nil record for line1")
	}
	if r1.User != "root" || r1.PID != 18545 || r1.PPID != 1 || r1.PRI != 19 || r1.CPU != 0.0 || r1.Mem != 1.2 {
		t.Errorf("r1 parsed basic fields mismatch: %+v", r1)
	}
	if r1.VSZ != 3667740 || r1.RSS != 3265144 || r1.Wchan != "poll_s" || r1.State != "S" {
		t.Errorf("r1 parsed detail fields mismatch: %+v", r1)
	}
	if r1.Command != "/usr/lib/udisks2/udisksd --no-debug" {
		t.Errorf("r1 parsed command mismatch: expected '/usr/lib/udisks2/udisksd --no-debug', got '%s'", r1.Command)
	}

	// Test case 2: STARTED is "10:03:26" (1 token)
	line2 := "root     34659 34168  19  0.0  0.0 216028  3156 wait   S 10:03:26 00:00:00 su - oracle"
	r2 := parser.ParsePsLine(line2, now)
	if r2 == nil {
		t.Fatalf("expected non-nil record for line2")
	}
	if r2.User != "root" || r2.PID != 34659 || r2.PPID != 34168 || r2.State != "S" {
		t.Errorf("r2 parsed basic fields mismatch: %+v", r2)
	}
	if r2.Command != "su - oracle" {
		t.Errorf("r2 parsed command mismatch: expected 'su - oracle', got '%s'", r2.Command)
	}

	// Test case 3: header line should return nil
	line3 := "USER       PID  PPID PRI %CPU %MEM    VSZ   RSS WCHAN  S  STARTED     TIME COMMAND"
	r3 := parser.ParsePsLine(line3, now)
	if r3 != nil {
		t.Errorf("expected header line to return nil, got %+v", r3)
	}
}
