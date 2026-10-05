// cmd/upload.go — Upload Command
//
// This file defines the `transfera upload <file>` subcommand.
// It uploads a file to the Transfera server and returns an invite code.
//
// This is the CORE feature of the CLI. The workflow is:
//   1. Validate the file (exists, not too large, not empty)
//   2. Show a progress bar while streaming the file to the server
//   3. Display the invite code the user can share
//
// The actual upload is done by the API client (internal/api/client.go)
// using io.Pipe for zero-copy streaming. This command just handles
// user interaction: flags, validation, progress display, and output.
//
// Usage examples:
//   transfera upload report.pdf
//   transfera upload video.mp4 --max-downloads 5
//   transfera upload huge-file.zip --max-size 500 --max-downloads 10

package cmd

import (
	"fmt"           // fmt — formatted printing
	"path/filepath" // filepath — extracting filename from path

	"github.com/spf13/cobra" // cobra — CLI framework

	"github.com/dasanik2001/transfera-client/cli/internal/api"        // Our API client
	"github.com/dasanik2001/transfera-client/cli/internal/progress"   // Our progress bar
	"github.com/dasanik2001/transfera-client/cli/internal/validation" // Our file validator
)

// ---------------------------------------------------------------------------
// Command-specific flags (only available on `transfera upload`)
// ---------------------------------------------------------------------------

var (
	// maxDownloads controls how many times the invite code can be used.
	// Default is 1 (one-time download), matching the web UI's DEFAULT_MAX_DOWNLOADS.
	// Range: 1-100, matching the server's kMinMaxDownloads/kMaxMaxDownloads.
	maxDownloads int

	// maxSizeMB is the client-side file size limit in megabytes.
	// Default is 500 MB, matching the web UI's MAX_UPLOAD_MB.
	// Users can increase this if the server is configured for larger files
	// (via TRANSFERA_MAX_UPLOAD_MB environment variable on the server, up to 8192 MB).
	maxSizeMB int
)

// ---------------------------------------------------------------------------
// Upload Command Definition
// ---------------------------------------------------------------------------

var uploadCmd = &cobra.Command{
	Use:   "upload <file-path>",
	Short: "Upload a file and get an invite code",
	Long: `Upload a file to the Transfera server and receive an invite code.

Share the invite code with anyone to let them download the file (e.g. laptop to desktop).
The file is streamed to the server — only ~128KB of memory is used
regardless of file size.

Examples:
  transfera upload photo.jpg
  transfera upload video.mp4 --max-downloads 5
  transfera upload huge-file.zip --max-size 500
  transfera --api https://transfera-api.onrender.com upload photo.jpg`,

	// Args: cobra.ExactArgs(1) tells cobra that this command requires
	// EXACTLY one positional argument (the file path). If the user types
	// `transfera upload` without a file, cobra automatically shows an error:
	//   "accepts 1 arg(s), received 0"
	Args: cobra.ExactArgs(1),

	// RunE is the function that executes when the user runs `transfera upload <file>`.
	// We use RunE (not Run) because uploads can fail, and returning an error
	// lets cobra handle it with a proper exit code.
	RunE: func(cmd *cobra.Command, args []string) error {
		// args[0] is the file path — the first (and only) positional argument.
		filePath := validation.ResolvePath(args[0])

		// filepath.Base extracts the filename: "C:\Users\docs\report.pdf" → "report.pdf"
		filename := filepath.Base(filePath)

		// =====================================================================
		// Step 1: Validate the file
		// =====================================================================
		// Check that the file exists, is a regular file, and is within
		// the size limit. This is the Go equivalent of the web client's
		// validateUploadFile(file) call in page.tsx line 52.
		fileSize, err := validation.ValidateFile(filePath, maxSizeMB)
		if err != nil {
			// Return the error — cobra will print it and exit with code 1.
			// Example: "file must be 100 MB or smaller (selected: 256.00 MB)"
			return err
		}

		// =====================================================================
		// Step 2: Validate maxDownloads range
		// =====================================================================
		// This mirrors clampMaxDownloads() from client/src/lib/uploadLimits.ts
		// and the server's parseMaxDownloadsField() in fileController.cpp.
		if maxDownloads < 1 {
			maxDownloads = 1
		}
		if maxDownloads > 100 {
			maxDownloads = 100
		}

		// =====================================================================
		// Step 3: Show the file info
		// =====================================================================
		fmt.Printf("\n  File: %s (%s)\n", filename, validation.FormatFileSize(fileSize))
		fmt.Printf("  Max downloads: %d\n\n", maxDownloads)

		// =====================================================================
		// Step 4: Create progress bar and start upload
		// =====================================================================

		// Create a progress bar with the total file size.
		// As the upload streams data, we'll call bar.Set64(bytesSent)
		// to update the bar.
		bar := progress.NewUploadBar(fileSize, filename)

		// Create the API client using global flags from root.go
		client := api.NewClient(apiBaseURL, verbose)

		// Upload the file! This is where the magic happens.
		//
		// The last argument is a callback function (closure). The API client
		// calls it repeatedly with the total bytes sent so far. We use it
		// to update the progress bar.
		//
		// Closure = a function that "captures" variables from its outer scope.
		// Here, `bar` is captured from the outer function scope. When the API
		// client calls this function, it can access `bar` even though `bar`
		// was defined outside the function.
		result, err := client.Upload(filePath, maxDownloads, func(bytesSent int64) {
			// Set64 updates the progress bar to show bytesSent / totalBytes.
			// The library redraws the bar on the same terminal line.
			bar.Set64(bytesSent)
		})

		if err != nil {
			// Upload failed — show error details.
			fmt.Printf("\n")
			return fmt.Errorf("upload failed: %w", err)
		}

		// Ensure the progress bar shows 100% complete
		bar.Finish()

		// =====================================================================
		// Step 5: Display the invite code
		// =====================================================================
		// This is the equivalent of the InviteCode component in the web UI
		// (client/src/components/InviteCode.tsx).
		fmt.Printf("\n  ✓ File ready to share!\n")
		fmt.Printf("  ┌────────────────────────────────────────\n")
		fmt.Printf("  │  Invite code: %d\n", result.Port)
		fmt.Printf("  │  Max downloads: %d\n", result.MaxDownloads)
		fmt.Printf("  └────────────────────────────────────────\n")
		fmt.Printf("\n  Share this code to download on your desktop, laptop, or server:\n")
		fmt.Printf("    transfera download %d\n\n", result.Port)

		return nil // Success!
	},
}

// init() registers the upload command and its flags.
func init() {
	// Add upload as a subcommand of the root command
	rootCmd.AddCommand(uploadCmd)

	// --- Register upload-specific flags ---
	// These use Flags() (not PersistentFlags) because they only apply
	// to the upload command, not to download or health.

	// IntVarP registers an integer flag:
	//   &maxDownloads — pointer to the variable
	//   "max-downloads" — long name (--max-downloads)
	//   "n" — short name (-n)
	//   1 — default value
	//   "..." — description
	uploadCmd.Flags().IntVarP(
		&maxDownloads,
		"max-downloads", "n",
		1,
		"Maximum number of downloads allowed (1-100)",
	)

	uploadCmd.Flags().IntVarP(
		&maxSizeMB,
		"max-size", "s",
		validation.DefaultMaxUploadMB,
		"Maximum file size in MB (increase if server allows larger uploads)",
	)
}
