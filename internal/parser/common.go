package parser

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var sanitizeRe = regexp.MustCompile(`[^a-zA-Z0-9_]`)

// ReadFileContent reads a .dat, .dat.gz, or .zip file and returns its content as string.
// For .zip files, it reads the first .dat or .dat.gz file found inside.
func ReadFileContent(filePath string) (string, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	if ext == ".zip" {
		return readZipContent(filePath)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read file %s: %w", filePath, err)
	}

	if strings.HasSuffix(filePath, ".gz") {
		reader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return "", fmt.Errorf("gzip reader %s: %w", filePath, err)
		}
		defer reader.Close()
		decompressed, err := io.ReadAll(reader)
		if err != nil {
			return "", fmt.Errorf("decompress %s: %w", filePath, err)
		}
		return string(decompressed), nil
	}

	return string(data), nil
}

// readZipContent reads the first .dat or .dat.gz file from a zip archive.
func readZipContent(zipPath string) (string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("open zip %s: %w", zipPath, err)
	}
	defer r.Close()

	// Find first .dat or .dat.gz file
	for _, f := range r.File {
		name := strings.ToLower(f.Name)
		if strings.HasSuffix(name, ".dat") || strings.HasSuffix(name, ".dat.gz") {
			rc, err := f.Open()
			if err != nil {
				return "", fmt.Errorf("open zip entry %s: %w", f.Name, err)
			}
			defer rc.Close()

			data, err := io.ReadAll(rc)
			if err != nil {
				return "", fmt.Errorf("read zip entry %s: %w", f.Name, err)
			}

			// If it's a .gz file inside the zip, decompress it
			if strings.HasSuffix(name, ".gz") {
				reader, err := gzip.NewReader(bytes.NewReader(data))
				if err != nil {
					return "", fmt.Errorf("gzip reader for %s: %w", f.Name, err)
				}
				defer reader.Close()
				decompressed, err := io.ReadAll(reader)
				if err != nil {
					return "", fmt.Errorf("decompress %s: %w", f.Name, err)
				}
				return string(decompressed), nil
			}

			return string(data), nil
		}
	}

	return "", fmt.Errorf("no .dat file found in zip: %s", zipPath)
}

// ParseTimestamp parses a "zzz ***<date>" line and returns a time.Time.
// The date format is like "Fri Jan 10 12:00:00 CST 2026".
func ParseTimestamp(line string) (time.Time, error) {
	if !strings.HasPrefix(line, "zzz ***") {
		return time.Time{}, fmt.Errorf("not a zzz timestamp line")
	}
	timeStr := strings.TrimPrefix(line, "zzz ***")
	timeStr = strings.TrimSpace(timeStr)

	// Try multiple layouts
	layouts := []string{
		"Mon Jan  2 15:04:05 MST 2006",
		"Mon Jan 2 15:04:05 MST 2006",
		"Mon Jan  2 15:04:05 2006",
		"Mon Jan 2 15:04:05 2006",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, timeStr); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse timestamp: %s", timeStr)
}

// ParseTimestampFormatted parses a zzz line and returns "YYYY-MM-DD HH:mm:ss" string.
func ParseTimestampFormatted(line string) string {
	t, err := ParseTimestamp(line)
	if err != nil {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

// ParseTimestampFromFilename extracts timestamp from filename pattern _{YY}.{MM}.{DD}.{HHMM}.dat(.gz)?
func ParseTimestampFromFilename(filename string) string {
	base := filepath.Base(filename)
	re := regexp.MustCompile(`_(\d{2})\.(\d{2})\.(\d{2})\.(\d{4})\.dat(\.gz)?$`)
	m := re.FindStringSubmatch(base)
	if m == nil {
		return ""
	}
	yy, mm, dd, hhmm := m[1], m[2], m[3], m[4]
	year := 2000 + parseInt(yy)
	hour := hhmm[:2]
	minute := hhmm[2:4]
	return fmt.Sprintf("%04d-%s-%s %s:%s:00", year, mm, dd, hour, minute)
}

// GetHostFromFilename extracts the hostname from a filename (part before first _).
func GetHostFromFilename(filename string) string {
	base := filepath.Base(filename)
	idx := strings.Index(base, "_")
	if idx > 0 {
		return base[:idx]
	}
	return "unknown"
}

// SanitizeTableName replaces non-alphanumeric/underscore characters with _.
func SanitizeTableName(name string) string {
	return sanitizeRe.ReplaceAllString(name, "_")
}

func parseInt(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}
