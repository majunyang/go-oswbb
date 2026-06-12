package archive

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ExtractTarArchive extracts a .tar or .tar.gz archive to destDir and returns the OSWatcher archive path.
func ExtractTarArchive(archivePath, destDir string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()

	var tr *tar.Reader

	if strings.HasSuffix(archivePath, ".gz") || strings.HasSuffix(archivePath, ".tgz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return "", fmt.Errorf("gzip reader: %w", err)
		}
		defer gz.Close()
		tr = tar.NewReader(gz)
	} else {
		tr = tar.NewReader(f)
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("create dest dir: %w", err)
	}

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read tar entry: %w", err)
		}

		// Sanitize path to prevent path traversal
		cleanName := filepath.Clean(header.Name)
		if strings.HasPrefix(cleanName, "..") || strings.HasPrefix(cleanName, "/") {
			continue
		}

		target := filepath.Join(destDir, cleanName)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return "", fmt.Errorf("create dir %s: %w", target, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return "", fmt.Errorf("create parent dir: %w", err)
			}
			outFile, err := os.Create(target)
			if err != nil {
				return "", fmt.Errorf("create file %s: %w", target, err)
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return "", fmt.Errorf("extract file %s: %w", target, err)
			}
			outFile.Close()
		}
	}

	// Find the OSWatcher archive directory
	archivePath = FindArchivePath(destDir)
	return archivePath, nil
}

// FindArchivePath scans the directory tree for the OSWatcher archive root.
// It looks for a directory containing subdirectories starting with "osw".
func FindArchivePath(extractedDir string) string {
	entries, err := os.ReadDir(extractedDir)
	if err != nil {
		return extractedDir
	}

	// Check if current directory has osw* subdirectories
	hasOsw := false
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(strings.ToLower(entry.Name()), "osw") {
			hasOsw = true
			break
		}
	}
	if hasOsw {
		return extractedDir
	}

	// Check one level deeper (hostname directory)
	for _, entry := range entries {
		if entry.IsDir() {
			subPath := filepath.Join(extractedDir, entry.Name())
			subEntries, err := os.ReadDir(subPath)
			if err != nil {
				continue
			}
			for _, sub := range subEntries {
				if sub.IsDir() && strings.HasPrefix(strings.ToLower(sub.Name()), "osw") {
					return subPath
				}
			}
		}
	}

	return extractedDir
}

// ScanDirectory scans the archive path for .dat/.dat.gz files organized by type.
// Returns a map like {"vmstat": [...], "iostat": [...], "ifconfig": [...], "mpstat": [...]}.
func ScanDirectory(archivePath string) map[string][]string {
	result := map[string][]string{
		"vmstat":   {},
		"iostat":   {},
		"ifconfig": {},
		"mpstat":   {},
		"meminfo":  {},
		"top":      {},
	}

	typeMap := map[string]string{
		"vmstat":   "vmstat",
		"iostat":   "iostat",
		"ifconfig": "ifconfig",
		"mpstat":   "mpstat",
		"meminfo":  "meminfo",
		"top":      "top",
	}

	entries, err := os.ReadDir(archivePath)
	if err != nil {
		return result
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		lowerName := strings.ToLower(entry.Name())

		for key, pattern := range typeMap {
			if strings.Contains(lowerName, pattern) {
				dirPath := filepath.Join(archivePath, entry.Name())
				files := collectDatFiles(dirPath)
				result[key] = append(result[key], files...)
				break
			}
		}
	}

	// Sort files for consistent ordering
	for key := range result {
		sort.Strings(result[key])
	}

	return result
}

func collectDatFiles(dirPath string) []string {
	var files []string
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return files
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if strings.HasSuffix(name, ".dat") || strings.HasSuffix(name, ".dat.gz") || strings.HasSuffix(name, ".zip") {
			files = append(files, filepath.Join(dirPath, entry.Name()))
		}
	}
	return files
}

// CleanupDir removes a directory and all its contents.
func CleanupDir(dirPath string) error {
	return os.RemoveAll(dirPath)
}
