package sandbox

import (
	"archive/tar"
	"bytes"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// createTar creates a tar archive containing a single file
// It also includes parent directory entries to ensure they are created
func createTar(path string, content []byte) io.Reader {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	// Ensure path is absolute
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Remove leading slash for tar
	tarPath := path[1:]

	// Add parent directory entries
	dir := filepath.Dir(tarPath)
	if dir != "." && dir != "" {
		dirs := []string{}
		for d := dir; d != "." && d != ""; d = filepath.Dir(d) {
			dirs = append([]string{d}, dirs...)
		}
		for _, d := range dirs {
			dirHdr := &tar.Header{
				Name:     d + "/",
				Mode:     0755,
				Typeflag: tar.TypeDir,
			}
			_ = tw.WriteHeader(dirHdr)
		}
	}

	// Add file entry
	hdr := &tar.Header{
		Name: tarPath,
		Mode: 0644,
		Size: int64(len(content)),
	}

	_ = tw.WriteHeader(hdr)
	_, _ = tw.Write(content)
	_ = tw.Close()

	// Return a new reader from the buffer bytes
	return bytes.NewReader(buf.Bytes())
}

// extractTar extracts content from a tar archive
func extractTar(reader io.Reader) ([]byte, error) {
	tr := tar.NewReader(reader)

	for {
		_, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// Read file content
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, tr); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}

	return nil, io.EOF
}

// parseLsOutput parses the output of ls -la command
// Supports both GNU coreutils and BusyBox ls output formats
func parseLsOutput(output string, basePath string) []FileInfo {
	var files []FileInfo

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "total") {
			continue
		}

		// Parse ls -la output
		// GNU format: -rw-r--r-- 1 root root 1234 Jan 15 10:30 filename
		// BusyBox format: -rw-r--r--    1 root     root          1234 Jan 15 10:30 filename
		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}

		// The filename is always the last field
		name := fields[len(fields)-1]

		// Skip . and ..
		if name == "." || name == ".." {
			continue
		}

		perms := fields[0]
		isDir := strings.HasPrefix(perms, "d")

		// Size is at index 4
		size, _ := strconv.ParseInt(fields[4], 10, 64)

		// Parse date/time (fields 5, 6, 7 are typically: Month Day Time/Year)
		// We'll use current time as default since parsing ls date format is complex
		modTime := time.Now()

		files = append(files, FileInfo{
			Name:    name,
			Path:    filepath.Join(basePath, name),
			Size:    size,
			IsDir:   isDir,
			ModTime: modTime,
		})
	}

	return files
}
