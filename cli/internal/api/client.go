// internal/api/client.go — HTTP Client for the Transfera API
//
// This file is the BRIDGE between the CLI commands and the C++ server.
// Every HTTP request (health check, upload, download) goes through this client.
//
// WHY a separate package?
//   - Separation of concerns: commands handle user interaction (flags, output),
//     this package handles HTTP communication
//   - Testability: you can mock this client in tests
//   - Reusability: all three commands share the same HTTP configuration
//
// WHY "internal/"?
//   In Go, the "internal" directory is special — code inside it can ONLY be
//   imported by code in the parent directory. This means external packages
//   can't use our API client directly. It's Go's way of saying "private".

package api

import (
	"bytes"
	"encoding/json" // json — encode/decode JSON (for parsing server responses)
	"fmt"           // fmt — string formatting (building error messages)
	"io"            // io — input/output interfaces (streaming data between sources)
	"mime"          // mime — MIME type parsing (extracting filenames from headers)
	"mime/multipart" // multipart — building multipart/form-data requests (for file uploads)
	"net/http"      // net/http — Go's built-in HTTP client/server (no external library needed!)
	"os"            // os — file operations (opening files to upload)
	"path/filepath" // filepath — cross-platform file path manipulation
	"regexp"        // regexp — regular expressions (for parsing Content-Disposition)
	"strconv"       // strconv — string conversions (parsing port numbers from responses)
	"strings"       // strings — string utilities (trimming, checking prefixes)
	"time"          // time — durations and timeouts
)

// =========================================================================
// Client struct — holds the configuration for all API requests
// =========================================================================

// Client is the main struct that all commands use to talk to the server.
// It wraps Go's built-in http.Client with our specific configuration.
type Client struct {
	// BaseURL is the server address, e.g., "https://transfera-api.onrender.com"
	// This comes from the --api flag in root.go (or TRANSFERA_API_URL env)
	BaseURL string

	// Verbose controls debug output. When true, we log HTTP details.
	Verbose bool

	// httpClient is Go's built-in HTTP client. We configure it with
	// generous timeouts for large file transfers. This is NOT exported
	// (lowercase first letter = private in Go) — only this package uses it.
	httpClient *http.Client
}

// =========================================================================
// NewClient — constructor function
// =========================================================================

// NewClient creates a new API client configured for the given server.
//
// WHY a constructor function instead of just Client{...}?
//   Because we need to set up the http.Client with specific timeouts.
//   A constructor ensures every Client is properly configured.
//
// Parameters:
//   baseURL — the server address (from --api flag)
//   verbose — whether to print debug info (from --verbose flag)
func NewClient(baseURL string, verbose bool) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"), // Remove trailing slash to avoid double-slashes
		Verbose: verbose,
		httpClient: &http.Client{
			// Timeout: maximum time for the ENTIRE request (connect + send + receive).
			// 10 minutes because uploading 100MB+ over slow connections can take a while.
			// The C++ server has 300s (5 min) read/write timeouts, so we set ours higher
			// to ensure the client doesn't give up before the server does.
			Timeout: 10 * time.Minute,
		},
	}
}

// =========================================================================
// Health — GET /api/health
// =========================================================================

// Health checks if the API server is reachable and responding.
//
// Returns:
//   bool  — true if server responded with 200 OK
//   error — non-nil if we couldn't connect or got an unexpected response
//
// This mirrors the web client's health check in page.tsx:
//   await axios.get(apiUrl('/api/health'), { timeout: 3000 });
func (c *Client) Health() (bool, error) {
	// Build the full URL: "http://127.0.0.1:8080" + "/api/health"
	url := c.BaseURL + "/api/health"

	if c.Verbose {
		fmt.Printf("  → GET %s\n", url)
	}

	// Create a custom client with 30s timeout for health checks.
	// On free cloud tiers (Render), an idle server may take 15-30s to cold-start.
	// 30 seconds allows the server to wake up without a false timeout error.
	healthClient := &http.Client{Timeout: 30 * time.Second}

	// http.Get sends an HTTP GET request. It returns:
	//   resp — the response (status code, headers, body)
	//   err  — non-nil if the request failed (network error, timeout, etc.)
	resp, err := healthClient.Get(url)
	if err != nil {
		// The server is unreachable. This could be:
		//   - Server cold-starting on Render
		//   - Server not running
		//   - Wrong URL/port
		//   - Network or firewall issue
		return false, fmt.Errorf("cannot reach API at %s: %w", c.BaseURL, err)
	}

	// IMPORTANT: Always close the response body when done!
	// `defer` schedules this to run when the function returns.
	// If we don't close it, we leak TCP connections (Go keeps them open
	// for connection reuse, but they'll pile up).
	defer resp.Body.Close()

	if c.Verbose {
		fmt.Printf("  ← %d %s\n", resp.StatusCode, resp.Status)
	}

	// 200 = OK, the server is healthy
	return resp.StatusCode == http.StatusOK, nil
}

// =========================================================================
// UploadResponse — the JSON the server sends back after a successful upload
// =========================================================================

// UploadResponse maps to the server's JSON response:
//   {"port": 52341, "maxDownloads": 3}
//
// The `json:"port"` tags tell Go's JSON decoder which JSON field maps to
// which struct field. Without these tags, Go would look for "Port" (capital P)
// in the JSON, which wouldn't match.
type UploadResponse struct {
	Port         int `json:"port"`
	MaxDownloads int `json:"maxDownloads"`
}

// =========================================================================
// Upload — POST /api/upload (multipart/form-data with streaming)
// =========================================================================

// Upload streams a file to the server and returns the invite code (port).
//
// This is the MOST IMPORTANT function for 100MB+ files. Here's the key insight:
//
// NAIVE APPROACH (bad for large files):
//   1. Read entire file into memory (100MB = 100MB of RAM)
//   2. Build multipart body in memory (another 100MB)
//   3. Send it all at once
//   Total RAM: ~200MB for a 100MB file!
//
// OUR APPROACH (streaming with io.Pipe):
//   1. Create an io.Pipe — a connected pair of (reader, writer)
//   2. In a SEPARATE goroutine: write the file in 64KB chunks into the pipe
//   3. The HTTP request reads FROM the pipe as its body
//   4. Go's HTTP client sends each chunk as it arrives — never buffering the whole file
//   Total RAM: ~128KB regardless of file size!
//
// Parameters:
//   filePath     — path to the file on disk
//   maxDownloads — how many times the invite can be used (1-100)
//   onProgress   — callback called with bytes sent so far (for progress bar)
//
// Returns:
//   *UploadResponse — contains the invite port and confirmed maxDownloads
//   error           — non-nil if upload failed
func (c *Client) Upload(filePath string, maxDownloads int, onProgress func(int64)) (*UploadResponse, error) {
	url := c.BaseURL + "/api/upload"

	// --- Step 1: Open the file ---
	// os.Open opens the file for reading. It returns a *os.File which
	// implements io.Reader — we can read from it in chunks.
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot open file: %w", err)
	}
	defer file.Close() // Always close when done

	// Get file info (name, size) for the multipart header
	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("cannot stat file: %w", err)
	}

	// filepath.Base extracts just the filename: "/home/user/docs/report.pdf" → "report.pdf"
	// This is what the server receives as the upload filename.
	filename := filepath.Base(filePath)

	if c.Verbose {
		fmt.Printf("  → POST %s\n", url)
		fmt.Printf("    file: %s (%d bytes)\n", filename, stat.Size())
		fmt.Printf("    maxDownloads: %d\n", maxDownloads)
	}

	// --- Step 2: Create an io.Pipe for zero-copy streaming ---
	//
	// io.Pipe() creates a synchronous, in-memory pipe:
	//   pipeReader — reads data out of the pipe (used as HTTP request body)
	//   pipeWriter — writes data into the pipe (we write the file here)
	//
	// The magic: writing to pipeWriter BLOCKS until someone reads from pipeReader.
	// This means data flows directly from file → pipe → HTTP socket without
	// ever being fully buffered in memory.
	pipeReader, pipeWriter := io.Pipe()

	// multipart.NewWriter creates a writer that formats data as
	// multipart/form-data (the encoding browsers use for file uploads).
	// It writes the boundary markers, content headers, etc.
	multipartWriter := multipart.NewWriter(pipeWriter)

	// --- Step 3: Write the file into the pipe in a goroutine ---
	//
	// A goroutine is Go's lightweight thread. The `go` keyword starts a
	// function running concurrently. We need this because:
	//   - The multipart writer WRITES to the pipe
	//   - The HTTP request READS from the pipe
	//   - These must happen simultaneously (writer produces, reader consumes)
	//   - If we did both on the same goroutine, it would deadlock!
	//
	// errChan lets us check if the writing goroutine had any errors.
	errChan := make(chan error, 1)

	go func() {
		// defer pipeWriter.Close() ensures the pipe is closed when this
		// goroutine finishes. This signals to the reader (HTTP client) that
		// there's no more data coming — the upload is complete.
		defer pipeWriter.Close()

		// --- Write the "maxDownloads" form field ---
		// This is like adding: <input name="maxDownloads" value="3"> in a form.
		// The server reads this in parseMaxDownloadsField() in fileController.cpp.
		err := multipartWriter.WriteField("maxDownloads", strconv.Itoa(maxDownloads))
		if err != nil {
			errChan <- fmt.Errorf("failed to write maxDownloads field: %w", err)
			return
		}

		// --- Create the file part in the multipart body ---
		// CreateFormFile adds a file field named "file" with the given filename.
		// It writes the multipart headers:
		//   Content-Disposition: form-data; name="file"; filename="report.pdf"
		//   Content-Type: application/octet-stream
		//
		// The returned `part` is an io.Writer — we write the file content to it.
		part, err := multipartWriter.CreateFormFile("file", filename)
		if err != nil {
			errChan <- fmt.Errorf("failed to create form file: %w", err)
			return
		}

		// --- Stream the file content in 64KB chunks ---
		// buf is our streaming buffer. Only 64KB of the file is in memory at any time.
		// This is the same buffer size the C++ server uses (char buffer[65536]).
		buf := make([]byte, 64*1024) // 64 KB buffer

		var totalSent int64
		for {
			// file.Read reads up to len(buf) bytes from the file into buf.
			// n = number of bytes actually read (may be less than 64KB at end of file)
			// err = io.EOF when we've read the entire file
			n, readErr := file.Read(buf)

			if n > 0 {
				// Write the chunk to the multipart body (which flows through the pipe
				// to the HTTP request body, and out to the network).
				_, writeErr := part.Write(buf[:n])
				if writeErr != nil {
					errChan <- fmt.Errorf("failed to write file chunk: %w", writeErr)
					return
				}

				totalSent += int64(n)

				// Call the progress callback so the progress bar updates.
				// This is how the upload command knows how many bytes have been sent.
				if onProgress != nil {
					onProgress(totalSent)
				}
			}

			if readErr == io.EOF {
				// End of file — we've sent everything!
				break
			}
			if readErr != nil {
				errChan <- fmt.Errorf("failed to read file: %w", readErr)
				return
			}
		}

		// Close the multipart writer to write the final boundary marker.
		// Without this, the server would wait forever for more data.
		if err := multipartWriter.Close(); err != nil {
			errChan <- fmt.Errorf("failed to close multipart writer: %w", err)
			return
		}

		// nil error = success
		errChan <- nil
	}()

	// --- Step 4: Create the HTTP request with the pipe as body ---
	//
	// http.NewRequest creates a request but doesn't send it yet.
	// The pipeReader is set as the request body — Go's HTTP client will
	// read from it and send chunks as they become available.
	req, err := http.NewRequest("POST", url, pipeReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set Content-Type with the multipart boundary.
	// The boundary is a random string that separates parts in multipart data.
	// The server uses it to find where the file starts and ends.
	// Example: "Content-Type: multipart/form-data; boundary=abc123xyz"
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())

	// --- Step 5: Send the request ---
	// httpClient.Do() sends the request. Because the body is a pipe,
	// Go reads chunks from pipeReader → sends them over TCP.
	// Meanwhile, our goroutine is writing chunks from the file → pipeWriter.
	// Data flows: file → pipeWriter → pipeReader → TCP → server
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upload request failed: %w", err)
	}
	defer resp.Body.Close()

	// --- Step 6: Check if the writing goroutine succeeded ---
	writeErr := <-errChan // Wait for the goroutine to finish
	if writeErr != nil {
		return nil, writeErr
	}

	// --- Step 7: Read and parse the server response ---
	// Read the entire response body. For uploads, the response is small JSON:
	//   {"port": 52341, "maxDownloads": 3}
	// So reading it all into memory is fine (it's tiny).
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if c.Verbose {
		fmt.Printf("  ← %d %s\n", resp.StatusCode, resp.Status)
		fmt.Printf("    body: %s\n", string(body))
	}

	// Check for HTTP errors
	if resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusServiceUnavailable {
		return nil, fmt.Errorf("server is currently waking up (HTTP %d). Please wait ~15 seconds and try again", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse the JSON response into our UploadResponse struct.
	// json.Unmarshal converts JSON bytes → Go struct.
	var result UploadResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("invalid JSON response: %w", err)
	}

	// Validate the response
	if result.Port <= 0 || result.Port > 65535 {
		return nil, fmt.Errorf("invalid port in response: %d", result.Port)
	}

	return &result, nil
}

// =========================================================================
// DownloadResult — what the download function returns
// =========================================================================

// DownloadResult contains information about a completed download.
type DownloadResult struct {
	Filename           string // The original filename (e.g., "report.pdf")
	FilePath           string // Full path where the file was saved
	FileSize           int64  // Size in bytes
	DownloadsRemaining int    // How many more downloads are allowed on this invite
}

// =========================================================================
// Download — GET /api/download/:port (streaming download)
// =========================================================================

// Download streams a file from the server to disk.
//
// Like Upload, this uses streaming to handle large files:
//   - The HTTP response body is an io.Reader (stream of bytes)
//   - We read from it in 64KB chunks and write directly to a file on disk
//   - At no point is the entire file in memory
//
// Parameters:
//   port       — the invite code (port number from the upload response)
//   outputDir  — directory to save the file in (empty = current directory)
//   outputName — explicit filename to save as (empty = use server's filename)
//   onProgress — callback with (filename, bytesReceived, totalBytes) for progress bar
func (c *Client) Download(port int, outputDir string, outputName string, onProgress func(string, int64, int64)) (*DownloadResult, error) {
	// Build the download URL: /api/download/52341
	url := fmt.Sprintf("%s/api/download/%d", c.BaseURL, port)

	if c.Verbose {
		fmt.Printf("  → GET %s\n", url)
	}

	// --- Step 1: Send the GET request ---
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if c.Verbose {
		fmt.Printf("  ← %d %s\n", resp.StatusCode, resp.Status)
		for key, vals := range resp.Header {
			for _, val := range vals {
				fmt.Printf("    %s: %s\n", key, val)
			}
		}
	}

	// --- Step 2: Handle error responses ---
	// Accept both 200 (full file) and 206 (partial/range) responses.
	// This matches the web client's validateStatus in page.tsx.
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("invite code %d was already used or is no longer valid", port)
	}
	if resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusServiceUnavailable {
		return nil, fmt.Errorf("server is currently waking up (HTTP %d). Please wait ~15 seconds and try again", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	// --- Step 3: Extract the filename from response headers ---
	// The C++ server sends the filename in TWO headers (for compatibility):
	//   Content-Disposition: attachment; filename="report.pdf"
	//   X-Filename: report.pdf
	// We try Content-Disposition first (standard), then X-Filename (custom fallback).
	filename := resolveDownloadFilename(resp.Header)

	// --- Step 4: Get the total file size for the progress bar ---
	// Content-Length tells us how many bytes to expect.
	// It might be -1 if the server doesn't send it (but ours always does).
	totalSize := resp.ContentLength

	// --- Step 5: Read X-Downloads-Remaining header ---
	// This tells the user how many more downloads the invite allows.
	downloadsRemaining := -1 // -1 = unknown
	if drHeader := resp.Header.Get("X-Downloads-Remaining"); drHeader != "" {
		if n, err := strconv.Atoi(drHeader); err == nil {
			downloadsRemaining = n
		}
	}

	// --- Step 6: Determine the output file path ---
	if outputName != "" {
		filename = outputName
	}

	var savePath string
	if outputDir != "" {
		// Ensure output directory exists (create it if needed)
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return nil, fmt.Errorf("cannot create output directory %s: %w", outputDir, err)
		}
		savePath = filepath.Join(outputDir, filename)
	} else {
		savePath = filename
	}

	// Notify progress callback that download is starting with filename & size
	if onProgress != nil {
		onProgress(filename, 0, totalSize)
	}

	// --- Step 7: Create the output file ---
	// os.Create creates a new file (or truncates existing one) for writing.
	// It returns a *os.File that implements io.Writer.
	outFile, err := os.Create(savePath)
	if err != nil {
		return nil, fmt.Errorf("cannot create output file %s: %w", savePath, err)
	}
	defer outFile.Close()

	// --- Step 8: Stream the response body to the file ---
	// We read in 64KB chunks, matching the server's streaming buffer size.
	buf := make([]byte, 64*1024) // 64 KB buffer
	var totalReceived int64

	for {
		// Read a chunk from the HTTP response
		n, readErr := resp.Body.Read(buf)

		if n > 0 {
			// Write the chunk to the file on disk
			_, writeErr := outFile.Write(buf[:n])
			if writeErr != nil {
				return nil, fmt.Errorf("failed to write to file: %w", writeErr)
			}

			totalReceived += int64(n)

			// Update the progress bar
			if onProgress != nil {
				onProgress(filename, totalReceived, totalSize)
			}
		}

		if readErr == io.EOF {
			// All data received — download complete!
			break
		}
		if readErr != nil {
			return nil, fmt.Errorf("failed to read response: %w", readErr)
		}
	}

	return &DownloadResult{
		Filename:           filename,
		FilePath:           savePath,
		FileSize:           totalReceived,
		DownloadsRemaining: downloadsRemaining,
	}, nil
}

// =========================================================================
// Filename Resolution — mirrors client/src/lib/downloadFilename.ts
// =========================================================================

// resolveDownloadFilename extracts the filename from HTTP response headers.
// This is a Go port of the web client's resolveDownloadFilename() function
// in downloadFilename.ts. The logic is:
//   1. Try Content-Disposition header (standard HTTP way to suggest a filename)
//   2. Try X-Filename header (custom Transfera header, set by the C++ server)
//   3. Fall back to "downloaded-file"
//   4. Strip the 32-char hex storage prefix the server adds
func resolveDownloadFilename(headers http.Header) string {
	// Try Content-Disposition first
	disposition := headers.Get("Content-Disposition")
	if filename := parseContentDispositionFilename(disposition); filename != "" {
		return stripStoragePrefix(filename)
	}

	// Try X-Filename (custom header from the C++ server)
	if xFilename := headers.Get("X-Filename"); xFilename != "" {
		return stripStoragePrefix(strings.TrimSpace(xFilename))
	}

	// Default fallback
	return "downloaded-file"
}

// parseContentDispositionFilename extracts the filename from a Content-Disposition header.
//
// The header looks like: `attachment; filename="report.pdf"`
// Or with RFC 5987 encoding: `attachment; filename*=UTF-8''report%20v2.pdf`
//
// This is a direct port of the TypeScript version in downloadFilename.ts.
func parseContentDispositionFilename(disposition string) string {
	if disposition == "" {
		return ""
	}

	// Try the standard mime.ParseMediaType parser first.
	// This handles: attachment; filename="report.pdf"
	_, params, err := mime.ParseMediaType(disposition)
	if err == nil {
		if fn, ok := params["filename"]; ok && fn != "" {
			return fn
		}
	}

	// Fallback: use regex like the TypeScript version.
	// This handles edge cases that mime.ParseMediaType might miss.

	// Try filename*= (RFC 5987 encoded filename — supports Unicode)
	starRe := regexp.MustCompile(`(?i)filename\*=(?:UTF-8''|utf-8'')([^;\s]+)`)
	if matches := starRe.FindStringSubmatch(disposition); len(matches) > 1 {
		return strings.ReplaceAll(matches[1], `"`, "")
	}

	// Try filename="quoted"
	quotedRe := regexp.MustCompile(`(?i)filename="([^"]+)"`)
	if matches := quotedRe.FindStringSubmatch(disposition); len(matches) > 1 {
		return matches[1]
	}

	// Try filename=unquoted
	unquotedRe := regexp.MustCompile(`(?i)filename=([^;\s]+)`)
	if matches := unquotedRe.FindStringSubmatch(disposition); len(matches) > 1 {
		return strings.ReplaceAll(matches[1], `"`, "")
	}

	return ""
}

// stripStoragePrefix removes the 32-char hex prefix that the server adds
// for uniqueness. Example: "a1b2c3d4e5f6...1234_report.pdf" → "report.pdf"
//
// This matches the TypeScript: /^[0-9a-f]{32}_(.+)$/i
// The C++ server creates this prefix in FileController::makeUniqueName().
func stripStoragePrefix(name string) string {
	re := regexp.MustCompile(`(?i)^[0-9a-f]{32}_(.+)$`)
	if matches := re.FindStringSubmatch(name); len(matches) > 1 {
		return matches[1]
	}
	return name
}

// =========================================================================
// Room API Structs & Methods
// =========================================================================

// RoomParticipant represents a user in a room
type RoomParticipant struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsCreator bool   `json:"isCreator"`
	JoinedAt  int64  `json:"joinedAt"`
}

// RoomMessage represents a message or file share event in a room
type RoomMessage struct {
	ID         string `json:"id"`
	Type       string `json:"type"` // "system", "chat", "file"
	SenderID   string `json:"senderId"`
	SenderName string `json:"senderName"`
	Text       string `json:"text"`
	FileID     string `json:"fileId,omitempty"`
	FileName   string `json:"fileName,omitempty"`
	FileSize   int64  `json:"fileSize,omitempty"`
	Timestamp  int64  `json:"timestamp"`
}

// RoomFile represents a file uploaded to a room
type RoomFile struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	UploaderID   string `json:"uploaderId"`
	UploaderName string `json:"uploaderName"`
	UploadedAt   int64  `json:"uploadedAt"`
}

// RoomSyncResponse contains full state of a room
type RoomSyncResponse struct {
	RoomID          int               `json:"roomId"`
	MaxParticipants int               `json:"maxParticipants"`
	Participants    []RoomParticipant `json:"participants"`
	Messages        []RoomMessage     `json:"messages"`
	Files           []RoomFile        `json:"files"`
}

// CreateRoomResponse is returned by the server on room creation
type CreateRoomResponse struct {
	Status          string `json:"status"`
	RoomID          int    `json:"roomId"`
	Port            int    `json:"port"`
	UserID          string `json:"userId"`
	MaxParticipants int    `json:"maxParticipants"`
	Message         string `json:"message,omitempty"`
}

// JoinRoomResponse is returned by the server when joining a room
type JoinRoomResponse struct {
	Status          string `json:"status"`
	RoomID          int    `json:"roomId"`
	Port            int    `json:"port"`
	UserID          string `json:"userId"`
	MaxParticipants int    `json:"maxParticipants"`
	Message         string `json:"message,omitempty"`
}

// CreateRoom sends POST /api/rooms to create a new collaboration room.
func (c *Client) CreateRoom(creatorName string, maxParticipants int, requestedPort int) (*CreateRoomResponse, error) {
	url := c.BaseURL + "/api/rooms"
	payload := map[string]interface{}{
		"creatorName":     creatorName,
		"maxParticipants": maxParticipants,
	}
	if requestedPort > 0 {
		payload["port"] = requestedPort
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var roomResp CreateRoomResponse
	if err := json.Unmarshal(respBody, &roomResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %s", string(respBody))
	}

	if resp.StatusCode != http.StatusOK {
		if roomResp.Message != "" {
			return nil, fmt.Errorf("%s", roomResp.Message)
		}
		return nil, fmt.Errorf("server error (%d): %s", resp.StatusCode, string(respBody))
	}

	return &roomResp, nil
}

// JoinRoom sends POST /api/rooms/:port/join to join an existing room.
func (c *Client) JoinRoom(port int, userName string) (*JoinRoomResponse, error) {
	url := fmt.Sprintf("%s/api/rooms/%d/join", c.BaseURL, port)
	payload := map[string]interface{}{
		"userName": userName,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var joinResp JoinRoomResponse
	if err := json.Unmarshal(respBody, &joinResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %s", string(respBody))
	}

	if resp.StatusCode != http.StatusOK {
		if joinResp.Message != "" {
			return nil, fmt.Errorf("%s", joinResp.Message)
		}
		return nil, fmt.Errorf("server error (%d): %s", resp.StatusCode, string(respBody))
	}

	return &joinResp, nil
}

// LeaveRoom sends POST /api/rooms/:port/leave to leave a room.
func (c *Client) LeaveRoom(port int, userId string) error {
	url := fmt.Sprintf("%s/api/rooms/%d/leave", c.BaseURL, port)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-User-Id", userId)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// SyncRoom retrieves messages, members, and files for a room.
func (c *Client) SyncRoom(port int, userId string, since int64) (*RoomSyncResponse, error) {
	url := fmt.Sprintf("%s/api/rooms/%d/sync?since=%d", c.BaseURL, port, since)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-User-Id", userId)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to sync room (%d): %s", resp.StatusCode, string(b))
	}

	var syncResp RoomSyncResponse
	if err := json.NewDecoder(resp.Body).Decode(&syncResp); err != nil {
		return nil, fmt.Errorf("failed to decode sync response: %w", err)
	}

	return &syncResp, nil
}

// SendRoomMessage sends a text message to a room.
func (c *Client) SendRoomMessage(port int, userId string, text string) error {
	url := fmt.Sprintf("%s/api/rooms/%d/messages", c.BaseURL, port)
	payload := map[string]string{"text": text}
	b, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", url, strings.NewReader(string(b)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Id", userId)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respB, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to send message: %s", string(respB))
	}
	return nil
}

// UploadRoomFiles uploads one or more files to a room with an optional note.
func (c *Client) UploadRoomFiles(port int, userId string, filePaths []string, note string) error {
	url := fmt.Sprintf("%s/api/rooms/%d/upload", c.BaseURL, port)

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		defer writer.Close()

		_ = writer.WriteField("userId", userId)
		if note != "" {
			_ = writer.WriteField("note", note)
		}

		for _, filePath := range filePaths {
			file, err := os.Open(filePath)
			if err != nil {
				pw.CloseWithError(fmt.Errorf("failed to open %s: %w", filePath, err))
				return
			}

			part, err := writer.CreateFormFile("files", filepath.Base(filePath))
			if err != nil {
				file.Close()
				pw.CloseWithError(err)
				return
			}

			_, err = io.Copy(part, file)
			file.Close()
			if err != nil {
				pw.CloseWithError(err)
				return
			}
		}
	}()

	req, err := http.NewRequest("POST", url, pr)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-User-Id", userId)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respB, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to upload files (%d): %s", resp.StatusCode, string(respB))
	}
	return nil
}

// DownloadRoomFile downloads a file from a room.
func (c *Client) DownloadRoomFile(port int, fileId string, destDir string) (string, error) {
	url := fmt.Sprintf("%s/api/rooms/%d/files/%s", c.BaseURL, port, fileId)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return "", fmt.Errorf("file download failed with status %d", resp.StatusCode)
	}

	filename := resolveDownloadFilename(resp.Header)
	outPath := filepath.Join(destDir, filename)

	outFile, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("failed to create destination file: %w", err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, resp.Body); err != nil {
		return "", fmt.Errorf("failed to stream download: %w", err)
	}

	return outPath, nil
}

// RemoveRoomParticipant removes a participant from a room.
func (c *Client) RemoveRoomParticipant(port int, byUserId string, targetUserId string) error {
	url := fmt.Sprintf("%s/api/rooms/%d/remove", c.BaseURL, port)
	payload := map[string]string{
		"byUserId":     byUserId,
		"targetUserId": targetUserId,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if byUserId != "" {
		req.Header.Set("X-User-Id", byUserId)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respB, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to remove participant (%d): %s", resp.StatusCode, string(respB))
	}
	return nil
}


