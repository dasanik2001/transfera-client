// cmd/root.go — Root Command & Global Configuration
//
// This file sets up the "parent" command that all subcommands (upload, download,
// health) attach to. Think of it as the "main menu" of the CLI.
//
// When a user types just `transfera`, this root command runs and shows help.
// When they type `transfera upload ...`, cobra routes to the upload subcommand.
//
// GLOBAL FLAGS defined here are available to ALL subcommands:
//   --api <url>     → which server to talk to
//   --verbose       → show extra debug output

package cmd

import (
	"fmt" // fmt = "format" — Go's string formatting & printing (like printf in C)
	"os"  // os = operating system — gives us os.Exit() to quit with an error code

	"github.com/spf13/cobra" // cobra = the CLI framework

	"github.com/dasanik2001/transfera-client/cli/internal/tui" // Interactive terminal UI
)

// ---------------------------------------------------------------------------
// Global variables — accessible by all subcommands (upload.go, download.go, etc.)
// ---------------------------------------------------------------------------

const (
	// Version is the current version of Transfera CLI.
	Version = "1.0.0"

	// DefaultAPIURL is the official production Transfera API server hosted on Render.
	DefaultAPIURL = "https://transfera-api.onrender.com"
)

var (
	// apiBaseURL holds the server address. Every HTTP request the CLI makes
	// will be sent to this URL. Default is https://transfera-api.onrender.com.
	// Users can override with:
	//   1. --api flag
	//   2. TRANSFERA_API_URL environment variable
	//   3. NEXT_PUBLIC_API_BASE_URL environment variable
	apiBaseURL string

	// verbose controls whether we print extra debug information.
	// When false (default), the CLI only shows essential output.
	// When true (--verbose flag), it prints HTTP request details, timing, etc.
	verbose bool
)

// ---------------------------------------------------------------------------
// Root Command Definition
// ---------------------------------------------------------------------------

// rootCmd is the base command when called without any subcommands.
// cobra.Command is a struct with many fields; we only set the ones we need:
//
//	Use:   — the one-word name shown in usage text
//	Short: — brief description shown in the parent's help
//	Long:  — detailed description shown when user types `transfera --help`
var rootCmd = &cobra.Command{
	Use:     "transfera",
	Version: Version,
	Short:   "Transfera CLI — Secure P2P File Sharing",

	// Long is a multi-line description shown when the user runs `transfera`
	// with no subcommand. The backtick ` allows multi-line strings in Go.
	Long: `
   ___________                          _____                    
  |           |                        / ____|                   
  |--   ------'_ __ __ _ _ __  ___  | |__ ___ _ __ __ _        
     |  |    | '__/ _' | '_ \/ __|  |  __/ _ \ '__/ _' |       
     |  |    | | | (_| | | | \__ \  | | |  __/ | | (_| |       
     |__|    |_|  \__,_|_| |_|___/  |_|  \___|_|  \__,_|       

  Transfera CLI — Secure P2P File Sharing from the terminal.

  Upload files (photos, documents, videos), share invite codes, and download.
  Fast, cross-platform transfer between laptops, desktops, and servers.
  Optimized for large file transfers (100MB+) with minimal memory usage.

  Usage:
    transfera upload <file>           Upload a file and get an invite code
    transfera download <invite-code>  Download a file using an invite code
    transfera room create             Create a collaborative multi-user room
    transfera room join <room-id>     Join an existing room
    transfera health                  Check if the API server is reachable

  Official server: https://transfera-api.onrender.com (override with --api or TRANSFERA_API_URL)`,

	// SilenceUsage: when a command returns an error, cobra normally prints
	// the usage text again. We silence this because our error messages are
	// already descriptive enough — showing usage again would be noise.
	SilenceUsage: true,

	// SilenceErrors: we handle error printing ourselves in Execute(),
	// so we tell cobra not to print errors automatically.
	SilenceErrors: true,

	// RunE: When the user types just `transfera` with no subcommand, launch
	// the interactive terminal UI instead of just printing help text.
	// This makes double-clicking transfera.exe useful and provides a
	// guided experience for new users.
	RunE: func(cmd *cobra.Command, args []string) error {
		tui.RunMenu(&apiBaseURL, &verbose)
		return nil
	},
}

// ---------------------------------------------------------------------------
// Execute — called by main.go
// ---------------------------------------------------------------------------

// Execute runs the root command. This is the entry point for the entire CLI.
//
// How cobra works internally:
//  1. It reads os.Args (the command-line arguments)
//  2. It finds which subcommand matches (upload, download, health)
//  3. It parses flags (--api, --verbose, --max-downloads, etc.)
//  4. It calls that subcommand's RunE function
//  5. If RunE returns an error, we print it and exit with code 1
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		// Print the error in red-ish formatting to stderr.
		// os.Stderr is the error output stream (separate from normal output).
		// In terminals, stderr is usually shown even when stdout is redirected.
		fmt.Fprintf(os.Stderr, "\n  ✗ Error: %s\n\n", err)

		// os.Exit(1) terminates the program with exit code 1.
		// Exit code 0 = success, anything else = failure.
		// This lets shell scripts check: `transfera upload ... && echo "success"`
		os.Exit(1)
	}
}

// ---------------------------------------------------------------------------
// init() — runs automatically when this package is loaded
// ---------------------------------------------------------------------------

// init() is a special Go function that runs BEFORE main(). Every package
// can have an init() function. Go guarantees all init() functions run before
// main() starts. We use it to register global flags.
func init() {
	defaultAPI := DefaultAPIURL
	if envAPI := os.Getenv("TRANSFERA_API_URL"); envAPI != "" {
		defaultAPI = envAPI
	} else if envAPI := os.Getenv("NEXT_PUBLIC_API_BASE_URL"); envAPI != "" {
		defaultAPI = envAPI
	}

	// PersistentFlags are "global" — they work on the root command AND all
	// subcommands. This is different from Flags() which only works on the
	// specific command.
	//
	// StringVarP registers a string flag:
	//   &apiBaseURL  — pointer to the variable where the value is stored
	//   "api"        — the long flag name (--api)
	//   "a"          — the short flag name (-a)
	//   defaultAPI   — default official URL (or env var if set)
	//   "API server" — description shown in --help
	rootCmd.PersistentFlags().StringVarP(
		&apiBaseURL,
		"api", "a",
		defaultAPI,
		"API server URL (default: https://transfera-api.onrender.com or TRANSFERA_API_URL env)",
	)

	// BoolVarP registers a boolean flag:
	//   &verbose — pointer to the bool variable
	//   "verbose" / "V" — long/short names
	//   false — default value (off)
	rootCmd.PersistentFlags().BoolVarP(
		&verbose,
		"verbose", "V",
		false,
		"Enable verbose output (show HTTP details, timing)",
	)
}
