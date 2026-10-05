// internal/validation/file.go — File Validation
//
// This file validates files BEFORE uploading them. It's the Go equivalent
// of the web client's uploadValidation.ts.
//
// WHY validate on the client side?
//   - Fail fast: don't waste time uploading a 500MB file just to have the
//     server reject it. Tell the user immediately.
//   - Better UX: a clear error message like "File must be 100 MB or smaller"
//     is much friendlier than a server timeout or cryptic error.
//   - The server ALSO validates (defense in depth), but client-side validation
//     is faster and more user-friendly.

package validation

import (
	"fmt" // fmt — formatting error messages
	"os"  // os — file system operations (stat, check existence)
	"path/filepath"
	"strings"
)

// =========================================================================
// Constants — must match the web client's values!
// =========================================================================

// DefaultMaxUploadMB is the default maximum file size in megabytes.
// This matches MAX_UPLOAD_MB = 500 in client/src/lib/uploadValidation.ts
// and the server's default in FileController constructor (configurable up to 8192 MB).
const DefaultMaxUploadMB = 500

// =========================================================================
// ResolvePath — expand home directory (~) and strip quotes
// =========================================================================

// ResolvePath resolves tilde (~) expansion and strips quotes from a file path.
func ResolvePath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.Trim(path, "\"'")

	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	} else if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// =========================================================================
// ValidateFile — check if a file is suitable for upload
// =========================================================================

// ValidateFile checks that a file exists, is a regular file (not a directory),
// and is within the size limit.
//
// Parameters:
//   path      — the file path to validate
//   maxSizeMB — maximum allowed size in megabytes (0 = use default 100MB)
//
// Returns:
//   fileSize (int64) — the file size in bytes (useful for progress bar)
//   error            — nil if valid, descriptive error message if not
//
// This mirrors validateUploadFile() from uploadValidation.ts:
//   if (file.size > MAX_UPLOAD_BYTES) {
//     return `File must be ${MAX_UPLOAD_MB} MB or smaller`;
//   }
func ValidateFile(path string, maxSizeMB int) (int64, error) {
	path = ResolvePath(path)

	// Use default if not specified
	if maxSizeMB <= 0 {
		maxSizeMB = DefaultMaxUploadMB
	}

	// --- Check 1: Does the file exist? ---
	// os.Stat returns file metadata (name, size, permissions, etc.)
	// If the file doesn't exist, it returns an error.
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			// os.IsNotExist checks if the error means "file not found".
			// We give a clearer message than Go's default error.
			return 0, fmt.Errorf("file not found: %s", path)
		}
		// Some other error (permissions, broken symlink, etc.)
		return 0, fmt.Errorf("cannot access file: %w", err)
	}

	// --- Check 2: Is it a regular file (not a directory/symlink/device)? ---
	// info.IsDir() returns true if the path is a directory.
	// We can only upload regular files, not directories.
	if info.IsDir() {
		return 0, fmt.Errorf("%s is a directory, not a file", path)
	}

	// --- Check 3: Is the file within the size limit? ---
	// info.Size() returns the file size in bytes.
	// We convert maxSizeMB to bytes: 100 MB = 100 * 1024 * 1024 = 104,857,600 bytes
	fileSize := info.Size()
	maxSizeBytes := int64(maxSizeMB) * 1024 * 1024

	if fileSize > maxSizeBytes {
		return fileSize, fmt.Errorf(
			"file must be %d MB or smaller (selected: %s)",
			maxSizeMB,
			FormatFileSize(fileSize),
		)
	}

	// --- Check 4: Is the file empty? ---
	// The server rejects empty files anyway, but better to catch it early.
	if fileSize == 0 {
		return 0, fmt.Errorf("file is empty: %s", path)
	}

	// All checks passed!
	return fileSize, nil
}

// =========================================================================
// FormatFileSize — human-readable file size
// =========================================================================

// FormatFileSize converts bytes to a human-readable string.
// This mirrors formatFileSize() from uploadValidation.ts:
//   if (bytes < 1024) return `${bytes} B`;
//   if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
//   return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
//
// Examples:
//   FormatFileSize(500)         → "500 B"
//   FormatFileSize(15360)       → "15.0 KB"
//   FormatFileSize(104857600)   → "100.00 MB"
//   FormatFileSize(1073741824)  → "1.00 GB"
func FormatFileSize(bytes int64) string {
	// We use 1024-based units (KiB/MiB/GiB) but label them KB/MB/GB
	// because that's what users expect and what the web UI shows.

	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case bytes < KB:
		return fmt.Sprintf("%d B", bytes)
	case bytes < MB:
		// %.1f = one decimal place: "15.0 KB"
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	case bytes < GB:
		// %.2f = two decimal places: "100.00 MB"
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	default:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	}
}
